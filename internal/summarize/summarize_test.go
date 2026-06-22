package summarize

import (
	"strings"
	"testing"
)

func TestExtract_FromDescription(t *testing.T) {
	s := Extract("This is a meta description.", "# Title\n\nSome body text.", "My Title")

	if s.Source != SourceDescription {
		t.Errorf("expected source %q, got %q", SourceDescription, s.Source)
	}
	if s.Text != "This is a meta description." {
		t.Errorf("unexpected text: %q", s.Text)
	}
}

func TestExtract_FromFirstParagraph(t *testing.T) {
	markdown := `# Getting Started

This is the first paragraph of the document. It explains what the document is about.

This is the second paragraph.`

	s := Extract("", markdown, "Getting Started")

	if s.Source != SourceFirstPara {
		t.Errorf("expected source %q, got %q", SourceFirstPara, s.Source)
	}
	if !strings.Contains(s.Text, "first paragraph") {
		t.Errorf("expected first paragraph text, got: %q", s.Text)
	}
}

func TestExtract_SkipsFrontmatter(t *testing.T) {
	markdown := `---
type: Guide
title: Test
---

# Heading

This is the actual first paragraph.`

	s := Extract("", markdown, "Test")

	if s.Source != SourceFirstPara {
		t.Errorf("expected source %q, got %q", SourceFirstPara, s.Source)
	}
	if !strings.Contains(s.Text, "actual first paragraph") {
		t.Errorf("expected to skip frontmatter, got: %q", s.Text)
	}
}

func TestExtract_SkipsCodeBlocks(t *testing.T) {
	markdown := "# Title\n\n```go\nfunc main() {}\n```\n\nThis is the real paragraph."

	s := Extract("", markdown, "Title")

	if !strings.Contains(s.Text, "real paragraph") {
		t.Errorf("expected to skip code block, got: %q", s.Text)
	}
}

func TestExtract_SkipsCallouts(t *testing.T) {
	markdown := `# Title

> [!note]
> This is a note callout.

This is the actual paragraph.`

	s := Extract("", markdown, "Title")

	if strings.Contains(s.Text, "note callout") {
		t.Errorf("should skip callouts, got: %q", s.Text)
	}
	if !strings.Contains(s.Text, "actual paragraph") {
		t.Errorf("expected actual paragraph, got: %q", s.Text)
	}
}

func TestExtract_FallbackToTitle(t *testing.T) {
	s := Extract("", "", "My Document Title")

	if s.Source != SourceTitle {
		t.Errorf("expected source %q, got %q", SourceTitle, s.Source)
	}
	if s.Text != "My Document Title" {
		t.Errorf("unexpected text: %q", s.Text)
	}
}

func TestExtract_NoSummary(t *testing.T) {
	s := Extract("", "", "")

	if s.Source != SourceNone {
		t.Errorf("expected source %q, got %q", SourceNone, s.Source)
	}
	if s.Text != "" {
		t.Errorf("expected empty text, got: %q", s.Text)
	}
}

func TestExtract_TruncatesLongDescription(t *testing.T) {
	longDesc := strings.Repeat("word ", 100) // ~500 chars
	s := Extract(longDesc, "", "Title")

	if len(s.Text) > MaxSummaryLength+3 { // +3 for "..."
		t.Errorf("text too long: %d chars", len(s.Text))
	}
	if !strings.HasSuffix(s.Text, "...") {
		t.Errorf("expected truncation suffix, got: %q", s.Text)
	}
}

func TestExtract_TruncatesAtWordBoundary(t *testing.T) {
	// Create text where char 200 is in middle of a word
	// 190 chars + space + "longword" = 199 chars, under limit so won't truncate
	// Use 195 chars + space + longword to exceed 200
	desc := strings.Repeat("a", 195) + " longword"
	s := Extract(desc, "", "Title")

	// Should truncate at the space before longword, adding "..."
	if strings.Contains(s.Text, "longword") {
		t.Errorf("should truncate before longword: %q", s.Text)
	}
	if !strings.HasSuffix(s.Text, "...") {
		t.Errorf("should end with ...: %q", s.Text)
	}
}

func TestFormatCallout(t *testing.T) {
	s := Summary{Text: "This is a summary.", Source: SourceDescription}
	callout := FormatCallout(s)

	expected := "> [!summary]\n> This is a summary."
	if callout != expected {
		t.Errorf("expected %q, got %q", expected, callout)
	}
}

func TestFormatCallout_Empty(t *testing.T) {
	s := Summary{Text: "", Source: SourceNone}
	callout := FormatCallout(s)

	if callout != "" {
		t.Errorf("expected empty callout for empty summary, got %q", callout)
	}
}

func TestParseCallout(t *testing.T) {
	body := `> [!summary]
> This is the summary text.
> It spans multiple lines.

# Heading

Content here.`

	text, found := ParseCallout(body)
	if !found {
		t.Error("expected to find callout")
	}
	if text != "This is the summary text. It spans multiple lines." {
		t.Errorf("unexpected parsed text: %q", text)
	}
}

func TestParseCallout_SingleLine(t *testing.T) {
	body := `> [!summary]
> Single line summary.

Other content.`

	text, found := ParseCallout(body)
	if !found {
		t.Error("expected to find callout")
	}
	if text != "Single line summary." {
		t.Errorf("unexpected parsed text: %q", text)
	}
}

func TestParseCallout_NotFound(t *testing.T) {
	body := `# Title

Just regular content.`

	_, found := ParseCallout(body)
	if found {
		t.Error("should not find callout in body without one")
	}
}

func TestHasCallout(t *testing.T) {
	withCallout := "> [!summary]\n> Some text"
	if !HasCallout(withCallout) {
		t.Error("should detect callout")
	}

	withoutCallout := "# Title\n\nNo callout here."
	if HasCallout(withoutCallout) {
		t.Error("should not detect callout when absent")
	}
}

func TestCleanMarkdown(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"**bold** text", "bold text"},
		{"*italic* text", "italic text"},
		{"__bold__ text", "bold text"},
		{"_italic_ text", "italic text"},
		{"`code` here", "code here"},
		{"[link text](http://example.com)", "link text"},
		{"![alt](image.png)", ""},
		{"multiple   spaces", "multiple spaces"},
	}

	for _, tc := range tests {
		result := cleanMarkdown(tc.input)
		if result != tc.expected {
			t.Errorf("cleanMarkdown(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestExtract_SkipsListItems(t *testing.T) {
	markdown := `# Title

- First list item
- Second list item

This is the actual paragraph.`

	s := Extract("", markdown, "Title")

	if strings.Contains(s.Text, "list item") {
		t.Errorf("should skip list items, got: %q", s.Text)
	}
	if !strings.Contains(s.Text, "actual paragraph") {
		t.Errorf("expected actual paragraph, got: %q", s.Text)
	}
}

func TestExtract_SkipsOrderedListItems(t *testing.T) {
	markdown := `# Title

1. First item
2. Second item

This is the paragraph.`

	s := Extract("", markdown, "Title")

	if strings.Contains(s.Text, "First item") {
		t.Errorf("should skip ordered list items, got: %q", s.Text)
	}
}
