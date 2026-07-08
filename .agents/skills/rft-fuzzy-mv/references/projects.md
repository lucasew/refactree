Machine-readable catalog: [`testdata/fuzzy/projects.toml`](../../../../testdata/fuzzy/projects.toml)

Setup tasks honor `RFT_FUZZY_OFFLINE=1` (set by harness `--offline`) so cached deps work without registry access after prefetch.

# Python
- id: `ritm_annotation`
- https://github.com/lucasew-graveyard/ritm_annotation
- scoped ingest `ritm_annotation`; `mise run setup` / `mise run test`
- preserve `.venv`, `.uv`; setup uses `uv sync --no-install-project` (no clang/cython); offline adds `--offline`

# Go
- id: `workspaced`
- https://github.com/lucasew/workspaced/
- ingest root `.`; `mise run setup` / `mise run test`
- modules in mise-data `go-mod`; offline via `GOPROXY=off`

# JavaScript
- id: `astro`
- https://github.com/withastro/astro
- scoped ingest `packages/astro` (not the old `packages/compiler`); filter `astro...`
- preserve `node_modules`, `.pnpm-store`; setup asserts `node_modules` exists; offline uses `pnpm install --offline`

# Java / JVM
- id: `gson`
- https://github.com/google/gson
- scoped ingest `gson`; `mise run setup` / `mise run test`
- Maven cache in mise-data `m2`; offline setup uses `mvn -o`

- id: `kotlin`
- https://github.com/JetBrains/kotlin (pin v2.3.21)
- `family = "jvm"`; scoped ingest `libraries/stdlib`
- setup/check: `./gradlew :kotlin-stdlib:compileKotlinJvm` (`--offline` when `RFT_FUZZY_OFFLINE=1`)
- Gradle cache in mise-data `gradle` via `GRADLE_USER_HOME`
- First prefetch is heavy (bootstrap + deps). `.kt` move targets need kotlin ingest surface.
