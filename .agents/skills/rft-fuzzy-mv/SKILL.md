---
name: rft-fuzzy-mv
description: Make refactoring implementation more exhaustive, complete and stable by fuzzing mv on repositories in the wild with isolated project checks via the internal/fuzzy harness (go test, not rft CLI).
---

Shared harness notes: [`references/harness.md`](references/harness.md)
Catalog index: [`references/projects.md`](references/projects.md)
Fixtures: `testdata/mv/` on `class=bug`; ingest phase uses `testdata/ingest/` (`ingestor-fixtures`).

# Process
1. Warm caches (online, once):
   ```bash
   mise run fuzzy:prefetch
   # or: RFT_FUZZY_WARMUP=1 go test ./internal/fuzzy -run '^TestPrefetchWarmup$' -count=1 -timeout 0 -v
   ```
2. Drive the harness from tests or a small Go main via `fuzzy.Run` / `fuzzy.PrefetchOnce` (see `harness.md`).
3. Local fixture smoke (no catalog network):
   ```bash
   go test ./internal/fuzzy -run 'TestRunLocalIngestAndMv|TestPrefetchThenOfflineIngest' -count=1 -v
   ```
4. On `class=bug`: curate `testdata/mv/...`, fix `pkg/ingest`, re-run with the same seed.

# Airgapped
```bash
RFT_FUZZY_WARMUP=1 RFT_FUZZY_WORK_ROOT=/var/cache/rft-fuzzy \
  go test ./internal/fuzzy -run '^TestPrefetchWarmup$' -count=1 -timeout 0 -v
# unplug network, then offline fuzzy.Run against the same work-root
```
