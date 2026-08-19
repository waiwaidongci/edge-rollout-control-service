package audit

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type Timeline struct {
	ResourceType string    `json:"resource_type"`
	ResourceID   string    `json:"resource_id"`
	Events       []Event   `json:"events"`
	FirstAt      time.Time `json:"first_at,omitempty"`
	LastAt       time.Time `json:"last_at,omitempty"`
}

func BuildTimeline(events []Event) (Timeline, error) {
	if len(events) == 0 {
		return Timeline{Events: []Event{}}, nil
	}
	resourceType := events[0].ResourceType
	resourceID := events[0].ResourceID
	ordered := append([]Event(nil), events...)
	for _, event := range ordered {
		if event.ResourceType != resourceType || event.ResourceID != resourceID {
			return Timeline{}, fmt.Errorf("timeline events must belong to the same resource")
		}
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].CreatedAt.Equal(ordered[j].CreatedAt) {
			return ordered[i].ID < ordered[j].ID
		}
		return ordered[i].CreatedAt.Before(ordered[j].CreatedAt)
	})
	return Timeline{ResourceType: resourceType, ResourceID: resourceID, Events: ordered, FirstAt: ordered[0].CreatedAt, LastAt: ordered[len(ordered)-1].CreatedAt}, nil
}

func BuildTimelineAndRun(events []Event, consume func(*Timeline) error) (err error) {
	timeline, err := BuildTimeline(events)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			ClearTimeline(&timeline)
		}
	}()
	return consume(&timeline)
}

func RunTimelineSafely(events []Event, consume func(*Timeline) error) error {
	return BuildTimelineAndRun(events, consume)
}

func (t Timeline) Actions() []string {
	seen := make(map[string]struct{})
	result := make([]string, 0)
	for _, event := range t.Events {
		if _, ok := seen[event.Action]; ok {
			continue
		}
		seen[event.Action] = struct{}{}
		result = append(result, event.Action)
	}
	sort.Strings(result)
	return result
}

func (t Timeline) Actors() []string {
	seen := make(map[string]struct{})
	result := make([]string, 0)
	for _, event := range t.Events {
		if event.Actor == "" {
			continue
		}
		if _, ok := seen[event.Actor]; ok {
			continue
		}
		seen[event.Actor] = struct{}{}
		result = append(result, event.Actor)
	}
	sort.Strings(result)
	return result
}

func (t Timeline) Duration() time.Duration {
	if t.FirstAt.IsZero() || t.LastAt.IsZero() || t.LastAt.Before(t.FirstAt) {
		return 0
	}
	return t.LastAt.Sub(t.FirstAt)
}

func (t Timeline) Find(action string) []Event {
	result := make([]Event, 0)
	for _, event := range t.Events {
		if event.Action == action {
			result = append(result, event)
		}
	}
	return result
}

func (t Timeline) Contains(action string) bool {
	for _, event := range t.Events {
		if event.Action == action {
			return true
		}
	}
	return false
}

func (t Timeline) ValidateSequence(required ...string) error {
	position := 0
	for _, event := range t.Events {
		if position < len(required) && event.Action == required[position] {
			position++
		}
	}
	if position != len(required) {
		return fmt.Errorf("timeline does not contain required action sequence %s", strings.Join(required, " -> "))
	}
	return nil
}

func (t Timeline) JSON() ([]byte, error) {
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal audit timeline: %w", err)
	}
	return append(data, '\n'), nil
}

func MergeTimelines(timelines ...Timeline) []Event {
	result := make([]Event, 0)
	seen := make(map[string]struct{})
	for _, timeline := range timelines {
		for _, event := range timeline.Events {
			if _, ok := seen[event.ID]; ok {
				continue
			}
			seen[event.ID] = struct{}{}
			result = append(result, event)
		}
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result
}
