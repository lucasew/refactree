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

func TestGoImportHygiene_PruneNamedUnused(t *testing.T) {
	h := goImportHygiene{}
	// fmt only used in the masked region; strings still used outside.
	src := []byte("package p\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n\nfunc f() {\n\tfmt.Println()\n\t_ = strings.TrimSpace(\"\")\n}\n")
	// Mask the fmt.Println line.
	start := strings.Index(string(src), "fmt.Println()")
	if start < 0 {
		t.Fatal("fixture")
	}
	end := start + len("fmt.Println()")
	edits := h.PruneNamedUnusedEdits("p.go", src, ingest.PruneImportOpts{
		MaskSpans:      []ingest.Span{{StartByte: uint32(start), EndByte: uint32(end)}},
		OnlyCandidates: []string{"fmt"},
	})
	if len(edits) == 0 {
		t.Fatal("expected prune of fmt")
	}
	out := string(ingest.ApplyEditsInMemory(src, edits))
	if strings.Contains(out, `"fmt"`) {
		t.Fatalf("fmt should be pruned:\n%s", out)
	}
	if !strings.Contains(out, `"strings"`) {
		t.Fatalf("strings must remain:\n%s", out)
	}
	// Blank import never pruned.
	side := []byte("package p\n\nimport _ \"image/png\"\n\nfunc f() {}\n")
	if got := h.PruneNamedUnusedEdits("p.go", side, ingest.PruneImportOpts{}); len(got) != 0 {
		t.Fatalf("blank import must not prune: %+v", got)
	}
}
