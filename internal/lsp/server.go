package lsp

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/lucasew/refactree/pkg/ingest"
	"github.com/lucasew/refactree/pkg/version"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// Server is the refactree language server (complement code intelligence).
type Server struct {
	protocol.UnimplementedServer
	session *Session

	mu     sync.Mutex
	client protocol.Client
}

// New creates a server.
func New() *Server {
	return &Server{session: newSession()}
}

// RunStdio serves LSP over stdin/stdout.
func RunStdio(ctx context.Context) error {
	return Run(ctx, stdio{})
}

type stdio struct{}

func (stdio) Read(p []byte) (int, error)  { return os.Stdin.Read(p) }
func (stdio) Write(p []byte) (int, error) { return os.Stdout.Write(p) }
func (stdio) Close() error                { return nil }

// Run serves over any ReadWriteCloser (tests use net.Pipe).
func Run(ctx context.Context, conn io.ReadWriteCloser) error {
	s := New()
	ctx, jconn, client := protocol.NewServer(ctx, s, jsonrpc2.NewHeaderStream(conn))
	s.mu.Lock()
	s.client = client
	s.session.setClient(client)
	s.mu.Unlock()
	select {
	case <-ctx.Done():
		_ = jconn.Close()
		return ctx.Err()
	case <-jconn.Done():
		return jconn.Err()
	}
}

// RunWith returns server + client dispatcher for in-process tests.
func RunWith(ctx context.Context, stream jsonrpc2.Stream) (*Server, jsonrpc2.Conn, protocol.Client) {
	s := New()
	_, jconn, client := protocol.NewServer(ctx, s, stream)
	s.mu.Lock()
	s.client = client
	s.session.setClient(client)
	s.mu.Unlock()
	return s, jconn, client
}

func (s *Server) Initialize(_ context.Context, params *protocol.InitializeParams) (*protocol.InitializeResult, error) {
	if params.RootURI != nil {
		s.session.ensureRoot(uriToPath(*params.RootURI))
	} else if p, ok := params.RootPath.Get(); ok && p != "" {
		s.session.ensureRoot(p)
	}
	syncKind := protocol.TextDocumentSyncKindFull
	trueVal := protocol.Boolean(true)
	ver := version.GetBuildID()
	return &protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			TextDocumentSync:        syncKind,
			CompletionProvider:      &protocol.CompletionOptions{},
			HoverProvider:           trueVal,
			DefinitionProvider:      trueVal,
			ReferencesProvider:      trueVal,
			DocumentSymbolProvider:  trueVal,
			WorkspaceSymbolProvider: trueVal,
			RenameProvider:          trueVal,
		},
		ServerInfo: protocol.ServerInfo{
			Name:    "rft",
			Version: protocol.NewOptional(ver),
		},
	}, nil
}

func (s *Server) Initialized(context.Context, *protocol.InitializedParams) error { return nil }
func (s *Server) Shutdown(context.Context) error                                 { return nil }
func (s *Server) Exit(context.Context) error                                     { return nil }

func (s *Server) DidOpen(_ context.Context, params *protocol.DidOpenTextDocumentParams) error {
	path := uriToPath(params.TextDocument.URI)
	text := params.TextDocument.Text
	s.session.ensureRoot(path)
	s.session.overlay.SetString(path, text)
	s.session.mu.Lock()
	s.session.openDocs[path] = params.TextDocument.Version
	s.session.mu.Unlock()
	s.session.fastParse(path, text)
	s.session.markDirty()
	return nil
}

func (s *Server) DidChange(_ context.Context, params *protocol.DidChangeTextDocumentParams) error {
	path := uriToPath(params.TextDocument.URI)
	if len(params.ContentChanges) == 0 {
		return nil
	}
	ch := params.ContentChanges[len(params.ContentChanges)-1]
	var text string
	switch c := ch.(type) {
	case *protocol.TextDocumentContentChangeWholeDocument:
		text = c.Text
	case *protocol.TextDocumentContentChangePartial:
		return nil
	default:
		return nil
	}
	s.session.overlay.SetString(path, text)
	s.session.mu.Lock()
	s.session.openDocs[path] = params.TextDocument.Version
	s.session.mu.Unlock()
	s.session.fastParse(path, text)
	s.session.markDirty()
	return nil
}

func (s *Server) DidSave(_ context.Context, params *protocol.DidSaveTextDocumentParams) error {
	path := uriToPath(params.TextDocument.URI)
	if params.Text != nil {
		s.session.overlay.SetString(path, *params.Text)
		s.session.fastParse(path, *params.Text)
	}
	s.session.mu.Lock()
	s.session.dirty = true
	s.session.mu.Unlock()
	s.session.scheduleRebuild()
	return nil
}

