---
name: rft-fuzzy-ingest
description: Run real-world ingest invariant checks via the rft fuzzy harness on ephemeral hosts, then curate failures into testdata/ingest fixtures.
---

# Restrictions
Docker isolation is the default. Guard applies only with `--no-isolate` unless `RFT_FUZZY_ALLOW=1` / `--allow` / `CI=true`.

# Source of truth
- Catalog: `testdata/fuzzy/projects.toml`
- Isolation: testcontainers + `jdxcode/mise:latest` for setup; ingest runs on the host.

# Process
1. `go run ./cmd/rft fuzzy ingest --project <slug>`
2. Read `report:`; on invariant failures add `testdata/ingest/` fixtures and fix language packages.
