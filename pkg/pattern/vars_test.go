package pattern

import "testing"

func TestCaptureNames(t *testing.T) {
	pat, err := ParsePattern(`func $name:{/^Test(?P<rest>.*)/} ( $t $$$_ @go:testing::T )`)
	if err != nil {
		t.Fatal(err)
	}
	names := CaptureNames(pat)
	want := []string{"name", "rest", "t"}
	if len(names) != len(want) {
		t.Fatalf("got %v want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("got %v want %v", names, want)
		}
	}
}
