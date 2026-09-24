package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/makeplane/plane/apps/cli-go/internal/api"
)

func TestHumanSize(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1024 * 1024, "1.0 MiB"},
	}
	for _, c := range cases {
		if got := humanSize(c.n); got != c.want {
			t.Errorf("humanSize(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}

// TestOpenWorkItemPreloadsAttachments checks opening a work item (the board's enter, or a
// parent/sub-task jump) also resets and re-fetches its attachments, not just its comments — so
// the detail meta block's "Attachments:" line (attachmentLines) has something to show without
// the user needing to press "f" first.
func TestOpenWorkItemPreloadsAttachments(t *testing.T) {
	m := relationsFixture(1)
	m.client = api.New("http://example.invalid", "tok")
	m.attachments = []api.Attachment{{ID: "stale"}}
	m.attachmentsLoading = false

	next, cmd := m.openWorkItem(m.items[0])
	m = next.(Model)
	if m.attachments != nil {
		t.Errorf("attachments = %v, want reset to nil for the newly opened item", m.attachments)
	}
	if !m.attachmentsLoading {
		t.Error("attachmentsLoading = false, want true while the fresh copy is in flight")
	}
	if cmd == nil {
		t.Fatal("openWorkItem returned no command")
	}
}

// TestAttachmentLinesReflectsState checks the detail meta block's "Attachments:" row: a loading
// placeholder while the fetch is in flight, an em dash for none, and the list (name + size)
// once some have landed — subTaskLines' equivalent for attachments.
func TestAttachmentLinesReflectsState(t *testing.T) {
	m := detailFixture()

	m.attachmentsLoading = true
	if got := m.attachmentLines(); len(got) != 1 || !strings.Contains(got[0], "loading") {
		t.Fatalf("attachmentLines while loading = %v, want a single loading placeholder", got)
	}

	m.attachmentsLoading = false
	m.attachments = nil
	if got := m.attachmentLines(); len(got) != 1 || !strings.Contains(got[0], "—") {
		t.Fatalf("attachmentLines with none = %v, want a single em-dash row", got)
	}

	att := api.Attachment{ID: "a1"}
	att.Attributes.Name = "spec.pdf"
	att.Attributes.Size = 2048
	m.attachments = []api.Attachment{att}
	got := m.attachmentLines()
	if len(got) != 2 || !strings.Contains(got[0], "Attachments: 1") || !strings.Contains(got[1], "spec.pdf") {
		t.Fatalf("attachmentLines with one attachment = %v, want a count row plus spec.pdf", got)
	}
}

// TestOpenAttachmentsPickerFetches checks "f" opens the picker in a loading state and fires
// off the list request.
func TestOpenAttachmentsPickerFetches(t *testing.T) {
	m := detailFixture()
	m.client = api.New("http://example.invalid", "tok")

	next, cmd := m.openAttachmentsPicker()
	m = next.(Model)
	if m.pickerOpen != "attachments" || !m.attachmentsLoading {
		t.Fatalf("pickerOpen=%q attachmentsLoading=%v, want attachments/true", m.pickerOpen, m.attachmentsLoading)
	}
	if cmd == nil {
		t.Fatal("openAttachmentsPicker returned no command")
	}
}

// TestHandleAttachmentsPopulatesListAndClampsCursor checks a fetch landing fills m.attachments
// and clamps a cursor that ran past the end of a now-shorter list.
func TestHandleAttachmentsPopulatesListAndClampsCursor(t *testing.T) {
	m := detailFixture()
	m.pickerIdx = 5

	next, _ := m.handleAttachments(attachmentsMsg{items: []api.Attachment{{ID: "a1"}}})
	m = next.(Model)
	if m.attachmentsLoading {
		t.Error("attachmentsLoading stayed true after the fetch landed")
	}
	if len(m.attachments) != 1 {
		t.Fatalf("attachments = %v, want 1 entry", m.attachments)
	}
	if m.pickerIdx != 0 {
		t.Errorf("pickerIdx = %d, want clamped to 0", m.pickerIdx)
	}
}

// TestSaveDownloadWritesUnderDownloadsDir checks saveDownload lands the file under
// ~/Downloads/plane-cli and returns that path.
func TestSaveDownloadWritesUnderDownloadsDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path, err := saveDownload("report.pdf", []byte("pdf bytes"))
	if err != nil {
		t.Fatalf("saveDownload: %v", err)
	}
	want := filepath.Join(home, "Downloads", "plane-cli", "report.pdf")
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading saved file: %v", err)
	}
	if string(data) != "pdf bytes" {
		t.Errorf("saved content = %q, want %q", data, "pdf bytes")
	}
}

// TestUpdatePickerAttachmentsAKeyOpensPathPrompt checks "a" on the attachments picker opens
// the attach-a-file prompt (and closes the picker underneath it) rather than doing nothing —
// "a" is otherwise unused across the plain-list pickers.
func TestUpdatePickerAttachmentsAKeyOpensPathPrompt(t *testing.T) {
	m := detailFixture()
	m.pickerOpen = "attachments"

	next, _ := m.updatePicker(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = next.(Model)
	if m.pickerOpen != "" {
		t.Errorf("pickerOpen = %q, want the attachments picker closed", m.pickerOpen)
	}
	if !m.attachPathPromptOpen {
		t.Error("attachPathPromptOpen was not set")
	}
}

// TestUpdateAttachPathPromptRejectsMissingFile checks enter on a path that does not exist
// reports an error and leaves the prompt open, rather than trying to upload it anyway.
func TestUpdateAttachPathPromptRejectsMissingFile(t *testing.T) {
	m := detailFixture()
	next, _ := m.openAttachPathPrompt()
	m = next.(Model)
	m.attachPathInput.SetValue(filepath.Join(t.TempDir(), "does-not-exist.txt"))

	next, cmd := m.updateAttachPathPrompt(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if !m.attachPathPromptOpen {
		t.Error("the prompt closed for a path that does not exist")
	}
	if m.err == "" {
		t.Error("a missing file must report an error")
	}
	if cmd != nil {
		t.Error("a missing file must not start an upload")
	}
}

// TestUpdateAttachPathPromptUploadsExistingFile checks enter on a real file closes the prompt
// and starts the upload.
func TestUpdateAttachPathPromptUploadsExistingFile(t *testing.T) {
	m := detailFixture()
	m.client = api.New("http://example.invalid", "tok")
	next, _ := m.openAttachPathPrompt()
	m = next.(Model)

	path := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("writing fixture file: %v", err)
	}
	m.attachPathInput.SetValue(path)

	next, cmd := m.updateAttachPathPrompt(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.attachPathPromptOpen {
		t.Error("the prompt stayed open for a valid file")
	}
	if !m.attachUploading {
		t.Error("attachUploading was not set")
	}
	if cmd == nil {
		t.Fatal("no upload command was returned")
	}
}
