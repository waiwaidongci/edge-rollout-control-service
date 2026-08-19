package receipt

import (
	"testing"
	"time"
)

func TestSortByReceivedAtDoesNotMutateInput(t *testing.T) {
	first := time.Date(2026, 8, 19, 9, 0, 0, 0, time.UTC)
	second := first.Add(time.Minute)
	input := []Receipt{{ID: "old", ReceivedAt: first}, {ID: "new", ReceivedAt: second}}
	_ = SortByReceivedAt(input, true)
	if input[0].ID != "old" {
		t.Fatalf("input order changed: %#v", input)
	}
	fresh := []Receipt{{ID: "stable", ReceivedAt: first}}
	snapshot := ReceiptSnapshot(fresh)
	snapshot[0].ID = "changed"
	if fresh[0].ID != "stable" {
		t.Fatal("snapshot aliases source")
	}
	stamp := first
	clone := CloneReceiptTimestamp(&stamp)
	*clone = second
	if stamp != first {
		t.Fatal("timestamp clone aliases source")
	}
	if !ReceiptSnapshotStable([]Receipt{{ID: "stable"}}) {
		t.Fatal("snapshot stability check failed")
	}
}
