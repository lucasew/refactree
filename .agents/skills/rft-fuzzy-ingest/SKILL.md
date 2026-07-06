---
name: rft-fuzzy-ingest
description: Run real-world ingest invariant checks via the rft fuzzy harness on ephemeral hosts, then curate failures into testdata/ingest fixtures.
---

Shared harness notes: [`../rft-fuzzy-mv/references/harness.md`](../rft-fuzzy-mv/references/harness.md)
Catalog index: [`../rft-fuzzy-mv/references/projects.md`](../rft-fuzzy-mv/references/projects.md)
Fixtures: `testdata/ingest/` (see `ingestor-fixtures` skill). Ingest reuses `runs/<slug>/ingest`.

# Process
1. Docker must be available unless `--no-isolate`.
2. Choose a project slug from the catalog.
3. Run:
   ```bash
   go run ./cmd/rft fuzzy ingest --project <slug>
   ```
4. Inspect `report:` (`events.jsonl`, `logs/`).
5. On `class=bug`: curate `testdata/ingest/<lang>_.../` with `ingestor-fixtures`, fix the language package, rerun.

# Flags
Common flags in `harness.md`, plus:
- `--strict-refs` fail on dangling path targets

Offline example:
```bash
go run ./cmd/rft fuzzy ingest --project <slug> --work-root /var/cache/rft-fuzzy --offline
```
