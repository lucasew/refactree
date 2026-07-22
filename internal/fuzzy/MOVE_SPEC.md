# Move fuzzer SPEC

## Terminology

| Term | Meaning |
|---|---|
| **Grain** | Level in the language module lattice (what kind of node is moved) |
| **Atom** (grain) | Named symbol definition (function, type, constant, …) |
| **Module** | Language import/visibility unit (Go package directory; Java package; JavaScript file; Python module file or package directory) |
| **File** (placement) | File placement *inside* a module (Go: which `.go` file holds an atom) |
| **Placement** | Where the node goes: module destination × layout destination × name destination |
| **Move intent** | Structured choice before materializing references |
| **Source reference / destination reference** | `path:…` strings passed to `ingest.Rename` |
| **Move plan** | Logged plan: placement + source reference + destination reference |
| **Plan input** | Minimizable integer decision surface for the fuzzer |

### Grain names

| Grain | Used when |
|---|---|
| `atom` | Move or rename an atom (defined name) |
| `module` | File-as-module languages (JavaScript; Python module file) |
| `package` | Directory/package grain (Go package; Java package; Python package directory) |

### Placement names

| Placement | Meaning |
|---|---|
| `rename` | Same module, same layout file, new name |
| `file` | Same module, different or new file, keep name (no import boundary crossed) |
| `module` | Different existing module, keep name |
| `new_module` | New empty module, keep name |
| `package` | Package-grain relocate (whole package directory tree) |

## Product mapping

| Fuzzer | Product lattice |
|--------|-----------------|
| grain `atom` (was declaration) | **Atom** |
| placement `file` (was layout) | **File** inside a multi-file **Module** |
| grain `module` | file-as-module **Module** relocate |
| grain `package` | directory **Module** (Go/Java package dir) relocate |

## Plan input

```text
GrainIndex      // into allowed grains for the project language
SourceIndex     // into ListNodes(grain)
PlacementIndex  // into placement menu for that grain
PeerIndex       // which existing peer module or layout file
Entropy         // new names / new module path suffixes
```

## Placement matrix (atom grain)

| Module destination | Layout destination | Name destination | Placement |
|---|---|---|---|
| same | same | new | `rename` |
| same | existing or new | keep | `file` |
| existing | * | keep | `module` |
| new | new | keep | `new_module` |

Rename combined with container change in one plan is out of scope.

## Package grain

| Module destination | Placement |
|---|---|
| new path | `package` |

## Module grain (file-as-module)

Relocate the whole file module (`module` or `new_module` placements) via path references without symbols.

## Language families

Families group honest language ids that share a module lattice. See `ingest.Family*`.
Catalog projects select by **`family`** (not a single language id): `family = "ecma"`, `family = "jvm"`, etc.

### ECMA (`ingest.FamilyECMA`)

Catalog: **`family = "ecma"`**. Surfaces under this family share the file-as-module lattice
(import/export). Vue/Astro remain out of scope.

| Language id | Extensions | Tree-sitter grammar |
|---|---|---|
| `javascript` | `.js` `.mjs` `.cjs` `.ts` `.tsx` `.jsx` | javascript / typescript / tsx |
| `svelte` | `.svelte` | svelte (script bodies re-parsed as ECMA) |

Shared: module lattice (file / SFC = module), import resolve, move driver (`ecmaMoveModel`), extract.

### JVM (`ingest.FamilyJVM`)

| Language id | Extensions | Status |
|---|---|---|
| `java` | `.java` | implemented |
| `kotlin` | `.kt` / `.kts` | **not registered** (platform ready; needs grammar + surface) |

Shared lattice (fuzzer `jvmMoveModel`): package directory = module; atom = types/members; file = layout within package module.

Catalog: **`family = "jvm"`**. When Kotlin is added, register language id `kotlin` with
`Family: FamilyJVM` — do not invent a separate catalog family for Kotlin.
