---
name: rft-fuzzy-ingest
description: Run real-world ingest invariant checks via the rft fuzzy harness on ephemeral hosts, then curate failures into testdata/ingest fixtures.
---

Shared harness notes: [`../rft-fuzzy-mv/references/harness.md`](../rft-fuzzy-mv/references/harness.md)
Catalog index: [`../rft-fuzzy-mv/references/projects.md`](../rft-fuzzy-mv/references/projects.md)
Fixtures: `testdata/ingest/` (see `ingestor-fixtures` skill). Ingest reuses `runs/<slug>/ingest`.

# Process
1. Docker must be available unless `--no-isolate`.
2. Prefer a warm work-root: `go run ./cmd/rft fuzzy prefetch` (once online).
3. Choose a project slug from the catalog.
4. Run:
   ```bash
   go run ./cmd/rft fuzzy ingest --project <slug>
   ```
5. Inspect `report:` (`events.jsonl`, `logs/`).
6. On `class=bug`: curate `testdata/ingest/<lang>_.../` with `ingestor-fixtures`, fix the language package, rerun.

# Flags
Common flags in `harness.md`, plus:
- `--strict-refs` fail on dangling path targets

Airgapped example:
```bash
go run ./cmd/rft fuzzy ingest --project <slug> --work-root /var/cache/rft-fuzzy --offline
```
