// Package emoji is a Starlark module for converting between text and emoji.
//
// Two kinds of conversion live here:
//
//   - Shortcode <-> emoji, driven by a generated data table (tables_gen.go). The
//     table is built offline by internal/gen, which merges pinned datasets from
//     several language ecosystems (carpedm20/emoji — Python; github/gemoji —
//     Ruby) into one Go map. Refreshing the data is a regenerate-and-review step,
//     never a runtime dependency: at run time the module reads only the embedded
//     Go maps, so it has zero third-party data dependencies.
//
//   - "Look-alike" emoji for plain numbers, clock times, letters, and symbols
//     (convert.go). These are pure Unicode arithmetic and need no data at all.
//
// All functions are pure and deterministic. Text-accepting functions bound their
// input with the max_input_bytes host config.
package emoji

//go:generate go run ./internal/gen

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/1set/starlet"
	"github.com/1set/starlet/dataconv/types"
	"github.com/starpkg/base"
	"go.starlark.net/starlark"
)

// ModuleName is the name used in Starlark's load() for this module.
const ModuleName = "emoji"

const configKeyMaxInputBytes = "max_input_bytes"

const defaultMaxInputBytes = 5 << 20 // 5 MiB

var none = starlark.None

// Module wraps a ConfigurableModule with the emoji conversion functions.
type Module struct {
	cfgMod *base.ConfigurableModule
	ext    *base.ConfigurableModuleExt
}

// NewModule creates a new Module with default configuration.
func NewModule() *Module {
	cm, _ := base.NewConfigurableModuleWithConfigOptions(
		genConfigOption(configKeyMaxInputBytes, "Maximum input size in bytes for text conversions", defaultMaxInputBytes),
	)
	return &Module{cfgMod: cm, ext: cm.Extend()}
}

func genConfigOption[T any](name, description string, defaultValue T) *base.ConfigOption[T] {
	return base.NewConfigOption(defaultValue).
		WithName(name).
		WithDescription(description).
		WithEnvVar("EMOJI_" + upper(name))
}

// upper uppercases an ASCII config-key name for the environment-variable prefix.
func upper(s string) string {
	out := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		out[i] = c
	}
	return string(out)
}

// LoadModule returns the Starlark module loader.
func (m *Module) LoadModule() starlet.ModuleLoader {
	funcs := starlark.StringDict{
		"emojize":         starlark.NewBuiltin(ModuleName+".emojize", m.emojize),
		"demojize":        starlark.NewBuiltin(ModuleName+".demojize", m.demojize),
		"get":             starlark.NewBuiltin(ModuleName+".get", m.get),
		"name":            starlark.NewBuiltin(ModuleName+".name", m.name),
		"describe":        starlark.NewBuiltin(ModuleName+".describe", m.describe),
		"number_to_emoji": starlark.NewBuiltin(ModuleName+".number_to_emoji", m.numberToEmoji),
		"emoji_to_number": starlark.NewBuiltin(ModuleName+".emoji_to_number", m.emojiToNumber),
		"time_to_emoji":   starlark.NewBuiltin(ModuleName+".time_to_emoji", m.timeToEmoji),
		"letter_to_emoji": starlark.NewBuiltin(ModuleName+".letter_to_emoji", m.letterToEmoji),
		"symbol_to_emoji": starlark.NewBuiltin(ModuleName+".symbol_to_emoji", m.symbolToEmoji),
		"convert":         starlark.NewBuiltin(ModuleName+".convert", m.convert),
		"info":            starlark.NewBuiltin(ModuleName+".info", m.info),
	}
	return m.cfgMod.LoadModule(ModuleName, funcs)
}

// checkInputSize rejects input longer than the configured max_input_bytes.
func (m *Module) checkInputSize(text string) error {
	if maxBytes := m.ext.GetInt(configKeyMaxInputBytes); maxBytes > 0 && len(text) > maxBytes {
		return fmt.Errorf("%s: input exceeds max_input_bytes (%d)", ModuleName, maxBytes)
	}
	return nil
}

// checkValueSize bounds a value's input size when it carries text (string or
// bytes); non-text values (ints, floats) are never large and pass through. This
// keeps every text-accepting entry point honoring max_input_bytes.
func (m *Module) checkValueSize(v starlark.Value) error {
	switch x := v.(type) {
	case starlark.String:
		return m.checkInputSize(string(x))
	case starlark.Bytes:
		return m.checkInputSize(string(x))
	default:
		return nil
	}
}

