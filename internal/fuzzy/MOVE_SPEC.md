# Move fuzzer SPEC

## Terminology

| Term | Meaning |
|---|---|
| **Grain** | Level in the language module lattice (what kind of node is moved) |
| **Declaration** | Named symbol definition (function, type, constant, …) |
| **Module** | Language import/visibility unit (Go package directory; Java package; JavaScript file; Python module file or package directory) |
| **Layout** | File placement *inside* a module (Go: which `.go` file holds a declaration) |
| **Placement** | Where the node goes: module destination × layout destination × name destination |
| **Move intent** | Structured choice before materializing references |
| **Source reference / destination reference** | `path:…` strings passed to `ingest.Rename` |
| **Move plan** | Logged plan: placement + source reference + destination reference |
| **Plan input** | Minimizable integer decision surface for the fuzzer |

### Grain names

| Grain | Used when |
|---|---|
| `declaration` | Move or rename a symbol-level node |
| `module` | File-as-module languages (JavaScript; Python module file) |
| `package` | Directory/package grain (Go package; Java package; Python package directory) |

### Placement names

| Placement | Meaning |
|---|---|
| `rename` | Same module, same layout file, new name |
| `layout` | Same module, different or new layout file, keep name (no import boundary crossed) |
| `module` | Different existing module, keep name |
| `new_module` | New empty module, keep name |
| `package` | Package-grain relocate (whole package directory tree) |

## Plan input

```text
GrainIndex      // into allowed grains for the project language
SourceIndex     // into ListNodes(grain)
PlacementIndex  // into placement menu for that grain
PeerIndex       // which existing peer module or layout file
Entropy         // new names / new module path suffixes
```

## Placement matrix (declaration grain)

| Module destination | Layout destination | Name destination | Placement |
|---|---|---|---|
| same | same | new | `rename` |
| same | existing or new | keep | `layout` |
| existing | * | keep | `module` |
| new | new | keep | `new_module` |

Rename combined with container change in one plan is out of scope.

## Package grain

| Module destination | Placement |
|---|---|
| new path | `package` |

## Module grain (file-as-module)

Relocate the whole file module (`module` or `new_module` placements) via path references without symbols.
