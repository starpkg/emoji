# Emoji data sources

The shortcode ⇄ emoji table in `tables_gen.go` is generated offline by
`internal/gen` from the pinned datasets vendored here. Nothing in this directory
is read at run time — it exists so the generated table is reproducible and so
refreshing the data is a reviewable diff.

## Pinned sources

| File | Upstream | Ecosystem | Version | License |
|------|----------|-----------|---------|---------|
| `carpedm20-emoji-v2.15.0.json` | [carpedm20/emoji](https://github.com/carpedm20/emoji) `emoji/unicode_codes/emoji.json` | Python | `v2.15.0` (Emoji 17.0) | BSD-3-Clause |
| `gemoji-v4.1.0.json` | [github/gemoji](https://github.com/github/gemoji) `db/emoji.json` | Ruby | `v4.1.0` | MIT |

Upstream license texts are kept alongside the data as
`carpedm20-emoji-LICENSE.txt` and `gemoji-LICENSE.txt`. Only text data (names,
shortcodes, code points) is used — no image assets — so there is no icon-artwork
licensing entanglement.

### Schemas

- **carpedm20/emoji** — a JSON object keyed by emoji glyph; each value has
  `en` (`":shortcode:"`), optional `alias` (`[]`), `status`
  (1 component / 2 fully-qualified / 3 minimally / 4 unqualified), `E` (emoji
  version, may be fractional), optional `variant` (bool).
- **github/gemoji** — a JSON array; each element has `emoji`, `description`,
  `category`, `aliases` (`[]`, bare shortcodes), `tags`, `unicode_version`.

## How the merge works

`internal/gen` applies the two sources in this order:

1. **gemoji first** — GitHub's canonical short aliases (`:smile:`, `:+1:`) win
   the forward map, and provide the preferred reverse alias and human name.
2. **carpedm20 next**, fully-qualified glyphs before the rest — fills every gap,
   including the newest emoji gemoji never shipped, mapping each alias to the
   canonical (fully-qualified) glyph.

The result is three maps: `dataForward` (shortcode → glyph), `dataReverse`
(glyph → primary shortcode), and `dataNames` (glyph → human name). Output is
sorted and ASCII-escaped with no timestamps, so regenerating only changes the
file when the source data changes.

## Refreshing

```bash
./data/fetch.sh      # re-download the pinned files (edit versions there to bump)
go generate ./...    # regenerate tables_gen.go
go test ./...        # verify
git diff             # review the data delta
```

To bump a source to a newer release: change its tag in `data/fetch.sh` and the
matching provenance constant in `internal/gen/main.go`, then run the steps above.

To add a new source (e.g. a third ecosystem): vendor its file here, record it in
this table, and teach `internal/gen` one more parser that contributes to the
`build()` merge.
