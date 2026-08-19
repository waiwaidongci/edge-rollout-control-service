package configuration

import "testing"

func TestCompareJSONRejectsTrailingDocument(t *testing.T) {
	if _, err := CompareJSON(`{"a":1} {"b":2}`, `{"a":1}`); err == nil {
		t.Fatal("trailing JSON document was accepted")
	}
}
