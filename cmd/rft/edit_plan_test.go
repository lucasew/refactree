package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lucasew/refactree/pkg/ingest"
	"github.com/spf13/cobra"
)

func TestPrintEditPlan(t *testing.T) {
	var buf bytes.Buffer
	edits := []ingest.Edit{
		{File: "a.go", Span: ingest.Span{StartByte: 1, EndByte: 2}, NewText: "x"},
	}
	if err := printEditPlan(&buf, edits); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "Edit plan (1 edits)") || !strings.Contains(got, "a.go [1:2]") {
		t.Fatalf("%q", got)
	}
}

func TestConfirmApply(t *testing.T) {
	var w bytes.Buffer
	ok, err := confirmApply(&w, strings.NewReader("y\n"))
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	ok, err = confirmApply(&w, strings.NewReader("n\n"))
	if err != nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
}

func TestApplyEditPlan_DryRunNoWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.go")
	if err := os.WriteFile(path, []byte("package p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	edits := []ingest.Edit{{
		File:    "f.go",
		Span:    ingest.Span{StartByte: 0, EndByte: 9},
		NewText: "package q",
	}}
	cmd := &cobra.Command{}
	var errBuf bytes.Buffer
	cmd.SetErr(&errBuf)
	cmd.SetIn(strings.NewReader(""))
	if err := applyEditPlan(cmd, dir, edits, applyEditPlanOptions{DryRun: true}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "package p\n" {
		t.Fatalf("dry-run wrote file: %q", got)
	}
	if !strings.Contains(errBuf.String(), "Edit plan") {
		t.Fatalf("plan not printed: %q", errBuf.String())
	}
}

func TestApplyEditPlan_InteractiveCancel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.go")
	if err := os.WriteFile(path, []byte("package p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	edits := []ingest.Edit{{
		File:    "f.go",
		Span:    ingest.Span{StartByte: 0, EndByte: 9},
		NewText: "package q",
	}}
	cmd := &cobra.Command{}
	var errBuf bytes.Buffer
	cmd.SetErr(&errBuf)
	cmd.SetIn(strings.NewReader("n\n"))
	err := applyEditPlan(cmd, dir, edits, applyEditPlanOptions{Interactive: true})
	if err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("err=%v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "package p\n" {
		t.Fatalf("cancelled write: %q", got)
	}
}

func TestWriteEdits_Applies(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.go")
	if err := os.WriteFile(path, []byte("package p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	edits := []ingest.Edit{{
		File:    "f.go",
		Span:    ingest.Span{StartByte: 8, EndByte: 9},
		NewText: "q",
	}}
	if err := writeEdits(dir, edits, false); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "package q\n" {
		t.Fatalf("%q", got)
	}
}
