package tui

import "testing"

func TestFormatRichTextStructure(t *testing.T) {
	html := "<p>Hello <strong>world</strong></p><ul><li>one</li><li>two</li></ul>"
	got := formatRichText(html)
	// lipgloss only emits ANSI styling against a real terminal, so under `go test` this
	// collapses to plain text; the block/list structure is what's under test here.
	want := "Hello world\n\n- one\n- two"
	if got != want {
		t.Errorf("formatRichText(%q) = %q, want %q", html, got, want)
	}
}

func TestFormatRichTextEntitiesAndBreaks(t *testing.T) {
	html := "<p>Tom &amp; Jerry<br>line two</p>"
	got := formatRichText(html)
	want := "Tom & Jerry\nline two"
	if got != want {
		t.Errorf("formatRichText(%q) = %q, want %q", html, got, want)
	}
}

func TestPlainRichTextHasNoANSI(t *testing.T) {
	html := "<p>Hello <strong>world</strong></p>"
	got := plainRichText(html)
	want := "Hello world"
	if got != want {
		t.Errorf("plainRichText(%q) = %q, want %q", html, got, want)
	}
}

func TestPlainToHTMLRoundTrip(t *testing.T) {
	text := "line one\nline <two>\nAT&T"
	html := plainToHTML(text)
	want := "<p>line one</p><p>line &lt;two&gt;</p><p>AT&amp;T</p>"
	if html != want {
		t.Errorf("plainToHTML(%q) = %q, want %q", text, html, want)
	}
	if back := plainRichText(html); back != "line one\nline <two>\nAT&T" {
		t.Errorf("round trip through plainRichText = %q, want original text", back)
	}
}

func TestPlainToHTMLEmpty(t *testing.T) {
	if got := plainToHTML(""); got != "<p></p>" {
		t.Errorf("plainToHTML(\"\") = %q, want %q", got, "<p></p>")
	}
}