func (s *Server) DidClose(_ context.Context, params *protocol.DidCloseTextDocumentParams) error {
	path := uriToPath(params.TextDocument.URI)
	s.session.overlay.Delete(path)
	s.session.mu.Lock()
	delete(s.session.openDocs, path)
	delete(s.session.fastExtract, path)
	s.session.mu.Unlock()
	s.session.markDirty()
	return nil
}

func (s *Server) Definition(_ context.Context, params *protocol.DefinitionParams) (protocol.DefinitionResult, error) {
	path := uriToPath(params.TextDocument.URI)
	text, ok := s.session.docText(path)
	if !ok {
		return nil, nil
	}
	snap := s.session.snapshot()
	if snap == nil || snap.Result == nil {
		return nil, nil
	}
	off := byteOffset(text, params.Position)
	hit, ok := ingest.HitAtByte(snap.Result, s.session.relPath(path), off)
	if !ok || hit.Reference == "" {
		// Still try identifier + on-demand seed of current file for vendor buffers.
		hitRef := s.hitFromIdentifier(path, text, off)
		if hitRef == "" {
			return nil, nil
		}
		hit = ingest.Hit{Reference: hitRef}
	}
	result, ref := ingest.NavigateReference(snap.Root, s.session.overlay, snap.Result, ingest.ParseReference(hit.Reference))
	loc, err := s.definitionLocation(snap.Root, result, ref)
	if err != nil || loc == nil {
		return nil, nil
	}
	return loc, nil
}

// hitFromIdentifier builds a provisional path:./file::Name from the token under the cursor.
func (s *Server) hitFromIdentifier(path, text string, off int) string {
	tok, _, _ := ingest.IdentifierAt(text, off)
	if tok == "" {
		return ""
	}
	rel := s.session.relPath(path)
	return ingest.SymbolRef("./"+strings.TrimPrefix(rel, "./"), tok)
}

func (s *Server) definitionLocation(root string, result *ingest.Result, ref ingest.Reference) (*protocol.Location, error) {
	canon := ref.String()
	var ent *ingest.Entity
	for i := range result.Entities {
		if result.Entities[i].Reference == canon {
			ent = &result.Entities[i]
			break
		}
	}
	if ent == nil && ref.Symbol != "" {
		for i := range result.Entities {
			er := ingest.ParseReference(result.Entities[i].Reference)
			if er.Symbol == ref.Symbol && (ref.Path == "" || samePath(ref, er)) {
				ent = &result.Entities[i]
				ref = er
				break
			}
		}
	}
	if ent == nil && ref.Symbol != "" {
		// last resort: unique symbol name in navigated result
		for i := range result.Entities {
			er := ingest.ParseReference(result.Entities[i].Reference)
			if er.Symbol == ref.Symbol {
				ent = &result.Entities[i]
				ref = er
				break
			}
		}
	}
	if ent == nil {
		return nil, fmt.Errorf("no entity")
	}
	er := ingest.ParseReference(ent.Reference)
	abs := er.Path
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(er.Path, "./")))
	}
	defText, err := s.readPath(abs)
	if err != nil {
		return nil, err
	}
	r := rangeFromBytes(defText, int(ent.StartByte), int(ent.EndByte), nil)
	return &protocol.Location{URI: pathToURI(abs), Range: r}, nil
}

func samePath(a, b ingest.Reference) bool {
	pa := strings.TrimPrefix(filepath.ToSlash(a.Path), "./")
	pb := strings.TrimPrefix(filepath.ToSlash(b.Path), "./")
	return pa == pb
}

func (s *Server) References(_ context.Context, params *protocol.ReferenceParams) ([]protocol.Location, error) {
	path := uriToPath(params.TextDocument.URI)
	text, ok := s.session.docText(path)
	if !ok {
		return nil, nil
	}
	snap := s.session.snapshot()
	if snap == nil || snap.Result == nil {
		return nil, nil
	}
	off := byteOffset(text, params.Position)
	hit, ok := ingest.HitAtByte(snap.Result, s.session.relPath(path), off)
	if !ok || hit.Reference == "" {
		if r := s.hitFromIdentifier(path, text, off); r != "" {
			hit = ingest.Hit{Reference: r}
		} else {
			return nil, nil
		}
	}
	// Navigation: on-demand expand target neighborhood (node_modules ok).
	// Project-wide usages still come from the snapshot; we merge nav for declaration.
	result, ref := ingest.NavigateReference(snap.Root, s.session.overlay, snap.Result, ingest.ParseReference(hit.Reference))
	target := ref.String()
	var locs []protocol.Location
	if params.Context.IncludeDeclaration {
		if loc, err := s.definitionLocation(snap.Root, result, ref); err == nil && loc != nil {
			locs = append(locs, *loc)
		}
	}
	for _, reln := range result.Relations {
		t := reln.Target
		if t != target {
			ct := ingest.CanonicalizeInResult(result, ingest.ParseReference(reln.Target))
			if ct.String() != target {
				continue
			}
		}
		rr := ingest.ParseReference(reln.Reference)
		abs := rr.Path
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(snap.Root, filepath.FromSlash(strings.TrimPrefix(rr.Path, "./")))
		}
		src, err := s.readPath(abs)
		if err != nil {
			continue
		}
		r := rangeFromBytes(src, int(reln.StartByte), int(reln.EndByte), nil)
		locs = append(locs, protocol.Location{URI: pathToURI(abs), Range: r})
	}
	return locs, nil
}

