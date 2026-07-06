---
name: rft-fuzzy-mv
description: Make refactoring implementation more exhaustive, complete and stable by fuzzing mv on repositories in the wild with isolated project checks via the rft fuzzy harness.
---

Shared harness notes: [`references/harness.md`](references/harness.md)
Catalog index: [`references/projects.md`](references/projects.md)
Fixtures: `testdata/mv/` on `class=bug`; ingest phase uses `testdata/ingest/` (`ingestor-fixtures`).
`mv`/`run` worktrees are `runs/<slug>/<seed>` (pass the same `--seed` to reproduce).

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
Common flags in `harness.md`, plus:
- `--iterations`, `--seed` (rng + worktree name), `--ops` (`rename,cross_file,package`)
- `--strict-refs` fail on dangling path targets

Offline example:
```bash
go run ./cmd/rft fuzzy run --project <slug> --work-root /var/cache/rft-fuzzy --offline --iterations 10 --seed 1
```
