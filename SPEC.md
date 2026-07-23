# refactree

Tool to do queries on symbols and some refactorings like move and rename items in modules


## CLI: rft
- Out of scope: preprocessing index, all findings are done on time, like how ripgrep and fd works
- Refactorings must check for every reference and referee and make a plan that makes the code work the same way. We don't alter logic, only structure
- Parsers for each language give a list of things and usages
  - import os; os.system("Hello, world") -> Uses the python lookup mechanism to find the implementation of "os", then the implementation of "system" and add the edge on the graph
  - Parsers are made based on the tree-sitter conversion to Golang: https://github.com/modernc-tree-sitter/ccgo-tree-sitter/
    - Using the scm query language is not forbidden if the code is able to filter the noise
- All subcommands have `-v/--verbose` to switch the slog level from info to verbose/debug, the highest one there

## Architecture: extract stream and materialize

Structural spine for discovery and graphs. Goal: one walk/parse path, no duplicated WalkDir + parse collectors. Lookup is lazy by default; only jobs that need a closed graph go eager.

### Shared bottom
- Per-file unit: parse (tree-sitter / `ParseSource*`) → `FileExtract` (atoms, imports, usages, reexports)
- Disk entry: `parseFileCached` (root + path + mtime). No second ad-hoc parser for listing vs mv vs serve
- Language drivers only implement extract/import rules; they do not own project walks

### WalkExtracts
- Pull stream of `*FileExtract` for a set of paths
- Applies skip-dir rules and recursive policy once
- Drivers only decide **which paths** enter the stream (not how parse works):
  - **Dir**: filesystem walk under a root (full tree or non-recursive top level)
  - **Seed**: BFS from seed file(s) through neighbors / import edges (bounded; no deep vendor crawl)
  - **Hop**: re-seed a single path (or small set) for canonicalize barrel hops
- Consumers may stop early (e.g. fzf selection). Stopping is normal for lazy list; invalid for a graph job that claimed a complete materialize

### Materialize
- Input: a **closed** set of extracts (drained stream or explicit collect)
- Optional **ExpandImports** (one-hop import/reexport targets under root, same idea as today’s append-import-targets)
- Then **Resolve** → `*Result` (files, atoms, aliases, uses)
- ExpandImports is **not** the default and is **not** part of WalkExtracts
- ExpandImports is **only** for full-Dir project-complete loads (mv / “whole project graph”)
- Seed and Hop materialize with ExpandImports **off** (Seed already grew the set via BFS; Hop is minimal)
- Resolve never runs on an open-ended live stream; no incremental half-graph visible to mv

### WalkAtoms
- Thin convenience over WalkExtracts (Dir or single-file scope) + visibility/provider filters → yield `AtomInfo`
- Must not reimplement walk/parse. No Materialize, no ExpandImports, no relations
- Used by `ls` and `edit` picker

### Canonical public API (migrate all consumers)
- Prefer spine names: `WalkExtracts`, `Materialize`, drivers Dir/Seed/Hop
- Drop `Ingest` / `IngestWithRecursion` / `IngestForFile` as the long-term brand (call sites move to spine)
- `WalkAtoms` stays as the list convenience only

### Entry points (who is lazy vs eager)
- **ls**, **edit** picker: lazy — WalkExtracts / WalkAtoms only; stream atoms as files parse
- **edit** open / navigation canonicalize: Hop → Materialize(ExpandImports=false) or equivalent minimal result → `CanonicalizeInResult`; not full Dir
- **doc**: minimal file/hop extract; not full project Materialize
- **mv**: eager on purpose — Dir WalkExtracts over project root (skip rules) → Materialize(**ExpandImports=true**) → plan from full `*Result`. Must not use list-only path
- **serve** / **desktop** / browse file annotate: Seed → Materialize(ExpandImports=false); target SPA uses the same lazy spine via GraphQL (see Web browser)
- **lsp**: another surface on the same spine (see Language server); two-tier recompute over ProjectFS overlays
- **fuzzy / full-graph tests**: same as mv-style full Dir Materialize when the check needs the whole picture

