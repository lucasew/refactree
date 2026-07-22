package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderExpectedIngestJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	payload, err := renderExpectedIngestJSON(dir)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	var got struct {
		Files []struct {
			Language string `json:"language"`
			Path     string `json:"path"`
		} `json:"files"`
	}
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("json parse: %v\n%s", err, payload)
	}
	if len(got.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(got.Files))
	}
	if got.Files[0].Path != "main.go" {
		t.Fatalf("unexpected path: %q", got.Files[0].Path)
	}
}

func TestRenderExpectedIngestText(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	payload, err := renderExpectedIngestText(dir)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	text := string(payload)
	if !strings.Contains(text, "Files (1):") {
		t.Fatalf("expected files header, got:\n%s", text)
	}
	if !strings.Contains(text, "- main.go [go]") {
		t.Fatalf("expected file entry, got:\n%s", text)
	}
	if !strings.Contains(text, "Atoms (1):") {
		t.Fatalf("expected entities header, got:\n%s", text)
	}
}
