package emoji

// Tests for the shortcode core and the module wiring (emoji.go).
//
// Sections:
//   - emojize / demojize round-trip, unknown tokens, flag fallback, delimiters
//   - get / name / describe single-emoji lookups
//   - the convert() dispatcher (auto detection + explicit kinds)
//   - info() data provenance
//   - the max_input_bytes host config cap
//   - pure helpers: parseDelimiters / textOf / autoKind / upper / regionalPair

import (
	"strings"
	"testing"

	"github.com/1set/starlet"
	"go.starlark.net/starlark"
)

func run(t *testing.T, script string) (map[string]interface{}, error) {
	t.Helper()
	m := starlet.NewDefault()
	m.SetScriptContent([]byte(script))
	m.SetLazyloadModules(map[string]starlet.ModuleLoader{ModuleName: NewModule().LoadModule()})
	return m.Run()
}

func mustRun(t *testing.T, script string) map[string]interface{} {
	t.Helper()
	res, err := run(t, script)
	if err != nil {
		t.Fatalf("script failed: %v", err)
	}
	return res
}

const (
	rocket = "\U0001F680" // 🚀
	heart  = "❤️"         // ❤️
)

// --- emojize / demojize ------------------------------------------------------

func TestEmojizeDemojize(t *testing.T) {
	res := mustRun(t, `
load("emoji", "emojize", "demojize")
forward = emojize("i :heart: :rocket:")
roundtrip = demojize(emojize("i :heart: :rocket:"))
unknown = emojize("x :no_such_code: y")
flag = emojize(":flag-fr:")
`)
	if got := res["forward"].(string); !strings.Contains(got, rocket) || !strings.Contains(got, heart) {
		t.Errorf("emojize did not substitute: %q", got)
	}
	if got := res["roundtrip"].(string); got != "i :heart: :rocket:" {
		t.Errorf("round-trip = %q, want %q", got, "i :heart: :rocket:")
	}
	if got := res["unknown"].(string); got != "x :no_such_code: y" {
		t.Errorf("unknown token should be left as-is, got %q", got)
	}
	if got := res["flag"].(string); got != "\U0001F1EB\U0001F1F7" {
		t.Errorf("flag fallback = %q, want 🇫🇷", got)
	}
}

func TestDemojizeDelimiters(t *testing.T) {
	res := mustRun(t, `
load("emoji", "demojize")
out = demojize("hi \U0001F680", delimiters=("[", "]"))
`)
	if got := res["out"].(string); got != "hi [rocket]" {
		t.Errorf("custom delimiters = %q, want %q", got, "hi [rocket]")
	}
}

// --- get / name / describe ---------------------------------------------------

func TestGetNameDescribe(t *testing.T) {
	res := mustRun(t, `
load("emoji", "get", "name", "describe")
g1 = get("rocket")
g2 = get(":rocket:")
missing = get("definitely_not_a_code")
n = name("\U0001F680")
d = describe("\U0001F680")
# 💪 is a glyph whose short alias ("muscle") differs from its human-readable
# description ("flexed biceps"), so it proves name() reads the reverse-alias
# table and describe() reads the names table (not the same map).
n_muscle = name("\U0001F4AA")
d_muscle = describe("\U0001F4AA")
`)
	if got := res["g1"].(string); got != rocket {
		t.Errorf("get(rocket) = %q, want 🚀", got)
	}
	if got := res["g2"].(string); got != rocket {
		t.Errorf("get(:rocket:) = %q, want 🚀", got)
	}
	if res["missing"] != nil {
		t.Errorf("get(unknown) should be None, got %v", res["missing"])
	}
	if got := res["n"].(string); got != "rocket" {
		t.Errorf("name(🚀) = %q, want rocket", got)
	}
	if got := res["d"].(string); got != "rocket" {
		t.Errorf("describe(🚀) = %q, want rocket", got)
	}
	// name() returns the short shortcode alias; describe() returns the human name.
	if got := res["n_muscle"].(string); got != "muscle" {
		t.Errorf("name(💪) = %q, want muscle (short alias)", got)
	}
	if got := res["d_muscle"].(string); got != "flexed biceps" {
		t.Errorf("describe(💪) = %q, want flexed biceps (human name)", got)
	}
}

// --- convert dispatcher ------------------------------------------------------

