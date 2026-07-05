---
name: rft-fuzzy-ingest
description: Run real-world ingest invariant checks via the rft fuzzy harness on ephemeral hosts, then curate failures into testdata/ingest fixtures.
---

# Restrictions
Setup/check run in Docker by default via testcontainers. Pass `--no-isolate` to run setup/check on the host (requires `RFT_FUZZY_ALLOW=1` / `--allow` / `CI=true` on a non-ephemeral host).

# Source of truth
- Catalog: `testdata/fuzzy/projects.toml` (`[projects.<slug>]` + `[projects.<slug>.mise]`)
- Human index: `../rft-fuzzy-mv/references/projects.md`
- Harness: `internal/fuzzy`, CLI `rft fuzzy ingest` / `rft fuzzy prefetch`
- Fixtures: `testdata/ingest/` (see `ingestor-fixtures` skill)
- Work root: `--work-root` / `RFT_FUZZY_WORK_ROOT` (`cache/`, `mise-data/`, `preserve/`)
- Isolation: Docker default (`DefaultMiseImage` digest pin in `internal/fuzzy`); ingest runs on the host

# Airgapped flow
1. Online (idempotent): `go run ./cmd/rft fuzzy prefetch --project <slug> --work-root /var/cache/rft-fuzzy`
2. Copy work-root and the mise image to the airgapped host.
3. Offline: `go run ./cmd/rft fuzzy ingest --project <slug> --work-root /var/cache/rft-fuzzy --offline`
4. In an isolated env without Docker: add `--no-isolate --allow` (or `RFT_FUZZY_ALLOW=1`).

# Process
1. Docker must be available unless `--no-isolate`.
2. Choose a project slug from the catalog.
3. Run:
   ```bash
   go run ./cmd/rft fuzzy ingest --project <slug>
   ```
4. Inspect `report:` (`events.jsonl`, `logs/`).
5. On `class=bug`: curate `testdata/ingest/<lang>_.../` with `ingestor-fixtures`, fix the language package, rerun.

# Flags
- `--project <slug>` (repeatable), `--work-root`, `--report-dir`
- `--offline` use work-root caches only (run `prefetch` first)
- `--no-isolate` opt out of Docker; run setup/check on the host
- `--fail-fast`, `--strict-refs`
- override image per project: `[projects.<slug>.isolate] image = "..."`
