package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/makeplane/plane/apps/cli-go/internal/api"
)

// stubOpenURL swaps openURLFunc for the duration of a test so opening a link does not really
// spawn a browser — it just records the URLs it was asked to open.
func stubOpenURL(t *testing.T) *[]string {
	t.Helper()
	var opened []string
	prev := openURLFunc
	openURLFunc = func(url string) error {
		opened = append(opened, url)
		return nil
	}
	t.Cleanup(func() { openURLFunc = prev })
	return &opened
}

// TestOpenFocusedLinksNoneReportsError checks "o" with no links in the focused pane sets an
// error rather than silently doing nothing.
func TestOpenFocusedLinksNoneReportsError(t *testing.T) {
	stubOpenURL(t)
	m := detailFixture()
	m.detailItem.DescriptionHTML = "<p>Nothing to open here.</p>"
	m.detailFocus = detailPaneDescription

	next, cmd := m.openFocusedLinks()
	m = next.(Model)
	if cmd != nil {
		t.Error("openFocusedLinks with no links returned a command")
	}
	if m.err == "" {
		t.Error("openFocusedLinks with no links must report an error")
	}
}

// TestOpenFocusedLinksSingleOpensDirectly checks a pane with exactly one link opens it right
// away rather than showing a one-item picker.
func TestOpenFocusedLinksSingleOpensDirectly(t *testing.T) {
	opened := stubOpenURL(t)
	m := detailFixture()
	m.detailItem.DescriptionHTML = `<p>See <a href="https://example.com/only">this</a>.</p>`
	m.detailFocus = detailPaneDescription

	next, _ := m.openFocusedLinks()
	m = next.(Model)
	if m.pickerOpen == "links" {
		t.Error("a single link opened the links picker instead of opening it directly")
	}
	if len(*opened) != 1 || (*opened)[0] != "https://example.com/only" {
		t.Fatalf("opened = %v, want exactly [https://example.com/only]", *opened)
	}
	if m.status == "" {
		t.Error("opening the single link should report it in the status line")
	}
}

// TestOpenFocusedLinksMultipleOpensPicker checks a pane with more than one link opens the
// links picker rather than guessing which one to open.
func TestOpenFocusedLinksMultipleOpensPicker(t *testing.T) {
	stubOpenURL(t)
	m := detailFixture()
	m.detailItem.DescriptionHTML = `<p><a href="https://example.com/a">a</a> and ` +
		`<a href="https://example.com/b">b</a></p>`
	m.detailFocus = detailPaneDescription

	next, _ := m.openFocusedLinks()
	m = next.(Model)
	if m.pickerOpen != "links" {
		t.Fatalf("pickerOpen = %q, want links", m.pickerOpen)
	}
	if len(m.linkOptions) != 2 {
		t.Fatalf("linkOptions = %v, want 2 entries", m.linkOptions)
	}
}

// TestOpenFocusedLinksReadsFocusedComment checks that with the comments pane focused, the
// links come from the focused comment's HTML rather than the description.
func TestOpenFocusedLinksReadsFocusedComment(t *testing.T) {
	stubOpenURL(t)
	m := detailFixture()
	m.detailItem.DescriptionHTML = `<p><a href="https://example.com/in-description">d</a></p>`
	m.comments = []api.Comment{
		{ID: "c1", CommentHTML: `<p><a href="https://example.com/in-comment">c</a></p>`},
	}
	m.commentCursor = 0
	m.detailFocus = detailPaneComments

	next, _ := m.openFocusedLinks()
	m = next.(Model)
	if m.err != "" {
		t.Fatalf("unexpected error: %s", m.err)
	}
	if m.status != "Opened https://example.com/in-comment" {
		t.Fatalf("status = %q, want the comment's link opened, not the description's", m.status)
	}
}

// TestUpdatePickerLinksEnterOpensChosenLink checks enter on the links picker opens the
// highlighted URL and closes the picker.
func TestUpdatePickerLinksEnterOpensChosenLink(t *testing.T) {
	opened := stubOpenURL(t)
	m := detailFixture()
	m.pickerOpen = "links"
	m.linkOptions = []string{"https://example.com/a", "https://example.com/b"}
	m.pickerIdx = 1

	next, _ := m.updatePicker(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.pickerOpen != "" {
		t.Error("enter on the links picker did not close it")
	}
	if len(*opened) != 1 || (*opened)[0] != "https://example.com/b" {
		t.Fatalf("opened = %v, want exactly [https://example.com/b]", *opened)
	}
}