func TestConvert(t *testing.T) {
	res := mustRun(t, `
load("emoji", "convert")
auto_number = convert(42)
auto_time = convert("3:30")
auto_emojize = convert(":smile:")
letter = convert("AB", kind="letter")
symbol = convert("!?", kind="symbol")
demoji = convert("\U0001F680", kind="demojize")
`)
	if got := res["auto_number"].(string); got != "4️⃣"+"2️⃣" {
		t.Errorf("convert(42) = %q", got)
	}
	if got := res["auto_time"].(string); got != "\U0001F55E" {
		t.Errorf("convert(3:30) = %q, want 🕞", got)
	}
	if got := res["auto_emojize"].(string); got != "\U0001f604" {
		t.Errorf("convert(:smile:) = %q, want 😄", got)
	}
	if got := res["letter"].(string); got != "\U0001F1E6\U0001F1E7" {
		t.Errorf("convert(AB, letter) = %q, want 🇦🇧", got)
	}
	if got := res["symbol"].(string); got != "❗❓" {
		t.Errorf("convert(!?, symbol) = %q", got)
	}
	if got := res["demoji"].(string); got != ":rocket:" {
		t.Errorf("convert(🚀, demojize) = %q, want :rocket:", got)
	}
}

func TestConvertAutoTimeFallthrough(t *testing.T) {
	// A time-shaped but out-of-range string must NOT hard-error under auto; it
	// degrades to emojize and (having no shortcodes) is returned untouched.
	res := mustRun(t, `
load("emoji", "convert")
out = convert("99:99")
`)
	if got := res["out"].(string); got != "99:99" {
		t.Errorf("convert(99:99) = %q, want %q (emojize fallthrough)", got, "99:99")
	}
}

func TestConvertUnknownKind(t *testing.T) {
	_, err := run(t, `
load("emoji", "convert")
convert("x", kind="bogus")
`)
	if err == nil || !strings.Contains(err.Error(), "unknown kind") {
		t.Fatalf("expected unknown-kind error, got %v", err)
	}
}

// Explicit kinds drive a value down a branch that auto-detection would not pick:
// an int hour through kind="time", a numeric string through kind="number", and
// a float through kind="number" (which auto would also do, but we assert the
// formatted output). These exercise the dispatcher's number/time arms directly.
func TestConvertExplicitKinds(t *testing.T) {
	res := mustRun(t, `
load("emoji", "convert")
time_int = convert(3, kind="time")
num_str  = convert("07", kind="number")
num_flt  = convert(3.5, kind="number")
emojize_kind = convert(":fire:", kind="emojize")
`)
	if got := res["time_int"].(string); got != "\U0001F552" { // 3 o'clock face
		t.Errorf("convert(3, time) = %q, want 🕒", got)
	}
	if got := res["num_str"].(string); got != "0️⃣"+"7️⃣" {
		t.Errorf("convert(\"07\", number) = %q", got)
	}
	if got := res["num_flt"].(string); got != "3️⃣"+"."+"5️⃣" {
		t.Errorf("convert(3.5, number) = %q", got)
	}
	if got := res["emojize_kind"].(string); got != "\U0001F525" { // 🔥
		t.Errorf("convert(:fire:, emojize) = %q, want 🔥", got)
	}
}

// A bytes value flows through convert: auto routes it to emojize via textOf, and
// an explicit non-string/non-bytes value to a text kind is a clean error.
func TestConvertBytesAndTextErrors(t *testing.T) {
	res := mustRun(t, `
load("emoji", "convert")
auto_bytes = convert(b":rocket:")
letter_bytes = convert(b"ab", kind="letter")
`)
	if got := res["auto_bytes"].(string); got != rocket {
		t.Errorf("convert(b\":rocket:\") = %q, want 🚀", got)
	}
	if got := res["letter_bytes"].(string); got != "\U0001F1E6\U0001F1E7" {
		t.Errorf("convert(b\"ab\", letter) = %q, want 🇦🇧", got)
	}
	for _, c := range []struct{ script, wantErr string }{
		{`load("emoji","convert")
convert(True, kind="symbol")`, "expected a string"},
		{`load("emoji","convert")
convert([1], kind="number")`, "must be an int, float, or string"},
		{`load("emoji","convert")
convert(3.5, kind="time")`, "must be an int or a string"},
		// missing required value -> UnpackArgs error
		{`load("emoji","convert")
convert(kind="number")`, "missing argument for value"},
		// explicit kind="time" with an out-of-range time -> clockEmoji error
		// (auto would fall through to emojize, but the explicit path must error).
		{`load("emoji","convert")
convert("25:00", kind="time")`, "out of range"},
		{`load("emoji","convert")
convert(99, kind="time")`, "out of range"},
	} {
		_, err := run(t, c.script)
		if err == nil || !strings.Contains(err.Error(), c.wantErr) {
			t.Errorf("script %q: want error containing %q, got %v", c.script, c.wantErr, err)
		}
	}
}

