package annotate

import (
	"strings"
	"testing"

	"github.com/lucasew/refactree/pkg/ingest"
)

func TestBuild_LinksUsageAndAnchorsDef(t *testing.T) {
	source := []byte("package main\n\nfunc main() {\n\thelper()\n}\n")
	result := &ingest.Result{
		Entities: []ingest.Entity{
			{Reference: "path:./main.go::main", StartByte: 19, EndByte: 23},
		},
		Relations: []ingest.Relation{
			{Reference: "path:./main.go::main", StartByte: 29, EndByte: 35, Target: "path:./helper.go::helper"},
		},
	}

	segs := Build(source, "main.go", result, func(ref string) string {
		return "/code/" + ref
	})

	var sawDef, sawLink bool
	for _, s := range segs {
		if s.IsDef && s.ID == AnchorID("path:./main.go::main") && s.Text == "main" {
			sawDef = true
		}
		if s.IsLink && s.Href == "/code/path:./helper.go::helper" && s.Text == "helper" {
			sawLink = true
		}
	}
	if !sawDef {
		t.Fatal("expected definition segment for main")
	}
	if !sawLink {
		t.Fatal("expected link segment for helper usage")
	}

	joined := ""
	for _, s := range segs {
		joined += s.Text
	}
	if joined != string(source) {
		t.Fatalf("segments do not reconstruct source:\nwant %q\ngot  %q", source, joined)
	}
}

func TestAnchorID_Sanitizes(t *testing.T) {
	id := AnchorID("path:./main.go::main")
	if !strings.HasPrefix(id, "sym-") {
		t.Fatalf("expected sym- prefix, got %q", id)
	}
	if strings.ContainsAny(id, ":/") {
		t.Fatalf("id should not contain : or /, got %q", id)
	}
}
