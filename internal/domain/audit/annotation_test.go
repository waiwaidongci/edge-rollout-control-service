package audit

import (
	"errors"
	"testing"
	"time"
)

func TestBuildTimelineAndRunCleansFailedTimeline(t *testing.T) {
	events := []Event{{ID: "a", Action: "created", ResourceType: "rollout", ResourceID: "r", CreatedAt: time.Now()}}
	var seen *Timeline
	err := BuildTimelineAndRun(events, func(timeline *Timeline) error { seen = timeline; return errors.New("stop") })
	if err == nil || seen == nil || seen.Events != nil || seen.ResourceID != "" {
		t.Fatalf("failed timeline retained state: err=%v timeline=%#v", err, seen)
	}
}
