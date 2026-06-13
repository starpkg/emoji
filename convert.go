package emoji

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/1set/starlet/dataconv/types"
	"go.starlark.net/starlark"
)

// This file holds the "look-alike" conversions — turning a plain number, clock
// time, letter, or punctuation mark into the most similar emoji. They are pure
// Unicode arithmetic over well-defined code-point runs (keycap digits, clock
// faces, regional indicators, symbol emoji); no data table is involved, so they
// stay correct regardless of how fresh the shortcode dataset is.
//
// The two invisible combining code points used by keycap sequences are kept as
// numeric constants so the source stays free of invisible characters.
//
// Sections:
//   - number  : digits  -> keycap emoji (and back)
//   - time    : H:MM    -> clock-face emoji
//   - letter  : A-Z     -> regional-indicator / squared / circled emoji
//   - symbol  : ! ? # * -> symbol emoji

const (
	variationSelector rune = 0xFE0F // VARIATION SELECTOR-16 (emoji presentation)
	enclosingKeycap   rune = 0x20E3 // COMBINING ENCLOSING KEYCAP
)

// keycap builds the canonical keycap emoji sequence for a base rune:
// base + U+FE0F + U+20E3 (the FE0F is required for the RGI form).
func keycap(base rune) string {
	return string([]rune{base, variationSelector, enclosingKeycap})
}

// --- number ------------------------------------------------------------------

// numberToEmojiStr maps each ASCII digit to its keycap emoji; '-'/'+' become the
// heavy minus/plus dingbats. Any other rune passes through unchanged. When
// keycapTen is set and the whole input is exactly "10", the dedicated KEYCAP TEN
// glyph (U+1F51F) is emitted instead of two keycaps.
func numberToEmojiStr(text string, keycapTen bool) string {
	if keycapTen && text == "10" {
		return "\U0001F51F"
	}
	var b strings.Builder
	for _, r := range text {
		switch {
		case r >= '0' && r <= '9':
			b.WriteString(keycap(r))
		case r == '-':
			b.WriteString("➖") // heavy minus sign (U+2796)
		case r == '+':
			b.WriteString("➕") // heavy plus sign (U+2795)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// emojiToNumberStr is the inverse of numberToEmojiStr: keycap sequences become
// their digit, KEYCAP TEN becomes "10", heavy plus/minus become +/-, and every
// other rune passes through unchanged.
func emojiToNumberStr(s string) string {
	runes := []rune(s)
	var b strings.Builder
	for i := 0; i < len(runes); {
		if i+2 < len(runes) && runes[i] >= '0' && runes[i] <= '9' &&
			runes[i+1] == variationSelector && runes[i+2] == enclosingKeycap {
			b.WriteRune(runes[i])
			i += 3
			continue
		}
		switch runes[i] {
		case 0x1F51F: // KEYCAP TEN
			b.WriteString("10")
		case 0x2796: // heavy minus
			b.WriteByte('-')
		case 0x2795: // heavy plus
			b.WriteByte('+')
		default:
			b.WriteRune(runes[i])
		}
		i++
	}
	return b.String()
}

// --- time --------------------------------------------------------------------

// clockEmoji returns the single clock-face emoji nearest to hour:minute. Only
// :00 and :30 faces exist, so the minute is rounded to the nearest half hour
// (round-half-up: 15 -> :30, 45 -> next hour). The hour is taken mod 12, so AM
// and PM share a face.
//
//	full hours : U+1F550 (one o'clock) .. U+1F55B (twelve o'clock)
//	half hours : U+1F55C (one-thirty)  .. U+1F567 (twelve-thirty)
func clockEmoji(hour, minute int) (string, error) {
	if hour < 0 || hour > 23 {
		return "", fmt.Errorf("hour out of range (0-23): %d", hour)
	}
	if minute < 0 || minute > 59 {
		return "", fmt.Errorf("minute out of range (0-59): %d", minute)
	}
	step := int(math.Round(float64(minute) / 30.0)) // 0, 1, or 2
	half := step == 1
	if step == 2 {
		hour++ // rounded up to the next full hour
	}
	h12 := ((hour + 11) % 12) + 1 // 0/12/24 -> 12, 13 -> 1, ...
	base := 0x1F550               // one o'clock
	if half {
		base = 0x1F55C // one-thirty
	}
	return string(rune(base + (h12 - 1))), nil
}

// parseTimeValue extracts hour and minute from a Starlark value that is either
// an int (hour, with the optional minute argument) or a "H" / "H:MM" string.
func parseTimeValue(v starlark.Value, minuteArg starlark.Value) (int, int, error) {
	switch x := v.(type) {
	case starlark.Int:
		h, ok := x.Int64()
		if !ok {
			return 0, 0, fmt.Errorf("hour is too large")
		}
		m, err := optMinute(minuteArg)
		if err != nil {
			return 0, 0, err
		}
		return int(h), m, nil
	case starlark.String:
		return parseTimeString(string(x), minuteArg)
	default:
		return 0, 0, fmt.Errorf("time value must be an int or a string, got %s", v.Type())
	}
}

func parseTimeString(s string, minuteArg starlark.Value) (int, int, error) {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, ":")
	if len(parts) > 2 {
		return 0, 0, fmt.Errorf("invalid time %q", s)
	}
	h, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid hour in time %q", s)
	}
	if len(parts) == 2 {
		m, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return 0, 0, fmt.Errorf("invalid minute in time %q", s)
		}
		return h, m, nil
	}
	m, err := optMinute(minuteArg)
	if err != nil {
		return 0, 0, err
	}
	return h, m, nil
}

