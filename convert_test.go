package emoji

// Tests for the look-alike conversions (convert.go).
//
// Sections:
//   - number : keycap digits, signs, keycap-ten, round-trip
//   - time   : the full 24-face table, minute rounding, 24h, parsing, errors
//   - letter : regional / squared / circled styles + pass-through
//   - symbol : the fixed punctuation table + pass-through
//   - parsers: parseTimeString / optMinute / parseTimeValue / numericText edge cases

import (
	"math/big"
	"strings"
	"testing"

	"go.starlark.net/starlark"
)

// --- number ------------------------------------------------------------------

func TestKeycapBytes(t *testing.T) {
	// A golden byte check that is independent of the rune constants: the keycap
	// for '7' is U+0037 U+FE0F U+20E3 = 37 EF B8 8F E2 83 A3 in UTF-8.
	got := []byte(keycap('7'))
	want := []byte{0x37, 0xEF, 0xB8, 0x8F, 0xE2, 0x83, 0xA3}
	if string(got) != string(want) {
		t.Fatalf("keycap('7') = % x, want % x", got, want)
	}
}

func TestNumberToEmoji(t *testing.T) {
	cases := []struct {
		in        string
		keycapTen bool
		want      string
	}{
		{"0", false, keycap('0')},
		{"42", false, keycap('4') + keycap('2')},
		{"-3", false, "➖" + keycap('3')},
		{"+8", false, "➕" + keycap('8')},
		{"10", false, keycap('1') + keycap('0')},
		{"10", true, "\U0001F51F"},
		{"3.5", false, keycap('3') + "." + keycap('5')}, // '.' passes through
		{"", false, ""},
	}
	for _, c := range cases {
		if got := numberToEmojiStr(c.in, c.keycapTen); got != c.want {
			t.Errorf("numberToEmojiStr(%q, %v) = %q, want %q", c.in, c.keycapTen, got, c.want)
		}
	}
}

func TestNumberRoundTrip(t *testing.T) {
	for _, in := range []string{"0", "42", "-3", "+8", "100", "2024"} {
		if got := emojiToNumberStr(numberToEmojiStr(in, false)); got != in {
			t.Errorf("round-trip %q -> %q", in, got)
		}
	}
	if got := emojiToNumberStr("\U0001F51F"); got != "10" {
		t.Errorf("keycap-ten -> %q, want 10", got)
	}
}

