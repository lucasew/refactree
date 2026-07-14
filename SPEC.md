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
- Per-file unit: parse (tree-sitter / `ParseSource*`) → `FileExtract` (entities, imports, usages, reexports)
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
- Then **Resolve** → `*Result` (files, entities, aliases, relations)
- ExpandImports is **not** the default and is **not** part of WalkExtracts
- ExpandImports is **only** for full-Dir project-complete loads (mv / “whole project graph”)
- Seed and Hop materialize with ExpandImports **off** (Seed already grew the set via BFS; Hop is minimal)
- Resolve never runs on an open-ended live stream; no incremental half-graph visible to mv

### WalkSymbols
- Thin convenience over WalkExtracts (Dir or single-file scope) + visibility/provider filters → yield `SymbolInfo`
- Must not reimplement walk/parse. No Materialize, no ExpandImports, no relations
- Used by `ls` and `edit` picker

### Canonical public API (migrate all consumers)
- Prefer spine names: `WalkExtracts`, `Materialize`, drivers Dir/Seed/Hop
- Drop `Ingest` / `IngestWithRecursion` / `IngestForFile` as the long-term brand (call sites move to spine)
- `WalkSymbols` stays as the list convenience only

### Entry points (who is lazy vs eager)
- **ls**, **edit** picker: lazy — WalkExtracts / WalkSymbols only; stream entities as files parse
- **edit** open / navigation canonicalize: Hop → Materialize(ExpandImports=false) or equivalent minimal result → `CanonicalizeInResult`; not full Dir
- **doc**: minimal file/hop extract; not full project Materialize
- **mv**: eager on purpose — Dir WalkExtracts over project root (skip rules) → Materialize(**ExpandImports=true**) → plan from full `*Result`. Must not use list-only path
- **serve** / browse file annotate: Seed → Materialize(ExpandImports=false)
- **fuzzy / full-graph tests**: same as mv-style full Dir Materialize when the check needs the whole picture

### Non-goals / hard rules
- Do not put import expansion inside the list stream
- Do not make “lookup” or WalkSymbols full-project by default
- Do not maintain a second WalkDir implementation beside WalkExtracts
- Micro-dedup (shared ParseSource helpers, path joiners, etc.) stacks under this spine; it does not replace it

### Subcommand: ls
- Lists symbols in a reference (WalkSymbols / lazy extract stream)
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

### Subcommand: edit
- Opens the definition of a reference in `$EDITOR` (or jumps to a file), blocking until the editor exits
- Argument modes:
  - no args: interactive picker over all entities under cwd, then open the selection
  - symbol ref (`::what`): canonicalize (barrels, re-exports, aliases — same as navigation elsewhere) then open the defining entity
  - file ref (no symbol): open that file at line 1 column 1
  - directory / module ref (no symbol): interactive picker scoped to entities under that container, then open the selection
- Interactive picker:
  - default backend shells out to `fzf` (must be on PATH); pluggable later
  - candidates are streamed into fzf stdin as symbols are discovered (parse each file on demand; not a full-module ingest first)
  - each candidate line is the full reference string (e.g. `path:./pkg/foo.go::Bar`)
  - `-a`: include normally hidden symbols (same idea as `ls`)
- `-C` / `--dir`: project root (default `.`), same idea as `serve`
- Editor selection (first wins): `--editor` flag, `RFT_EDITOR`, `$VISUAL`, `$EDITOR`; hard error if none
- Editor argv (default, swappable via interface later): single argument `path:line:column`
  - line is 1-based; column is 1-based for the editor
  - definition position: entity `StartByte` converted with `grammar.LineIndex` from ccgo-tree-sitter (`LineColumnAt` is 1-based line / 0-based byte column; pass column+1 to the editor)
  - file-only open: `path:1:1`
- Success is silent on stdout; propagate the editor process exit code
- Hard errors (non-zero, clear stderr): unresolvable symbol after canonicalize, missing file, missing editor, missing `fzf` when a picker is required
- Equivalent semantic of opening a target for editing in general; add flags on demand, don't invent useless flags

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
