# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`starpkg/emoji` is an **L4 domain module** of the Star\* ecosystem: it exposes
emoji ⇄ text conversion to Starlark scripts. A script imports the module and
calls pure functions that turn `:shortcodes:` into glyphs (and back), or turn a
plain number, clock time, letter, or punctuation mark into the most similar
emoji.

The starpkg remit is *support for necessary **local** operations plus simple
abstractions over common **online** services, for ease of use.* `emoji` is
entirely on the **local** side — there is no network, no service, no
credentials. At run time it reads only embedded Go maps (`tables_gen.go`) and
does pure Unicode arithmetic, so it has **zero third-party data dependencies** and
every function is pure and deterministic.

Layer position: depends downward on `starpkg/base` (the module/config system),
`1set/starlet` (the Machine + `dataconv/types`), and transitively
`1set/starlight` + `go.starlark.net`. Nothing in the ecosystem depends on it.

## Dev commands

Pure Go library with a Makefile. From this repo:

```bash
make test                                  # -race -cover, the working bar
make ci                                    # -race -cover profile + bench compile (what CI runs)
make bench                                 # benchmarks only
go test ./... -run TestConvert             # a single test
gofmt -l . && go vet ./...                 # must be clean before commit
go run github.com/1set/meta/doccov@master .  # README documents every builtin (the doc gate)
```

**Verify on the go floor in Docker** — this repo's floor is **go 1.19** (see
Release discipline), older than the local toolchain. Behavior on the floor must
be checked in a container:

```bash
docker run --rm -v "$PWD":/src -v "$HOME/go/pkg/mod":/go/pkg/mod -w /src golang:1.19 go test -race -count=1 ./...
```

The data table is regenerated, never edited by hand — `go generate ./...` runs
`internal/gen`. Integration scripts under `../test/emoji/*.star` live in the
**private `starpkg/test` repo** and auto-skip when that directory is absent
(e.g. in CI); this module currently has none.

## Architecture (the part that spans files)

The module is a **two-track converter** behind one Starlark surface: a
data-table track (shortcodes) and a pure-arithmetic track (look-alikes). Both
are exposed as plain builtins — there are no stateful objects.

- **`emoji.go`** — the module entry and the shortcode track. `Module` holds a
  `base.ConfigurableModule` plus its `Extend()` accessor; `NewModule()`
  constructs it with the `max_input_bytes` config option. `LoadModule()`
  registers the twelve builtins: `emojize`, `demojize`, `get`, `name`,
  `describe`, `number_to_emoji`, `emoji_to_number`, `time_to_emoji`,
  `letter_to_emoji`, `symbol_to_emoji`, `convert`, `info`. Shortcode forward
  (`emojizeText`) and reverse (`demojizeText`, longest-match) live here, with the
  `:flag-xx:` regional-pair fallback and the `convert` dispatcher / `autoKind`
  detection.
- **`convert.go`** — the look-alike track: number ⇄ keycap emoji, `H:MM` →
  clock-face emoji (`clockEmoji`, round-half-up to `:00`/`:30`), letter → emoji
  in `regional`/`squared`/`circled` styles, and a fixed punctuation → symbol map.
  Pure Unicode arithmetic over well-defined code-point runs; no data table.
- **`tables_gen.go`** — generated, **DO NOT EDIT**. Three unexported maps
  (`dataForward`, `dataReverse`, `dataNames`) plus the provenance constants
  (`dataSourcePrimary`, `dataSourceSecondary`, `dataEmojiVersion`) surfaced by
  `info()`.
- **`internal/gen/main.go`** — the offline generator (`package main`). Merges two
  pinned, vendored datasets (gemoji first so its canonical short aliases win,
  then carpedm20 to fill gaps and the newest emoji) into the Go maps. Output is
  deterministic (sorted, ASCII-escaped, no timestamps) so a data refresh is a
  reviewable diff. Run via `go generate ./...`.
- **`data/`** — vendored source JSON (`data/sources/*.json`), upstream license
  texts, `fetch.sh` (pull pinned sources), and `SOURCES.md` (provenance).

