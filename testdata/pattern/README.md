# Pattern fixtures (`rft grep` / `rft rewrite`)

Same shape as `testdata/mv/`: **input/**, optional **expected/**, **op.json**.

Engine + harness: `pkg/pattern` (`go test ./pkg/pattern/`).

Fixtures under this tree are **kept as authored** (including A1’s strip behavior). This document is the **locked product dialect** from design grilling; the engine should converge on it. Where a fixture is ahead of a pure-template rule (e.g. A1 string strip), treat the fixture as a stretch case, not a contradiction of the locks below.

---

## Locked dialect (authoritative)

### Product role

| Command | Role |
|---------|------|
| `rft grep <pattern> [paths…]` | Find only; stream per file |
| `rft rewrite <pattern> <replacement> [paths…]` | Match + **template** replacement |

- **Not** a fancy `mv` (symbol identity / import graph renames stay on `mv`).
- **Site codemod / search**: structural token patterns over source.

### Matching model (one way)

1. **Token stream** — ordered significant leaves from the tree-sitter grammar for that file’s language (not a semantic `call_expression` matcher as the primary model).
2. **Hyperlinks** — tokens may carry a resolved ref target (same notion as code-view links / ingest `Uses`).
3. **Whitespace** — pattern spaces only separate atoms; between matched tokens, source may have any whitespace/newlines (flexible).
4. **Groups** — `$name:{ … }` matches the interior sequence; the capture’s **span** is the **smallest AST node covering** the matched pieces; capture **text** is source in that node’s byte offsets.
5. **Streaming** — per-file map (hop/parse/match); emit/apply as each file finishes; no full-project materialize barrier before first output.

### Pattern syntax

| Form | Meaning |
|------|---------|
| plain text | Exact token text (`func`, `(`, `*`, `.`, `,`, `interface{}`, `2`, …) |
| `@provider:path::Symbol` | Token whose **link target** is that ref (product ref form; e.g. `@go:testing::T`) |
| `/regex/` | Token **text** matches regex (**not** `//…`) |
| `$name` | Capture **one** token |
| `$name:@ref` | Match ref on the **link leaf**; bind **selector** ending at that leaf (see below) |
| `$name:/regex/` | Capture one token whose text matches regex |
| `$name:{ … }` | Capture **group**: match sequence `…`; bind covering AST node |
| `$$$_` | Zero or more tokens (rest / skip), ast-grep-familiar |

### `$F:@ref` = selector only

- Match constraint: hyperlink on the **leaf** that ends the name (`Errorf`, `SplitN`, `ListenAndServe`, `T`).
- **Bound text/node**: the **selector expression** ending at that leaf — **not** the call args, **not** `(…)`.
- The selector **ends where the ref ends**.

Examples:

| Source | Pattern hole | `$F` holds |
|--------|--------------|------------|
| `fmt.Errorf(...)` | `$F:@go:fmt::Errorf` | `fmt.Errorf` |
| `f.Errorf(...)` | same | `f.Errorf` |
| `strings.SplitN(...)` | `$F:@go:strings::SplitN` | `strings.SplitN` |
| `http.ListenAndServe(...)` | `$F:@go:net/http::ListenAndServe` | `http.ListenAndServe` |

No extra `$pkg` is required for rewrite of the callee name chain.

### String / text holes `/regex/`

- Regex **filters** which tokens match (string lits: match unquoted content; idents: raw text).
- Outer bind `$name` is still the **full token** (including quotes for strings), e.g. `"failed to open image: %w"` or `TestFoo`.
- **Named groups** `(?P<rest>…)` (Go syntax) become **extra captures** available in the template as `$rest`, without changing `$name`:

  ```text
  $name:{/^Test(?P<rest>.*)/}
  # $name = TestFoo
  # $rest = Foo
  ```

- Unnamed group 1 may still override emit for the same name on string lits (A1 stretch strip).
- Fixture **go_failed_to_prefix** keeps its strip expected rewrite (stretch).

### Tokens like `interface{}` / `any`

- Matched as **grammar node text** (exact span the tree exposes), not `go:builtin::…` refs.
- Language-agnostic token algebra; any registered language grammar may expose such nodes.

### Replacement (template only)

- Replacement is **not** a second match dialect.
- Emit template: literal text + `$name` placeholders filled from match captures (bound node text).
- Example: `$F($MSG, $ERR)` with `$F` = `fmt.Errorf`.

### Execution / CLI (locked intent)

```text
rft grep <pattern> [paths…]
rft rewrite <pattern> <replacement> [paths…]
```

| Flag | Meaning |
|------|---------|
| `-C` / `--dir` | Project root |
| `-l` / `--lang` | Optional language filter (empty = all registered) |
| `-n` / `--dry-run` | rewrite: plan only |
| `-i` / `--interactive` | rewrite: plan + confirm |
| `-b` / `--backup` | rewrite: `.bak` before write |

Grep exit: `0` matches, `1` none, `2` error. Stream matches as files complete.

### Non-goals (for this dialect)

- Replacing `mv` for symbol renames / package moves
- Typed-as (“expression of type T”) without a hyperlink or token
- Full ast-grep YAML rule packs as day-one requirement
- Semantic-only call AST as the primary match engine

---

## Layout

```text
testdata/pattern/<lang>_<behavior>/
  input/           # sources before
  expected/        # after rewrite only (omit for grep)
  op.json          # pattern + replacement + IR
```

## CLI ↔ `op.json`

| CLI | `op.json` |
|-----|-----------|
| `rft grep <pattern> [files…]` | `"mode":"grep"`, `"pattern":"…"` |
| `rft rewrite <pattern> <replacement> [files…]` | `"mode":"rewrite"`, `"pattern":"…"`, `"replacement":"…"` |

| String (CLI) | IR (engine / fixture) |
|--------------|------------------------|
| `pattern` | `pattern_ir` |
| `replacement` | `replacement_ir` (`null` for grep) |

`pattern_ir` may still use nested `call` sugar in existing fixtures; that is **sugar for a token sequence** (callee selector + `(` + args + `)`), not a mandate to match only `call_expression` nodes forever.

## `op.json` fields

| Field | Required | Meaning |
|-------|----------|---------|
| `mode` | yes | `grep` \| `rewrite` |
| `lang` | yes | e.g. `go` (fixture filter; CLI may be broader) |
| `description` | yes | Intent |
| `pattern` | yes | Match pattern string |
| `replacement` | rewrite | **Template** string; `null` for grep |
| `pattern_ir` | yes | Canonical match IR |
| `replacement_ir` | rewrite | Canonical emit IR; `null` for grep |
| `expect_match_count` | grep optional | Match count over `input/` |
| `notes` | no | Caveats |

### Harness

- **rewrite:** copy `input/` → temp → apply → compare `expected/`
- **grep:** copy `input/` → temp → count matches

## Golden set (kept as-is)

| Directory | Mode | Notes vs locks |
|-----------|------|----------------|
| `go_failed_to_prefix` | rewrite | Stretch: strip via regex group / expected beyond pure full-token `$MSG` |
| `go_interface_to_any` | rewrite | Aligns: grammar tokens |
| `go_strings_splitn` | grep | Aligns: `$F` = `strings.SplitN`, lit `2` |
| `go_listen_and_serve` | grep | Aligns: `$F` = `http.ListenAndServe` |

## Example patterns (locked style)

```text
# A1-shaped (selector $F)
$F:@go:fmt::Errorf($MSG:/(?i)^failed to\s+(.*)/, $ERR)

# A2
interface{}

# A3
$F:@go:strings::SplitN($S, $SEP, 2)

# B4
$F:@go:net/http::ListenAndServe($ADDR, $HANDLER)

# Test-shaped (future)
func $name:{/^Test/} ( $$$_ $t * @go:testing::T $$$_ )

# Bare ref leaf scan
@go:testing::T
```

## Checks

```bash
go test ./pkg/pattern/
go run ./cmd/rft grep 'interface{}' testdata/pattern/go_interface_to_any/input
diff -ru testdata/pattern/go_failed_to_prefix/input testdata/pattern/go_failed_to_prefix/expected
```
