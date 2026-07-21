package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestGraphQL_FilesystemAndCode(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv, err := New(Options{RootDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	body := map[string]any{
		"query": `{ rootDir filesystem { name reference isDir } }`,
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Data *struct {
			RootDir    string `json:"rootDir"`
			Filesystem []struct {
				Name string `json:"name"`
			} `json:"filesystem"`
		} `json:"data"`
		Errors []any `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Errors) > 0 {
		t.Fatalf("errors: %v", resp.Errors)
	}
	if resp.Data == nil || len(resp.Data.Filesystem) == 0 {
		t.Fatalf("expected filesystem entries: %+v", resp)
	}

	// SPA index
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	rec2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec2, req2)
	if rec2.Code != 200 {
		t.Fatalf("spa status=%d", rec2.Code)
	}
	if !bytes.Contains(rec2.Body.Bytes(), []byte("root")) {
		t.Fatalf("expected spa html, got %s", rec2.Body.String()[:min(200, rec2.Body.Len())])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
