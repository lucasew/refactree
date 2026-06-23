# refactree

Tool to do queries on symbols and some refactorings like move and rename items in modules


## CLI: rft
- Out of scope: preprocessing index, all findings are done on time, like how ripgrep and fd works
- Refactorings must check for every reference and referee and make a plan that makes the code work the same way. We don't alter logic, only structure
- Parsers for each language give a list of things and usages
  - import os; os.system("Hello, world") -> Uses the python lookup mechanism to find the implementation of "os", then the implementation of "system" and add the edge on the graph
  - Parsers are made based on the tree-sitter conversion to Golang: https://github.com/lucasew/ccgo-tree-sitter/
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
  - `rust`: looks up some rust cache somehow
