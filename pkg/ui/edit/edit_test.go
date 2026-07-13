package edit

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucasew/refactree/pkg/ingest"
)

type captureEditor struct {
	last Location
	err  error
}

func (c *captureEditor) Open(loc Location) error {
	c.last = loc
	return c.err
}

type fixedPicker struct {
	sel string
	err error
}

func (p fixedPicker) Pick(stream func(emit func(string) error) error) (string, error) {
	if p.err != nil {
		return "", p.err
	}
	if p.sel != "" {
		// Still drain the stream so StreamRefs is exercised.
		_ = stream(func(string) error { return nil })
		return p.sel, nil
	}
	var first string
	err := stream(func(s string) error {
		if first == "" {
			first = s
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if first == "" {
		return "", errors.New("no candidates")
	}
	return first, nil
}

func TestResolveEditorBin(t *testing.T) {
	env := map[string]string{}
	getenv := func(k string) string { return env[k] }

	if _, err := ResolveEditorBin("", getenv); err == nil {
		t.Fatal("expected error when no editor configured")
	}

	env["EDITOR"] = "ed"
	got, err := ResolveEditorBin("", getenv)
	if err != nil || got != "ed" {
		t.Fatalf("EDITOR: got %q %v", got, err)
	}

	env["VISUAL"] = "vi"
	got, err = ResolveEditorBin("", getenv)
	if err != nil || got != "vi" {
		t.Fatalf("VISUAL wins over EDITOR: got %q %v", got, err)
	}

	env["RFT_EDITOR"] = "hx"
	got, err = ResolveEditorBin("", getenv)
	if err != nil || got != "hx" {
		t.Fatalf("RFT_EDITOR wins: got %q %v", got, err)
	}

	got, err = ResolveEditorBin("nvim", getenv)
	if err != nil || got != "nvim" {
		t.Fatalf("flag wins: got %q %v", got, err)
	}
}

func TestLocationAtByte(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.go")
	// bytes: "a\nbb\nccc" — 'c' at line 3 col 0 (0-based) → editor col 1
	src := []byte("a\nbb\nccc")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}
	// offset of first 'c' is 5
	loc, err := locationAtByte(path, 5)
	if err != nil {
		t.Fatal(err)
	}
	if loc.Line != 3 || loc.Column != 1 {
		t.Fatalf("got line=%d col=%d want 3,1", loc.Line, loc.Column)
	}
}

func TestRun_FileRefOpensAtStart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ed := &captureEditor{}
	err := Run(Options{
		BaseDir: dir,
		Input:   path,
		Editor:  ed,
		Getenv:  func(string) string { return "true" },
	})
	if err != nil {
		t.Fatal(err)
	}
	if ed.last.Path != path {
		t.Fatalf("path: got %q want %q", ed.last.Path, path)
	}
	if ed.last.Line != 1 || ed.last.Column != 1 {
		t.Fatalf("want 1:1 got %d:%d", ed.last.Line, ed.last.Column)
	}
}

func TestRun_SymbolRefOpensDefinition(t *testing.T) {
	dir := t.TempDir()
	// Simple go file with a function at a known offset.
	src := "package main\n\nfunc Hello() {}\n"
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	// go.mod so module context is fine if needed
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ed := &captureEditor{}
	err := Run(Options{
		BaseDir: dir,
		Input:   "path:./main.go::Hello",
		Editor:  ed,
		Getenv:  func(string) string { return "true" },
	})
	if err != nil {
		t.Fatal(err)
	}
	if ed.last.Path != path {
		t.Fatalf("path: got %q want %q", ed.last.Path, path)
	}
	// "Hello" starts after "package main\n\nfunc "
	idx := strings.Index(src, "Hello")
	if idx < 0 {
		t.Fatal("fixture")
	}
	want, err := locationAtByte(path, uint32(idx))
	if err != nil {
		t.Fatal(err)
	}
	if ed.last.Line != want.Line || ed.last.Column != want.Column {
		t.Fatalf("got %d:%d want %d:%d", ed.last.Line, ed.last.Column, want.Line, want.Column)
	}
}

func TestRun_MissingSymbol(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ed := &captureEditor{}
	err := Run(Options{
		BaseDir: dir,
		Input:   "path:./main.go::Nope",
		Editor:  ed,
		Getenv:  func(string) string { return "true" },
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRun_EmptyUsesPicker(t *testing.T) {
	dir := t.TempDir()
	src := "package main\n\nfunc Hello() {}\n"
	path := filepath.Join(dir, "main.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	ed := &captureEditor{}
	var listed bool
	err := Run(Options{
		BaseDir: dir,
		Input:   "",
		Editor:  ed,
		Picker:  fixedPicker{sel: "path:./main.go::Hello"},
		Getenv:  func(string) string { return "true" },
		StreamRefs: func(baseDir string, ref ingest.Reference, includeHidden bool, emit func(string) error) error {
			listed = true
			return emit("path:./main.go::Hello")
		},
	})
	if !listed {
		t.Fatal("expected StreamRefs for empty input")
	}
	if err != nil {
		t.Fatal(err)
	}
	if ed.last.Path != path {
		t.Fatalf("path: got %q want %q", ed.last.Path, path)
	}
}

func TestRun_DirUsesScopedPicker(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "pkg")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	src := "package pkg\n\nfunc X() {}\n"
	path := filepath.Join(sub, "x.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	ed := &captureEditor{}
	var gotRef ingest.Reference
	err := Run(Options{
		BaseDir: dir,
		Input:   "path:./pkg",
		Editor:  ed,
		Picker:  fixedPicker{sel: "path:./pkg/x.go::X"},
		Getenv:  func(string) string { return "true" },
		StreamRefs: func(baseDir string, ref ingest.Reference, includeHidden bool, emit func(string) error) error {
			gotRef = ref
			return emit("path:./pkg/x.go::X")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotRef.Path != "./pkg" && gotRef.Path != "pkg" && !strings.Contains(gotRef.Path, "pkg") {
		t.Fatalf("expected scoped list under pkg, got %+v", gotRef)
	}
	if ed.last.Path != path {
		t.Fatalf("path: got %q want %q", ed.last.Path, path)
	}
}

func TestPathLineColumnEditor_ArgFormat(t *testing.T) {
	// Use a tiny script that records argv
	dir := t.TempDir()
	script := filepath.Join(dir, "ed.sh")
	out := filepath.Join(dir, "args.txt")
	body := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"" + out + "\"\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	ed := PathLineColumnEditor{Bin: script}
	err := ed.Open(Location{Path: "/tmp/x.go", Line: 4, Column: 7})
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(string(b))
	if got != "/tmp/x.go:4:7" {
		t.Fatalf("argv: got %q", got)
	}
}

func TestFZFPicker_StreamsLines(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "fzf.sh")
	// Fake fzf: read all stdin lines, write the second line to stdout.
	body := "#!/bin/sh\nawk 'NR==2{print; exit}'\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	p := FZFPicker{
		LookPath: func(string) (string, error) { return script, nil },
		Stderr:   os.Stderr,
	}
	var emitted []string
	got, err := p.Pick(func(emit func(string) error) error {
		for _, s := range []string{"path:./a.go::A", "path:./b.go::B", "path:./c.go::C"} {
			emitted = append(emitted, s)
			if err := emit(s); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "path:./b.go::B" {
		t.Fatalf("selection: got %q", got)
	}
	if len(emitted) != 3 {
		t.Fatalf("expected all lines streamed, got %v", emitted)
	}
}
