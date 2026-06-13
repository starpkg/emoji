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
