# Pattern fixtures (`rft grep` / `rft rewrite`)

Same shape as `testdata/mv/`: **input tree**, optional **expected tree**, **op.json**.

Engine + harness: `pkg/pattern` (`go test ./pkg/pattern/`).

- copy `input/` → temp
- match `pattern_ir` (Go tree-sitter + ingest uses for `@ref`)
- **rewrite:** instantiate `replacement_ir`, `ApplyEdits`, compare to `expected/`
- **grep:** assert `expect_match_count`

## Layout

```text
testdata/pattern/<lang>_<behavior>/
  input/           # sources before
  expected/        # sources after rewrite only (omit for mode=grep)
  op.json          # pattern (+ replacement for rewrite)
```

Mirrors:

```text
testdata/mv/<case>/
  input/
  expected/
  op.json
```

## CLI ↔ `op.json`

| CLI | `op.json` |
|-----|-----------|
| `rft grep <pattern> [files…]` | `"mode":"grep"`, `"pattern":"…"` |
| `rft rewrite <pattern> <replacement> [files…]` | `"mode":"rewrite"`, `"pattern":"…"`, `"replacement":"…"` |

**String fields** are the CLI argv forms. **`_ir` fields** are the canonical trees the engine should use (fixtures do not depend on a perfect pretty-parser).

| String (CLI) | IR (engine / fixture truth) |
|--------------|-----------------------------|
| `pattern` | `pattern_ir` |
| `replacement` | `replacement_ir` (`null` for grep) |

```json
{
  "mode": "rewrite",
  "pattern": "$F:@go:fmt::Errorf(\"failed to open image: %w\", $ERR)",
  "replacement": "$F(\"open image: %w\", $ERR)",
  "pattern_ir": { "kind": "call", "…": "…" },
  "replacement_ir": { "kind": "call", "…": "…" }
}
```

```bash
rft rewrite \
  '$F:@go:fmt::Errorf("failed to open image: %w", $ERR)' \
  '$F("open image: %w", $ERR)' \
  .
```

## `op.json` fields

| Field | Required | Meaning |
|-------|----------|---------|
| `mode` | yes | `grep` \| `rewrite` |
| `lang` | yes | e.g. `go` |
| `description` | yes | Intent / inventory id |
| `pattern` | yes | Match pattern string (CLI arg) |
| `replacement` | rewrite | Replacement pattern string (CLI arg); `null` for grep |
| `pattern_ir` | yes | Canonical match IR |
| `replacement_ir` | rewrite | Canonical replacement IR; `null` for grep |
| `expect_match_count` | grep (optional) | How many matches in `input/` |
| `notes` | no | Non-goals, caveats |

### Rewrite

1. Start from `input/`
2. Match with `pattern_ir` (string form is documentation / future parser input)
3. Instantiate `replacement_ir` with captures → edits
4. Result must match `expected/` file-for-file (same idea as `mv_test.compareDir`)

### Grep

- Only `input/` + `op.json`
- No `expected/`
- `"replacement": null`, `"replacement_ir": null`
- Optional `expect_match_count`

## IR kinds (v1)

| `kind` | Role |
|--------|------|
| `call` | Call expression; `callee` + `args` |
| `ref` | Hyperlink hole → target ref (`go:fmt::Errorf`) |
| `string` | String literal; optional `equals` / `regex` |
| `type_token` | Fixed type text (`interface{}`, `any`) |
| `capture` | Structural `$Name` (in replacement_ir: emit bound text) |
| `rest` | `$$$Name` |
| `lit` | Non-string literal (`2`) |

In `pattern`, `@go:fmt::Errorf` is hole sugar for `{ "kind": "ref", "ref": "go:fmt::Errorf" }`.

In `replacement_ir`, prefer `{ "kind": "capture", "as": "F" }` for holes filled from the match (not re-resolving refs).

## Golden set

| Directory | Mode | Inventory |
|-----------|------|-----------|
| `go_failed_to_prefix` | rewrite | A1 `fmt.Errorf` format string |
| `go_interface_to_any` | rewrite | A2 `interface{}` → `any` |
| `go_strings_splitn` | grep | A3 `SplitN` N=2 only |
| `go_listen_and_serve` | grep | B4 `ListenAndServe` sites |

## Checks without an engine

```bash
go run ./cmd/rft ingest testdata/pattern/go_failed_to_prefix/input
diff -ru testdata/pattern/go_failed_to_prefix/input testdata/pattern/go_failed_to_prefix/expected
diff -ru testdata/pattern/go_interface_to_any/input testdata/pattern/go_interface_to_any/expected
```