func (s *Server) readPath(abs string) (string, error) {
	if t, ok := s.session.docText(abs); ok {
		return t, nil
	}
	b, err := s.session.overlay.ReadFile(abs)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (s *Server) Hover(_ context.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
	path := uriToPath(params.TextDocument.URI)
	text, ok := s.session.docText(path)
	if !ok {
		return nil, nil
	}
	snap := s.session.snapshot()
	if snap == nil || snap.Result == nil {
		return nil, nil
	}
	off := byteOffset(text, params.Position)
	hit, ok := ingest.HitAtByte(snap.Result, s.session.relPath(path), off)
	if !ok || hit.Reference == "" {
		if r := s.hitFromIdentifier(path, text, off); r != "" {
			hit = ingest.Hit{Reference: r}
		} else {
			return nil, nil
		}
	}
	_, ref := ingest.NavigateReference(snap.Root, s.session.overlay, snap.Result, ingest.ParseReference(hit.Reference))
	doc, err := ingest.DocFor(snap.Root, ref.String())
	if err != nil || doc == nil {
		return nil, nil
	}
	var b strings.Builder
	if doc.Signature != "" {
		b.WriteString(doc.Signature)
		b.WriteByte('\n')
	}
	if doc.DocString != "" {
		b.WriteString(doc.DocString)
	}
	body := strings.TrimSpace(b.String())
	if body == "" {
		return nil, nil
	}
	mc := &protocol.MarkupContent{Kind: protocol.MarkupKindPlainText, Value: body}
	r := rangeFromBytes(text, int(hit.StartByte), int(hit.EndByte), nil)
	return &protocol.Hover{Contents: mc, Range: &r}, nil
}

func (s *Server) Completion(_ context.Context, params *protocol.CompletionParams) (protocol.CompletionResult, error) {
	path := uriToPath(params.TextDocument.URI)
	text, ok := s.session.docText(path)
	if !ok {
		return protocol.CompletionItemSlice{}, nil
	}
	off := byteOffset(text, params.Position)
	tok, tokStart, _ := ingest.IdentifierAt(text, off)
	if tok == "" && tokStart == off {
		// allow empty prefix at identifier boundary with min 0 — still require something typed for noise control
	}
	const maxItems = 100
	prefix := strings.ToLower(tok)
	var items protocol.CompletionItemSlice
	seen := map[string]bool{}

	add := func(name string) {
		if name == "" || seen[name] {
			return
		}
		if prefix != "" && !strings.HasPrefix(strings.ToLower(name), prefix) {
			return
		}
		seen[name] = true
		te := &protocol.TextEdit{
			Range:   rangeFromBytes(text, tokStart, off, nil),
			NewText: name,
		}
		items = append(items, protocol.CompletionItem{
			Label:    name,
			Kind:     protocol.CompletionItemKindFunction,
			TextEdit: te,
		})
	}

	// document symbols from fast extract
	s.session.mu.RLock()
	fe := s.session.fastExtract[path]
	s.session.mu.RUnlock()
	if fe != nil {
		for _, e := range fe.Entities {
			add(e.Name)
		}
	}

	snap := s.session.snapshot()
	if snap != nil && snap.Result != nil {
		for _, ent := range snap.Result.Entities {
			r := ingest.ParseReference(ent.Reference)
			add(ingest.SymbolLeaf(r.Symbol))
			if len(items) >= maxItems {
				break
			}
		}
	}

	sort.Slice(items, func(i, j int) bool { return items[i].Label < items[j].Label })
	if len(items) > maxItems {
		items = items[:maxItems]
	}
	return items, nil
}

func (s *Server) DocumentSymbol(_ context.Context, params *protocol.DocumentSymbolParams) (protocol.DocumentSymbolResult, error) {
	path := uriToPath(params.TextDocument.URI)
	text, ok := s.session.docText(path)
	if !ok {
		return nil, nil
	}
	s.session.mu.RLock()
	fe := s.session.fastExtract[path]
	s.session.mu.RUnlock()
	if fe == nil {
		return protocol.DocumentSymbolSlice{}, nil
	}
	var out protocol.DocumentSymbolSlice
	for _, e := range fe.Entities {
		r := rangeFromBytes(text, int(e.StartByte), int(e.EndByte), nil)
		out = append(out, protocol.DocumentSymbol{
			Name:           e.Name,
			Kind:           protocol.SymbolKindFunction,
			Range:          r,
			SelectionRange: r,
		})
	}
	return out, nil
}

func (s *Server) WorkspaceSymbol(_ context.Context, params *protocol.WorkspaceSymbolParams) (protocol.WorkspaceSymbolResult, error) {
	snap := s.session.snapshot()
	if snap == nil || snap.Result == nil {
		return protocol.SymbolInformationSlice{}, nil
	}
	q := strings.ToLower(params.Query)
	var out protocol.SymbolInformationSlice
	for _, ent := range snap.Result.Entities {
		ref := ingest.ParseReference(ent.Reference)
		name := ingest.SymbolLeaf(ref.Symbol)
		if q != "" && !strings.Contains(strings.ToLower(name), q) && !strings.Contains(strings.ToLower(ent.Reference), q) {
			continue
		}
		abs := ref.Path
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(snap.Root, filepath.FromSlash(strings.TrimPrefix(ref.Path, "./")))
		}
		src, err := s.readPath(abs)
		if err != nil {
			continue
		}
		r := rangeFromBytes(src, int(ent.StartByte), int(ent.EndByte), nil)
		out = append(out, protocol.SymbolInformation{
			BaseSymbolInformation: protocol.BaseSymbolInformation{
				Name: name,
				Kind: protocol.SymbolKindFunction,
			},
			Location: protocol.Location{
				URI:   pathToURI(abs),
				Range: r,
			},
		})
		if len(out) >= 200 {
			break
		}
	}
	return out, nil
}

