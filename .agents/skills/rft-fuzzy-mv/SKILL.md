---
name: rft-fuzzy-mv
description: Make refactoring implementation more exhaustive, complete and stable by fuzzing mv on repositories in the wild with isolated project checks via the internal/fuzzy harness (go test, not rft CLI).
---

Shared harness notes: [`references/harness.md`](references/harness.md)
Catalog index: [`references/projects.md`](references/projects.md)
Fixtures: `testdata/mv/` on `class=bug`; ingest phase uses `testdata/ingest/` (`ingestor-fixtures`).

# Process
1. Warm (online, once): `mise run fuzzy:prefetch`
2. Run suite: `mise run fuzzy:run`  
   Catalog canvas / `FuzzMvOneOp` seeds need step 1. Mutator campaign: `FUZZTIME=30s mise run fuzzy:run`
3. Or drive from Go: `fuzzy.Run` / `fuzzy.PrefetchOnce` (see `harness.md`).
4. On `class=bug`: curate `testdata/mv/...` from scaffolds, fix `pkg/ingest`, re-run.

# Airgapped
```bash
RFT_FUZZY_WORK_ROOT=/var/cache/rft-fuzzy mise run fuzzy:prefetch
# unplug network, then:
RFT_FUZZY_WORK_ROOT=/var/cache/rft-fuzzy mise run fuzzy:run
```
