package device

import "testing"

func TestDeviceLabelNilContracts(t *testing.T) {
	t.Run("clone nil and isolate values", TestCloneLabelsHandlesNilAndCopiesValues)
	t.Run("new device labels", TestNewDeviceLabelsHandlesNilInput)
	t.Run("writable labels", TestWritableLabelsHandlesNilInput)
	t.Run("zero value device", TestEnsureDeviceLabelsInitializesZeroValue)
}

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

func TestNewDeviceLabelsHandlesNilInput(t *testing.T) {
	if labels := NewDeviceLabels(nil); labels == nil {
		t.Fatal("device labels are nil")
	}
}

func TestWritableLabelsHandlesNilInput(t *testing.T) {
	writable := WritableLabels(nil)
	writable["zone"] = "east"
}

func TestEnsureDeviceLabelsInitializesZeroValue(t *testing.T) {
	entity := &Device{}
	EnsureDeviceLabels(entity)
	entity.Labels["model"] = "pump"
	if len(entity.Labels) != 1 {
		t.Fatal("device labels were not writable")
	}
}
