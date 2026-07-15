package python

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucasew/refactree/pkg/ingest"
)

func writePy(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func applyRemove(src string, decl ingest.DeclExtract) string {
	return src[:decl.RemoveStart] + src[decl.RemoveEnd:]
}

func entityAt(t *testing.T, content, name string) ingest.Entity {
	t.Helper()
	// name starts at first occurrence of "def name" or "async def name"
	idx := strings.Index(content, "def "+name)
	if idx < 0 {
		t.Fatalf("def %s not found", name)
	}
	// point at the name identifier, not "def "
	start := uint32(idx + len("def "))
	return ingest.Entity{
		Reference: "path:./mod.py::C." + name,
		StartByte: start,
		EndByte:   start + uint32(len(name)),
	}
}

func TestExtractDecl_LastMethodRemovesEmptyClass(t *testing.T) {
	dir := t.TempDir()
	src := "class C:\n    def foo(self):\n        return 1\n\nx = 1\n"
	path := writePy(t, dir, "mod.py", src)

	decl, err := moveDriver{}.ExtractDecl(path, entityAt(t, src, "foo"))
	if err != nil {
		t.Fatal(err)
	}
	if decl.Preamble != "C" {
		t.Fatalf("preamble: got %q want C", decl.Preamble)
	}
	if !strings.Contains(decl.DeclText, "def foo") {
		t.Fatalf("DeclText missing method: %q", decl.DeclText)
	}
	got := applyRemove(src, decl)
	if strings.Contains(got, "class C") {
		t.Fatalf("empty class left behind:\n%s", got)
	}
	if !strings.Contains(got, "x = 1") {
		t.Fatalf("trailing module code lost:\n%s", got)
	}
	// Must remain valid Python structure-wise: no bare "class C:"
	if strings.Contains(got, "class C:") {
		t.Fatalf("invalid empty class:\n%s", got)
	}
}

func TestExtractDecl_LastMethodOnlyFile(t *testing.T) {
	dir := t.TempDir()
	src := "class C:\n    def foo(self):\n        return 1\n"
	path := writePy(t, dir, "mod.py", src)

	decl, err := moveDriver{}.ExtractDecl(path, entityAt(t, src, "foo"))
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(applyRemove(src, decl))
	if got != "" {
		t.Fatalf("want empty source after sole-method class remove, got %q", got)
	}
}

func TestExtractDecl_KeepsClassWithSiblingMethod(t *testing.T) {
	dir := t.TempDir()
	src := "class C:\n    def foo(self):\n        return 1\n\n    def bar(self):\n        return 2\n"
	path := writePy(t, dir, "mod.py", src)

	decl, err := moveDriver{}.ExtractDecl(path, entityAt(t, src, "foo"))
	if err != nil {
		t.Fatal(err)
	}
	got := applyRemove(src, decl)
	if !strings.Contains(got, "class C:") {
		t.Fatalf("class should remain:\n%s", got)
	}
	if !strings.Contains(got, "def bar") {
		t.Fatalf("sibling method lost:\n%s", got)
	}
	if strings.Contains(got, "def foo") {
		t.Fatalf("moved method still present:\n%s", got)
	}
}

func TestInsertDecl_TwoSpaceClassBody(t *testing.T) {
	dst := []byte("class C:\n  def other(self):\n    return 3\n")
	decl := ingest.DeclExtract{
		Preamble: "C",
		DeclText: "def foo(self):\n    return 1",
	}
	edit := moveDriver{}.InsertDecl("b.py", dst, decl)
	got := string(dst[:edit.StartByte]) + edit.NewText + string(dst[edit.EndByte:])
	// foo must be a sibling of other at 2-space indent, not nested under other.
	if !strings.Contains(got, "\n  def foo(self):\n") {
		t.Fatalf("expected 2-space class-body method:\n%s", got)
	}
	// Must not appear nested inside other's body (4 spaces after other's return line pattern).
	if strings.Contains(got, "return 3\n    def foo") || strings.Contains(got, "return 3\n         def foo") {
		t.Fatalf("method nested inside other:\n%s", got)
	}
}

func TestInsertDecl_TabClassBody(t *testing.T) {
	dst := []byte("class C:\n\tdef other(self):\n\t\treturn 3\n")
	decl := ingest.DeclExtract{
		Preamble: "C",
		DeclText: "def foo(self):\n\treturn 1",
	}
	edit := moveDriver{}.InsertDecl("b.py", dst, decl)
	got := string(dst[:edit.StartByte]) + edit.NewText + string(dst[edit.EndByte:])
	if !strings.Contains(got, "\n\tdef foo(self):\n") {
		t.Fatalf("expected tab-indented class-body method:\n%q", got)
	}
	// No space-indented def at class body level.
	if strings.Contains(got, "\n    def foo") {
		t.Fatalf("got 4-space indent in tab class:\n%q", got)
	}
}

func TestInsertDecl_NewModuleUsesSourceIndent(t *testing.T) {
	decl := ingest.DeclExtract{
		Preamble: "C",
		DeclText: "def foo(self):\n\treturn 1",
	}
	edit := moveDriver{}.InsertDecl("new.py", nil, decl)
	got := edit.NewText
	if !strings.HasPrefix(got, "class C:\n") {
		t.Fatalf("missing class shell: %q", got)
	}
	if !strings.Contains(got, "\n\tdef foo(self):\n\t\treturn 1\n") {
		t.Fatalf("expected tab class body from source style:\n%q", got)
	}
	if strings.Contains(got, "    def foo") {
		t.Fatalf("hardcoded 4-space shell:\n%q", got)
	}
}

func TestInsertDecl_PassOnlyClassReplacesPass(t *testing.T) {
	dst := []byte("class C:\n    pass\n")
	decl := ingest.DeclExtract{
		Preamble: "C",
		DeclText: "def foo(self):\n    return 1",
	}
	edit := moveDriver{}.InsertDecl("b.py", dst, decl)
	got := string(dst[:edit.StartByte]) + edit.NewText + string(dst[edit.EndByte:])
	if strings.Contains(got, "pass") {
		t.Fatalf("dead pass retained:\n%s", got)
	}
	if !strings.Contains(got, "def foo") {
		t.Fatalf("method missing:\n%s", got)
	}
}

func TestDetectIndentUnit(t *testing.T) {
	if g := pythonDetectIndentUnit("def f():\n  return 1"); g != "  " {
		t.Fatalf("2-space: got %q", g)
	}
	if g := pythonDetectIndentUnit("def f():\n\treturn 1"); g != "\t" {
		t.Fatalf("tab: got %q", g)
	}
	if g := pythonDetectIndentUnit("def f():\n    return 1"); g != "    " {
		t.Fatalf("4-space: got %q", g)
	}
	if g := pythonDetectIndentUnit("x = 1"); g != "    " {
		t.Fatalf("default: got %q", g)
	}
}
