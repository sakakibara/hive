package project

import "testing"

func TestSubdirs(t *testing.T) {
	expected := []string{"docs", "assets", "links"}
	if len(Subdirs) != len(expected) {
		t.Fatalf("Subdirs = %v, want %v", Subdirs, expected)
	}
	for i, s := range Subdirs {
		if s != expected[i] {
			t.Errorf("Subdirs[%d] = %q, want %q", i, s, expected[i])
		}
	}
}
