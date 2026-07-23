package ingestgo

import (
	"strings"
	"testing"

	"github.com/lucasew/refactree/pkg/ingest"
)

func TestGoImportHygiene_NeedsFromRef(t *testing.T) {
	h := goImportHygiene{}
	n, ok := h.NeedsFromRef("@go:fmt::Errorf")
	if !ok || n.ImportPath != "fmt" {
		t.Fatalf("fmt: %+v ok=%v", n, ok)
	}
	n, ok = h.NeedsFromRef("go:net/http::Get")
	if !ok || n.ImportPath != "net/http" {
		t.Fatalf("net/http: %+v ok=%v", n, ok)
	}
	if _, ok := h.NeedsFromRef("path:./pkg/a.go::Foo"); ok {
		t.Fatal("path ref should not need go import")
	}
	if _, ok := h.NeedsFromRef("go:./local::X"); ok {
		t.Fatal("relative go path should be skipped")
	}
}

func TestGoImportHygiene_EnsureImportEdits(t *testing.T) {
	h := goImportHygiene{}
	src := []byte("package p\n\nfunc f() {}\n")
	edits := h.EnsureImportEdits("p.go", src, []ingest.ImportNeed{{ImportPath: "fmt"}})
	if len(edits) == 0 {
		t.Fatal("expected import edit")
	}
	out := ingest.ApplyEditsInMemory(src, edits)
	if !strings.Contains(string(out), `"fmt"`) {
		t.Fatalf("missing fmt import:\n%s", out)
	}
	// Already present
	with := []byte("package p\n\nimport \"fmt\"\n\nfunc f() { fmt.Println() }\n")
	if got := h.EnsureImportEdits("p.go", with, []ingest.ImportNeed{{ImportPath: "fmt"}}); len(got) != 0 {
		t.Fatalf("expected no change, got %+v", got)
	}
}
