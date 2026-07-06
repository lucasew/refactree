Shared notes for `rft-fuzzy-ingest` and `rft-fuzzy-mv`. Mode-specific steps stay in each skill.

# Restrictions
Setup/check run in Docker by default via testcontainers. Pass `--no-isolate` to run setup/check on the host (requires `RFT_FUZZY_ALLOW=1` / `--allow` / `CI=true` on a non-ephemeral host).

# Source of truth
- Catalog: `testdata/fuzzy/projects.toml` (`[projects.<slug>]` + `[projects.<slug>.mise]`)
- Human index: `references/projects.md` (from this skill dir; ingest skill links here via `../rft-fuzzy-mv/references/projects.md`)
- Harness: `internal/fuzzy`, CLI `rft fuzzy {prefetch,ingest,mv,run}`
- Work root: `--work-root` / `RFT_FUZZY_WORK_ROOT` (`cache/`, `mise-data/`, `preserve/`, `runs/`)
- Isolation: Docker default (`DefaultMiseImage` digest pin in `internal/fuzzy`); ingest/mv always run on the host
- Worktrees: `prefetch` uses stable `runs/<slug>/prefetch`; `ingest` uses `runs/<slug>/ingest`; `mv`/`run` use `runs/<slug>/<seed>`

# Airgapped flow
1. Online (idempotent): `go run ./cmd/rft fuzzy prefetch --project <slug> --work-root /var/cache/rft-fuzzy`
2. Copy work-root and the pinned `DefaultMiseImage` (skip the image if using `--no-isolate`).
3. Offline on the airgapped host with the same `--work-root` and `--offline`.
4. Host-only env without Docker: add `--no-isolate --allow` (or `RFT_FUZZY_ALLOW=1`).

# Common flags
- `--project <slug>` (repeatable), `--work-root`, `--report-dir`, `--catalog`
- `--offline` use work-root caches only (run `prefetch` first); not valid on `prefetch`
- `--no-isolate` opt out of Docker; run setup/check on the host
- `--fail-fast` stop on first bug-class failure (also stops multi-project loops)
- override image per project: `[projects.<slug>.isolate] image = "..."`
