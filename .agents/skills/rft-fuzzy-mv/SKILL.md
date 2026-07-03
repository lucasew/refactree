---
name: rft-fuzzy-mv
description: Make refactoring implementation more exhaustive, complete and stable by fuzzing mv on repositories in the wild with isolated project checks via the rft fuzzy harness.
---

# Restrictions
Setup/check run in Docker via testcontainers, so a personal machine is fine. Only refuse `--no-isolate` on a non-ephemeral host unless `RFT_FUZZY_ALLOW=1` / `--allow` / `CI=true`.

# Source of truth
- Catalog: `testdata/fuzzy/projects.toml` (`[projects.<slug>]` + `[projects.<slug>.mise]`)
- Human index: `references/projects.md`
- Harness: `internal/fuzzy`, CLI `rft fuzzy mv` / `rft fuzzy run`
- Isolation: testcontainers + Docker, default image `jdxcode/mise:latest`

# Process
1. Docker must be available.
2. Choose a project slug from the catalog.
3. Run:
   ```bash
   go run ./cmd/rft fuzzy run --project <slug> --iterations 10 --seed 1
   ```
   Setup/check run in `jdxcode/mise` via testcontainers (`mise install` then `mise run setup` / `mise run test`). Ingest/mv stay on the host.
4. Inspect `report:` (`events.jsonl`, `logs/`, `scaffold/` on failures).
5. On `class=bug`: curate `testdata/mv/...`, fix `pkg/ingest`, rerun with the same `--seed`.

# Flags
- `--project <slug>`, `--iterations`, `--seed`
- `--no-isolate` host setup/check (no Docker)
- `--fail-fast`, `--strict-refs`, `--work-root`, `--report-dir`
- override image per project: `[projects.<slug>.isolate] image = "..."`