func optMinute(minuteArg starlark.Value) (int, error) {
	if minuteArg == nil || minuteArg == starlark.None {
		return 0, nil
	}
	mi, ok := minuteArg.(starlark.Int)
	if !ok {
		return 0, fmt.Errorf("minute must be an int, got %s", minuteArg.Type())
	}
	m, ok := mi.Int64()
	if !ok {
		return 0, fmt.Errorf("minute is too large")
	}
	return int(m), nil
}

// --- letter ------------------------------------------------------------------

// letterToEmojiStr maps Latin letters to emoji in one of three styles:
//
//	regional : A-Z -> regional-indicator symbols. NOTE two adjacent indicators
//	           that form a valid country code render as that flag.
//	squared  : A-Z -> negative-squared latin capitals. Only A/B/O/P are colour
//	           emoji (they get the FE0F presentation selector); the rest render
//	           as monochrome symbols.
//	circled  : A-Z/a-z -> circled latin letters, monochrome.
//
// Runes with no mapping in the chosen style pass through unchanged.
func letterToEmojiStr(text, style string) (string, error) {
	var b strings.Builder
	for _, r := range text {
		up := unicode.ToUpper(r)
		switch style {
		case "regional":
			if up >= 'A' && up <= 'Z' {
				b.WriteRune(0x1F1E6 + up - 'A') // REGIONAL INDICATOR SYMBOL LETTER A
			} else {
				b.WriteRune(r)
			}
		case "squared", "button":
			if up >= 'A' && up <= 'Z' {
				b.WriteRune(0x1F170 + up - 'A') // NEGATIVE SQUARED LATIN CAPITAL LETTER A
				if up == 'A' || up == 'B' || up == 'O' || up == 'P' {
					b.WriteRune(variationSelector) // only these four are RGI colour emoji
				}
			} else {
				b.WriteRune(r)
			}
		case "circled":
			switch {
			case r >= 'A' && r <= 'Z':
				b.WriteRune(0x24B6 + r - 'A') // CIRCLED LATIN CAPITAL LETTER A
			case r >= 'a' && r <= 'z':
				b.WriteRune(0x24D0 + r - 'a') // CIRCLED LATIN SMALL LETTER A
			default:
				b.WriteRune(r)
			}
		default:
			return "", fmt.Errorf("unknown letter style %q (want regional, squared, or circled)", style)
		}
	}
	return b.String(), nil
}