// info() takes no arguments; an extra positional argument is a clean error.
func TestInfoRejectsArgs(t *testing.T) {
	_, err := run(t, `
load("emoji", "info")
info("unexpected")
`)
	if err == nil || !strings.Contains(err.Error(), "want at most 0") {
		t.Fatalf("info() with an arg: want arity error, got %v", err)
	}
}

// --- info --------------------------------------------------------------------

func TestInfo(t *testing.T) {
	res := mustRun(t, `
load("emoji", "info")
i = info()
ev = i["emoji_version"]
many = i["shortcode_count"] > 1000
primary = i["primary_source"]
secondary = i["secondary_source"]
ecount = i["emoji_count"]
keys = sorted(i.keys())
`)
	if got := res["ev"].(string); got != "17.0" {
		t.Errorf("emoji_version = %q, want 17.0", got)
	}
	if got := res["many"].(bool); !got {
		t.Error("shortcode_count should exceed 1000")
	}
	if got := res["primary"].(string); !strings.Contains(got, "carpedm20") {
		t.Errorf("primary_source = %q, want carpedm20 source", got)
	}
	if got := res["secondary"].(string); !strings.Contains(got, "gemoji") {
		t.Errorf("secondary_source = %q, want gemoji source", got)
	}
	if got := res["ecount"].(int64); got <= 1000 {
		t.Errorf("emoji_count = %d, want > 1000", got)
	}
	// info() must surface exactly the five documented keys.
	got := res["keys"].([]interface{})
	want := []string{"emoji_count", "emoji_version", "primary_source", "secondary_source", "shortcode_count"}
	if len(got) != len(want) {
		t.Fatalf("info() keys = %v, want %v", got, want)
	}
	for i, k := range want {
		if got[i].(string) != k {
			t.Errorf("info() key[%d] = %q, want %q", i, got[i], k)
		}
	}
}

// --- dedicated converters ----------------------------------------------------

func TestDedicatedConverters(t *testing.T) {
	res := mustRun(t, `
load("emoji", "number_to_emoji", "emoji_to_number", "time_to_emoji", "letter_to_emoji", "symbol_to_emoji")
n = number_to_emoji(42)
ten = number_to_emoji(10, keycap_ten=True)
back = emoji_to_number(number_to_emoji(2024))
t_str = time_to_emoji("3:30")
t_args = time_to_emoji(15, minute=30)
let = letter_to_emoji("ab")
sq = letter_to_emoji("A", style="squared")
sym = symbol_to_emoji("50%!")
`)
	if got := res["n"].(string); got != "4️⃣"+"2️⃣" {
		t.Errorf("number_to_emoji(42) = %q", got)
	}
	if got := res["ten"].(string); got != "\U0001F51F" {
		t.Errorf("number_to_emoji(10, keycap_ten) = %q, want 🔟", got)
	}
	if got := res["back"].(string); got != "2024" {
		t.Errorf("emoji_to_number round-trip = %q", got)
	}
	if got := res["t_str"].(string); got != "\U0001F55E" {
		t.Errorf("time_to_emoji(3:30) = %q, want 🕞", got)
	}
	if got := res["t_args"].(string); got != "\U0001F55E" {
		t.Errorf("time_to_emoji(15, 30) = %q, want 🕞 (3:30 face)", got)
	}
	if got := res["let"].(string); got != "\U0001F1E6\U0001F1E7" {
		t.Errorf("letter_to_emoji(ab) = %q, want 🇦🇧", got)
	}
	if got := res["sq"].(string); got != "\U0001F170️" {
		t.Errorf("letter_to_emoji(A, squared) = %q", got)
	}
	if got := res["sym"].(string); !strings.Contains(got, "❗") {
		t.Errorf("symbol_to_emoji(50%%!) = %q", got)
	}
}

// --- error paths -------------------------------------------------------------

