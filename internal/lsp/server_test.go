package lsp

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func TestLSP_DefinitionCompletionHover(t *testing.T) {
	root := t.TempDir()
	// minimal go module
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(root, "main.go")
	src := strings.Join([]string{
		`package main`,
		``,
		`// Hello greets.`,
		`func Hello() string {`,
		`	return "hi"`,
		`}`,
		``,
		`func main() {`,
		`	_ = Hello()`,
		`}`,
		``,
	}, "\n")
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()

	srv, connA, _ := RunWith(ctx, jsonrpc2.NewHeaderStream(a))
	defer connA.Close()

	_, connB, client := protocol.NewClient(ctx, protocol.UnimplementedClient{}, jsonrpc2.NewHeaderStream(b))
	defer connB.Close()

	rootURI := uri.File(root)
	_, err := client.Initialize(ctx, &protocol.InitializeParams{
		RootURI: &rootURI,
	})
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	if err := client.Initialized(ctx, &protocol.InitializedParams{}); err != nil {
		t.Fatalf("initialized: %v", err)
	}

	docURI := uri.File(mainPath)
	if err := client.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        docURI,
			LanguageID: "go",
			Version:    1,
			Text:       src,
		},
	}); err != nil {
		t.Fatalf("didOpen: %v", err)
	}

	// wait for snapshot
	deadline := time.Now().Add(10 * time.Second)
	for {
		snap := srv.session.snapshot()
		if snap != nil && snap.Result != nil && len(snap.Result.Atoms) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timeout waiting for snapshot entities")
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Definition on Hello() call in main
	idx := strings.LastIndex(src, "Hello()")
	if idx < 0 {
		t.Fatal("token not found")
	}
	pos := offsetToPos(src, idx+1)
	def, err := client.Definition(ctx, &protocol.DefinitionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
			Position:     pos,
		},
	})
	if err != nil {
		t.Fatalf("definition: %v", err)
	}
	loc, ok := def.(*protocol.Location)
	if !ok || loc == nil {
		t.Fatalf("definition result %#v", def)
	}
	if !strings.Contains(string(loc.URI), "main.go") {
		t.Fatalf("def uri %s", loc.URI)
	}

	// Completion on "Hel"
	partial := strings.Replace(src, `_ = Hello()`, `_ = Hel`, 1)
	if err := client.DidChange(ctx, &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: docURI},
			Version:                2,
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			&protocol.TextDocumentContentChangeWholeDocument{Text: partial},
		},
	}); err != nil {
		t.Fatalf("didChange: %v", err)
	}
	off := strings.LastIndex(partial, "Hel") + len("Hel")
	cpos := offsetToPos(partial, off)
	comp, err := client.Completion(ctx, &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
			Position:     cpos,
		},
	})
	if err != nil {
		t.Fatalf("completion: %v", err)
	}
	items, ok := comp.(protocol.CompletionItemSlice)
	if !ok {
		t.Fatalf("completion type %T", comp)
	}
	found := false
	for _, it := range items {
		if it.Label == "Hello" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected Hello in completion, got %v", labels(items))
	}

	// Hover on Hello definition
	_ = client.DidChange(ctx, &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: docURI},
			Version:                3,
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			&protocol.TextDocumentContentChangeWholeDocument{Text: src},
		},
	})
	// wait rebuild after change
	time.Sleep(400 * time.Millisecond)
	hidx := strings.Index(src, "func Hello")
	hpos := offsetToPos(src, hidx+len("func H"))
	hover, err := client.Hover(ctx, &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: docURI},
			Position:     hpos,
		},
	})
	if err != nil {
		t.Fatalf("hover: %v", err)
	}
	if hover == nil {
		t.Fatal("nil hover")
	}
	body := hoverText(hover)
	if !strings.Contains(body, "Hello") && !strings.Contains(body, "greets") {
		t.Fatalf("hover body %q", body)
	}
}

func labels(items protocol.CompletionItemSlice) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Label
	}
	return out
}

func hoverText(h *protocol.Hover) string {
	if h == nil || h.Contents == nil {
		return ""
	}
	if mc, ok := h.Contents.(*protocol.MarkupContent); ok {
		return mc.Value
	}
	return ""
}

func offsetToPos(text string, byteOff int) protocol.Position {
	if byteOff < 0 {
		byteOff = 0
	}
	if byteOff > len(text) {
		byteOff = len(text)
	}
	line, col := 0, 0
	for i := 0; i < byteOff; i++ {
		if text[i] == '\n' {
			line++
			col = 0
		} else {
			col++
		}
	}
	return protocol.Position{Line: uint32(line), Character: uint32(col)}
}
