Shared notes for `rft-fuzzy-ingest` and `rft-fuzzy-mv`. Mode-specific steps stay in each skill.

# Restrictions
Setup/check run in Docker by default via testcontainers. Pass `--no-isolate` to run setup/check on the host (requires `RFT_FUZZY_ALLOW=1` / `--allow` / `CI=true` on a non-ephemeral host).

# Source of truth
- Catalog: `testdata/fuzzy/projects.toml` (`[projects.<slug>]` + `[projects.<slug>.mise]`)
- Human index: `references/projects.md` (from this skill dir; ingest skill links here via `../rft-fuzzy-mv/references/projects.md`)
- Harness: `internal/fuzzy`, CLI `rft fuzzy {prefetch,ingest,mv,run}`
- Work root: `--work-root` / `RFT_FUZZY_WORK_ROOT` (`cache/`, `mise-data/`, `preserve/`, `manifest.json`, `runs/`)
- Isolation: Docker default (`DefaultMiseImage` digest pin in `internal/fuzzy`); ingest/mv always run on the host
- Worktrees: `prefetch` uses stable `runs/<slug>/prefetch`; `ingest` uses `runs/<slug>/ingest`; `mv`/`run` use `runs/<slug>/<seed>`

# Airgapped flow
1. Online (idempotent, all projects if no `--project`):
   ```bash
   go run ./cmd/rft fuzzy prefetch --work-root /var/cache/rft-fuzzy
   ```
   This fills work-root and, with Docker, `docker pull`s pinned session/cleanup images into the **local daemon** (images are not stored under work-root).
2. Disable network (same machine) or stay offline.
3. Offline loop with the same `--work-root`:
   ```bash
   go run ./cmd/rft fuzzy run --work-root /var/cache/rft-fuzzy --offline --iterations 10
   ```
4. Host-only (no Docker): add `--no-isolate --allow` (or `RFT_FUZZY_ALLOW=1`) on both prefetch and offline runs. Host setup uses `work-root/mise-data`.

Prefetch writes `manifest.json` and verifies offline readiness by default (`--no-verify-offline` to skip). Offline preflight fails fast if git cache, preserve snapshot, mise-data, or local images are missing.

# Common flags
- `--project <slug>` (repeatable), `--work-root`, `--report-dir`, `--catalog`
- `--offline` work-root + local images only; not valid on `prefetch`
- `--no-verify-offline` (prefetch only) skip post-prefetch readiness check
- `--no-isolate` opt out of Docker; run setup/check on the host
- `--fail-fast` stop on first bug-class failure (also stops multi-project loops)
- override image per project: `[projects.<slug>.isolate] image = "..."`
