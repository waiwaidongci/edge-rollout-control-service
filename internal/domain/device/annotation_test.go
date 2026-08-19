package device

import "testing"

func TestCloneLabelsHandlesNilAndCopiesValues(t *testing.T) {
	if got := CloneLabels(nil); got == nil {
		t.Fatal("nil labels should become an empty map")
	}
	original := map[string]string{"zone": "east"}
	clone := CloneLabels(original)
	clone["zone"] = "west"
	if original["zone"] != "east" {
		t.Fatalf("clone aliases input: %#v", original)
	}
}
