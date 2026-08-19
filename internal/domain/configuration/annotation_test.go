package configuration

import "testing"

func TestCompareJSONRejectsTrailingDocument(t *testing.T) {
	if _, err := CompareJSON(`{"a":1} {"b":2}`, `{"a":1}`); err == nil {
		t.Fatal("trailing JSON document was accepted")
	}
	if _, err := DecodeConfigurationDocument(`{"a":1} {"b":2}`); err == nil {
		t.Fatal("content decoder accepted trailing document")
	}
	if err := ValidateDiffInputs(`{"a":1} {"b":2}`, `{"a":1}`); err == nil {
		t.Fatal("diff inputs accepted trailing document")
	}
	if err := ValidateSingleConfiguration(`{"a":1} {"b":2}`); err == nil {
		t.Fatal("single configuration accepted trailing document")
	}
}
