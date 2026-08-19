package api

import (
	"sync"
	"testing"
)

func TestMetricRegistryConcurrentSnapshot(t *testing.T) {
	registry := NewMetricRegistry()
	if err := registry.Register(MetricDescriptor{Name: "events_total", Help: "events", Type: MetricCounter}); err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	start := make(chan struct{})
	for worker := 0; worker < 4; worker++ {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			for iteration := 0; iteration < 200; iteration++ {
				if err := registry.Add("events_total", nil, 1); err != nil {
					t.Error(err)
					return
				}
				_ = registry.Snapshot()
				_, _ = registry.SnapshotValue("events_total")
				_ = MetricSnapshotData(registry)
				_ = MetricSnapshotCount(registry)
			}
		}()
	}
	close(start)
	group.Wait()
	if len(MetricSnapshotData(registry)) == 0 {
		t.Fatal("snapshot was empty")
	}
	if MetricSnapshotCount(registry) != 1 {
		t.Fatal("snapshot count was inconsistent")
	}
	if value, ok := registry.SnapshotValue("events_total"); !ok || value != 800 {
		t.Fatalf("snapshot value: %v %v", value, ok)
	}
	if MetricSnapshotData(nil) == nil {
		t.Fatal("nil registry did not produce an empty snapshot")
	}
}
