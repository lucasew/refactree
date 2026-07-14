package ingest

import "testing"

func TestChildByTypeNil(t *testing.T) {
	if ChildByType(nil, "identifier") != nil {
		t.Fatal("expected nil for nil node")
	}
}

func TestChildByFieldNil(t *testing.T) {
	if ChildByField(nil, "name") != nil {
		t.Fatal("expected nil for nil node")
	}
}
