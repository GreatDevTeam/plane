package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// TestPastelStateColorLightensTowardWhite checks a state's own color comes out visibly
// lighter (closer to white) than the original, rather than being rendered at full saturation
// — the point of the task that asked for this ("make state colors pastel, less bright") — and
// that a malformed/empty color falls back to colorMuted instead of a zero-value black.
func TestPastelStateColorLightensTowardWhite(t *testing.T) {
	got := pastelStateColor("#16a34a") // Plane's default "completed" green
	if got != "#b1e0c2" {
		t.Errorf("pastelStateColor(#16a34a) = %s, want #b1e0c2", got)
	}

	for _, bad := range []string{"", "not-a-color", "#zzzzzz", "#fff"} {
		if got := pastelStateColor(bad); got != colorMuted {
			t.Errorf("pastelStateColor(%q) = %v, want the colorMuted fallback %v", bad, got, colorMuted)
		}
	}
}

// TestLabelColor checks a label's own color is used as-is (unlike pastelStateColor, which
// blends toward white) and that a malformed/empty color falls back to colorMuted rather than a
// zero-value black — the task that asked for labels to carry their own color at all.
func TestLabelColor(t *testing.T) {
	if got := labelColor("#f59e0b"); got != lipgloss.Color("#f59e0b") {
		t.Errorf("labelColor(#f59e0b) = %v, want #f59e0b unchanged", got)
	}
	for _, bad := range []string{"", "not-a-color", "#zzzzzz"} {
		if got := labelColor(bad); got != colorMuted {
			t.Errorf("labelColor(%q) = %v, want the colorMuted fallback %v", bad, got, colorMuted)
		}
	}
}

func TestParseHexColor(t *testing.T) {
	r, g, b, ok := parseHexColor("#16a34a")
	if !ok || r != 0x16 || g != 0xa3 || b != 0x4a {
		t.Errorf("parseHexColor(#16a34a) = %d,%d,%d,%v, want 22,163,74,true", r, g, b, ok)
	}
	if _, _, _, ok := parseHexColor("16a34a"); !ok {
		t.Error("parseHexColor should also accept a hex string without a leading #")
	}
	if _, _, _, ok := parseHexColor("#abc"); ok {
		t.Error("a 3-digit shorthand should not be accepted (State.Color is always 6 digits)")
	}
}