### Non-goals / hard rules
- Do not put import expansion inside the list stream
- Do not make “lookup” or WalkAtoms full-project by default
- Do not maintain a second WalkDir implementation beside WalkExtracts
- Micro-dedup (shared ParseSource helpers, path joiners, etc.) stacks under this spine; it does not replace it
- Language-specific logic stays in language packages behind interfaces; LSP package has none

### Subcommand: ls
- Lists atoms in a reference (WalkAtoms / lazy extract stream)
- `-a`: List symbols normally hidden symbols, like symbols that start lower case on Golang or start with _ in python
- `-l`: Use text/table
- Equivalent semantic of ls in general, make sure to not be confusing, add flags on demand, don't invent useless flags or use flags that  dont make sense in the problem

### Subcommand: mv
- Renames symbols in a reference (variables, functions, classes)
- Moves symbols in a reference to another place (like a function to another file, updating imports)
- Does all the necessary checks before editing any files looking for references (full Dir Materialize with ExpandImports — not the lazy list path)
- `-b/--backup`: Create a .bak copy before writing the destination file
- `-i`: Make some kind of edit plan showing each operation that is going to be done and ask for confirmation from the user
- Equivalent semantic of ls in general, make sure to not be confusing, add flags on demand, don't invent useless flags or use flags that  dont make sense in the problem

### Subcommand: doc
- Gets the docstring of the reference
- Just the doc, and if it makes sense the signature of functions
- `# name\nSignature: $signature\n$actual_docstring`

### Transform core (shared data model)
- **`ingest.Span`**: half-open byte range; embedded on **`ingest.Edit`**
- **`ingest.Edit`**: apply unit (`File` + `Span` + `NewText`) — `StageEdits` / `ApplyEdits` / LSP stage
- **`pattern.Rule`**: site unit (`Pattern` + `Replacement` + optional `SetCapture`) → NFA match → `[]Edit`
  - `rft rewrite` is one Rule over a file stream
  - `mv` remains a symbol-identity **planner** (full graph); use-site leaf rewrites may be expressed as site Rules (e.g. `RefLeafRule(@old, newLeaf)`) without a second match engine
- Planners differ (identity/graph vs stream/codemod); site expansion and apply do not

### Subcommands: grep / rewrite
- Structural find (`grep`) and site rewrite (`rewrite`) — **not** symbol identity ops (`mv`)
- **Dialect locks** (authoritative detail: `testdata/pattern/README.md`):
  - Match unit: **token stream** from tree-sitter leaves (flexible whitespace between tokens)
  - Literals: exact token text; patterns: `/regex/` on token text (not `//`); refs: `@provider:path::Symbol` on link leaf
  - `$name` = one token; `$name:{…}` = group whose capture span is the **smallest AST node covering** the match; text = that node’s byte range
  - `$F:@ref` binds the **selector ending at the ref leaf** (e.g. `fmt.Errorf`, not only `Errorf`; does not include call args)
  - `$$$_` = zero or more tokens (rest)
  - String `/regex/`: filter only; capture is the **full string token** (including quotes) unless a fixture documents a stretch
  - Grammar tokens (`interface{}`, `any`, …): exact node text from the grammar — **not** synthetic `go:builtin` refs
  - **Replacement is a template only** (literals + `$captures`), not a second match dialect
- Execution: **per-file stream** (map); no full-project materialize barrier before first grep output
- CLI: `rft grep <pattern> [paths…]`; `rft rewrite <pattern> <replacement> [paths…]`; `-C` root; `-l` optional lang filter; rewrite `-n`/`-i`/`-b` like plan/confirm/backup
- Fixtures: `testdata/pattern/` (`input/` + optional `expected/` + `op.json`), same idea as `testdata/mv/`
- Language-specific parse/extract/ref resolution stays in language packages; pattern core stays token + ref algebra

### Subcommand: edit
- Opens the definition of a reference in `$EDITOR` (or jumps to a file), blocking until the editor exits
- Argument modes:
  - no args: interactive picker over all entities under cwd, then open the selection
  - symbol ref (`::what`): canonicalize (barrels, re-exports, aliases — same as navigation elsewhere) then open the defining entity
  - file ref (no symbol): open that file at line 1 column 1
  - directory / module ref (no symbol): interactive picker scoped to atoms under that container, then open the selection