// --- shortcode <-> emoji -----------------------------------------------------

// shortcodeRe matches a ":token:" where the token has no spaces or colons.
var shortcodeRe = regexp.MustCompile(`:([^\s:]+):`)

// flagShortRe matches the :flag-xx:/:flag_xx: alias form for an arbitrary
// two-letter region code that may not be in the table.
var flagShortRe = regexp.MustCompile(`^flag[-_]([a-zA-Z]{2})$`)

var (
	reverseOnce     sync.Once
	reverseMaxRunes int
)

func initReverse() {
	for glyph := range dataReverse {
		if n := len([]rune(glyph)); n > reverseMaxRunes {
			reverseMaxRunes = n
		}
	}
}

// regionalPair turns a two-letter region code into a regional-indicator pair
// (the Unicode flag sequence).
func regionalPair(cc string) string {
	cc = strings.ToLower(cc)
	a := rune(0x1F1E6) + rune(cc[0]) - 'a'
	b := rune(0x1F1E6) + rune(cc[1]) - 'a'
	return string(a) + string(b)
}

// emojizeText replaces every known :shortcode: in s with its emoji glyph.
// Unknown tokens are left untouched; :flag-xx: falls back to a regional pair.
func emojizeText(s string) string {
	return shortcodeRe.ReplaceAllStringFunc(s, func(tok string) string {
		inner := tok[1 : len(tok)-1]
		if glyph, ok := dataForward[inner]; ok {
			return glyph
		}
		if mm := flagShortRe.FindStringSubmatch(inner); mm != nil {
			return regionalPair(mm[1])
		}
		return tok
	})
}

// demojizeText replaces every emoji glyph in s with its :shortcode:, using a
// longest-match scan so multi-rune sequences (variation selectors, ZWJ) win.
func demojizeText(s, open, closing string) string {
	reverseOnce.Do(initReverse)
	runes := []rune(s)
	var b strings.Builder
	for i := 0; i < len(runes); {
		hi := i + reverseMaxRunes
		if hi > len(runes) {
			hi = len(runes)
		}
		matched := false
		for j := hi; j > i; j-- {
			if name, ok := dataReverse[string(runes[i:j])]; ok {
				b.WriteString(open)
				b.WriteString(name)
				b.WriteString(closing)
				i = j
				matched = true
				break
			}
		}
		if !matched {
			b.WriteRune(runes[i])
			i++
		}
	}
	return b.String()
}

func (m *Module) emojize(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var text types.StringOrBytes
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "text", &text); err != nil {
		return none, err
	}
	s := text.GoString()
	if err := m.checkInputSize(s); err != nil {
		return none, err
	}
	return starlark.String(emojizeText(s)), nil
}

func (m *Module) demojize(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		text       types.StringOrBytes
		delimiters starlark.Value = none
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "text", &text, "delimiters?", &delimiters); err != nil {
		return none, err
	}
	open, closing, err := parseDelimiters(delimiters)
	if err != nil {
		return none, fmt.Errorf("%s: %w", b.Name(), err)
	}
	s := text.GoString()
	if err := m.checkInputSize(s); err != nil {
		return none, err
	}
	return starlark.String(demojizeText(s, open, closing)), nil
}

// parseDelimiters reads an optional (open, close) pair of strings; the default
// is (":", ":").
func parseDelimiters(v starlark.Value) (string, string, error) {
	if v == nil || v == starlark.None {
		return ":", ":", nil
	}
	var items []starlark.Value
	switch x := v.(type) {
	case starlark.Tuple:
		items = []starlark.Value(x)
	case *starlark.List:
		items = make([]starlark.Value, 0, x.Len())
		for i := 0; i < x.Len(); i++ {
			items = append(items, x.Index(i))
		}
	default:
		return "", "", fmt.Errorf("delimiters must be a (open, close) tuple, got %s", v.Type())
	}
	if len(items) != 2 {
		return "", "", fmt.Errorf("delimiters must have exactly 2 elements, got %d", len(items))
	}
	open, ok1 := starlark.AsString(items[0])
	closing, ok2 := starlark.AsString(items[1])
	if !ok1 || !ok2 {
		return "", "", fmt.Errorf("delimiters must both be strings")
	}
	return open, closing, nil
}

func (m *Module) get(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var nameArg string
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "name", &nameArg); err != nil {
		return none, err
	}
	bare := strings.Trim(nameArg, ":")
	if glyph, ok := dataForward[bare]; ok {
		return starlark.String(glyph), nil
	}
	if mm := flagShortRe.FindStringSubmatch(bare); mm != nil {
		return starlark.String(regionalPair(mm[1])), nil
	}
	return none, nil
}

