// Package summarize provides summary extraction and formatting for OKF concepts.
package summarize

import (
	"regexp"
	"strings"
	"unicode"
)

// MaxSummaryLength is the maximum character length for summaries.
const MaxSummaryLength = 200

// Summary represents an extracted or generated summary.
type Summary struct {
	Text   string // The summary text (max 200 chars)
	Source string // "description" | "firstpara" | "title" | "none"
}

// SourceDescription indicates summary came from meta description.
const SourceDescription = "description"

// SourceFirstPara indicates summary came from first paragraph.
const SourceFirstPara = "firstpara"

// SourceTitle indicates summary came from document title.
const SourceTitle = "title"

// SourceNone indicates no summary could be extracted.
const SourceNone = "none"

// Extract generates a summary from a description and markdown body.
// Priority: description > first paragraph > title fallback
func Extract(description, markdown, title string) Summary {
	// Try description first (from meta tags)
	if desc := strings.TrimSpace(description); desc != "" {
		return Summary{
			Text:   truncateAtWord(desc, MaxSummaryLength),
			Source: SourceDescription,
		}
	}

	// Try first meaningful paragraph from markdown
	if para := extractFirstParagraph(markdown); para != "" {
		return Summary{
			Text:   truncateAtWord(para, MaxSummaryLength),
			Source: SourceFirstPara,
		}
	}

	// Fallback to title
	if t := strings.TrimSpace(title); t != "" {
		return Summary{
			Text:   truncateAtWord(t, MaxSummaryLength),
			Source: SourceTitle,
		}
	}

	return Summary{
		Text:   "",
		Source: SourceNone,
	}
}

// FormatCallout formats a summary as a callout block.
// Returns empty string if summary is empty.
func FormatCallout(s Summary) string {
	if s.Text == "" {
		return ""
	}
	return "> [!summary]\n> " + s.Text
}

// ParseCallout extracts summary text from an existing callout in markdown body.
// Returns the summary text and true if found, empty string and false otherwise.
func ParseCallout(body string) (string, bool) {
	lines := strings.Split(body, "\n")
	inCallout := false
	var parts []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Start of summary callout
		if strings.HasPrefix(trimmed, "> [!summary]") {
			inCallout = true
			continue
		}

		if inCallout {
			// Continue reading callout lines
			if after, found := strings.CutPrefix(trimmed, ">"); found {
				// Extract text after >
				text := strings.TrimSpace(after)
				if text != "" {
					parts = append(parts, text)
				}
			} else {
				// End of callout
				break
			}
		}
	}

	if len(parts) > 0 {
		return strings.Join(parts, " "), true
	}
	return "", false
}

// HasCallout checks if the body contains a summary callout.
func HasCallout(body string) bool {
	return strings.Contains(body, "> [!summary]")
}

// extractFirstParagraph extracts the first meaningful paragraph from markdown.
// Skips headings, code blocks, frontmatter, and empty lines.
func extractFirstParagraph(markdown string) string {
	lines := strings.Split(markdown, "\n")
	inCodeBlock := false
	inFrontmatter := false
	frontmatterCount := 0
	var paraLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Handle frontmatter
		if trimmed == "---" {
			frontmatterCount++
			if frontmatterCount == 1 {
				inFrontmatter = true
				continue
			} else if frontmatterCount == 2 {
				inFrontmatter = false
				continue
			}
		}
		if inFrontmatter {
			continue
		}

		// Handle code blocks
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock {
			continue
		}

		// Skip empty lines before we've started collecting
		if trimmed == "" {
			if len(paraLines) > 0 {
				// End of paragraph
				break
			}
			continue
		}

		// Skip headings
		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Skip callouts (including summary callouts)
		if strings.HasPrefix(trimmed, "> [!") {
			continue
		}

		// Skip blockquotes that are callout continuations
		if strings.HasPrefix(trimmed, ">") && len(paraLines) == 0 {
			continue
		}

		// Skip HTML comments
		if strings.HasPrefix(trimmed, "<!--") {
			continue
		}

		// Skip horizontal rules
		if isHorizontalRule(trimmed) {
			continue
		}

		// Skip list items as first paragraph (they're usually not good summaries)
		if (strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") ||
			strings.HasPrefix(trimmed, "+ ") || isOrderedListItem(trimmed)) && len(paraLines) == 0 {
			continue
		}

		// This is a paragraph line
		paraLines = append(paraLines, trimmed)
	}

	if len(paraLines) == 0 {
		return ""
	}

	// Join and clean up the paragraph
	para := strings.Join(paraLines, " ")
	para = cleanMarkdown(para)
	return para
}

// truncateAtWord truncates text to maxLen, breaking at word boundaries.
func truncateAtWord(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}

	// Find the last space before maxLen
	truncated := text[:maxLen]
	lastSpace := strings.LastIndexFunc(truncated, unicode.IsSpace)

	if lastSpace > maxLen/2 {
		truncated = truncated[:lastSpace]
	}

	return strings.TrimSpace(truncated) + "..."
}

// cleanMarkdown removes common markdown formatting from text.
func cleanMarkdown(text string) string {
	// Remove inline code
	text = regexp.MustCompile("`[^`]+`").ReplaceAllStringFunc(text, func(s string) string {
		return strings.Trim(s, "`")
	})

	// Remove bold/italic markers
	text = regexp.MustCompile(`\*\*([^*]+)\*\*`).ReplaceAllString(text, "$1")
	text = regexp.MustCompile(`\*([^*]+)\*`).ReplaceAllString(text, "$1")
	text = regexp.MustCompile(`__([^_]+)__`).ReplaceAllString(text, "$1")
	text = regexp.MustCompile(`_([^_]+)_`).ReplaceAllString(text, "$1")

	// Remove images first (before links, since images use similar syntax)
	text = regexp.MustCompile(`!\[[^\]]*\]\([^)]+\)`).ReplaceAllString(text, "")

	// Remove links, keep text
	text = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`).ReplaceAllString(text, "$1")

	// Collapse multiple spaces
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")

	return strings.TrimSpace(text)
}

// isHorizontalRule checks if a line is a horizontal rule.
func isHorizontalRule(line string) bool {
	clean := strings.ReplaceAll(line, " ", "")
	if len(clean) < 3 {
		return false
	}
	allDashes := strings.Trim(clean, "-") == ""
	allStars := strings.Trim(clean, "*") == ""
	allUnders := strings.Trim(clean, "_") == ""
	return allDashes || allStars || allUnders
}

// isOrderedListItem checks if a line starts with a numbered list marker.
func isOrderedListItem(line string) bool {
	return regexp.MustCompile(`^\d+\.\s`).MatchString(line)
}
