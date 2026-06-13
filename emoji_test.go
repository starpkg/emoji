package emoji

// Tests for the shortcode core and the module wiring (emoji.go).
//
// Sections:
//   - emojize / demojize round-trip, unknown tokens, flag fallback, delimiters
//   - get / name / describe single-emoji lookups
//   - the convert() dispatcher (auto detection + explicit kinds)
//   - info() data provenance
//   - the max_input_bytes host config cap

import (
	"strings"
	"testing"

	"github.com/1set/starlet"
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

// --- info --------------------------------------------------------------------

func TestInfo(t *testing.T) {
	res := mustRun(t, `
load("emoji", "info")
i = info()
ev = i["emoji_version"]
many = i["shortcode_count"] > 1000
`)
	if got := res["ev"].(string); got != "17.0" {
		t.Errorf("emoji_version = %q, want 17.0", got)
	}
	if got := res["many"].(bool); !got {
		t.Error("shortcode_count should exceed 1000")
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
}
