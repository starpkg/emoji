# 😀 `emoji` — Emoji ⇄ text for Starlark

[![Go Reference](https://pkg.go.dev/badge/github.com/starpkg/emoji.svg)](https://pkg.go.dev/github.com/starpkg/emoji)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

`emoji` is an **L4 domain module** of the Star\* ecosystem. The ecosystem's
remit is *support for necessary **local** operations plus simple abstractions
over common **online** services, for ease of use* — and `emoji` sits squarely on
the **local** side: it is a pure, offline text utility. There is no network, no
service, and no credentials; at run time the module reads only embedded Go maps
and does pure Unicode arithmetic, so it has **zero third-party data dependencies**.

It converts between text and emoji two ways:

- **Shortcodes** — `:rocket:` ⇄ 🚀, driven by a generated data table that merges
  several actively-maintained upstream datasets (see [Data](#data--the-conversion-pipeline)).
- **Look-alikes** — turn a plain number, clock time, letter, or punctuation mark
  into the most similar emoji: `42` → 4️⃣2️⃣, `3:30` → 🕞, `AB` → 🇦🇧, `!?` → ❗❓.

## Installation

```bash
go get github.com/starpkg/emoji
```

## Quick start

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

## Starlark API

The module exposes twelve script-facing builtins. They split into a
shortcode group (backed by the data table) and a look-alike group (pure Unicode
arithmetic). All are pure and deterministic; every text-accepting builtin bounds
its input by `max_input_bytes` (see [Host configuration](#host-configuration)).

### Shortcode ⇄ emoji

#### `emojize(text)`

Replaces every known `:shortcode:` in `text` with its emoji glyph and returns
the result string. Unknown tokens are left untouched; a `:flag-xx:` /
`:flag_xx:` token whose two-letter code is not in the table falls back to the
corresponding regional-indicator flag sequence. Accepts a string or bytes.

```python
emojize("ship it :rocket: :no_such_code:")   # ship it 🚀 :no_such_code:
emojize(":flag-fr:")                          # 🇫🇷
```

#### `demojize(text, delimiters=(":", ":"))`

The inverse of `emojize`: replaces every emoji glyph in `text` with its
`:shortcode:`, returning the result string. The scan is longest-match, so
multi-rune sequences (variation selectors, ZWJ joins) win. `delimiters` is an
optional `(open, close)` pair of strings (a tuple or a list); it defaults to
`(":", ":")`. Accepts a string or bytes.

```python
demojize("i ❤️ 🚀")                            # i :heart: :rocket:
demojize("hi 🚀", delimiters=("[", "]"))      # hi [rocket]
```

#### `get(name)`

Returns the emoji glyph for a single shortcode, or `None` if the shortcode is
unknown. Surrounding colons are optional, so `get("rocket")` and
`get(":rocket:")` are equivalent. A `flag-xx` code falls back to a
regional-indicator flag.

```python
get("rocket")    # 🚀
get(":rocket:")  # 🚀
get("nope")      # None
```

#### `name(emoji)`

Returns the primary (canonical) shortcode for a single emoji glyph, without
colons, or `None` if the glyph is not in the table.

```python
name("🚀")   # "rocket"
```

#### `describe(emoji)`

Returns the human-readable name for a single emoji glyph, or `None` if the glyph
is not in the table. The name comes from the source datasets (GitHub's
description where available, otherwise the de-underscored canonical shortcode).

```python
describe("🚀")   # "rocket"
```

### Look-alike conversions

#### `number_to_emoji(value, keycap_ten=False)`

Maps each ASCII digit in `value` to its keycap emoji and returns the result
string; `-` and `+` become the heavy minus/plus dingbats, and any other
character passes through unchanged. `value` may be an int, a float, or a string.
When `keycap_ten=True` and the whole input is exactly `10`, the dedicated
🔟 (KEYCAP TEN) glyph is emitted instead of two keycaps.

```python
number_to_emoji(42)                 # 4️⃣2️⃣
number_to_emoji(10, keycap_ten=True)  # 🔟
number_to_emoji(3.5)                # 3️⃣.5️⃣
```

#### `emoji_to_number(text)`

The inverse of `number_to_emoji`: keycap sequences become their digit,
🔟 becomes `10`, the heavy plus/minus become `+`/`-`, and every other rune
passes through unchanged. Returns the result string. Accepts a string or bytes.

```python
emoji_to_number("4️⃣2️⃣")   # "42"
```

#### `time_to_emoji(value, minute=None)`

Returns the single clock-face emoji nearest to a time. `value` is either an int
hour (with the optional `minute` int) or a `"H"` / `"H:MM"` string. Only `:00`
and `:30` faces exist, so the minute is rounded to the nearest half hour
(round-half-up: `:15` → `:30`, `:45` → next hour). The hour is taken mod 12, so
AM and PM share a face. An out-of-range hour (`0`–`23`) or minute (`0`–`59`)
is an error.

```python
time_to_emoji("3:30")        # 🕞
time_to_emoji(15, minute=30) # 🕞  (15:30 shares the 3:30 face)
```

#### `letter_to_emoji(text, style="regional")`

Maps Latin letters in `text` to emoji in one of three styles, returning the
result string; runes with no mapping in the chosen style pass through unchanged.

- `regional` (default): `A`–`Z` → regional-indicator symbols (`🇦`). Two adjacent
  indicators that form a valid country code render as that flag.
- `squared` (alias `button`): `A`–`Z` → negative-squared latin capitals. Only
  `A`/`B`/`O`/`P` are colour emoji (🅰️); the rest render as monochrome symbols.
- `circled`: `A`–`Z` and `a`–`z` → circled latin letters (`Ⓐ`), monochrome.

An unknown `style` is an error.

```python
letter_to_emoji("AB")                  # 🇦🇧
letter_to_emoji("A", style="squared")  # 🅰️
letter_to_emoji("a", style="circled")  # ⓐ
```

#### `symbol_to_emoji(text)`

Maps the well-defined punctuation characters `! ? # * + - / × ÷` in `text` to
their emoji and returns the result string; every other character passes through
unchanged.

```python
symbol_to_emoji("!?")   # ❗❓
symbol_to_emoji("#")    # #️⃣
symbol_to_emoji("50%!") # 50%❗
```

### Dispatcher and metadata

#### `convert(value, kind="auto")`

One entry point that dispatches to the conversion family named by `kind` and
returns the result string. Recognised kinds: `auto`, `emojize`, `demojize`,
`number`, `time`, `letter`, `symbol`. With `kind="auto"` (the default), ints and
floats are treated as `number`, an in-range `"H:MM"` string as `time`, and any
other string as `emojize`. The `letter` dispatch uses the `regional` style. A
time-shaped but out-of-range string (e.g. `"99:99"`) degrades to `emojize` under
`auto` rather than erroring. An unknown `kind` is an error.

```python
convert(42)                    # 4️⃣2️⃣  (auto → number)
convert("3:30")                # 🕞     (auto → time)
convert(":fire:")              # 🔥     (auto → emojize)
convert("AB", kind="letter")   # 🇦🇧
convert("🚀", kind="demojize")  # :rocket:
```

#### `info()`

Returns a dict describing the embedded dataset — useful for verifying which
emoji generation the module was built against. Keys: `primary_source`,
`secondary_source`, `emoji_version`, `shortcode_count`, `emoji_count`.

```python
info()["emoji_version"]   # "17.0"
info()["shortcode_count"] # e.g. 4000+
```

## Host configuration

Text conversions bound their input before processing it, so a hostile or buggy
script cannot force an unbounded allocation. The single config option:

| Option | Type | Default | Environment Variable | Description |
|--------|------|---------|----------------------|-------------|
| `max_input_bytes` | `int` | `5242880` | `EMOJI_MAX_INPUT_BYTES` | Maximum input size in bytes for text conversions (5 MiB); `0` disables the cap. |

The option can be set three ways:

- **Environment** — set `EMOJI_MAX_INPUT_BYTES` before the module is loaded.
- **From Starlark** — the configurable-module layer (`starpkg/base`) auto-exposes
  a setter/getter pair for the option: `set_max_input_bytes(value)` updates the
  cap and `get_max_input_bytes()` returns the current value.
- **From Go** — construct with `NewModule()` (default) and configure via `base`.

```python
load("emoji", "set_max_input_bytes", "get_max_input_bytes", "emojize")
set_max_input_bytes(1024)        # cap input at 1 KiB
get_max_input_bytes()            # 1024
emojize(":rocket:")              # 🚀  (well under the cap)
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

## License

This project is licensed under the MIT License — see [LICENSE](LICENSE).

The vendored datasets keep their upstream licenses (carpedm20/emoji: BSD-3-Clause;
github/gemoji: MIT). Only text data (names, shortcodes, code points) is used — no
image assets. Attribution and license texts are in
[`data/SOURCES.md`](data/SOURCES.md).