func (s *Server) Rename(_ context.Context, params *protocol.RenameParams) (*protocol.WorkspaceEdit, error) {
	path := uriToPath(params.TextDocument.URI)
	text, ok := s.session.docText(path)
	if !ok {
		return nil, fmt.Errorf("document not open")
	}
	snap := s.session.snapshot()
	if snap == nil || snap.Result == nil {
		return nil, fmt.Errorf("no project snapshot yet")
	}
	off := byteOffset(text, params.Position)
	hit, ok := ingest.HitAtByte(snap.Result, s.session.relPath(path), off)
	if !ok || hit.Reference == "" {
		return nil, fmt.Errorf("no symbol at position")
	}
	// Refactor: closed project graph only (no on-demand vendor expansion).
	// Rename planner uses ProjectResult semantics via ingest.Rename.
	srcRef := ingest.CanonicalizeInResult(snap.Result, ingest.ParseReference(hit.Reference))
	if srcRef.Symbol == "" {
		return nil, fmt.Errorf("cannot rename non-symbol")
	}
	dstRef := srcRef
	dstRef.Symbol = params.NewName
	// leaf rename keeps path
	if leaf := ingest.SymbolLeaf(srcRef.Symbol); leaf != srcRef.Symbol {
		// preserve qualifier prefix if any
		if i := strings.LastIndex(srcRef.Symbol, "."); i >= 0 {
			dstRef.Symbol = srcRef.Symbol[:i+1] + params.NewName
		}
	}

	edits, err := ingest.Rename(snap.Root, srcRef.String(), dstRef.String())
	if err != nil {
		return nil, err
	}
	// Stage in memory on current overlay, validate, then emit WorkspaceEdit (client applies).
	staged, err := ingest.StageEdits(snap.Root, s.session.overlay, edits)
	if err != nil {
		return nil, err
	}
	if err := ingest.ValidateStagedProject(snap.Root, staged); err != nil {
		return nil, fmt.Errorf("rename validation failed: %w", err)
	}

	// Build WorkspaceEdit from planned edits (using pre-stage content for ranges).
	changes := map[uri.URI][]protocol.TextEdit{}
	for _, e := range edits {
		abs := filepath.Join(snap.Root, filepath.FromSlash(strings.TrimPrefix(e.File, "./")))
		src, err := s.readPath(abs)
		if err != nil {
			// new file
			src = ""
		}
		r := rangeFromBytes(src, int(e.StartByte), int(e.EndByte), nil)
		u := pathToURI(abs)
		changes[u] = append(changes[u], protocol.TextEdit{
			Range:   r,
			NewText: e.NewText,
		})
	}
	return &protocol.WorkspaceEdit{Changes: changes}, nil
}

var _ protocol.Server = (*Server)(nil)

var (
	_ = slog.Info
)
