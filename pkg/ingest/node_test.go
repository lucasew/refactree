package ingest

import "testing"

func TestChildByTypeNil(t *testing.T) {
	if ChildByType(nil, "identifier") != nil {
		t.Fatal("expected nil for nil node")
	}
}