func TestErrorPaths(t *testing.T) {
	cases := []struct {
		name, script, wantErr string
	}{
		{"time bad string", `load("emoji","time_to_emoji")
time_to_emoji("nope")`, "invalid hour"},
		{"time hour range", `load("emoji","time_to_emoji")
time_to_emoji(25)`, "out of range"},
		{"letter bad style", `load("emoji","letter_to_emoji")
letter_to_emoji("A", style="fancy")`, "unknown letter style"},
		{"number bad type", `load("emoji","number_to_emoji")
number_to_emoji([1,2])`, "must be an int"},
		{"delimiters wrong len", `load("emoji","demojize")
demojize("x", delimiters=("only",))`, "exactly 2"},
		{"convert letter non-string", `load("emoji","convert")
convert(42, kind="letter")`, "expected a string"},
		// Missing required arguments: every builtin must surface a clean
		// "missing argument" via UnpackArgs rather than panicking.
		{"emojize missing arg", `load("emoji","emojize")
emojize()`, "missing argument for text"},
		{"demojize missing arg", `load("emoji","demojize")
demojize()`, "missing argument for text"},
		{"letter missing arg", `load("emoji","letter_to_emoji")
letter_to_emoji()`, "missing argument for text"},
		{"symbol missing arg", `load("emoji","symbol_to_emoji")
symbol_to_emoji()`, "missing argument for text"},
		{"number missing arg", `load("emoji","number_to_emoji")
number_to_emoji()`, "missing argument for value"},
		{"emoji_to_number missing arg", `load("emoji","emoji_to_number")
emoji_to_number()`, "missing argument for text"},
		{"time missing arg", `load("emoji","time_to_emoji")
time_to_emoji()`, "missing argument for value"},
		{"get missing arg", `load("emoji","get")
get()`, "missing argument for name"},
		{"name missing arg", `load("emoji","name")
name()`, "missing argument for emoji"},
		{"describe missing arg", `load("emoji","describe")
describe()`, "missing argument for emoji"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := run(t, c.script)
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("want error containing %q, got %v", c.wantErr, err)
			}
		})
	}
}

// Every dedicated text builtin enforces max_input_bytes via checkInputSize; the
// letter/symbol/emoji_to_number paths exercise the cap arm that the number/time
// regression test does not.
func TestInputCapAllTextBuiltins(t *testing.T) {
	scripts := map[string]string{
		"letter_to_emoji": `load("emoji","letter_to_emoji")
letter_to_emoji("AAAAAAAAAA")`,
		"symbol_to_emoji": `load("emoji","symbol_to_emoji")
symbol_to_emoji("!!!!!!!!!!")`,
		"emoji_to_number": `load("emoji","emoji_to_number")
emoji_to_number("1234567890")`,
		"demojize": `load("emoji","demojize")
demojize("xxxxxxxxxx")`,
		// a bytes time value is bounded by checkValueSize before parsing.
		"time_to_emoji_bytes": `load("emoji","time_to_emoji")
time_to_emoji(b"1234567890")`,
	}
	for name, script := range scripts {
		t.Run(name, func(t *testing.T) {
			t.Setenv("EMOJI_MAX_INPUT_BYTES", "4")
			if _, err := run(t, script); err == nil || !strings.Contains(err.Error(), "max_input_bytes") {
				t.Errorf("%s: expected max_input_bytes error, got %v", name, err)
			}
		})
	}
}

func TestNumberFloatAndListDelimiters(t *testing.T) {
	res := mustRun(t, `
load("emoji", "number_to_emoji", "demojize")
f = number_to_emoji(3.5)
d = demojize("go \U0001F680", delimiters=["<", ">"])
`)
	if got := res["f"].(string); got != "3️⃣"+"."+"5️⃣" {
		t.Errorf("number_to_emoji(3.5) = %q", got)
	}
	if got := res["d"].(string); got != "go <rocket>" {
		t.Errorf("list delimiters = %q, want %q", got, "go <rocket>")
	}
}

func TestNameDescribeUnknown(t *testing.T) {
	res := mustRun(t, `
load("emoji", "name", "describe")
n = name("zzz not an emoji")
d = describe("zzz not an emoji")
`)
	if res["n"] != nil {
		t.Errorf("name(unknown) should be None, got %v", res["n"])
	}
	if res["d"] != nil {
		t.Errorf("describe(unknown) should be None, got %v", res["d"])
	}
}

// --- max_input_bytes ---------------------------------------------------------

