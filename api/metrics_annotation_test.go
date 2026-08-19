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
	for worker := 0; worker < 4; worker++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for iteration := 0; iteration < 200; iteration++ {
				if err := registry.Add("events_total", nil, 1); err != nil {
					t.Error(err)
					return
				}
				_ = registry.Snapshot()
			}
		}()
	}
	group.Wait()
}