// emojiToNumberStr must decode the heavy +/- dingbats and keycap-ten when they
// are embedded in surrounding text, and leave every other rune untouched.
func TestEmojiToNumberStrDecode(t *testing.T) {
	cases := []struct{ in, want string }{
		{"➕➖", "+-"},                             // heavy plus/minus -> ASCII
		{"a➖" + keycap('5'), "a-5"},              // mixed with passthrough text
		{"\U0001F51F kg", "10 kg"},               // keycap-ten in context
		{keycap('1') + "\U0001F51F", "1" + "10"}, // keycap digit then keycap-ten
		{"no digits here", "no digits here"},     // pure passthrough
		{"7" + string(variationSelector), "7️"},  // FE0F without enclosing keycap is not a keycap
		{string(rune(0x2796)) + "1", "-1"},       // bare heavy minus rune
	}
	for _, c := range cases {
		if got := emojiToNumberStr(c.in); got != c.want {
			t.Errorf("emojiToNumberStr(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// A keycap digit at the very end of the string (its enclosing-keycap rune is the
// last rune) must still decode — the i+2<len bound is an index check, not a
// length check, so the final valid keycap is included.
func TestEmojiToNumberStrTrailingKeycap(t *testing.T) {
	if got := emojiToNumberStr(keycap('9')); got != "9" {
		t.Errorf("trailing single keycap = %q, want 9", got)
	}
	if got := emojiToNumberStr("x" + keycap('3')); got != "x3" {
		t.Errorf("prefixed trailing keycap = %q, want x3", got)
	}
}

// --- time --------------------------------------------------------------------

func TestClockFacesTable(t *testing.T) {
	// Exact code points for all 24 faces (research-confirmed).
	fullHours := map[int]rune{
		1: 0x1F550, 2: 0x1F551, 3: 0x1F552, 4: 0x1F553, 5: 0x1F554, 6: 0x1F555,
		7: 0x1F556, 8: 0x1F557, 9: 0x1F558, 10: 0x1F559, 11: 0x1F55A, 12: 0x1F55B,
	}
	halfHours := map[int]rune{
		1: 0x1F55C, 2: 0x1F55D, 3: 0x1F55E, 4: 0x1F55F, 5: 0x1F560, 6: 0x1F561,
		7: 0x1F562, 8: 0x1F563, 9: 0x1F564, 10: 0x1F565, 11: 0x1F566, 12: 0x1F567,
	}
	for h, want := range fullHours {
		got, err := clockEmoji(h, 0)
		if err != nil || got != string(want) {
			t.Errorf("clockEmoji(%d, 0) = %q (err %v), want %q", h, got, err, string(want))
		}
	}
	for h, want := range halfHours {
		got, err := clockEmoji(h, 30)
		if err != nil || got != string(want) {
			t.Errorf("clockEmoji(%d, 30) = %q (err %v), want %q", h, got, err, string(want))
		}
	}
}

func TestClockRoundingAnd24h(t *testing.T) {
	cases := []struct {
		h, m int
		want rune
	}{
		{9, 47, 0x1F559},  // -> 10:00
		{2, 20, 0x1F55D},  // -> 2:30
		{6, 5, 0x1F555},   // -> 6:00
		{3, 15, 0x1F55E},  // 15 rounds up to :30
		{3, 44, 0x1F55E},  // still :30
		{3, 45, 0x1F553},  // rounds up to 4:00
		{15, 0, 0x1F552},  // 24h pm shares 3:00 face
		{0, 0, 0x1F55B},   // midnight -> 12:00
		{23, 50, 0x1F55B}, // -> 24:00 -> 12:00 face
	}
	for _, c := range cases {
		got, err := clockEmoji(c.h, c.m)
		if err != nil || got != string(c.want) {
			t.Errorf("clockEmoji(%d, %d) = %q (err %v), want %q", c.h, c.m, got, err, string(c.want))
		}
	}
}

func TestClockErrors(t *testing.T) {
	if _, err := clockEmoji(24, 0); err == nil {
		t.Error("expected error for hour 24")
	}
	if _, err := clockEmoji(3, 60); err == nil {
		t.Error("expected error for minute 60")
	}
}

// --- letter ------------------------------------------------------------------

func TestLetterStyles(t *testing.T) {
	cases := []struct {
		text, style, want string
	}{
		{"AB", "regional", "\U0001F1E6\U0001F1E7"},
		{"ab", "regional", "\U0001F1E6\U0001F1E7"}, // upper-cased first
		{"A", "squared", "\U0001F170️"},            // A button is RGI (gets FE0F)
		{"C", "squared", "\U0001F172"},             // C has no colour emoji form
		{"A", "circled", "Ⓐ"},
		{"a", "circled", "ⓐ"},
		{"A!", "regional", "\U0001F1E6!"}, // '!' passes through
	}
	for _, c := range cases {
		got, err := letterToEmojiStr(c.text, c.style)
		if err != nil || got != c.want {
			t.Errorf("letterToEmojiStr(%q, %q) = %q (err %v), want %q", c.text, c.style, got, err, c.want)
		}
	}
}

func TestLetterUnknownStyle(t *testing.T) {
	if _, err := letterToEmojiStr("A", "fancy"); err == nil {
		t.Error("expected error for unknown style")
	}
}

// "button" is a documented alias for "squared", and every style must pass
// non-letter runes through unchanged (the else/default arms of each switch).
func TestLetterStylesAliasAndPassthrough(t *testing.T) {
	cases := []struct {
		text, style, want string
	}{
		{"A", "button", "\U0001F170️"},    // button == squared, A is RGI colour
		{"B", "button", "\U0001F171️"},    // B is RGI colour
		{"1A", "regional", "1\U0001F1E6"}, // digit passes through, A maps
		{"5", "squared", "5"},             // non-letter passthrough (squared else)
		{"é", "circled", "é"},             // non-ASCII passthrough (circled default)
		{"Z", "circled", "Ⓩ"},             // CIRCLED LATIN CAPITAL LETTER Z
		{"z", "circled", "ⓩ"},             // CIRCLED LATIN SMALL LETTER Z
	}
	for _, c := range cases {
		got, err := letterToEmojiStr(c.text, c.style)
		if err != nil || got != c.want {
			t.Errorf("letterToEmojiStr(%q, %q) = %q (err %v), want %q", c.text, c.style, got, err, c.want)
		}
	}
}

// The arithmetic symbols × and ÷ map through the symbol table too; the multiply
// sign carries an FE0F presentation selector (U+2716 is text-default).
func TestSymbolsArithmetic(t *testing.T) {
	cases := []struct{ in, want string }{
		{"×", string([]rune{0x2716, variationSelector})},
		{"÷", "➗"},
		{"a×b÷c", "a" + string([]rune{0x2716, variationSelector}) + "b➗c"},
		{"", ""},     // empty in, empty out
		{"中文", "中文"}, // non-ASCII passthrough
	}
	for _, c := range cases {
		if got := symbolToEmojiStr(c.in); got != c.want {
			t.Errorf("symbolToEmojiStr(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// --- parsers -----------------------------------------------------------------

// parseTimeString covers the "H" (hour only, minute from arg), "H:MM", too-many
// colons, and the non-numeric hour/minute error arms.
func TestParseTimeString(t *testing.T) {
	cases := []struct {
		s          string
		minuteArg  starlark.Value
		wantH      int
		wantM      int
		wantErrSub string
	}{
		{"3:30", nil, 3, 30, ""},
		{"  9 : 15 ", nil, 9, 15, ""},          // whitespace is trimmed around both parts
		{"7", nil, 7, 0, ""},                   // hour only, no minute arg -> minute 0
		{"7", starlark.MakeInt(45), 7, 45, ""}, // hour only, minute from arg
		{"1:2:3", nil, 0, 0, "invalid time"},   // too many colons
		{"x:30", nil, 0, 0, "invalid hour"},    // non-numeric hour
		{"3:zz", nil, 0, 0, "invalid minute"},  // non-numeric minute
	}
	for _, c := range cases {
		h, m, err := parseTimeString(c.s, c.minuteArg)
		if c.wantErrSub != "" {
			if err == nil || !strings.Contains(err.Error(), c.wantErrSub) {
				t.Errorf("parseTimeString(%q) err = %v, want substring %q", c.s, err, c.wantErrSub)
			}
			continue
		}
		if err != nil || h != c.wantH || m != c.wantM {
			t.Errorf("parseTimeString(%q) = (%d,%d,%v), want (%d,%d,nil)", c.s, h, m, err, c.wantH, c.wantM)
		}
	}
}

// optMinute returns 0 for nil/None, errors on a non-int, and errors when the int
// overflows int64.
func TestOptMinute(t *testing.T) {
	if m, err := optMinute(nil); err != nil || m != 0 {
		t.Errorf("optMinute(nil) = (%d,%v), want (0,nil)", m, err)
	}
	if m, err := optMinute(starlark.None); err != nil || m != 0 {
		t.Errorf("optMinute(None) = (%d,%v), want (0,nil)", m, err)
	}
	if m, err := optMinute(starlark.MakeInt(30)); err != nil || m != 30 {
		t.Errorf("optMinute(30) = (%d,%v), want (30,nil)", m, err)
	}
	if _, err := optMinute(starlark.String("x")); err == nil || !strings.Contains(err.Error(), "minute must be an int") {
		t.Errorf("optMinute(string) err = %v, want type error", err)
	}
	huge := new(big.Int).Lsh(big.NewInt(1), 100) // 2^100, far beyond int64
	if _, err := optMinute(starlark.MakeBigInt(huge)); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Errorf("optMinute(huge) err = %v, want too-large error", err)
	}
}

// parseTimeValue dispatches on the value type: int (with optional minute), string
// (delegates to parseTimeString), and rejects everything else; the int branch
// must report an overflow rather than silently truncating.
func TestParseTimeValue(t *testing.T) {
	if h, m, err := parseTimeValue(starlark.MakeInt(15), starlark.MakeInt(30)); err != nil || h != 15 || m != 30 {
		t.Errorf("parseTimeValue(15, 30) = (%d,%d,%v), want (15,30,nil)", h, m, err)
	}
	if h, m, err := parseTimeValue(starlark.String("8:45"), starlark.None); err != nil || h != 8 || m != 45 {
		t.Errorf("parseTimeValue(\"8:45\") = (%d,%d,%v), want (8,45,nil)", h, m, err)
	}
	huge := new(big.Int).Lsh(big.NewInt(1), 100)
	if _, _, err := parseTimeValue(starlark.MakeBigInt(huge), starlark.None); err == nil || !strings.Contains(err.Error(), "hour is too large") {
		t.Errorf("parseTimeValue(huge) err = %v, want hour-too-large", err)
	}
	if _, _, err := parseTimeValue(starlark.Float(3.5), starlark.None); err == nil || !strings.Contains(err.Error(), "must be an int or a string") {
		t.Errorf("parseTimeValue(float) err = %v, want type error", err)
	}
	// An int hour with a bad minute arg surfaces optMinute's error.
	if _, _, err := parseTimeValue(starlark.MakeInt(3), starlark.String("x")); err == nil || !strings.Contains(err.Error(), "minute must be an int") {
		t.Errorf("parseTimeValue(3, \"x\") err = %v, want minute type error", err)
	}
}

// numericText renders ints, floats, and strings as decimal text and rejects
// other types.
func TestNumericText(t *testing.T) {
	cases := []struct {
		v          starlark.Value
		want       string
		wantErrSub string
	}{
		{starlark.MakeInt(42), "42", ""},
		{starlark.MakeInt(-7), "-7", ""},
		{starlark.Float(3.5), "3.5", ""},
		{starlark.Float(10), "10", ""}, // FormatFloat with -1 prec drops the .0
		{starlark.String("0099"), "0099", ""},
		{starlark.None, "", "must be an int, float, or string"},
		{starlark.NewList(nil), "", "must be an int, float, or string"},
	}
	for _, c := range cases {
		got, err := numericText(c.v)
		if c.wantErrSub != "" {
			if err == nil || !strings.Contains(err.Error(), c.wantErrSub) {
				t.Errorf("numericText(%v) err = %v, want substring %q", c.v, err, c.wantErrSub)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("numericText(%v) = (%q,%v), want (%q,nil)", c.v, got, err, c.want)
		}
	}
}

// --- symbol ------------------------------------------------------------------

func TestSymbols(t *testing.T) {
	cases := []struct{ in, want string }{
		{"!", "❗"},
		{"?", "❓"},
		{"!?", "❗❓"},
		{"#", "#️⃣"},
		{"*", "*️⃣"},
		{"+", "➕"},
		{"-", "➖"},
		{"/", "➗"},
		{"hi!", "hi❗"}, // letters pass through
	}
	for _, c := range cases {
		if got := symbolToEmojiStr(c.in); got != c.want {
			t.Errorf("symbolToEmojiStr(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
