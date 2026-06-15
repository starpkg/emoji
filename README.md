# 😀 `emoji` — Emoji ⇄ text for Starlark

[![Go Reference](https://pkg.go.dev/badge/github.com/starpkg/emoji.svg)](https://pkg.go.dev/github.com/starpkg/emoji)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![codecov](https://codecov.io/gh/starpkg/emoji/graph/badge.svg)](https://codecov.io/gh/starpkg/emoji)
![binary footprint](https://img.shields.io/badge/binary_footprint-%2B0.9_MB-blue)

`emoji` is an **L4 domain module** of the Star\* ecosystem. The ecosystem's
remit is *support for necessary **local** operations plus simple abstractions
over common **online** services, for ease of use* — and `emoji` sits squarely on
the **local** side: it is a pure, offline text utility. There is no network, no
service, and no credentials; at run time the module reads only embedded Go maps
and does pure Unicode arithmetic, so it has **zero third-party data dependencies**.

## Overview

It converts between text and emoji two ways:

- **Shortcodes** — `:rocket:` ⇄ 🚀, driven by a generated data table that merges
  several actively-maintained upstream datasets (see [Data](#data)).
- **Look-alikes** — turn a plain number, clock time, letter, or punctuation mark
  into the most similar emoji: `42` → 4️⃣2️⃣, `3:30` → 🕞, `AB` → 🇦🇧, `!?` → ❗❓.

For the complete per-builtin reference — signatures, parameters, returns,
errors, examples — and the configuration accessors, see
**[docs/API.md](docs/API.md)**.

## Installation

```bash
go get github.com/starpkg/emoji
```

## Quick start

Wire the module into a Starlet interpreter, then `load("emoji", …)` from a
script:

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

From Starlark:

```python
load("emoji", "emojize", "demojize", "convert", "number_to_emoji", "time_to_emoji")

emojize("i :heart: starlark :rocket:")   # i ❤️ starlark 🚀
demojize("i ❤️ starlark 🚀")              # i :heart: starlark :rocket:

number_to_emoji(2026)                     # 2️⃣0️⃣2️⃣6️⃣
time_to_emoji("9:15")                     # 🕤  (rounded to 9:30)
convert("AB", kind="letter")              # 🇦🇧
convert(":fire:")                         # 🔥  (auto-detected emojize)
```

## Starlark API at a glance

Top-level builtins (`load("emoji", …)`). Shortcode group (backed by the data
table):

- `emojize(text)` — replace every known `:shortcode:` with its emoji glyph.
- `demojize(text, delimiters?)` — the inverse; replace glyphs with `:shortcodes:`.
- `get(name)` — emoji glyph for a single shortcode, or `None`.
- `name(emoji)` — canonical shortcode for a single glyph, or `None`.
- `describe(emoji)` — human-readable name for a single glyph, or `None`.

Look-alike group (pure Unicode arithmetic):

- `number_to_emoji(value, keycap_ten?)` — digits → keycap emoji.
- `emoji_to_number(text)` — the inverse; keycap emoji → digits.
- `time_to_emoji(value, minute?)` — a time → the nearest clock-face emoji.
- `letter_to_emoji(text, style?)` — letters → regional / squared / circled emoji.
- `symbol_to_emoji(text)` — punctuation `! ? # * + - / × ÷` → symbol emoji.

Dispatcher and metadata:

- `convert(value, kind?)` — dispatch to a conversion family (default `kind="auto"`).
- `info()` — a dict describing the embedded dataset.

See **[docs/API.md](docs/API.md)** for the full signatures, return values,
errors, and examples of every builtin above.

## Configuration

The module's single option, `max_input_bytes`, bounds the input size of text
conversions. It is configured via the `EMOJI_MAX_INPUT_BYTES` environment
variable or the generated `get_max_input_bytes` / `set_max_input_bytes` accessor
builtins. See the
[Configuration section of docs/API.md](docs/API.md#configuration) for the full
option table, default, and accessors.

## Data

The shortcode table is **not** a runtime dependency on any single (and possibly
stale) emoji library. It is generated, offline, by merging pinned datasets from
different language ecosystems into one Go table:

| Source | Ecosystem | Pinned | Role |
|--------|-----------|--------|------|
| [carpedm20/emoji](https://github.com/carpedm20/emoji) | Python | `v2.15.0` | Spine: the freshest, fullest shortcode set (Emoji 17.0), aliases, names. |
| [github/gemoji](https://github.com/github/gemoji) | Ruby | `v4.1.0` | GitHub's canonical `:shortcodes:` (`:smile:`, `:+1:`) + tidy descriptions. |

`internal/gen` reads the vendored JSON, applies gemoji first (so its well-known
short aliases win) then carpedm20 (which fills the gaps and the newest emoji),
and writes `tables_gen.go`. Output is deterministic — sorted, ASCII-escaped, no
timestamps — so **refreshing the data is a reviewable diff**, not a black box.
See [`data/SOURCES.md`](data/SOURCES.md) for provenance and licenses.

## License

This project is licensed under the MIT License — see [LICENSE](LICENSE).

The vendored datasets keep their upstream licenses (carpedm20/emoji: BSD-3-Clause;
github/gemoji: MIT). Only text data (names, shortcodes, code points) is used — no
image assets. Attribution and license texts are in
[`data/SOURCES.md`](data/SOURCES.md).