Dependency-on-base note: `base.ConfigurableModule.LoadModule` auto-exposes a
`set_<key>` / `get_<key>` builtin pair for every config option, so
`set_max_input_bytes` and `get_max_input_bytes` are real script builtins even
though they are registered inside `base`, not in this repo's source.

## Invariants / hardening (preserve when editing)

1. **Bounded input.** Every text-accepting builtin runs `checkInputSize` /
   `checkValueSize` before processing, so input over `max_input_bytes` is
   rejected instead of allocating without bound. The `number`/`time` paths must
   honor the cap too — they once bypassed it while `letter`/`symbol` enforced it
   (a real regression, see `TestMaxInputBytes`). New text entry points must route
   through these checks.
2. **No host panics from script input.** Builtins validate argument types and
   return script-level errors (`UnpackArgs`, the `default:` arms in `numericText`
   / `parseTimeValue` / `textOf`), never `panic`.
3. **Graceful degradation, not hard errors, under `auto`.** `convert(value)` with
   `kind="auto"` only routes a string to `time` when it is a valid in-range clock
   string; a `"99:99"`-shaped value falls through to `emojize` (and is returned
   untouched) rather than erroring (`TestConvertAutoTimeFallthrough`).
4. **Deterministic, data-free look-alikes.** `convert.go` is pure Unicode
   arithmetic over fixed code-point runs (keycap digits, clock faces, regional
   indicators, symbol map) — correct regardless of how fresh the shortcode
   dataset is. Keep the invisible combining code points (`variationSelector`,
   `enclosingKeycap`) as named numeric constants, not literal characters.
5. **Generated data is reviewable.** `tables_gen.go` is produced only by
   `internal/gen`; never hand-edit it. Refreshing data = bump the pin in
   `data/fetch.sh` + the constants in `internal/gen`, re-run `fetch.sh` and
   `go generate`, review the diff.
6. **Backward compatibility.** `NewModule()` keeps the historical default
   (`max_input_bytes = 5 MiB`); any new safety lever must default to the
   historical behavior so existing scripts run unchanged.

## Test organization

Group by functional goal — **do not add one `*_test.go` per fix.** Two
source-shaped thematic files mirror the two source files, each opened with a
section list:

- **`emoji_test.go`** — the shortcode core + module wiring: emojize/demojize
  round-trip and delimiters, `get`/`name`/`describe`, the `convert` dispatcher,
  `info`, the dedicated converters, error paths, and the `max_input_bytes` cap.
- **`convert_test.go`** — the look-alike conversions: number/keycap, the full
  24-face clock table + rounding, letter styles, the symbol map.
- **`internal/gen/main_test.go`** — the generator's merge/sort/IO helpers.

Tests are table/example-driven; no third-party test framework. Add a new test as
a **section** in the matching file, not a new file.

## Documentation

Three layers must stay in sync (enforced by the doc standard,
`plan/starpkg文档标准（DOC-STD）`):

- **`README.md`** — every script-facing builtin documented as a backtick
  whole-word with correct name/signature/args/return/behaviour; the
  `max_input_bytes` host config (and its `set_max_input_bytes` /
  `get_max_input_bytes` builtins) under *Host configuration*. Gated by `doccov`
  (the `doc-coverage: true` leg of the reusable CI workflow), which fails if any
  builtin is undocumented.
- **GoDoc** — package comment + a doc comment on every exported symbol
  (`ModuleName`, `Module`, `NewModule`, `LoadModule`), gated by `revive`'s
  `exported` rule in CI. The generated and `internal/gen` files carry their own
  package/command comments.

## Release discipline

- **Floor = go 1.19** — this repo's `go.mod` floor; it only rises in this repo's
  own pin-upgrade PR (the SEP/ENG pin upgrade, done last in the series). Pins:
  `1set/starlet v0.2.1`, `starpkg/base v0.1.0`, `go.starlark.net` at the
  ecosystem baseline (`ffb3f39`).
- **CI matrix** = `[1.19.x, 1.25.x]` via the centralized reusable workflow in
  `1set/meta` (`.github/workflows/go-ci.yml`), pinned to a commit SHA, with
  `doc-coverage: true`.
- **Bumping the version, the go floor, or tagging are user-confirmed actions** —
  never tag autonomously; default to patch bumps; published tags are immutable.
