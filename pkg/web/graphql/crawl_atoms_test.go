package graphql

import (
  "context"
  "os"
  "path/filepath"
  "strings"
  "testing"
)

func TestStreamProject_EmitsAtoms(t *testing.T) {
  dir := t.TempDir()
  os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example\n\ngo 1.22\n"), 0644)
  os.WriteFile(filepath.Join(dir, "a.go"), []byte("package example\n\nfunc Hello() { World() }\nfunc World() {}\n"), 0644)
  c := NewSessionCorpus(dir)
  var atoms, pkgs int
  err := c.StreamProject(context.Background(), func(ev StreamEvent) bool {
    if ev.Type != "edge" || ev.Edge == nil {
      return true
    }
    t.Logf("%s %s → %s", ev.Edge.Kind, ev.Edge.From, ev.Edge.To)
    for _, id := range []string{ev.Edge.From, ev.Edge.To} {
      if strings.Contains(id, "::") {
        atoms++
      } else if strings.HasPrefix(id, "path:") {
        pkgs++
      }
    }
    return true
  })
  if err != nil {
    t.Fatal(err)
  }
  if atoms == 0 {
    t.Fatal("expected atom endpoints (::) from crawl USES/defs")
  }
  t.Logf("atom endpoints=%d package endpoints=%d", atoms, pkgs)
}
