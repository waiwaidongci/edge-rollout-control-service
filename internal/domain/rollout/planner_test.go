package rollout

import "testing"

func TestBuildPlanDeterministicAndValid(t *testing.T) {
	candidates := []Candidate{{DeviceID: "a", Online: true}, {DeviceID: "b", Online: false}, {DeviceID: "c", Online: true, Critical: true}}
	plan, err := BuildPlan(candidates, PlanOptions{RolloutID: "r1", BatchPercent: 50, PutCriticalFirst: true, PutOfflineLast: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.Validate(); err != nil {
		t.Fatal(err)
	}
	if plan.BatchCount != 2 || plan.BatchSize != 2 {
		t.Fatalf("unexpected plan: %#v", plan)
	}
}

func TestApplyTransition(t *testing.T) {
	entity := Rollout{ID: "r", Name: "test", ConfigurationID: "c", Status: StatusDraft}
	if _, err := ApplyTransition(&entity, StatusRunning, "tester", "", now()); err != nil {
		t.Fatal(err)
	}
	if entity.Status != StatusRunning {
		t.Fatalf("status: %s", entity.Status)
	}
}
