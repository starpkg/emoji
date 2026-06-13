# 😀 `emoji` — Emoji ⇄ text for Starlark

[![Go Reference](https://pkg.go.dev/badge/github.com/starpkg/emoji.svg)](https://pkg.go.dev/github.com/starpkg/emoji)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

Convert between text and emoji from Starlark:

- **Shortcodes** — `:rocket:` ⇄ 🚀, driven by a generated data table that merges
  several actively-maintained upstream datasets (see [Data](#data--the-conversion-pipeline)).
- **Look-alikes** — turn a plain number, clock time, letter, or punctuation mark
  into the most similar emoji: `42` → 4️⃣2️⃣, `3:30` → 🕞, `AB` → 🇦🇧, `!?` → ❗❓.

At run time the module reads only embedded Go maps and does pure Unicode
arithmetic — **zero third-party data dependencies**.

## Installation

```bash
go get github.com/starpkg/emoji
```

## Functions

| Function | Signature | Description |
|----------|-----------|-------------|
| `emojize` | `emojize(text) -> str` | Replace every `:shortcode:` with its emoji. Unknown codes are left untouched; `:flag-xx:` becomes a regional-indicator flag. |
| `demojize` | `demojize(text, delimiters=(":", ":")) -> str` | Replace every emoji with `:shortcode:` (longest-match). |
| `get` | `get(name) -> str \| None` | The emoji for one shortcode (`get("rocket")` → 🚀). Colons optional. |
| `name` | `name(emoji) -> str \| None` | The primary shortcode for one emoji (`name("🚀")` → `rocket`). |
| `describe` | `describe(emoji) -> str \| None` | The human-readable name (`describe("🚀")` → `rocket`). |
| `number_to_emoji` | `number_to_emoji(value, keycap_ten=False) -> str` | Digits → keycap emoji (`42` → 4️⃣2️⃣). `keycap_ten=True` maps a lone `10` to 🔟. |
| `emoji_to_number` | `emoji_to_number(text) -> str` | Keycap emoji → digits (inverse of `number_to_emoji`). |
| `time_to_emoji` | `time_to_emoji(value, minute=None) -> str` | A clock time → the nearest clock-face emoji (`"3:30"` or `15, 30` → 🕞). |
| `letter_to_emoji` | `letter_to_emoji(text, style="regional") -> str` | Letters → emoji. `style`: `regional` (🇦), `squared` (🅰️), `circled` (Ⓐ). |
| `symbol_to_emoji` | `symbol_to_emoji(text) -> str` | Punctuation → emoji (`!?` → ❗❓, `#` → #️⃣). |
| `convert` | `convert(value, kind="auto") -> str` | One entry point that dispatches by `kind`: `auto`, `emojize`, `demojize`, `number`, `time`, `letter`, `symbol`. `auto` treats numbers as `number`, `"H:MM"` strings as `time`, and other strings as `emojize`. |
| `info` | `info() -> dict` | Data provenance: source datasets, emoji version, and entry counts. |

## Usage

### In Go

```go
package main

import (
	"fmt"

	"github.com/1set/starlet"
	"github.com/starpkg/emoji"
)

func main() {
	mod := emoji.NewModule()
	interpreter := starlet.NewWithLoaders(nil, nil, starlet.ModuleLoaderMap{
		"emoji": mod.LoadModule(),
	})
	script := `
load("emoji", "emojize")
out = emojize("ship it :rocket::tada:")
`
	if _, err := interpreter.RunScript([]byte(script), nil); err != nil {
		fmt.Println(err)
	}
}
```

### In Starlark

```python
load("emoji", "emojize", "demojize", "convert", "number_to_emoji", "time_to_emoji")

emojize("i :heart: starlark :rocket:")   # i ❤️ starlark 🚀
demojize("i ❤️ starlark 🚀")              # i :heart: starlark :rocket:

number_to_emoji(2026)                     # 2️⃣0️⃣2️⃣6️⃣
time_to_emoji("9:15")                     # 🕤  (rounded to 9:30)
convert("AB", kind="letter")              # 🇦🇧
convert(":fire:")                         # 🔥  (auto-detected emojize)
```

## Data & the conversion pipeline

The shortcode table is **not** a runtime dependency on any single (and possibly
stale) emoji library. It is generated, offline, by merging pinned datasets from
different language ecosystems into one Go table:

| Source | Ecosystem | Pinned | Role |
|--------|-----------|--------|------|
| [carpedm20/emoji](https://github.com/carpedm20/emoji) | Python | `v2.15.0` | Spine: the freshest, fullest shortcode set (Emoji 17.0), aliases, names. |
| [github/gemoji](https://github.com/github/gemoji) | Ruby | `v4.1.0` | GitHub's canonical `:shortcodes:` (`:smile:`, `:+1:`) + tidy descriptions. |

```
data/sources/*.json   →   internal/gen   →   tables_gen.go   →   module
  (vendored, pinned)      (merge, dedupe)     (generated Go)      (runtime)
```

`internal/gen` reads the vendored JSON, applies gemoji first (so its well-known
short aliases win) then carpedm20 (which fills the gaps and the newest emoji),
and writes `tables_gen.go`. Output is deterministic — sorted, ASCII-escaped, no
timestamps — so **refreshing the data is a reviewable diff**, not a black box.

Refreshing:

```bash
./data/fetch.sh      # pull the pinned source files into data/sources/
go generate ./...    # rebuild tables_gen.go from them
go test ./...        # verify
```

To bump to a newer release, edit the version in `data/fetch.sh` (and the
provenance constants in `internal/gen`), re-run the two commands, and review the
diff. New sources can be added by teaching `internal/gen` one more parser. See
[`data/SOURCES.md`](data/SOURCES.md) for provenance and licenses.

## Configuration

Text conversions bound their input before processing it.

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `max_input_bytes` | `int` | `5242880` | Maximum input size in bytes (5 MiB); `0` disables the cap. |

Settable from Starlark via `set_max_input_bytes(n)` or from the environment via
`EMOJI_MAX_INPUT_BYTES`.

## License

This project is licensed under the MIT License — see [LICENSE](LICENSE).

The vendored datasets keep their upstream licenses (carpedm20/emoji: BSD-3-Clause;
github/gemoji: MIT). Only text data (names, shortcodes, code points) is used — no
image assets. Attribution and license texts are in
[`data/SOURCES.md`](data/SOURCES.md).
```
