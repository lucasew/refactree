Shared notes for `rft-fuzzy-ingest` and `rft-fuzzy-mv`. Mode-specific steps stay in each skill.

The harness lives only under `internal/fuzzy` (not linked into the `rft` binary).

# Desired model
1. **Prefetch** = make `work-root` offline-ready:
   - **no-op** if already warm (manifest + git pins + preserve + mise-data + local images when using Docker)
   - otherwise **only fill gaps** (per-project skip when warm; git fetch only if pin missing; docker pull only if image missing)
2. **Tests** run with `Offline: true` and `WorkRoot: fuzzy.DefaultWorkRoot()` (or the path returned by prefetch) so they use **only** that local cache.

# Mise tasks (only two)
```bash
mise run fuzzy:prefetch   # warm work-root (network + Docker unless RFT_FUZZY_NO_ISOLATE=1)
mise run fuzzy:run        # go test ./internal/fuzzy (FuzzMvOneOp skips if cold)
FUZZTIME=30s mise run fuzzy:run   # native mutator on catalog canvas
```

Second prefetch on the same work-root should print `prefetch: no-op (work-root warm)`.

API:
```go
root, err := fuzzy.PrefetchOnce(ctx) // mutex; retries after failure; no-op when warm
res, err := fuzzy.Run(ctx, fuzzy.Options{
  Mode: fuzzy.ModeRun, WorkRoot: root, Offline: true,
  Iterations: 10, Seed: 1, Allow: true, NoIsolate: /* host */,
})
```

# Go native fuzz (testing.F)
Canvas = **catalog** projects with `mv.enabled` (not `testdata/mv` fixtures).
Fixtures are **outputs**: curate `testdata/mv` / `testdata/ingest` from bug scaffolds
(`$TMPDIR/rft-fuzzy-fuzz-fail/…`). Shared surface: `fuzzy.PlanInput`.

# Source of truth
- Catalog: `testdata/fuzzy/projects.toml`
- Work root (everything on disk): `RFT_FUZZY_WORK_ROOT` or `$TMPDIR/rft-fuzzy`
  - `cache/`, `preserve/`, `runs/`, `mise-data/`, `reports/`
  - no separate `/tmp/rft-fuzzy-reports` or `/tmp/rft-fuzzy-mise-data`
- Isolation: Docker default; host via `RFT_FUZZY_NO_ISOLATE=1`

# Env
| Env | Purpose |
| --- | --- |
| `RFT_FUZZY_WARMUP=1` | Enable `TestPrefetchWarmup` |
| `RFT_FUZZY_WORK_ROOT` | Durable work-root |
| `RFT_FUZZY_NO_ISOLATE=1` | Host setup/check |
| `RFT_FUZZY_PROJECT` | Comma-separated slugs for prefetch |
