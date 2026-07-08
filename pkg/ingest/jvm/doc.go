// Package jvm documents the JVM language family platform.
//
// Family id: ingest.FamilyJVM ("jvm").
//
// Surfaces (honest language ids, separate drivers/grammars):
//   - java  — implemented (pkg/ingest/java), extensions .java
//   - kotlin — not registered yet (needs grammar + extract + resolve)
//
// Shared lattice for the fuzzer/move model: package directory is the module;
// declaration grain is types/members; file is layout within a package.
//
// Import this package only for side-effect documentation in tests if needed;
// language registration lives in each surface package (blank-import java).
package jvm