func TestMaxInputBytes(t *testing.T) {
	t.Run("rejects oversized", func(t *testing.T) {
		t.Setenv("EMOJI_MAX_INPUT_BYTES", "4")
		_, err := run(t, `
load("emoji", "emojize")
emojize("this is well over four bytes :smile:")
`)
		if err == nil || !strings.Contains(err.Error(), "max_input_bytes") {
			t.Fatalf("expected max_input_bytes error, got %v", err)
		}
	})
	t.Run("normal passes", func(t *testing.T) {
		res := mustRun(t, `
load("emoji", "emojize")
out = emojize(":smile:")
`)
		if got := res["out"].(string); got != "\U0001f604" {
			t.Errorf("emojize(:smile:) = %q, want 😄", got)
		}
	})
	// The number/time paths must honor the cap too (regression: they used to
	// bypass it while letter/symbol enforced it).
	t.Run("number paths enforce cap", func(t *testing.T) {
		t.Setenv("EMOJI_MAX_INPUT_BYTES", "4")
		for _, script := range []string{
			`load("emoji","number_to_emoji")
number_to_emoji("123456789")`,
			`load("emoji","emoji_to_number")
emoji_to_number("123456789")`,
			`load("emoji","convert")
convert("123456789", kind="number")`,
		} {
			if _, err := run(t, script); err == nil || !strings.Contains(err.Error(), "max_input_bytes") {
				t.Errorf("expected max_input_bytes error for %q, got %v", script, err)
			}
		}
	})
	// A bytes value (not just a string) routed through the dispatcher must also
	// be size-checked — checkValueSize handles the Bytes case.
	t.Run("bytes value enforces cap", func(t *testing.T) {
		t.Setenv("EMOJI_MAX_INPUT_BYTES", "4")
		if _, err := run(t, `
load("emoji","convert")
convert(b"way too many bytes here", kind="letter")
`); err == nil || !strings.Contains(err.Error(), "max_input_bytes") {
			t.Errorf("expected max_input_bytes error for bytes value, got %v", err)
		}
	})
	// A cap of 0 disables the limit, so an arbitrarily long input passes.
	t.Run("zero disables the cap", func(t *testing.T) {
		t.Setenv("EMOJI_MAX_INPUT_BYTES", "0")
		res := mustRun(t, `
load("emoji","letter_to_emoji")
out = letter_to_emoji("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
`)
		if got := res["out"].(string); got == "" {
			t.Error("expected non-empty output with cap disabled")
		}
	})
	// The cap can also be lowered from Starlark via the base-provided setter.
	t.Run("setter lowers cap", func(t *testing.T) {
		_, err := run(t, `
load("emoji","set_max_input_bytes","emojize")
set_max_input_bytes(3)
emojize("this is long :smile:")
`)
		if err == nil || !strings.Contains(err.Error(), "max_input_bytes") {
			t.Errorf("expected max_input_bytes error after set_max_input_bytes(3), got %v", err)
		}
	})
}

// --- pure helpers ------------------------------------------------------------

// parseDelimiters accepts the default (nil/None -> ":" / ":"), a 2-tuple, a
// 2-element list, and rejects a wrong type, a wrong length, and non-strings.
func TestParseDelimiters(t *testing.T) {
	tup := func(vs ...starlark.Value) starlark.Tuple { return starlark.Tuple(vs) }

	openC, closeC, err := parseDelimiters(nil)
	if err != nil || openC != ":" || closeC != ":" {
		t.Errorf("parseDelimiters(nil) = (%q,%q,%v), want (\":\",\":\",nil)", openC, closeC, err)
	}
	openC, closeC, err = parseDelimiters(starlark.None)
	if err != nil || openC != ":" || closeC != ":" {
		t.Errorf("parseDelimiters(None) = (%q,%q,%v), want default colons", openC, closeC, err)
	}
	openC, closeC, err = parseDelimiters(tup(starlark.String("["), starlark.String("]")))
	if err != nil || openC != "[" || closeC != "]" {
		t.Errorf("parseDelimiters(tuple) = (%q,%q,%v), want ([,])", openC, closeC, err)
	}
	lst := starlark.NewList([]starlark.Value{starlark.String("<"), starlark.String(">")})
	openC, closeC, err = parseDelimiters(lst)
	if err != nil || openC != "<" || closeC != ">" {
		t.Errorf("parseDelimiters(list) = (%q,%q,%v), want (<,>)", openC, closeC, err)
	}

	cases := []struct {
		v          starlark.Value
		wantErrSub string
	}{
		{starlark.MakeInt(1), "must be a (open, close) tuple"},
		{tup(starlark.String("only")), "exactly 2 elements"},
		{tup(starlark.MakeInt(1), starlark.MakeInt(2)), "must both be strings"},
		{starlark.NewList([]starlark.Value{starlark.String("a")}), "exactly 2 elements"},
	}
	for _, c := range cases {
		if _, _, err := parseDelimiters(c.v); err == nil || !strings.Contains(err.Error(), c.wantErrSub) {
			t.Errorf("parseDelimiters(%v) err = %v, want substring %q", c.v, err, c.wantErrSub)
		}
	}
}

