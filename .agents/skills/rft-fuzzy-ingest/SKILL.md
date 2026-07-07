---
name: rft-fuzzy-ingest
description: Run real-world ingest invariant checks via the internal/fuzzy harness (go test), then curate failures into testdata/ingest fixtures.
---

Shared harness notes: [`../rft-fuzzy-mv/references/harness.md`](../rft-fuzzy-mv/references/harness.md)
Catalog index: [`../rft-fuzzy-mv/references/projects.md`](../rft-fuzzy-mv/references/projects.md)
Fixtures: `testdata/ingest/` (see `ingestor-fixtures` skill).

# Process
1. Prefer a warm work-root: `mise run fuzzy:prefetch` / `TestPrefetchWarmup`.
2. Run ingest via `fuzzy.Run` with `Mode: fuzzy.ModeIngest` (and `Offline: true` after warmup), or local invariant tests under `internal/fuzzy`.
3. On `class=bug`: curate `testdata/ingest/<lang>_.../` with `ingestor-fixtures`, fix the language package, rerun.
