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

func TestFormatRichTextArrowEntity(t *testing.T) {
	html := "<p>before &rarr; after</p>"
	got := formatRichText(html)
	want := "before → after"
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

// TestExtractLinksHrefAndPlainText covers both shapes a link shows up in Plane's editor HTML:
// an <a href="...">'s target, and a bare "http(s)://..." typed as plain text — in the order
// they first appear, with duplicates (an href whose visible text is the same URL) collapsed to
// one entry.
func TestExtractLinksHrefAndPlainText(t *testing.T) {
	html := `<p>See <a href="https://example.com/a">this</a> and also ` +
		`https://example.com/b (in parens) and again <a href="https://example.com/a">dup</a>.</p>`
	got := extractLinks(html)
	want := []string{"https://example.com/a", "https://example.com/b"}
	if len(got) != len(want) {
		t.Fatalf("extractLinks(%q) = %v, want %v", html, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("extractLinks(%q)[%d] = %q, want %q", html, i, got[i], want[i])
		}
	}
}

func TestExtractLinksNoneFound(t *testing.T) {
	if got := extractLinks("<p>Nothing to see here.</p>"); len(got) != 0 {
		t.Errorf("extractLinks with no links = %v, want empty", got)
	}
}

// TestExtractLinksTrimsTrailingPunctuation checks a URL immediately followed by a sentence's
// closing punctuation (or an enclosing paren) does not drag that punctuation into the link.
func TestExtractLinksTrimsTrailingPunctuation(t *testing.T) {
	got := extractLinks("<p>Docs at https://example.com/docs.</p>")
	if len(got) != 1 || got[0] != "https://example.com/docs" {
		t.Fatalf("extractLinks = %v, want [https://example.com/docs]", got)
	}
}
