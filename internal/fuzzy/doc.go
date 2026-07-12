// Package fuzzy is the offline catalog fuzz harness for ingest and mv invariants.
// It is not linked into the rft CLI binary; drive it via go test or fuzzy.Run.
//
// Catalog: testdata/fuzzy/projects.toml (real-world pins, setup/check, mv grains).
//
// Run:
//
//	mise run fuzzy:prefetch   // warm work-root (network)
//	mise run fuzzy:run        // unit + local; catalog seeds skip if cold
//	go test ./internal/fuzzy  // same package; plain go test uses a private temp work-root
//
// With FUZZTIME set, fuzzy:run drives TestCatalogFuzzCampaign (catalog RNG stress).
// After prefetch, Offline:true runs use only the work-root cache.
package fuzzy