// textOf accepts string and bytes and rejects everything else.
func TestTextOf(t *testing.T) {
	if s, err := textOf(starlark.String("hi")); err != nil || s != "hi" {
		t.Errorf("textOf(string) = (%q,%v)", s, err)
	}
	if s, err := textOf(starlark.Bytes("by")); err != nil || s != "by" {
		t.Errorf("textOf(bytes) = (%q,%v)", s, err)
	}
	if _, err := textOf(starlark.MakeInt(3)); err == nil || !strings.Contains(err.Error(), "expected a string") {
		t.Errorf("textOf(int) err = %v, want type error", err)
	}
}

// autoKind classifies values for kind="auto": ints/floats -> number, an in-range
// "H:MM" string -> time, an out-of-range time-shaped string -> emojize, and any
// other type -> emojize.
func TestAutoKind(t *testing.T) {
	cases := []struct {
		v    starlark.Value
		want string
	}{
		{starlark.MakeInt(42), "number"},
		{starlark.Float(3.5), "number"},
		{starlark.String("3:30"), "time"},
		{starlark.String(" 9:15 "), "time"},   // surrounding space is trimmed
		{starlark.String("99:99"), "emojize"}, // time-shaped but out of range
		{starlark.String("24:00"), "emojize"}, // hour out of range -> emojize
		{starlark.String(":rocket:"), "emojize"},
		{starlark.Bytes(":rocket:"), "emojize"}, // non-string/int/float -> emojize
		{starlark.None, "emojize"},
	}
	for _, c := range cases {
		if got := autoKind(c.v); got != c.want {
			t.Errorf("autoKind(%v) = %q, want %q", c.v, got, c.want)
		}
	}
}

// upper uppercases ASCII letters for the env-var prefix and leaves other bytes
// (digits, underscores) untouched.
func TestUpper(t *testing.T) {
	cases := []struct{ in, want string }{
		{"max_input_bytes", "MAX_INPUT_BYTES"},
		{"abcXYZ", "ABCXYZ"},
		{"1_2", "1_2"},
		{"", ""},
	}
	for _, c := range cases {
		if got := upper(c.in); got != c.want {
			t.Errorf("upper(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// regionalPair turns a two-letter region code into its regional-indicator pair,
// case-insensitively.
func TestRegionalPair(t *testing.T) {
	if got := regionalPair("fr"); got != "\U0001F1EB\U0001F1F7" {
		t.Errorf("regionalPair(fr) = %q, want 🇫🇷", got)
	}
	if got := regionalPair("US"); got != "\U0001F1FA\U0001F1F8" {
		t.Errorf("regionalPair(US) = %q, want 🇺🇸 (upper-case accepted)", got)
	}
}

// get falls back to a regional flag for a flag-xx code that is not a table
// shortcode, and returns None for a code reduced to empty by colon trimming.
func TestGetFlagFallbackAndEmpty(t *testing.T) {
	res := mustRun(t, `
load("emoji", "get")
flag = get("flag-zz")          # not a real country, but a valid regional pair
flag_us = get(":flag-us:")     # colons stripped, then flag fallback
empty = get(":::")             # trims to "" -> None
`)
	if got := res["flag"].(string); got != "\U0001F1FF\U0001F1FF" {
		t.Errorf("get(flag-zz) = %q, want 🇿🇿", got)
	}
	if got := res["flag_us"].(string); got != "\U0001F1FA\U0001F1F8" {
		t.Errorf("get(:flag-us:) = %q, want 🇺🇸", got)
	}
	if res["empty"] != nil {
		t.Errorf("get(:::) should be None, got %v", res["empty"])
	}
}
