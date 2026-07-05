---
name: rft-fuzzy-mv
description: Make refactoring implementation more exhaustive, complete and stable by fuzzing mv on repositories in the wild with isolated project checks via the rft fuzzy harness.
---

# Restrictions
Setup/check run in Docker by default via testcontainers. Pass `--no-isolate` to run setup/check on the host (requires `RFT_FUZZY_ALLOW=1` / `--allow` / `CI=true` on a non-ephemeral host).

# Source of truth
- Catalog: `testdata/fuzzy/projects.toml` (`[projects.<slug>]` + `[projects.<slug>.mise]`)
- Human index: `references/projects.md`
- Harness: `internal/fuzzy`, CLI `rft fuzzy mv` / `rft fuzzy run` / `rft fuzzy prefetch`
- Fixtures: `testdata/mv/` on `class=bug`; ingest phase uses `testdata/ingest/` (`ingestor-fixtures`)
- Work root: `--work-root` / `RFT_FUZZY_WORK_ROOT` (`cache/`, `mise-data/`, `preserve/`)
- Isolation: Docker default (`DefaultMiseImage` digest pin in `internal/fuzzy`); ingest/mv run on the host

# Airgapped flow
1. Online (idempotent): `go run ./cmd/rft fuzzy prefetch --project <slug> --work-root /var/cache/rft-fuzzy`
2. Copy work-root + pinned `DefaultMiseImage` (unless using `--no-isolate`).
3. Offline: `go run ./cmd/rft fuzzy run --project <slug> --work-root /var/cache/rft-fuzzy --offline --iterations 10 --seed 1`
4. Host-only isolated env: add `--no-isolate --allow`.

# Process
1. Docker must be available unless `--no-isolate`.
2. Choose a project slug from the catalog.
3. Run:
   ```bash
   go run ./cmd/rft fuzzy run --project <slug> --iterations 10 --seed 1
   ```
   Setup/check share one Docker session (`mise install` then `mise run setup` / `mise run test`). Ingest/mv stay on the host.
4. Inspect `report:` (`events.jsonl`, `logs/`, `scaffold/` on failures).
5. On `class=bug`: curate `testdata/mv/...`, fix `pkg/ingest`, rerun with the same `--seed`.

# Flags
- `--project <slug>` (repeatable), `--iterations`, `--seed`, `--work-root`, `--report-dir`
- `--offline` use work-root caches only (run `prefetch` first)
- `--no-isolate` opt out of Docker; run setup/check on the host
- `--fail-fast`, `--strict-refs`, `--ops`
- override image per project: `[projects.<slug>.isolate] image = "..."`
