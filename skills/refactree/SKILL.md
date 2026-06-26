---
name: refactree
description: Teach the model how to use refactree, a CLI tool for structural refactoring and symbol querying.
license: MIT
---

# refactree (rft) Skill

`refactree` (CLI: `rft`) is a structural refactoring and symbol querying tool. It operates on code structure using tree-sitter parsers rather than plain text search, allowing for precise queries and safe structural changes like renaming and moving items while automatically updating references and imports.

## Progressive Disclosure & Best Practices

When using `refactree`, follow a progressive approach to ensure safety and correctness:

1. **Discover First**: Always use `rft ls` to verify the structure, discover symbols, and ensure exact spelling.
2. **Review Documentation**: Use `rft doc` to review the symbol's contract and documentation before deciding to refactor.
3. **Plan & Review**: When preparing to move or rename with `rft mv`, always pass the `-i` flag to generate and review the edit plan before applying.
4. **Execute Safely**: Pass the `-b` (backup) flag when applying structural changes to allow easy rollbacks if needed.
5. **Respect Scope**: `rft` handles structural changes (updating references and imports) and does not alter internal business logic. Use it exclusively for structural operations.

## Core Concepts

### The Reference Syntax
All operations in `rft` target **references**, which point to specific symbols.
Format: `[provider:]where::what`
- `provider`: (Optional) The lookup strategy. Defaults to `path`. Examples: `node` (node_modules), `python` (site_packages), `go` (GOPATH/vendor).
- `where`: The file path or module.
- `what`: The symbol name (e.g., function, class, variable).

**Example:**
`path:./src/main.py::main` -> Targets the `main` function in `./src/main.py`.

## Commands

All subcommands support `-v` or `--verbose` for detailed verbose/debug logging.

### `rft ls` - List Symbols
Lists all symbols inside a given reference (e.g., a file or a class).
**Usage:**
```bash
rft ls path:./src/utils.py::
```
**Flags:**
- `-a`: Show normally hidden/private symbols (e.g., symbols starting with `_` in Python or unexported lowercase symbols in Go).
- `-l`: Output in a structured text/table format instead of standard output.

### `rft doc` - Retrieve Documentation
Extracts the docstring and (if applicable) the signature of a symbol.
**Usage:**
```bash
rft doc path:./src/math.py::calculate_total
```
**Output Format:**
```markdown
# calculate_total
Signature: (items: list[int]) -> int
Calculates the sum of all items.
```

### `rft mv` - Move and Rename
Safely renames a symbol or moves it to another location. It analyzes the codebase, finds all references, and updates imports/usages automatically without altering the actual logic.

**Renaming a symbol:**
```bash
rft mv path:./src/user.py::getUser path:./src/user.py::get_user
```

**Moving a symbol to another file:**
```bash
rft mv path:./src/helpers.py::format_date path:./src/date_utils.py::format_date
```
**Flags:**
- `-i`: Interactive mode. Generates an edit plan showing all planned operations and prompts for user confirmation before applying.
- `-b` / `--backup`: Creates a `.bak` copy of any modified files before writing changes.
