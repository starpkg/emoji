// Command gen builds tables_gen.go from the vendored emoji data sources.
//
// It merges two pinned datasets from different language ecosystems into a single
// Go shortcode<->emoji table:
//
//	carpedm20/emoji v2.15.0 (Python, BSD-3-Clause) — the freshest spine (Emoji 17.0)
//	github/gemoji   v4.1.0  (Ruby,   MIT)          — GitHub's canonical :shortcodes:
//
// gemoji is consulted first so GitHub's well-known short aliases (:smile:, :+1:)
// win the forward map and provide the preferred reverse alias and human name;
// carpedm20 then fills in everything gemoji lacks, including the newest emoji.
//
// Run via `go generate ./...` or `go run ./internal/gen`. Output is fully
// deterministic (sorted keys, ASCII-escaped, no timestamps), so re-running only
// changes tables_gen.go when the underlying source data changes — which is what
// makes data refresh a reviewable diff.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
)

const (
	primarySource   = "carpedm20/emoji v2.15.0"
	secondarySource = "github/gemoji v4.1.0"
	emojiVersion    = "17.0"
)

// carpedmEntry is one value in carpedm20's emoji.json (keyed by glyph).
type carpedmEntry struct {
	En     string   `json:"en"`
	Alias  []string `json:"alias"`
	Status int      `json:"status"` // 1 component, 2 fully-qualified, 3 minimally, 4 unqualified
	E      float64  `json:"E"`      // emoji version (may be fractional, e.g. 0.6)
}