- Interactive picker:
  - default backend shells out to `fzf` (must be on PATH); pluggable later
  - candidates are streamed into fzf stdin as atoms are discovered (parse each file on demand; not a full-module ingest first)
  - each candidate line is the full reference string (e.g. `path:./pkg/foo.go::Bar`)
  - `-a`: include normally hidden symbols (same idea as `ls`)
- `-C` / `--dir`: project root (default `.`), same idea as `serve`
- Editor selection (first wins): `--editor` flag, `RFT_EDITOR`, `$VISUAL`, `$EDITOR`; hard error if none
- Editor argv (default, swappable via interface later): single argument `path:line:column`
  - line is 1-based; column is 1-based for the editor
  - definition position: atom `StartByte` converted with `grammar.LineIndex` from ccgo-tree-sitter (`LineColumnAt` is 1-based line / 0-based byte column; pass column+1 to the editor)
  - file-only open: `path:1:1`
- Success is silent on stdout; propagate the editor process exit code
- Hard errors (non-zero, clear stderr): unresolvable atom after canonicalize, missing file, missing editor, missing `fzf` when a picker is required
- Equivalent semantic of opening a target for editing in general; add flags on demand, don't invent useless flags

### Subcommand: serve
- Headless local HTTP code browser (same symbol hyperlinks / reference system as the CLI)
- `-l` / `--addr`: listen address (default `127.0.0.1:8080`)
- `-C` / `--dir`: project root (default `.`); no project-marker walk-up
- Prints a one-line banner on stdout (root, addr, openable URL); does not open a browser
- **Today:** Go `html/template` SSR (`pkg/web`), Seed → Materialize(ExpandImports=false) per annotated file
- **Target:** graph-model-centric SPA — see **Web browser (serve SPA)** below; desktop tracks the same UI

