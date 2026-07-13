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

### Subcommand: ls
- Lists symbols in a reference
- `-a`: List symbols normally hidden symbols, like symbols that start lower case on Golang or start with _ in python
- `-l`: Use text/table
- Equivalent semantic of ls in general, make sure to not be confusing, add flags on demand, don't invent useless flags or use flags that  dont make sense in the problem

### Subcommand: mv
- Renames symbols in a reference (variables, functions, classes)
- Moves symbols in a reference to another place (like a function to another file, updating imports)
- Does all the necessary checks before editing any files looking for references
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
  - each candidate line is the full reference string (e.g. `path:./pkg/foo.go::Bar`)
  - `-a`: include normally hidden symbols (same idea as `ls`)
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