// --- symbol ------------------------------------------------------------------

// symbolMap holds the well-defined single-character punctuation -> emoji
// mappings. '#' and '*' use the canonical keycap sequence; '×' needs FE0F
// because U+2716 is text-default by Unicode.
var symbolMap = map[rune]string{
	'!': "❗",                                       // U+2757 heavy exclamation mark
	'?': "❓",                                       // U+2753 question mark
	'#': keycap('#'),                               // #️⃣ keycap number sign
	'*': keycap('*'),                               // *️⃣ keycap asterisk
	'+': "➕",                                       // U+2795 heavy plus
	'-': "➖",                                       // U+2796 heavy minus
	'/': "➗",                                       // U+2797 heavy division
	'×': string([]rune{0x2716, variationSelector}), // ✖️ heavy multiplication
	'÷': "➗",                                       // U+2797 heavy division
}

func symbolToEmojiStr(text string) string {
	var b strings.Builder
	for _, r := range text {
		if e, ok := symbolMap[r]; ok {
			b.WriteString(e)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// --- Starlark builtins -------------------------------------------------------

func (m *Module) numberToEmoji(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		value     starlark.Value
		keycapTen bool
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "value", &value, "keycap_ten?", &keycapTen); err != nil {
		return none, err
	}
	if err := m.checkValueSize(value); err != nil {
		return none, err
	}
	text, err := numericText(value)
	if err != nil {
		return none, fmt.Errorf("%s: %w", b.Name(), err)
	}
	return starlark.String(numberToEmojiStr(text, keycapTen)), nil
}

func (m *Module) emojiToNumber(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var text types.StringOrBytes
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "text", &text); err != nil {
		return none, err
	}
	s := text.GoString()
	if err := m.checkInputSize(s); err != nil {
		return none, err
	}
	return starlark.String(emojiToNumberStr(s)), nil
}

func (m *Module) timeToEmoji(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		value  starlark.Value
		minute starlark.Value = none
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "value", &value, "minute?", &minute); err != nil {
		return none, err
	}
	if err := m.checkValueSize(value); err != nil {
		return none, err
	}
	h, mm, err := parseTimeValue(value, minute)
	if err != nil {
		return none, fmt.Errorf("%s: %w", b.Name(), err)
	}
	out, err := clockEmoji(h, mm)
	if err != nil {
		return none, fmt.Errorf("%s: %w", b.Name(), err)
	}
	return starlark.String(out), nil
}

func (m *Module) letterToEmoji(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var (
		text  types.StringOrBytes
		style = "regional"
	)
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "text", &text, "style?", &style); err != nil {
		return none, err
	}
	s := text.GoString()
	if err := m.checkInputSize(s); err != nil {
		return none, err
	}
	out, err := letterToEmojiStr(s, style)
	if err != nil {
		return none, fmt.Errorf("%s: %w", b.Name(), err)
	}
	return starlark.String(out), nil
}

func (m *Module) symbolToEmoji(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var text types.StringOrBytes
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "text", &text); err != nil {
		return none, err
	}
	s := text.GoString()
	if err := m.checkInputSize(s); err != nil {
		return none, err
	}
	return starlark.String(symbolToEmojiStr(s)), nil
}

// numericText renders an int/float/string Starlark value as the decimal text
// that numberToEmojiStr consumes.
func numericText(v starlark.Value) (string, error) {
	switch x := v.(type) {
	case starlark.Int:
		return x.String(), nil
	case starlark.Float:
		return strconv.FormatFloat(float64(x), 'f', -1, 64), nil
	case starlark.String:
		return string(x), nil
	default:
		return "", fmt.Errorf("number value must be an int, float, or string, got %s", v.Type())
	}
}
