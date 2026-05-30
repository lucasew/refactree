package browse

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/lucasew/refactree/pkg/ingest"
)

func TestBrowseScopeFromReference_Directory(t *testing.T) {
	dir := t.TempDir()
	ref := ingest.ParseReference(dir)

	root, rel, err := browseScopeFromReference(ref)
	if err != nil {
		t.Fatalf("browse scope: %v", err)
	}
	if root != dir {
		t.Fatalf("unexpected root: got %q want %q", root, dir)
	}
	if rel != "." {
		t.Fatalf("unexpected rel: got %q want %q", rel, ".")
	}
}

func TestBrowseScopeFromReference_File(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "main.go")
	if err := os.WriteFile(file, []byte("package main\n"), 0644); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}

	ref := ingest.ParseReference(file + "::main")
	root, rel, err := browseScopeFromReference(ref)
	if err != nil {
		t.Fatalf("browse scope: %v", err)
	}
	if root != dir {
		t.Fatalf("unexpected root: got %q want %q", root, dir)
	}
	if rel != "main.go" {
		t.Fatalf("unexpected rel: got %q want %q", rel, "main.go")
	}
}

func TestBrowseSetCurrentRel_RejectsOutsideRoot(t *testing.T) {
	dir := t.TempDir()
	model, err := newBrowseModel(dir, ".", false)
	if err != nil {
		t.Fatalf("new browse model: %v", err)
	}

	if err := model.setCurrentRel("../outside"); err == nil {
		t.Fatal("expected outside-root path error")
	}
}

