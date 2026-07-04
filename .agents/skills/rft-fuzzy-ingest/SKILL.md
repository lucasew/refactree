---
name: rft-fuzzy-ingest
description: Run real-world ingest invariant checks via the rft fuzzy harness on ephemeral hosts, then curate failures into testdata/ingest fixtures.
---

# Restrictions
Setup/check run in Docker via testcontainers, so a personal machine is fine. Only refuse `--no-isolate` on a non-ephemeral host unless `RFT_FUZZY_ALLOW=1` / `--allow` / `CI=true`.

# Source of truth
- Catalog: `testdata/fuzzy/projects.toml` (`[projects.<slug>]` + `[projects.<slug>.mise]`)
- Human index: `../rft-fuzzy-mv/references/projects.md`
- Harness: `internal/fuzzy`, CLI `rft fuzzy ingest`
- Fixtures: `testdata/ingest/` (see `ingestor-fixtures` skill)
- Isolation: testcontainers + Docker, default image `jdxcode/mise:latest`; ingest runs on the host

# Process
1. Docker must be available.
2. Choose a project slug from the catalog.
3. Run:
   ```bash
   go run ./cmd/rft fuzzy ingest --project <slug>
   ```
   Setup/check run in `jdxcode/mise` via testcontainers (`mise install` then `mise run setup` / `mise run test`). Ingest stays on the host.
4. Inspect `report:` (`events.jsonl`, `logs/`).
5. On `class=bug`: curate `testdata/ingest/<lang>_.../` with `ingestor-fixtures`, fix the language package, rerun.

# Flags
- `--project <slug>` (repeatable)
- `--no-isolate` host setup/check (no Docker)
- `--fail-fast`, `--strict-refs`, `--work-root`, `--report-dir`
- override image per project: `[projects.<slug>.isolate] image = "..."`
