package tui

import (
	"context"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/makeplane/plane/apps/cli-go/internal/api"
)

// attachmentTimeout is the deadline for one attachment round trip (list, or the download half
// of downloadAttachment) — more generous than requestTimeout since a file transfer, unlike a
// JSON list/PATCH, can legitimately take a while.
const attachmentTimeout = 60 * time.Second

// openAttachmentsPicker is the detail screen's "f": it opens the attachments list, fetching a
// fresh copy every time rather than showing a cached one — unlike comments there is no 30s
// auto-refresh keeping it current in the background.
func (m Model) openAttachmentsPicker() (tea.Model, tea.Cmd) {
	if m.detailItem == nil {
		return m, nil
	}
	m.pickerOpen = "attachments"
	m.pickerIdx = 0
	m.attachmentsLoading = true
	m.setError(nil)
	return m, fetchAttachments(m.client, m.workspaceSlug, m.project.ID, m.detailItem.ID)
}

type attachmentsMsg struct {
	items []api.Attachment
	err   error
}

func fetchAttachments(client *api.Client, workspaceSlug, projectID, workItemID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), attachmentTimeout)
		defer cancel()
		items, err := client.ListAttachments(ctx, workspaceSlug, projectID, workItemID)
		return attachmentsMsg{items: items, err: err}
	}
}

func (m Model) handleAttachments(msg attachmentsMsg) (tea.Model, tea.Cmd) {
	m.attachmentsLoading = false
	if msg.err != nil {
		m.setError(msg.err)
		return m, nil
	}
	m.setError(nil)
	m.attachments = msg.items
	if m.pickerIdx >= len(m.attachments) {
		m.pickerIdx = 0
	}
	return m, nil
}

// downloadAttachmentCmd is the attachments picker's enter key on an attachment: it saves the
// file under ~/Downloads/plane-cli and reports the path once it lands.
func (m Model) downloadAttachmentCmd(att api.Attachment) (tea.Model, tea.Cmd) {
	if m.detailItem == nil {
		return m, nil
	}
	m.status = "Downloading " + att.Name() + "..."
	return m, downloadAttachment(m.client, m.workspaceSlug, m.project.ID, m.detailItem.ID, att)
}

type attachmentDownloadedMsg struct {
	path string
	err  error
}

func downloadAttachment(client *api.Client, workspaceSlug, projectID, workItemID string, att api.Attachment) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), attachmentTimeout)
		defer cancel()
		data, err := client.DownloadAttachment(ctx, workspaceSlug, projectID, workItemID, att.ID)
		if err != nil {
			return attachmentDownloadedMsg{err: err}
		}
		path, err := saveDownload(att.Name(), data)
		return attachmentDownloadedMsg{path: path, err: err}
	}
}

// saveDownload writes a downloaded attachment's bytes under ~/Downloads/plane-cli, creating
// that directory if needed, and returns the path it landed at. A same-named file already there
// is overwritten: the user just explicitly asked to download this attachment again.
func saveDownload(name string, data []byte) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, "Downloads", "plane-cli")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, filepath.Base(name))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func (m Model) handleAttachmentDownloaded(msg attachmentDownloadedMsg) (tea.Model, tea.Cmd) {
	m.status = ""
	if msg.err != nil {
		m.setError(msg.err)
		return m, nil
	}
	m.setError(nil)
	m.status = "Saved to " + msg.path
	return m, nil
}

// openAttachPathPrompt is the attachments picker's "a": a bare path input, since plane-cli has
// no file browser of its own to pick one with.
func (m Model) openAttachPathPrompt() (tea.Model, tea.Cmd) {
	m.setError(nil)
	m.attachPathInput.Reset()
	m.attachPathInput.Focus()
	m.attachPathPromptOpen = true
	return m, nil
}

// updateAttachPathPrompt drives the attach-a-file path prompt: enter validates the path exists
// and is a regular file, then kicks off the three-step upload (see uploadAttachment).
func (m Model) updateAttachPathPrompt(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			m.attachPathPromptOpen = false
			m.attachPathInput.Blur()
			m.attachPathInput.Reset()
			return m, nil
		case "enter":
			path := strings.TrimSpace(m.attachPathInput.Value())
			info, err := os.Stat(path)
			if err != nil {
				m.setError(fmt.Errorf("attach: %w", err))
				return m, nil
			}
			if info.IsDir() {
				m.setError(fmt.Errorf("attach: %s is a directory", path))
				return m, nil
			}
			m.attachPathPromptOpen = false
			m.attachPathInput.Blur()
			m.attachPathInput.Reset()
			m.attachUploading = true
			m.setError(nil)
			m.status = "Uploading " + filepath.Base(path) + "..."
			mimeType := mime.TypeByExtension(filepath.Ext(path))
			if mimeType == "" {
				mimeType = "application/octet-stream"
			}
			return m, uploadAttachment(m.client, m.workspaceSlug, m.project.ID, m.detailItem.ID, path, mimeType, info.Size())
		}
	}
	var cmd tea.Cmd
	m.attachPathInput, cmd = m.attachPathInput.Update(msg)
	return m, cmd
}

// viewAttachPathPrompt renders the attach-a-file path prompt.
func (m Model) viewAttachPathPrompt() string {
	body := columnHeaderStyle.Render("Attach a file (local path)") + "\n" + m.attachPathInput.View() + "\n" +
		helpStyle.Render("enter  upload    esc  cancel")
	return focusedInputStyle.Render(body)
}

type attachmentUploadedMsg struct {
	err error
}

// uploadAttachment runs the three-step upload — CreateAttachment (reserve + presigned S3 POST),
// UploadAttachmentFile (the actual bytes), ConfirmAttachmentUploaded (mark it visible) — as one
// tea.Cmd, since bubbletea has no notion of a multi-step command and every step here is
// synchronous I/O anyway.
func uploadAttachment(client *api.Client, workspaceSlug, projectID, workItemID, path, mimeType string, size int64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), attachmentTimeout)
		defer cancel()
		name := filepath.Base(path)
		assetID, uploadURL, fields, err := client.CreateAttachment(ctx, workspaceSlug, projectID, workItemID, name, mimeType, size)
		if err != nil {
			return attachmentUploadedMsg{err: err}
		}
		if err := client.UploadAttachmentFile(ctx, uploadURL, fields, path, mimeType); err != nil {
			return attachmentUploadedMsg{err: err}
		}
		if err := client.ConfirmAttachmentUploaded(ctx, workspaceSlug, projectID, workItemID, assetID); err != nil {
			return attachmentUploadedMsg{err: err}
		}
		return attachmentUploadedMsg{}
	}
}

// handleAttachmentUploaded re-opens the attachments picker with a fresh list once an upload
// finishes, so the newly attached file shows up immediately instead of the user having to press
// "f" again.
func (m Model) handleAttachmentUploaded(msg attachmentUploadedMsg) (tea.Model, tea.Cmd) {
	m.attachUploading = false
	if msg.err != nil {
		m.status = ""
		m.setError(msg.err)
		return m, nil
	}
	m.setError(nil)
	m.status = "Attached"
	if m.detailItem == nil {
		return m, nil
	}
	m.pickerOpen = "attachments"
	m.pickerIdx = 0
	m.attachmentsLoading = true
	return m, fetchAttachments(m.client, m.workspaceSlug, m.project.ID, m.detailItem.ID)
}

// humanSize renders a byte count the way a file manager would ("1.2 MiB") rather than a bare
// number of bytes, which is what api.Attachment.Size returns.
func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
