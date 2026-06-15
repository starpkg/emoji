# `emoji` — Starlark API Reference

The complete reference for every script-facing builtin and configuration
accessor exposed by the `emoji` module. For an overview, installation, and a
quickstart, see the [README](../README.md).

The module exposes twelve top-level builtins via `load("emoji", …)`. They split
into a **shortcode** group (backed by a generated data table) and a
**look-alike** group (pure Unicode arithmetic), with a `convert` dispatcher and
an `info` metadata builtin. A pair of configuration accessors
(`get_max_input_bytes` / `set_max_input_bytes`) is generated from the module's
single option. All builtins are pure and deterministic; every text-accepting
builtin bounds its input by `max_input_bytes` (see [Configuration](#configuration)).

## Contents

- [Shortcode ⇄ emoji](#shortcode--emoji)
- [Look-alike conversions](#look-alike-conversions)
- [Dispatcher and metadata](#dispatcher-and-metadata)
- [Configuration](#configuration)

## Shortcode ⇄ emoji

These builtins are backed by the generated shortcode data table.

### `emojize(text)`

Replaces every known `:shortcode:` in `text` with its emoji glyph and returns
the result string.

**Parameters:**

- `text` (string or bytes): The input to scan for `:shortcode:` tokens.

**Returns:** The result string with known shortcodes replaced by glyphs.

**Behaviour:** Unknown tokens are left untouched. A `:flag-xx:` / `:flag_xx:`
token whose two-letter code is not in the table falls back to the corresponding
regional-indicator flag sequence.

**Errors:** Raises a script error if `text` exceeds `max_input_bytes`.

**Example:**

```python
emojize("ship it :rocket: :no_such_code:")   # ship it 🚀 :no_such_code:
emojize(":flag-fr:")                          # 🇫🇷
```

### `demojize(text, delimiters=(":", ":"))`

The inverse of `emojize`: replaces every emoji glyph in `text` with its
`:shortcode:`, returning the result string.

**Parameters:**

- `text` (string or bytes): The input to scan for emoji glyphs.
- `delimiters` (tuple or list, optional): An `(open, close)` pair of strings
  wrapping each replaced shortcode. Defaults to `(":", ":")`.

**Returns:** The result string with emoji glyphs replaced by delimited
shortcodes.

**Behaviour:** The scan is longest-match, so multi-rune sequences (variation
selectors, ZWJ joins) win over their shorter prefixes.

**Errors:** Raises a script error if `delimiters` is not a 2-element pair of
strings, or if `text` exceeds `max_input_bytes`.

**Example:**

```python
demojize("i ❤️ 🚀")                            # i :heart: :rocket:
demojize("hi 🚀", delimiters=("[", "]"))      # hi [rocket]
```

### `get(name)`

Returns the emoji glyph for a single shortcode, or `None` if the shortcode is
unknown.

**Parameters:**

- `name` (string): A shortcode, with or without surrounding colons
  (`"rocket"` and `":rocket:"` are equivalent).

**Returns:** The emoji glyph string, or `None` if the shortcode is unknown.

**Behaviour:** A `flag-xx` code falls back to a regional-indicator flag.

**Example:**

```python
get("rocket")    # 🚀
get(":rocket:")  # 🚀
get("nope")      # None
```

### `name(emoji)`

Returns the primary (canonical) shortcode for a single emoji glyph, without
colons, or `None` if the glyph is not in the table.

**Parameters:**

- `emoji` (string): A single emoji glyph.

**Returns:** The canonical shortcode string (no colons), or `None`.

**Example:**

```python
name("🚀")   # "rocket"
```

### `describe(emoji)`

Returns the human-readable name for a single emoji glyph, or `None` if the glyph
is not in the table.

**Parameters:**

- `emoji` (string): A single emoji glyph.

**Returns:** The human-readable name string, or `None`.

**Behaviour:** The name comes from the source datasets — GitHub's description
where available, otherwise the de-underscored canonical shortcode.

**Example:**

```python
describe("🚀")   # "rocket"
```

## Look-alike conversions

These builtins are pure Unicode arithmetic over fixed code-point runs; no data
table is involved.

### `number_to_emoji(value, keycap_ten=False)`

Maps each ASCII digit in `value` to its keycap emoji and returns the result
string.

**Parameters:**

- `value` (int, float, or string): The number or numeric text to convert.
- `keycap_ten` (bool, optional): When `True` and the whole input is exactly
  `10`, emit the dedicated 🔟 (KEYCAP TEN) glyph instead of two keycaps.
  Defaults to `False`.

**Returns:** The result string with digits replaced by keycap emoji.

**Behaviour:** `-` and `+` become the heavy minus/plus dingbats; any other
character passes through unchanged.

**Errors:** Raises a script error if `value` is not an int, float, or string,
or if a string `value` exceeds `max_input_bytes`.

**Example:**

```python
number_to_emoji(42)                   # 4️⃣2️⃣
number_to_emoji(10, keycap_ten=True)  # 🔟
number_to_emoji(3.5)                  # 3️⃣.5️⃣
```

### `emoji_to_number(text)`

The inverse of `number_to_emoji`.

**Parameters:**

- `text` (string or bytes): The input to scan for keycap emoji.

**Returns:** The result string with keycap emoji replaced by their characters.

**Behaviour:** Keycap sequences become their digit, 🔟 becomes `10`, the heavy
plus/minus become `+`/`-`, and every other rune passes through unchanged.

**Errors:** Raises a script error if `text` exceeds `max_input_bytes`.

**Example:**

```python
emoji_to_number("4️⃣2️⃣")   # "42"
```

### `time_to_emoji(value, minute=None)`

Returns the single clock-face emoji nearest to a time.

**Parameters:**

- `value` (int or string): An int hour, or a `"H"` / `"H:MM"` string.
- `minute` (int, optional): The minute, used only when `value` is an int hour.
  Defaults to `None` (treated as `0`).

**Returns:** The single nearest clock-face emoji string.

**Behaviour:** Only `:00` and `:30` faces exist, so the minute is rounded to the
nearest half hour (round-half-up: `:15` → `:30`, `:45` → next hour). The hour is
taken mod 12, so AM and PM share a face.

**Errors:** Raises a script error for an out-of-range hour (must be `0`–`23`) or
minute (must be `0`–`59`), an unparseable time, or a string `value` exceeding
`max_input_bytes`.

**Example:**

```python
time_to_emoji("3:30")        # 🕞
time_to_emoji(15, minute=30) # 🕞  (15:30 shares the 3:30 face)
```

### `letter_to_emoji(text, style="regional")`

Maps Latin letters in `text` to emoji in one of three styles, returning the
result string.

**Parameters:**

- `text` (string or bytes): The input to convert.
- `style` (string, optional): One of `regional`, `squared` (alias `button`), or
  `circled`. Defaults to `regional`.

**Returns:** The result string with letters mapped to emoji.

**Behaviour:** Runes with no mapping in the chosen style pass through unchanged.

- `regional` (default): `A`–`Z` → regional-indicator symbols (`🇦`). Two adjacent
  indicators that form a valid country code render as that flag.
- `squared` (alias `button`): `A`–`Z` → negative-squared latin capitals. Only
  `A`/`B`/`O`/`P` are colour emoji (🅰️); the rest render as monochrome symbols.
- `circled`: `A`–`Z` and `a`–`z` → circled latin letters (`Ⓐ`), monochrome.

**Errors:** Raises a script error for an unknown `style`, or if `text` exceeds
`max_input_bytes`.

**Example:**

```python
letter_to_emoji("AB")                  # 🇦🇧
letter_to_emoji("A", style="squared")  # 🅰️
letter_to_emoji("a", style="circled")  # ⓐ
```

### `symbol_to_emoji(text)`

Maps the well-defined punctuation characters `! ? # * + - / × ÷` in `text` to
their emoji and returns the result string.

**Parameters:**

- `text` (string or bytes): The input to convert.

**Returns:** The result string with mapped punctuation replaced by emoji.

**Behaviour:** Every character outside the fixed punctuation set passes through
unchanged.

**Errors:** Raises a script error if `text` exceeds `max_input_bytes`.

**Example:**

```python
symbol_to_emoji("!?")   # ❗❓
symbol_to_emoji("#")    # #️⃣
symbol_to_emoji("50%!") # 50%❗
```

## Dispatcher and metadata

### `convert(value, kind="auto")`

One entry point that dispatches to the conversion family named by `kind` and
returns the result string.

**Parameters:**

- `value` (int, float, string, or bytes): The value to convert. The accepted
  types depend on the resolved `kind`.
- `kind` (string, optional): One of `auto`, `emojize`, `demojize`, `number`,
  `time`, `letter`, `symbol`. Defaults to `auto`.

**Returns:** The result string from the selected conversion.

**Behaviour:** With `kind="auto"` (the default), ints and floats are treated as
`number`, an in-range `"H:MM"` string as `time`, and any other string as
`emojize`. The `letter` dispatch uses the `regional` style. A time-shaped but
out-of-range string (e.g. `"99:99"`) degrades to `emojize` under `auto` rather
than erroring.

**Errors:** Raises a script error for an unknown `kind`, for a value whose type
the resolved kind cannot accept, for an out-of-range `time`, or if a string
`value` exceeds `max_input_bytes`.

**Example:**

```python
convert(42)                    # 4️⃣2️⃣  (auto → number)
convert("3:30")                # 🕞     (auto → time)
convert(":fire:")              # 🔥     (auto → emojize)
convert("AB", kind="letter")   # 🇦🇧
convert("🚀", kind="demojize")  # :rocket:
```

### `info()`

Returns a dict describing the embedded dataset — useful for verifying which
emoji generation the module was built against.

**Parameters:** None.

**Returns:** A dict with keys `primary_source`, `secondary_source`,
`emoji_version`, `shortcode_count`, and `emoji_count`.

**Example:**

```python
info()["emoji_version"]   # "17.0"
info()["shortcode_count"] # e.g. 4000+
```

## Configuration

The module exposes its single option to scripts as a pair of generated accessor
builtins (loaded from the `emoji` module alongside the functions above):

- **`get_<key>()`** — returns the current value of the option.
- **`set_<key>(value)`** — sets the option (returns `None`).

An option's value resolves in priority order: an explicit `set_<key>` value, the
environment variable, then the default.

None of the `emoji` options are secret, so the option exposes **both**
`get_max_input_bytes` and `set_max_input_bytes`. (A secret option would expose
only its `set_<key>` accessor — never a getter — but this module has none.)

| Option | Getter | Setter | Type | Env var | Default | Description |
|--------|--------|--------|------|---------|---------|-------------|
| `max_input_bytes` | `get_max_input_bytes` | `set_max_input_bytes` | int | `EMOJI_MAX_INPUT_BYTES` | `5242880` | Maximum input size in bytes for text conversions (5 MiB); `0` disables the cap. |

Text conversions bound their input before processing it, so a hostile or buggy
script cannot force an unbounded allocation. The option can be set three ways:

- **Environment** — set `EMOJI_MAX_INPUT_BYTES` before the module is loaded.
- **From Starlark** — `set_max_input_bytes(value)` updates the cap and
  `get_max_input_bytes()` returns the current value.
- **From Go** — construct with `NewModule()` (default) and configure via `base`.

**Example:**

```python
load("emoji", "set_max_input_bytes", "get_max_input_bytes", "emojize")

set_max_input_bytes(1024)        # cap input at 1 KiB
get_max_input_bytes()            # 1024
emojize(":rocket:")              # 🚀  (well under the cap)
```
