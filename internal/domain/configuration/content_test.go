package configuration

import "testing"

func TestCompareJSON(t *testing.T) {
	diff, err := CompareJSON(`{"a":1,"nested":{"old":true}}`, `{"a":2,"nested":{"new":true}}`)
	if err != nil {
		t.Fatal(err)
	}
	if diff.Count(ChangeModified) != 1 || diff.Count(ChangeAdded) != 1 || diff.Count(ChangeRemoved) != 1 {
		t.Fatalf("unexpected diff: %#v", diff)
	}
}

func TestRenderVariables(t *testing.T) {
	value, err := RenderVariables(`{"url":"${HOST}","port":"${PORT}"}`, map[string]string{"HOST": "edge.local", "PORT": "8080"})
	if err != nil {
		t.Fatal(err)
	}
	if value != `{"url":"edge.local","port":"8080"}` {
		t.Fatalf("unexpected render: %s", value)
	}
}