// gemojiEntry is one element of gemoji's db/emoji.json array.
type gemojiEntry struct {
	Emoji       string   `json:"emoji"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Aliases     []string `json:"aliases"` // bare shortcodes, no colons
	Tags        []string `json:"tags"`
}

func main() {
	carpedmPath := flag.String("carpedm", "data/sources/carpedm20-emoji-v2.15.0.json", "path to carpedm20/emoji emoji.json")
	gemojiPath := flag.String("gemoji", "data/sources/gemoji-v4.1.0.json", "path to github/gemoji db/emoji.json")
	out := flag.String("out", "tables_gen.go", "output Go file")
	flag.Parse()

	carpedm := map[string]carpedmEntry{}
	mustReadJSON(*carpedmPath, &carpedm)
	var gemoji []gemojiEntry
	mustReadJSON(*gemojiPath, &gemoji)

	forward, reverse, names, collisions := build(carpedm, gemoji)
	writeGo(*out, forward, reverse, names, collisions)
}

// build merges the two source datasets into the forward (shortcode->glyph),
// reverse (glyph->primary shortcode), and names (glyph->human name) maps. gemoji
// is applied first so its canonical short aliases win; carpedm20 then fills the
// gaps, including the newest emoji gemoji never shipped.
func build(carpedm map[string]carpedmEntry, gemoji []gemojiEntry) (forward, reverse, names map[string]string, collisions int) {
	forward = map[string]string{} // bare alias -> glyph (first writer wins)
	reverse = map[string]string{} // glyph -> primary bare alias
	names = map[string]string{}   // glyph -> human-readable name

	addForward := func(alias, glyph string) {
		alias = bare(alias)
		if alias == "" || glyph == "" {
			return
		}
		if existing, ok := forward[alias]; ok {
			if existing != glyph {
				collisions++
			}
			return
		}
		forward[alias] = glyph
	}

	// 1) gemoji first: GitHub canonical shortcodes win the forward map and set
	//    the preferred (short) reverse alias and human name.
	for _, g := range gemoji {
		if g.Emoji == "" {
			continue
		}
		for _, a := range g.Aliases {
			addForward(a, g.Emoji)
		}
		if len(g.Aliases) > 0 {
			if _, ok := reverse[g.Emoji]; !ok {
				reverse[g.Emoji] = bare(g.Aliases[0])
			}
		}
		if g.Description != "" {
			if _, ok := names[g.Emoji]; !ok {
				names[g.Emoji] = g.Description
			}
		}
	}

	// 2) carpedm20 next, fully-qualified entries before the rest (see
	//    sortedCarpedm), so aliases resolve to the canonical glyph and the
	//    freshest emoji that gemoji never shipped still get covered.
	for _, c := range sortedCarpedm(carpedm) {
		addForward(c.entry.En, c.glyph)
		for _, a := range c.entry.Alias {
			addForward(a, c.glyph)
		}
		if _, ok := reverse[c.glyph]; !ok {
			reverse[c.glyph] = bare(c.entry.En)
		}
		if _, ok := names[c.glyph]; !ok {
			names[c.glyph] = humanize(c.entry.En)
		}
	}
	return forward, reverse, names, collisions
}

type carpedmGlyph struct {
	glyph string
	entry carpedmEntry
}

// sortedCarpedm orders carpedm entries so fully-qualified glyphs are processed
// first (and ties broken by glyph) for deterministic, canonical resolution.
func sortedCarpedm(m map[string]carpedmEntry) []carpedmGlyph {
	out := make([]carpedmGlyph, 0, len(m))
	for g, e := range m {
		out = append(out, carpedmGlyph{g, e})
	}
	rank := func(status int) int {
		switch status {
		case 2: // fully-qualified
			return 0
		case 1: // component
			return 1
		case 3: // minimally-qualified
			return 2
		case 4: // unqualified
			return 3
		default:
			return 4
		}
	}
	sort.Slice(out, func(i, j int) bool {
		ri, rj := rank(out[i].entry.Status), rank(out[j].entry.Status)
		if ri != rj {
			return ri < rj
		}
		return out[i].glyph < out[j].glyph
	})
	return out
}

// bare strips the surrounding colons from a ":shortcode:" form.
func bare(s string) string { return strings.Trim(s, ":") }

// humanize turns ":1st_place_medal:" into "1st place medal".
func humanize(en string) string {
	return strings.ReplaceAll(bare(en), "_", " ")
}

func mustReadJSON(path string, v interface{}) {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		log.Fatalf("parse %s: %v", path, err)
	}
}

func writeGo(path string, forward, reverse, names map[string]string, collisions int) {
	var b strings.Builder
	b.WriteString("// Code generated by internal/gen; DO NOT EDIT.\n//\n")
	b.WriteString("// Source datasets (pinned, vendored under data/sources/):\n")
	fmt.Fprintf(&b, "//   %s (Python, BSD-3-Clause) - emoji.json\n", primarySource)
	fmt.Fprintf(&b, "//   %s (Ruby, MIT)            - db/emoji.json\n", secondarySource)
	b.WriteString("//\n// Regenerate with: go generate ./...\n\n")
	b.WriteString("package emoji\n\n")

	b.WriteString("// Data provenance, surfaced through the module's info() builtin.\n")
	b.WriteString("const (\n")
	fmt.Fprintf(&b, "\tdataSourcePrimary   = %s\n", strconv.Quote(primarySource))
	fmt.Fprintf(&b, "\tdataSourceSecondary = %s\n", strconv.Quote(secondarySource))
	fmt.Fprintf(&b, "\tdataEmojiVersion    = %s\n", strconv.Quote(emojiVersion))
	b.WriteString(")\n\n")

	writeMap(&b, "dataForward", "maps a bare shortcode (no colons) to its emoji glyph.", forward)
	writeMap(&b, "dataReverse", "maps an emoji glyph to its primary bare shortcode.", reverse)
	writeMap(&b, "dataNames", "maps an emoji glyph to a human-readable name.", names)

	src, err := format.Source([]byte(b.String()))
	if err != nil {
		log.Fatalf("format generated source: %v", err)
	}
	if err := os.WriteFile(path, src, 0o644); err != nil {
		log.Fatalf("write %s: %v", path, err)
	}
	fmt.Printf("wrote %s: %d forward, %d reverse, %d names (%d alias collisions skipped)\n",
		path, len(forward), len(reverse), len(names), collisions)
}

func writeMap(b *strings.Builder, name, doc string, m map[string]string) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Fprintf(b, "// %s %s\n", name, doc)
	fmt.Fprintf(b, "var %s = map[string]string{\n", name)
	for _, k := range keys {
		fmt.Fprintf(b, "\t%s: %s,\n", strconv.QuoteToASCII(k), strconv.QuoteToASCII(m[k]))
	}
	b.WriteString("}\n\n")
}
