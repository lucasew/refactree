Shared notes for `rft-fuzzy-ingest` and `rft-fuzzy-mv`. Mode-specific steps stay in each skill.

The harness lives only under `internal/fuzzy` (not linked into the `rft` binary). Drive it with `go test` / `fuzzy.Run` / `fuzzy.PrefetchOnce`.

# Restrictions
Setup/check run in Docker by default via testcontainers. Host path: `RFT_FUZZY_NO_ISOLATE=1` (prefetch Once always sets allow for host).

# Source of truth
- Catalog: `testdata/fuzzy/projects.toml` (`[projects.<slug>]` + `[projects.<slug>.mise]`)
- Human index: `references/projects.md`
- Harness API: `internal/fuzzy` (`Run`, `PrefetchOnce`, `DefaultWorkRoot`)
- Work root: `RFT_FUZZY_WORK_ROOT` or `$TMPDIR/rft-fuzzy` (`cache/`, `mise-data/`, `preserve/`, `manifest.json`, `runs/`)
- Isolation: Docker default (`DefaultMiseImage` digest pin); ingest/mv always run on the host
- Worktrees: `prefetch` → `runs/<slug>/prefetch`; `ingest` → `runs/<slug>/ingest`; `mv`/`run` → `runs/<slug>/<seed>`

# Warmup then use
1. Online (once per machine / work-root):
   ```bash
   # mise:
   mise run fuzzy:prefetch
   # or:
   RFT_FUZZY_WARMUP=1 RFT_FUZZY_WORK_ROOT=/var/cache/rft-fuzzy \
     go test ./internal/fuzzy -run '^TestPrefetchWarmup$' -count=1 -timeout 0 -v
   ```
   Optional: `RFT_FUZZY_PROJECT=astro,workspaced`, `RFT_FUZZY_NO_ISOLATE=1`.

   `TestPrefetchWarmup` calls `PrefetchOnce` (`sync.Once`): fills work-root, pulls images when isolating, writes `manifest.json`, verify-offline by default.

2. Same process can call `PrefetchOnce` again; it no-ops after the first success/failure.

3. Offline catalog runs: call `fuzzy.Run` with `Offline: true` and `WorkRoot: fuzzy.DefaultWorkRoot()` (or the path from step 1). Local unit tests use per-test temp work-roots and do not need warmup.

# Env
| Env | Purpose |
| --- | --- |
| `RFT_FUZZY_WARMUP=1` | Enable `TestPrefetchWarmup` (skipped otherwise) |
| `RFT_FUZZY_WORK_ROOT` | Durable work-root |
| `RFT_FUZZY_NO_ISOLATE=1` | Host setup/check |
| `RFT_FUZZY_PROJECT` | Comma-separated project slugs for prefetch Once |
| `RFT_FUZZY_ALLOW` | Still used by `Run` when `NoIsolate` without Once's forced allow |

# API sketch
```go
root, err := fuzzy.PrefetchOnce(ctx) // once per process
res, err := fuzzy.Run(ctx, fuzzy.Options{
  Mode: fuzzy.ModeRun, WorkRoot: root, Offline: true,
  Iterations: 10, Seed: 1, Allow: true, /* NoIsolate if host */
})
```
