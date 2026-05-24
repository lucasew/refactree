---
name: ingestor-fixtures
description: Maintain `refactree` ingestor test fixtures under `testdata/ingest/`. Use when adding fixture cases, updating fixture source files, editing `expected.json`, or checking whether ingestor fixtures follow the repo standard.
---

# Ingestor Fixtures

This skill maintains the fixture standard for ingestor tests in this repo.

## Scope

Use this skill when you need to:

- add a new ingestor fixture
- update fixture source files
- update `expected.json`
- review whether a fixture still follows the expected schema

Do not use this skill for:

- writing the ingestor implementation
- writing the test harness
- changing unrelated CLI behavior

## Fixture Layout

All ingestor fixtures live under `testdata/ingest/`.

Each fixture is one directory:

- directory name must start with the language name
- fixtures may contain multiple source files
- each fixture directory must contain one `expected.json`

Example:

```text
testdata/ingest/go_basic_cross_file/
  main.go
  helper.go
  expected.json
```

## Naming Rules

- Start each fixture name with the language name: `go_...`, `python_...`, `javascript_...`
- Prefer short names that describe one behavior
- Prefer minimal multi-file fixtures that isolate one resolution case

Good examples:

- `go_basic_cross_file`
- `python_import_call`
- `javascript_named_import`

## Source File Rules

- Keep fixture source code minimal
- Include only the files needed to demonstrate the resolution behavior
- Prefer one behavior per fixture
- Do not add extra syntax just to make the example look realistic

## Expected JSON Standard

`expected.json` must contain:

- `files`
- `entities`
- `aliases` when imports bind names to file scope
- `relations`

### `files`

Each file entry contains:

- `language`
- `path`

Use relative paths within the fixture directory.

### `entities`

Each entity entry contains only:

- `reference`
- `start_byte`
- `end_byte`

Rules:

- `reference` uses the `SPEC.md` provider notation
- entity references use the full symbol form: `path:./file.ext::symbol`
- do not include `kind`
- do not include `name`
- do not include `file`

Example:

```json
{
  "reference": "path:./helper.go::helper",
  "start_byte": 19,
  "end_byte": 25
}
```

### `relations`

Each relation entry contains only:

- `reference`
- `start_byte`
- `end_byte`
- `target`

Rules:

- `target` is always a full symbol reference: `path:./file.ext::symbol`
- `reference` describes where the claim lives
- for top-level claims, `reference` is file-only: `path:./app.py`
- for claims inside a function, `reference` uses the scoped form: `path:./main.go::main`
- do not include `kind`
- do not include separate `scope`

Example:

```json
{
  "reference": "path:./main.go::main",
  "start_byte": 29,
  "end_byte": 35,
  "target": "path:./helper.go::helper"
}
```

### `aliases`

Use `aliases` when an import binds a name to a file-scope reference.

Each alias entry contains only:

- `reference`
- `start_byte`
- `end_byte`
- `target`

Rules:

- `reference` is the file where the alias is introduced
- `target` is the file-scope reference the alias points to
- use this for import aliases and namespace/package aliases
- do not encode import aliases as generic relations when the alias binding itself is the thing being tested

Example:

```json
{
  "reference": "path:./app.py",
  "start_byte": 18,
  "end_byte": 19,
  "target": "path:./helpers.py"
}
```

## Workflow

When creating or updating a fixture:

1. Inspect existing fixtures in `testdata/ingest/` first
2. Keep the new fixture minimal and language-prefixed
3. Add or update the source files
4. Verify the relevant byte offsets against the real source
5. Update `expected.json` by hand
6. Re-check that the references follow `SPEC.md`

## Byte Offset Verification

Prefer verifying offsets from actual parser output, not by guessing.

Use:

```bash
go run ./cmd/rft astdump <language> <file>
```

This is especially important when updating:

- entity name spans
- import name spans
- call target spans

## Biases

- Simplest fixture first
- Explicit JSON over clever abstractions
- Hand-authored expectations over generated snapshots
- Small isolated cases over broad integration examples
