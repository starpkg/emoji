package emoji

// Tests for the look-alike conversions (convert.go).
//
// Sections:
//   - number : keycap digits, signs, keycap-ten, round-trip
//   - time   : the full 24-face table, minute rounding, 24h, parsing, errors
//   - letter : regional / squared / circled styles + pass-through
//   - symbol : the fixed punctuation table + pass-through

import (
	"testing"
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
