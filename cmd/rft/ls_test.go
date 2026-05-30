package main

import "testing"

func TestMatchesPathScope_DirectoryNonRecursiveRoot(t *testing.T) {
	if !matchesPathScope("root.go", "", true, false) {
		t.Fatal("expected root.go to match non-recursive root dir")
	}
	if matchesPathScope("rft/root.go", "", true, false) {
		t.Fatal("did not expect nested file to match non-recursive root dir")
	}
}

func TestMatchesPathScope_DirectoryRecursiveRoot(t *testing.T) {
	if !matchesPathScope("root.go", "", true, true) {
		t.Fatal("expected root.go to match recursive root dir")
	}
	if !matchesPathScope("rft/root.go", "", true, true) {
		t.Fatal("expected nested file to match recursive root dir")
	}
}

func TestMatchesPathScope_FileReference(t *testing.T) {
	if !matchesPathScope("rft/root.go", "rft/root.go", false, false) {
		t.Fatal("expected exact file match")
	}
	if matchesPathScope("rft/mv.go", "rft/root.go", false, false) {
		t.Fatal("did not expect different file to match")
	}
}

func TestMatchesPathScope_DirectoryNonRecursiveNamed(t *testing.T) {
	if !matchesPathScope("rft/root.go", "rft", true, false) {
		t.Fatal("expected direct child file to match named dir")
	}
	if matchesPathScope("rft/sub/root.go", "rft", true, false) {
		t.Fatal("did not expect nested file to match named dir non-recursively")
	}
}
