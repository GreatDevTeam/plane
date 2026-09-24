package config

import "testing"

// TestStateColorForAndSetStateColor checks the state color override round-trips per project
// and returns "" (no override) for anything not explicitly set.
func TestStateColorForAndSetStateColor(t *testing.T) {
	var c Config
	if got := c.StateColorFor("proj-1", "state-1"); got != "" {
		t.Fatalf("StateColorFor with nothing set = %q, want empty", got)
	}

	c.SetStateColor("proj-1", "state-1", "#ff8800")
	if got := c.StateColorFor("proj-1", "state-1"); got != "#ff8800" {
		t.Fatalf("StateColorFor after Set = %q, want #ff8800", got)
	}
	// A different project's same state ID must not see this project's override.
	if got := c.StateColorFor("proj-2", "state-1"); got != "" {
		t.Fatalf("StateColorFor for a different project = %q, want empty", got)
	}

	c.SetStateColor("proj-1", "state-1", "#00ff00")
	if got := c.StateColorFor("proj-1", "state-1"); got != "#00ff00" {
		t.Fatalf("StateColorFor after overwriting = %q, want #00ff00", got)
	}
}

// TestLabelColorForAndSetLabelColor is TestStateColorForAndSetStateColor for labels — kept as
// its own test since StateColors/LabelColors are separate maps in Config, not a shared one keyed
// by kind, so a regression in one would not necessarily show up in the other.
func TestLabelColorForAndSetLabelColor(t *testing.T) {
	var c Config
	c.SetLabelColor("proj-1", "label-1", "#123456")
	if got := c.LabelColorFor("proj-1", "label-1"); got != "#123456" {
		t.Fatalf("LabelColorFor after Set = %q, want #123456", got)
	}
	if got := c.StateColorFor("proj-1", "label-1"); got != "" {
		t.Fatalf("StateColorFor must not see a label override: got %q", got)
	}
}
