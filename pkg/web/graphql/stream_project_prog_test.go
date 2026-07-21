package graphql

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Edges must appear before walk finishes (streaming), not only at the end.
func TestStreamProject_StreamsEdgesDuringWalk(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/app\n\ngo 1.22\n"), 0644)
	// Several packages so bulk would take longer than first batch
	for i, name := range []string{"a", "b", "c", "d", "e", "f"} {
		p := filepath.Join(dir, name)
		os.MkdirAll(p, 0755)
		imp := ""
		if i > 0 {
			imp = "import \"example.com/app/" + string(rune('a'+i-1)) + "\"\n"
		}
		body := "package " + name + "\n\n" + imp + "func F() {}\n"
		os.WriteFile(filepath.Join(p, name+".go"), []byte(body), 0644)
	}
	c := NewSessionCorpus(dir)
	var edgeTimes []time.Time
	var doneAt time.Time
	start := time.Now()
	err := c.StreamProject(context.Background(), func(ev StreamEvent) bool {
		switch ev.Type {
		case "edge":
			edgeTimes = append(edgeTimes, time.Now())
		case "done":
			doneAt = time.Now()
		}
		return true
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(edgeTimes) == 0 {
		t.Fatal("no edges")
	}
	// First edge should arrive well before done (streaming), not in the same instant as bulk finish.
	if !doneAt.After(edgeTimes[0]) {
		t.Fatal("done before first edge")
	}
	// At least one edge before the last 30% of total duration is a weak stream signal;
	// stronger: first edge within early portion of total time.
	total := doneAt.Sub(start)
	first := edgeTimes[0].Sub(start)
	t.Logf("first edge at %v / total %v (%d edges)", first, total, len(edgeTimes))
	if total > 50*time.Millisecond && first > total*3/4 {
		t.Fatalf("first edge too late: %v of %v (not streaming)", first, total)
	}
}