func TestParentProviderPath(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"fmt", ""},
		{"net/http", "net"},
		{"github.com/lucasew/refactree/cmd/rft", "github.com/lucasew/refactree/cmd"},
		{"", ""},
	}
	for _, tc := range cases {
		got := parentProviderPath(tc.in)
		if got != tc.want {
			t.Fatalf("parentProviderPath(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestNewBrowseModelFromReference_GoProvider(t *testing.T) {
	model, err := newBrowseModelFromReference(ingest.ParseReference("go:fmt"), false)
	if err != nil {
		t.Fatalf("new browse model from go reference: %v", err)
	}
	if model.mode != "provider" {
		t.Fatalf("unexpected mode: %q", model.mode)
	}
	if model.providerRef.Provider != "go" || model.providerRef.Path != "fmt" {
		t.Fatalf("unexpected provider ref: %+v", model.providerRef)
	}
	if model.providerDir == "" {
		t.Fatal("expected providerDir to be resolved")
	}
}

func TestDocToMarkdown(t *testing.T) {
	doc := &ingest.DocResult{
		Name:      "Printf",
		Signature: "func Printf(format string, a ...any) (n int, err error)",
		DocString: "Printf formats according to a format specifier.",
	}

	got := docToMarkdown(doc)
	if !strings.Contains(got, "# Printf") {
		t.Fatalf("missing title in markdown: %q", got)
	}
	if !strings.Contains(got, "```") || !strings.Contains(got, "func Printf") {
		t.Fatalf("missing fenced signature in markdown: %q", got)
	}
	if !strings.Contains(got, "Printf formats according to a format specifier.") {
		t.Fatalf("missing doc string in markdown: %q", got)
	}
}

func TestBrowseResize_ResponsiveLayout(t *testing.T) {
	dir := t.TempDir()
	model, err := newBrowseModel(dir, ".", false)
	if err != nil {
		t.Fatalf("new browse model: %v", err)
	}

	model.width = 90
	model.height = 24
	model.resize()
	if model.showSplit {
		t.Fatal("expected single-pane layout for narrow width")
	}

	model.width = 140
	model.height = 24
	model.resize()
	if !model.showSplit {
		t.Fatal("expected split layout for wide width")
	}
}

func TestBrowseEnterEscPushPopDirectory(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "pkg")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatalf("create subdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sub, "main.go"), []byte("package pkg\nfunc Exported() {}\n"), 0644); err != nil {
		t.Fatalf("write go file: %v", err)
	}

	model, err := newBrowseModel(dir, ".", false)
	if err != nil {
		t.Fatalf("new browse model: %v", err)
	}

	index := findBrowseItemIndex(model.list.Items(), func(it browseItem) bool {
		return it.kind == browseItemDir && it.targetRel == "pkg"
	})
	if index < 0 {
		t.Fatal("expected directory item for pkg/")
	}
	model.list.Select(index)

	if err := model.activateSelection(); err != nil {
		t.Fatalf("activate selection: %v", err)
	}
	if model.currentRel != "pkg" {
		t.Fatalf("unexpected current rel after enter: got %q want %q", model.currentRel, "pkg")
	}
	if len(model.navStack) != 1 {
		t.Fatalf("unexpected nav stack size after enter: got %d want %d", len(model.navStack), 1)
	}

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	_ = cmd
	restored, ok := next.(*browseModel)
	if !ok {
		t.Fatalf("unexpected model type after esc: %T", next)
	}
	if restored.currentRel != "." {
		t.Fatalf("unexpected current rel after esc: got %q want %q", restored.currentRel, ".")
	}
	if len(restored.navStack) != 0 {
		t.Fatalf("unexpected nav stack size after esc: got %d want %d", len(restored.navStack), 0)
	}
}

func TestBrowseEnterEscPushPopSymbol(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc Exported() {}\n"), 0644); err != nil {
		t.Fatalf("write go file: %v", err)
	}

	model, err := newBrowseModel(dir, ".", false)
	if err != nil {
		t.Fatalf("new browse model: %v", err)
	}

	index := findBrowseItemIndex(model.list.Items(), func(it browseItem) bool {
		return it.kind == browseItemSymbol && it.title == "Exported"
	})
	if index < 0 {
		t.Fatal("expected symbol item for Exported")
	}
	model.list.Select(index)

	if err := model.activateSelection(); err != nil {
		t.Fatalf("activate selection: %v", err)
	}
	if model.openedSymbol == "" {
		t.Fatal("expected opened symbol after enter on symbol")
	}
	if model.focus != browseFocusPreview {
		t.Fatalf("unexpected focus after symbol open: got %v want %v", model.focus, browseFocusPreview)
	}
	if len(model.navStack) != 1 {
		t.Fatalf("unexpected nav stack size after symbol open: got %d want %d", len(model.navStack), 1)
	}

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	_ = cmd
	restored, ok := next.(*browseModel)
	if !ok {
		t.Fatalf("unexpected model type after esc: %T", next)
	}
	if restored.openedSymbol != "" {
		t.Fatalf("expected opened symbol to be cleared, got %q", restored.openedSymbol)
	}
	if restored.focus != browseFocusList {
		t.Fatalf("unexpected focus after esc: got %v want %v", restored.focus, browseFocusList)
	}
}

func TestBrowseMouseClickSelectsListItem(t *testing.T) {
	dir := t.TempDir()
	model, err := newBrowseModel(dir, ".", false)
	if err != nil {
		t.Fatalf("new browse model: %v", err)
	}

	model.list.SetItems([]list.Item{
		browseItem{kind: browseItemInfo, title: "one"},
		browseItem{kind: browseItemInfo, title: "two"},
		browseItem{kind: browseItemInfo, title: "three"},
	})
	model.list.Select(0)

	model.width = 140
	model.height = 30
	model.resize()
	if !model.showSplit {
		t.Fatal("expected split layout in mouse selection test")
	}

	msg := tea.MouseMsg{
		X:      1,
		Y:      4,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}
	_ = model.handleMouse(msg)

	if model.focus != browseFocusList {
		t.Fatalf("unexpected focus after click: got %v want %v", model.focus, browseFocusList)
	}
	if model.list.Index() != 1 {
		t.Fatalf("unexpected selected index after click: got %d want %d", model.list.Index(), 1)
	}
}

