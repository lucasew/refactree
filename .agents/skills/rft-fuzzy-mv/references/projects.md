Machine-readable catalog: [`testdata/fuzzy/projects.toml`](../../../../testdata/fuzzy/projects.toml)

Setup tasks honor `RFT_FUZZY_OFFLINE=1` (set by harness `--offline`) so cached deps work without registry access after prefetch.

# Python
- id: `ritm_annotation`
- https://github.com/lucasew-graveyard/ritm_annotation
- scoped ingest `ritm_annotation`; `mise run setup` / `mise run test`
- preserve `.venv`, `.uv`; offline setup uses `uv sync --offline`

# Go
- id: `workspaced`
- https://github.com/lucasew/workspaced/
- ingest root `.`; `mise run setup` / `mise run test`
- modules in mise-data `go-mod`; offline via `GOPROXY=off`

# JavaScript
- id: `astro`
- https://github.com/withastro/astro
- scoped ingest `packages/compiler`; `mise run setup` / `mise run test`
- preserve `node_modules`, `.pnpm-store`; offline setup uses `pnpm install --offline`

# Java
- id: `gson`
- https://github.com/google/gson
- scoped ingest `gson`; `mise run setup` / `mise run test`
- Maven cache in mise-data `m2`; offline setup uses `mvn -o`