### Subcommand: desktop
- Same UI as `serve`, shelled via **[eletrocromo](https://github.com/lewtec/eletrocromo)** (Helium `--app` window)
- Wiring: `cmd/rft/desktop.go` only (no `internal/desktop` until it grows)
- `App.ID` = `br.tec.lew.refactree` (Helium profile isolation)
- `-C` / `--dir`: project root (default `.`), same semantics as `serve` — no walk-up
- No `--addr`: loopback bind and one-shot token auth are owned by eletrocromo
- Context: `cmd.Context()` (same pattern as `browse` / `lsp`)
- No refactree startup banner on stdout; do not swallow library output
- Hard fail if Helium/eletrocromo cannot start; **never** fall back to the system browser or to `serve --addr`
- **Out of scope (this cut):** not-a-TTY auto-launch rewrite, tray/background lifetime, deep-link args, Helium in CI
- Tests: no Helium window in refactree CI; exercise the UI via `serve` / package tests

## Concept: reference
- References something
- Format: provider:where::what
  - Example: function main located in src/main.py -> path:./src/main.py::main
- Semantic meaning of the language doesn't  matter much, we just care about links between things and making sure those are not broken

## Concept: provider
- Somewhere a code can be looked for
- `path` by default
- Examples:
  - `node`: looks up node_modules
  - `python`: looks up site_packages
  - `go`: looks up GOPATH and vendor
  - `java`: looks up package path under source roots (`src/main/java`, `src/test/java`, `src`, project root)
  - `rust`: looks up some rust cache somehow

## ProjectFS

Path-oriented file content abstraction for project reads (and later buffer overlays).

| Rule | Behavior |
|------|----------|
| Interface | Small reader: `ReadFile`, `Stat`, `ReadDir` (path-oriented, not a full `io/fs` rewrite of the tree) |
| CLI | Disk implementation |
| LSP | Overlay on disk: open-buffer text wins; closed files re-read from disk |
| Staging | In-memory overlay for transactional edits (no temp-dir materialization of the project) |
| Future | Editor buffers plug in as another FS implementation; drivers stay FS-agnostic |

Walk/parse/materialize prefer ProjectFS when provided; default remains disk so existing callers and fuzzy tests stay green without an LSP session.

## Language server (LSP)

Status: **specified; first dogfood cut in progress.** Same binary: `rft lsp` over stdio. Dogfood target: **Helix**. Product role: **complement** code intelligence — language-specific LSPs win when configured; put `rft` last in Helix `language-servers`. No proxy and no retry-on-empty: Helix first-wins for definition/hover/rename; merge for diagnostics/completion/symbols/code-action.

Identity: **code intelligence**, not a linter. Diagnostics at most high-confidence broken references; empty diags preferred over noise.

### First dogfood cut (definition of done)

| Capability | Behavior |
|------------|----------|
| `textDocument/definition` | Same as CLI navigation: symbol under cursor → real reference → **canonicalize** → definition location. Empty when unknown (no fake first-mention jump) |
| `textDocument/references` | Graph resolve for that atom (same rules as CLI/ref plumbing) |
| `textDocument/documentSymbol` | Entities from current file extract |
| `workspace/symbol` | Symbols from last-good project snapshot (prefix query) |
| `textDocument/hover` | Signature + docstring only (same truth as `rft doc`) |
| `textDocument/completion` | **Symbol-only** (Vim-style known names): document + workspace symbol index, prefix filter, capped. No snippets/keywords/omni |
| `textDocument/rename` | Structural plan (`Rename`) → in-memory stage → ref/coherence check → commit via workspace apply / `WorkspaceEdit`. **Transactional all-or-nothing** |
| `textDocument/publishDiagnostics` | At most broken-reference style signals; not a diagnostics product |

**Out of first dogfood cut:** code actions for full `mv`, semantic tokens, formatting, type inference, competing with gopls on type errors.

Languages: all registered drivers via existing interfaces; no per-language branches in the LSP package. Rename quality tracks move/rename planners; unsupported plans fail the request.

### Project model

| Rule | Behavior |
|------|----------|
| Unit of truth | One workspace root per server process |
| Root discovery | LSP `rootPath` / `RootURI` when present; else walk up for `.git` |
| Multi-project | No multi-root map in v1 |
| Open buffers | Overlay text wins over disk on recompute |
| Closed files | Disk on each recompute turn |
| Incremental index | Future; v1 uses snapshot swap (two-tier), not perfect incrementality |

### Recompute: two-tier + snapshot swap

```text
didChange / didSave
        │
        ├─ fast: re-extract dirty buffer(s) → document symbols / local truth immediately
        │
        └─ if dirty: debounce and/or save
                │
                ├─ extract/parse fails → keep last-good graph snapshot
                │
                └─ success → background Seed/Dir materialize (ctx-cancellable)
                             atomic swap of last-good snapshot
```

| Rule | Behavior |
|------|----------|
| Triggers | Debounce (default ~300ms) and save; only when dirty |
| Overlap | Cancel + restart via `context.Context` |
| Full Dir Materialize | When workspace symbols / project-wide refs / rename need a closed graph |
| Fast path | Hop/Seed-style dirty file extract; not full-project on every keystroke |
| node_modules / vendors | Bare imports resolve via **`node` provider** (then usually materialize as `path:./node_modules/…`). Dir walk still skips vendor trees. **Navigation** (definition / hover / refs) may **on-demand** Hop/Seed + one-hop ExpandImports around the target (incl. package reexports). **Refactor** (rename/mv) stays on the closed project graph until explicitly designed otherwise — no deep vendor crawl just because something was navigated |

### Mutations (rename)

1. Build structural plan (same planner as CLI `mv` rename path).
2. Apply on **in-memory** staged ProjectFS (copy-on-write overlay).
3. Re-check graph/refs on staged content.
4. Commit only if the gate passes: emit client `WorkspaceEdit` / workspace apply, or write disk through the FS commit path.
5. Fail closed on incomplete/unsupported plans — no partial file writes.
6. **Fuzzy / CLI:** keep using the shared plan + apply core; do not invent an LSP-only planner that drifts. Disk `ApplyEdits` remains valid for tests; transactional stage+verify is the preferred path for LSP (and may wrap apply without changing fuzzy entrypoints).

### Protocol stack

| Choice | Detail |
|--------|--------|
| Modules | `go.lsp.dev/protocol` + `go.lsp.dev/jsonrpc2` (same as contapila) |
| Server API | Embed `protocol.UnimplementedServer`; `protocol.NewServer` |
| Isolation | Protocol types only under `internal/lsp`; ingest stays free of LSP types |
| Stdio | stdout = protocol only; stderr = `log/slog` |
| Not | glsp-as-framework, hand-rolled framing, gopls `internal/*` |

### Positions

Tree-sitter / line index are **byte** offsets; LSP positions are commonly **UTF-16**. Convert at the LSP boundary (`grammar.LineIndex` + UTF-16 column mapping).

### Testing

| Layer | Method |
|-------|--------|
| Unit | ProjectFS overlay, stage→verify helpers, symbol-at-position without RPC |
| Integration | In-memory `jsonrpc2` client ↔ server on fixtures (definition, refs, hover, completion, symbols, rename) |
| Acceptance | Helix dogfood + example `languages.toml` snippet |
| Fuzzy | Continues to exercise shared plan/apply/materialize — not the JSON-RPC shell |

### Client packaging

Helix: define `[language-server.rft]` with `command = "rft"` / `args = ["lsp"]`, append `rft` **after** language-specific servers so they win on first-wins features. Ship an example under `testdata/lsp/` or docs.

### Architecture seams

| Seam | Intent |
|------|--------|
| ProjectFS | One read path for CLI disk and LSP overlays |
| Surfaces | LSP consumes spine APIs (`WalkExtracts`, `Materialize`, `Canonicalize*`, `DocFor`, `Rename`, …) |
| Context | Cancel in-flight slow rebuilds without publishing stale snapshots |

## Web browser (serve SPA)

Status: **specified; first cut in progress** (see serve SPA implementation). Replaces template-centric `rft serve` with a graph-model-centric SPA. First cut is **lazy-only**; eager analyses are backlog. `desktop` uses the same UI once serve ships it.

### Split of responsibility

| Side | Owns | Must not own |
|------|------|----------------|
| Backend | Discovery, resolve, **facts**: atoms / files / modules, parents, edge kinds; lazy Seed-style work by default | Layout, gravity, camera, animation, pixels |
| Frontend | **All dataviz**: force simulation, positions, zoom chrome, drill UX, what to draw | Source of truth for “what exists in the project” |

Shared **perception** means shared **ids, schema, and relation facts** — not shared layout. The picture of the graph is entirely a client artifact.

### Domain levels (existing lattice; product wording)

### Glossary

| Term | Meaning |
|------|---------|
| **Module** | Language loadable unit |
| **File** | One parse unit (filesystem / structure; not a force-graph edge kind) |
| **Atom** | One defined named thing; id `provider:path::name` |
| **Reference** | Canonical id string for any level |
| **Name** | The `::` segment of a reference (`Reference.Name`) |
| **Use** | Use-site → target atom fact (ingest `Use`; graph `USES`) |
| **Alias** | Import/reexport binding (feeds module `IMPORTS`) |

Do not use **package** or **symbol** as product lattice levels (package remains a Go/Java language unit / fuzzer grain; LSP protocol keeps `*Symbol*` names).

Three levels already exist in ingest/browse (`DirectoryModule`, path vs `::name`, scope navigation). Product names:

| Level | Meaning | Graph edges at this zoom |
|-------|---------|---------------------------|
| **Module** | Language loadable unit (Go/Java: package dir when `DirectoryModule`; Python/JS: often the file — driver decides; collapse when module ≡ file) | Import / depends-on between modules |
| **File** | One source/parse unit | **None** — filesystem list/rail (today-like), not a force graph |
| **Atom** | One defined thing (`provider:path::name` / ingest `Atom`) | Uses / used-by from `Use` (and reverse as query direction) |

- **Drill up/down** between levels (zoom), not three permanent node layers drawn at once.
- **Containment** (module→file→atom) is **structure** (parent / cluster / hull), **not** a default walk edge. Do not treat “lives in file” as a use edge that pollutes the force graph.
- Language-specific module boundaries stay in language packages / providers (`LanguageUsesDirectoryModule`, `ResolveScopeTarget`, …).

### UX shell

| Rule | Behavior |
|------|----------|
| Default entry | **Filesystem at project root** (cold start like today) — not a full-repo module hairball |
| Primary refactor | Whole serve view re-centered on the graph data model; fs + code are views of that model |
| Module zoom | Force graph of modules + import edges |
| File level | Filesystem navigation only (no force edges) |
| Atom zoom | Force graph of atoms + use/used-by edges around focus |
| Deep links | Keep `/` and `/code/...` (History API SPA); same reference URLs as today |
| Graph renderer | **`react-force-graph-2d`** (positions stay in the client sim) |

### Loading: lazy vs eager

Same spine rules as the rest of the product: lazy by default; eager only when a query claims completeness.

| Mode | Queries | Spine |
|------|---------|--------|
| **Lazy (first cut)** | Focus neighborhood, optional bounded module-import expand, filesystem list; relation delivery may be incremental | Seed / Hop-style; partial OK; UI must not claim completeness |
| **On demand** | Annotated **code**, **docs** | Separate GraphQL fields/ops — not on the relation stream |
| **Eager (backlog)** | **Backlinks** (complete used-by), **dead code** (unreferenced in closed project graph) | Full Dir Materialize (ExpandImports as required for closed truth) |

- **Focus-driven expansion (v1):** changing focus loads that ref’s neighborhood (about 1 hop). Client merges facts into a growing store; force layout keeps running as nodes appear. Other triggers (prefetch, viewport) are later iteration.
- Lazy browse perception is **open/incomplete**. Never use a half-stream as authority for refactor/`mv`.
- Streamy payload priority: **relations/edges** (plus node stubs: id, kind, label, parent). Code references and docs load later via their own queries.
- Optional GraphQL `@stream` / incremental delivery is an enhancement when the Go stack supports it — not a day-one gate. Batch neighborhood + client animation is enough for first feel.

### Stack (first implementation PR)

| Piece | Choice |
|-------|--------|
| UI | **React + Relay** from day one |
| Styling | **Tailwind CSS 4** + **daisyUI 5** (semantic components/colors; custom `refactree` dark amber theme) |
| API | **GraphQL**, **gqlgen** (schema-first) |
| Global id | **Canonical reference string** is the Relay/Node id (run canonicalize before minting ids); expose the same string for CLI/URL parity |
| JS toolchain | **Bun**; `package.json` at **repo root** |
| Build output | Bun → **`pkg/web/dist/`**; Go **`//go:embed`s `dist`** into the serve binary |
| Tooling | **`mise` `codegen:*`** (gqlgen + Relay compiler / related); `frontend:build` / `frontend:dev` |
| Dev | Bun/Vite-style dev server may proxy GraphQL to `rft serve`; prod is embed-only |
| Graph viz | **`react-force-graph-2d`** (canvas paints may stay theme-fixed; chrome uses daisyUI) |

### GraphQL surface (intent)

- Operations for: filesystem listing, focus **neighborhood** (nodes + relations), on-demand **code** / **doc**, later eager **backlinks** / **dead code**.
- Node kinds align with module / file / atom; edges carry a kind (import, uses, used-by, …).
- Responses for lazy ops may mark incompleteness; eager ops fail closed or report progress rather than silently partial.

### First cut — in scope

- SPA shell embedded in `rft serve` (and thus `desktop`)
- GraphQL + gqlgen + Relay wiring
- Filesystem default entry; focus-driven lazy relation graph
- Module map + atom ego via `react-force-graph-2d`
- Code (and docs if cheap) as separate on-demand queries
- Routes `/`, `/code/...`, GraphQL endpoint, embedded assets

### First cut — out of scope / backlog

| Item | Notes |
|------|--------|
| Eager **backlinks** UI | Closed used-by for a focus |
| Eager **dead code** UI | Project-wide unreferenced atoms |
| Dual-focus “path between A and B” | Optional later investigation mode |
| Live server graph session | Long-lived backend crawl session; prefer stateless focus queries + client merge first |
| Server-side layout | Forbidden |
| File-level force graph | Forbidden (fs view only) |
| Full-repo graph as homepage | Forbidden as default |
| sigma.js / other renderers | Upgrade path if 2d force-graph limits hit; not day one |
| ~~Product wording janitor~~ | **Done:** Module/File/Atom + `Use` across core, GraphQL, UI, fixtures |

### Architecture seams

| Seam | Intent |
|------|--------|
| Ingest spine | Serve GraphQL resolvers call WalkExtracts / Seed / Materialize / Canonicalize* — no second discovery stack |
| Relay store | Normalized **facts**; force-graph holds **positions** |
| Language packages | Module boundary and resolve rules stay behind existing interfaces |
| Embed | One binary: `rft serve` does not require a separate node process in prod |
