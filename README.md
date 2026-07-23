# refactree

**refactree** (`rft`) builds a common semantic model from your source and uses it for exploration and structural refactorings — list, navigate, document, move/rename — so references stay coherent.

Early software: the model and language coverage are evolving. Prefer a clean git tree before anything that writes.

## Install

**mise** (recommended):

```bash
mise use github:lucasew/refactree
```

Or take the static `rft` binary from [GitHub Releases](https://github.com/lucasew/refactree/releases) for your platform.

## How it works

`rft` parses source with tree-sitter, extracts **modules**, **files**, and **atoms** (named definitions), and the **uses** between them. Commands run against that model: query it, browse it, or plan structural edits.

There is no project-wide index to build or refresh. Work is on demand, in the spirit of `rg` / `fd`.

Contracts, architecture, and edge cases live in [`SPEC.md`](SPEC.md).

## References

Almost every command takes a **reference**:

```text
provider:where::what
```

- `provider` — how to resolve `where` (default `path`)
- `where` — file, package, or module location
- `what` — atom name (function, type, …); omit to mean the container

| Provider | Resolves | Example |
|----------|----------|---------|
| `path` | Project-relative file or directory | `path:./pkg/version/version.go::Version` |
| `go` | Go package (module / stdlib via `go list`) | `go:fmt::Errorf` |
| `python` | Python module (including stdlib) | `python:os::makedirs` |
| `node` | Node package (`node_modules`, builtins) | `node:react` |
| `java` | Java package under source roots | `java:java.util.List::size` |
| `nix` | Nix path / NIX_PATH style | `nix:nixpkgs` |

`path` is the usual choice inside a repo.

## Language support

| Language | `ls` / `doc` | navigate / `edit` | browse / `serve` | `grep` | `mv` |
|----------|:------------:|:-----------------:|:----------------:|:------:|:----:|
| Go | ✓ | ✓ | ✓ | ✓ | ✓ |
| Python | ✓ | ✓ | ✓ | ✓ | ✓ |
| Java | ✓ | ✓ | ✓ | ✓ | ✓ |
| JavaScript / TypeScript\* | ✓ | ✓ | ✓ | ✓ | ✓ |
| Nix\* | ✓ | ✓ | ✓ | ✓ | — |
| Svelte\* | ✓ | ✓ | ✓ | ✓ | — |

\* **JavaScript / TypeScript** — one surface (`.js` / `.mjs` / `.cjs` / `.ts` / `.tsx` / `.jsx`).

\* **Nix**, **Svelte** — basic parse / list / navigation only; no `mv` planner yet.

Coverage depth still varies by construct. Prefer `ls` / `doc` before `mv` on unfamiliar code.
## Plumbing

Examples below assume the **refactree** checkout as cwd (Go).

### List atoms

```bash
rft ls path:./pkg/version/version.go::
rft ls path:./pkg/ingest/
rft ls -a path:./cmd/rft/root.go::    # include unexported / hidden
rft ls -l path:./pkg/version/version.go::
```

### Documentation

```bash
rft doc path:./pkg/version/version.go::Version
rft doc go:fmt::Errorf
```

### Open definition

```bash
rft edit path:./pkg/version/version.go::Version
rft edit                            # interactive picker under cwd
rft edit path:./pkg/ingest/         # picker scoped to that tree
```

Interactive pick needs **`fzf`** on `PATH`. Editor is `--editor`, then `RFT_EDITOR`, `VISUAL`, `EDITOR`.

### AST dump

```bash
rft astdump go pkg/version/version.go
rft astdump go pkg/version/version.go --query '(function_declaration name: (identifier) @name)'
```

Useful when you want the raw tree-sitter view behind the model.

## Browse

### Terminal

```bash
rft browse
rft browse path:./pkg/version/
rft browse go:fmt
```

### Web UI

```bash
rft serve -C .
# listen default 127.0.0.1:8080 — open the printed URL
```

Same reference hyperlinks as the CLI. `rft desktop` opens the same UI in a Electron-like window.

### Structural search

```bash
rft grep 'interface{}' ./pkg
rft grep 'func Version' ./pkg/version
rft grep '$F:@go:fmt::Errorf($MSG, $ERR)' ./pkg
rft grep --format jsonl --vars 'func $name:{/^Test/}' ./pkg
```

Token-stream patterns (flexible whitespace). `@provider:path::Symbol` is a hyperlink hole, same targets as the code view. Details: [`testdata/pattern/README.md`](testdata/pattern/README.md).

### Language server

```bash
rft lsp
```

stdio LSP; wire it like any other LSP server. Put language-specific servers first if you can when you stack them so they take precedence.

## Refactor

Structural edits only: rename/move atoms and rewrite matched sites. Imports and references are updated when the planner can; **logic is not rewritten by design**.

**Best effort.** Have git (or another rollback) ready before writes. `-i` / `-b` help; they are not a warranty.

### Move / rename

```bash
# rename in place (-i shows the plan and asks before write)
rft mv -i path:./pkg/version/version.go::Version path:./pkg/version/version.go::VersionString

# cross-file move: source ref → destination ref
rft mv -i path:./pkg/a.go::Helper path:./pkg/b.go::Helper

# -b: write .bak next to each modified file
rft mv -i -b <src> <dst>
```### Rewrite (structural)

```bash
rft rewrite -n 'interface{}' 'any' ./pkg
rft rewrite -i 'interface{}' 'any' ./pkg
rft rewrite -i \
  '$F:@go:fmt::Errorf($MSG:/(?i)^failed to\s+(.*)/, $ERR)' \
  '$F($MSG, $ERR)' \
  ./pkg
```

`-n` dry-run, `-i` confirm, `-b` backup — same idea as `mv`.

### Lint (rulebook)

Project codemod catalog in `refactree.yaml` (walk-up from `-C`, or `--config`). If no config exists, a **built-in default** runs (unused named imports). Pattern dialect matches `grep`/`rewrite`; optional `replacement` or `builtin: dead-imports` → SARIF autofix and `--fix`.

```bash
# report (exit 1 if any finding); works with no refactree.yaml
rft lint
rft lint --format sarif

# apply non-conflicting fixes (first rule in YAML order wins on overlap)
rft lint --fix
rft lint -n --fix   # plan only
```

```yaml
# refactree.yaml (optional)
rules:
  - id: imports/unused-named
    builtin: dead-imports
    message: Unused named import
  - id: go/prefer-any
    language: go
    pattern: "interface{}"
    message: Prefer any over interface{}
    replacement: any
```

## Development

```bash
mise install
mise build          # deps, codegen, SPA embed, ./rft
go run ./cmd/rft -- ls path:./pkg/version/version.go::
```

Architecture and contributor detail: [`SPEC.md`](SPEC.md), [`AGENTS.md`](AGENTS.md).

This was heavily done with AI agents. Big PRs with new tests may come from fuzzying agents finding blind spots in the ingestion model, and there are a lot.

It uses a [Go transpiled tree-sitter](https://github.com/modernc-tree-sitter/ccgo-tree-sitter), which allows this to work without CGo.

## License

[MIT](LICENSE)