func (m *Module) name(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var glyph string
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "emoji", &glyph); err != nil {
		return none, err
	}
	if n, ok := dataReverse[glyph]; ok {
		return starlark.String(n), nil
	}
	return none, nil
}

func (m *Module) describe(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var glyph string
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "emoji", &glyph); err != nil {
		return none, err
	}
	if d, ok := dataNames[glyph]; ok {
		return starlark.String(d), nil
	}
	return none, nil
}

// --- convert dispatcher ------------------------------------------------------

var timeAutoRe = regexp.MustCompile(`^\d{1,2}:\d{2}$`)

func (m *Module) convert(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		value starlark.Value
		kind  = "auto"
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "value", &value, "kind?", &kind); err != nil {
		return none, err
	}
	if kind == "auto" {
		kind = autoKind(value)
	}
	// Bound any text input up front; this covers every kind that consumes a
	// string (number/time strings included), keeping the max_input_bytes
	// invariant uniform across the dispatcher.
	if err := m.checkValueSize(value); err != nil {
		return none, err
	}
	switch kind {
	case "number":
		text, err := numericText(value)
		if err != nil {
			return none, fmt.Errorf("%s: %w", b.Name(), err)
		}
		return starlark.String(numberToEmojiStr(text, false)), nil
	case "time":
		h, mm, err := parseTimeValue(value, none)
		if err != nil {
			return none, fmt.Errorf("%s: %w", b.Name(), err)
		}
		out, err := clockEmoji(h, mm)
		if err != nil {
			return none, fmt.Errorf("%s: %w", b.Name(), err)
		}
		return starlark.String(out), nil
	case "letter", "symbol", "emojize", "demojize":
		s, err := textOf(value)
		if err != nil {
			return none, fmt.Errorf("%s: %w", b.Name(), err)
		}
		switch kind {
		case "letter":
			out, err := letterToEmojiStr(s, "regional")
			if err != nil {
				return none, fmt.Errorf("%s: %w", b.Name(), err)
			}
			return starlark.String(out), nil
		case "symbol":
			return starlark.String(symbolToEmojiStr(s)), nil
		case "demojize":
			return starlark.String(demojizeText(s, ":", ":")), nil
		default: // emojize
			return starlark.String(emojizeText(s)), nil
		}
	default:
		return none, fmt.Errorf("%s: unknown kind %q (want auto, emojize, demojize, number, time, letter, symbol)", b.Name(), kind)
	}
}

// autoKind guesses the conversion family for kind="auto": ints/floats are
// numbers, "H:MM" strings are times, and everything else is emojize.
func autoKind(v starlark.Value) string {
	switch x := v.(type) {
	case starlark.Int, starlark.Float:
		return "number"
	case starlark.String:
		s := strings.TrimSpace(string(x))
		// Only auto-route to time when it is a valid in-range clock string;
		// otherwise a "99:99"-shaped value falls through to emojize (and is
		// left untouched) instead of hard-erroring.
		if timeAutoRe.MatchString(s) {
			if h, mm, err := parseTimeString(s, none); err == nil {
				if _, err := clockEmoji(h, mm); err == nil {
					return "time"
				}
			}
		}
		return "emojize"
	default:
		return "emojize"
	}
}

func textOf(v starlark.Value) (string, error) {
	switch x := v.(type) {
	case starlark.String:
		return string(x), nil
	case starlark.Bytes:
		return string(x), nil
	default:
		return "", fmt.Errorf("expected a string, got %s", v.Type())
	}
}

// --- info --------------------------------------------------------------------

// info reports where the shortcode dataset came from and how big it is — useful
// for verifying which Unicode/emoji generation the module was built against.
func (m *Module) info(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackArgs(b.Name(), args, kwargs); err != nil {
		return none, err
	}
	d := starlark.NewDict(5)
	_ = d.SetKey(starlark.String("primary_source"), starlark.String(dataSourcePrimary))
	_ = d.SetKey(starlark.String("secondary_source"), starlark.String(dataSourceSecondary))
	_ = d.SetKey(starlark.String("emoji_version"), starlark.String(dataEmojiVersion))
	_ = d.SetKey(starlark.String("shortcode_count"), starlark.MakeInt(len(dataForward)))
	_ = d.SetKey(starlark.String("emoji_count"), starlark.MakeInt(len(dataReverse)))
	return d, nil
}
