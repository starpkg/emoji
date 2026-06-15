package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBareAndHumanize(t *testing.T) {
	if got := bare(":rocket:"); got != "rocket" {
		t.Errorf("bare = %q", got)
	}
	if got := bare("plain"); got != "plain" {
		t.Errorf("bare(no colons) = %q", got)
	}
	if got := humanize(":1st_place_medal:"); got != "1st place medal" {
		t.Errorf("humanize = %q", got)
	}
}

func TestBuildMerge(t *testing.T) {
	carpedm := map[string]carpedmEntry{
		// fully-qualified rocket with an extra alias and a fresh emoji.
		"\U0001F680": {En: ":rocket:", Status: 2, E: 0.6},
		"\U0001FAE8": {En: ":shaking_face:", Status: 2, E: 15.1, Alias: []string{":shaking:"}},
		// an unqualified duplicate that must NOT win the canonical glyph.
		"A":          {En: ":letter_a:", Status: 4},
		"\U0001F170": {En: ":a_button:", Status: 2},
	}
	gemoji := []gemojiEntry{
		{Emoji: "\U0001F680", Description: "rocket", Aliases: []string{"rocket", "ship"}},
		{Emoji: ""}, // empty glyph is skipped
	}

	forward, reverse, names, _ := build(carpedm, gemoji)

	// gemoji aliases win and are present.
	if forward["rocket"] != "\U0001F680" || forward["ship"] != "\U0001F680" {
		t.Errorf("gemoji aliases missing: %v / %v", forward["rocket"], forward["ship"])
	}
	// carpedm-only fresh emoji is covered via its en + alias.
	if forward["shaking_face"] != "\U0001FAE8" || forward["shaking"] != "\U0001FAE8" {
		t.Errorf("carpedm fresh emoji missing: %v", forward["shaking_face"])
	}
	// reverse prefers gemoji's first (short) alias.
	if reverse["\U0001F680"] != "rocket" {
		t.Errorf("reverse rocket = %q, want rocket", reverse["\U0001F680"])
	}
	// gemoji description wins for the name.
	if names["\U0001F680"] != "rocket" {
		t.Errorf("name rocket = %q", names["\U0001F680"])
	}
	// carpedm-only name is humanized from its en.
	if names["\U0001FAE8"] != "shaking face" {
		t.Errorf("name shaking = %q", names["\U0001FAE8"])
	}
}

func TestSortedCarpedmFullyQualifiedFirst(t *testing.T) {
	m := map[string]carpedmEntry{
		"u": {En: ":u:", Status: 4}, // unqualified
		"q": {En: ":q:", Status: 2}, // fully-qualified
		"c": {En: ":c:", Status: 1}, // component
	}
	got := sortedCarpedm(m)
	if got[0].entry.Status != 2 {
		t.Errorf("fully-qualified should sort first, got status %d", got[0].entry.Status)
	}
}

// The full status ordering must be fully-qualified (2) < component (1) <
// minimally-qualified (3) < unqualified (4) < unknown, with glyph as the tie
// breaker, so the generated table is deterministic regardless of map order.
func TestSortedCarpedmFullOrderAndTies(t *testing.T) {
	m := map[string]carpedmEntry{
		"z2": {En: ":z2:", Status: 2}, // fully-qualified, later glyph
		"a2": {En: ":a2:", Status: 2}, // fully-qualified, earlier glyph
		"c1": {En: ":c1:", Status: 1}, // component
		"m3": {En: ":m3:", Status: 3}, // minimally-qualified
		"u4": {En: ":u4:", Status: 4}, // unqualified
		"x9": {En: ":x9:", Status: 9}, // unknown status -> ranks last
	}
	got := sortedCarpedm(m)
	wantGlyphOrder := []string{"a2", "z2", "c1", "m3", "u4", "x9"}
	if len(got) != len(wantGlyphOrder) {
		t.Fatalf("sortedCarpedm len = %d, want %d", len(got), len(wantGlyphOrder))
	}
	for i, g := range wantGlyphOrder {
		if got[i].glyph != g {
			t.Errorf("sortedCarpedm[%d].glyph = %q, want %q (full order: %v)", i, got[i].glyph, g, wantGlyphOrder)
		}
	}
}

// build must count (and skip) conflicting forward aliases: when two distinct
// glyphs claim the same bare alias, the first writer wins and the conflict is
// tallied as a collision; an identical re-assignment is not a collision.
func TestBuildCollisions(t *testing.T) {
	gemoji := []gemojiEntry{
		{Emoji: "\U0001F600", Aliases: []string{"dup", "grinning"}},
	}
	carpedm := map[string]carpedmEntry{
		// "dup" already maps to U+1F600 via gemoji; a different glyph claiming it
		// is a collision and must be skipped (first writer wins).
		"\U0001F601": {En: ":dup:", Status: 2},
		// re-stating the same (alias -> same glyph) is NOT a collision.
		"\U0001F600": {En: ":dup:", Status: 2},
	}
	forward, _, _, collisions := build(carpedm, gemoji)
	if forward["dup"] != "\U0001F600" {
		t.Errorf("first writer must win: forward[dup] = %q, want U+1F600", forward["dup"])
	}
	if collisions != 1 {
		t.Errorf("collisions = %d, want exactly 1 (the conflicting glyph)", collisions)
	}
}

// An empty alias or empty glyph must be ignored by addForward (the early return),
// and a carpedm entry whose en/alias is just colons reduces to an empty alias.
func TestBuildSkipsEmpty(t *testing.T) {
	gemoji := []gemojiEntry{
		{Emoji: "\U0001F680", Aliases: []string{"", "rocket"}}, // "" alias skipped
	}
	carpedm := map[string]carpedmEntry{
		"\U0001F4A9": {En: "::", Status: 2}, // bare("::") == "" -> no forward entry
	}
	forward, reverse, names, _ := build(carpedm, gemoji)
	if _, ok := forward[""]; ok {
		t.Error("empty alias must not be added to the forward map")
	}
	if forward["rocket"] != "\U0001F680" {
		t.Errorf("non-empty alias still added: forward[rocket] = %q", forward["rocket"])
	}
	// The poo glyph still gets a reverse/name entry from its (empty) en humanized.
	if reverse["\U0001F4A9"] != "" {
		t.Errorf("reverse for empty-en glyph = %q, want empty bare alias", reverse["\U0001F4A9"])
	}
	if names["\U0001F4A9"] != "" {
		t.Errorf("name for empty-en glyph = %q, want empty humanized name", names["\U0001F4A9"])
	}
}

func TestMustReadJSONAndWriteGo(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "in.json")
	if err := os.WriteFile(src, []byte(`{"🚀":{"en":":rocket:","status":2,"E":0.6}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	parsed := map[string]carpedmEntry{}
	mustReadJSON(src, &parsed)
	if parsed["\U0001F680"].En != ":rocket:" {
		t.Fatalf("parsed wrong: %+v", parsed)
	}

	out := filepath.Join(dir, "tables_gen.go")
	writeGo(out, map[string]string{"rocket": "\U0001F680"},
		map[string]string{"\U0001F680": "rocket"},
		map[string]string{"\U0001F680": "rocket"}, 0)
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "package emoji") ||
		!strings.Contains(text, "var dataForward") ||
		!strings.Contains(text, `"rocket":`) {
		t.Errorf("generated file missing expected content:\n%s", text)
	}
	// Output must be valid, gofmt-clean Go (writeGo runs format.Source).
	if !strings.Contains(text, "DO NOT EDIT") {
		t.Error("missing generated-code banner")
	}
}
