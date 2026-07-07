---
name: rft-fuzzy-ingest
description: Run real-world ingest invariant checks via the internal/fuzzy harness (go test), then curate failures into testdata/ingest fixtures.
---

Shared harness notes: [`../rft-fuzzy-mv/references/harness.md`](../rft-fuzzy-mv/references/harness.md)
Catalog index: [`../rft-fuzzy-mv/references/projects.md`](../rft-fuzzy-mv/references/projects.md)
Fixtures: `testdata/ingest/` (see `ingestor-fixtures` skill).

# Process
1. Prefer a warm work-root: `mise run fuzzy:prefetch`.
2. Run: `mise run fuzzy:run`, or `fuzzy.Run` with `Mode: fuzzy.ModeIngest` (`Offline: true` after warmup).
3. On `class=bug`: curate `testdata/ingest/<lang>_.../` with `ingestor-fixtures`, fix the language package, rerun.