func TestBrowseSymbolItems_UsesWalkSymbolsOrder(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc Zebra() {}\nfunc Alpha() {}\n"), 0644); err != nil {
		t.Fatalf("write go file: %v", err)
	}

	model, err := newBrowseModel(dir, ".", false)
	if err != nil {
		t.Fatalf("new browse model: %v", err)
	}

	items, err := model.symbolItems()
	if err != nil {
		t.Fatalf("symbol items: %v", err)
	}

	got := make([]string, 0, len(items))
	for _, raw := range items {
		item, ok := raw.(browseItem)
		if !ok {
			t.Fatalf("unexpected item type: %T", raw)
		}
		got = append(got, item.symbolRef)
	}

	want := []string{}
	err = ingest.WalkSymbols(dir, "path:./", ingest.ListOptions{}, func(sym ingest.SymbolInfo) bool {
		want = append(want, sym.Entity.Reference)
		return true
	})
	if err != nil {
		t.Fatalf("walk symbols: %v", err)
	}

	if len(got) != len(want) {
		t.Fatalf("unexpected item count: got %d want %d (%v vs %v)", len(got), len(want), got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("unexpected symbol order at %d: got %q want %q (%v vs %v)", i, got[i], want[i], got, want)
		}
	}
}

func TestBrowseSymbols_PathCmdRft_ContainsExecuteLikeLs(t *testing.T) {
	root := findRepoRoot(t)
	target := filepath.Join(root, "cmd", "rft")

	model, err := newBrowseModelFromReference(ingest.ParseReference("path:"+target), false)
	if err != nil {
		t.Fatalf("new browse model from path reference: %v", err)
	}

	items, err := model.symbolItems()
	if err != nil {
		t.Fatalf("symbol items: %v", err)
	}

	seen := map[string]bool{}
	for _, raw := range items {
		item, ok := raw.(browseItem)
		if !ok {
			t.Fatalf("unexpected item type: %T", raw)
		}
		seen[item.title] = true
	}

	if !seen["Execute"] {
		t.Fatalf("expected Execute symbol for %s, got symbols: %+v", target, seen)
	}

	want := []string{}
	err = ingest.WalkSymbols(target, "path:./", ingest.ListOptions{}, func(sym ingest.SymbolInfo) bool {
		want = append(want, sym.Entity.Reference)
		return true
	})
	if err != nil {
		t.Fatalf("walk symbols: %v", err)
	}

	got := make([]string, 0, len(items))
	for _, raw := range items {
		item := raw.(browseItem)
		got = append(got, item.symbolRef)
	}

	if len(got) != len(want) {
		t.Fatalf("unexpected symbol count: got %d want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("unexpected symbol at %d: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestBrowseNavigateIntoDirectory_ShowsSymbols(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "pkg")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatalf("create subdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sub, "main.go"), []byte("package pkg\nfunc Exported() {}\n"), 0644); err != nil {
		t.Fatalf("write go file: %v", err)
	}

	model, err := newBrowseModel(dir, ".", false)
	if err != nil {
		t.Fatalf("new browse model: %v", err)
	}

	index := findBrowseItemIndex(model.list.Items(), func(it browseItem) bool {
		return it.kind == browseItemDir && it.targetRel == "pkg"
	})
	if index < 0 {
		t.Fatal("expected directory item for pkg/")
	}
	model.list.Select(index)

	if err := model.activateSelection(); err != nil {
		t.Fatalf("activate selection: %v", err)
	}

	found := false
	for _, raw := range model.list.Items() {
		item, ok := raw.(browseItem)
		if !ok {
			continue
		}
		if item.kind == browseItemSymbol && item.title == "Exported" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected Exported symbol after entering pkg/, got items: %+v", model.list.Items())
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find repository root from %q", dir)
		}
		dir = parent
	}
}

func findBrowseItemIndex(items []list.Item, match func(browseItem) bool) int {
	for i, raw := range items {
		item, ok := raw.(browseItem)
		if !ok {
			continue
		}
		if match(item) {
			return i
		}
	}
	return -1
}
