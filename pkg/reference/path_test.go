package reference

import "testing"

func TestLastPathComponent(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"foo", "foo"},
		{"a/b/c", "c"},
		{"pkg/foo/", "foo"},
		{"/abs", "abs"},
	}
	for _, tc := range cases {
		if got := LastPathComponent(tc.in); got != tc.want {
			t.Errorf("LastPathComponent(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}
