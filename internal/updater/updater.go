// Package updater orchestrates incremental updates to OKF bundles.
package updater

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/chasedputnam/okf-cli/internal/changelog"
	"github.com/chasedputnam/okf-cli/internal/crawler"
	"github.com/chasedputnam/okf-cli/internal/differ"
	"github.com/chasedputnam/okf-cli/internal/importer"
	"github.com/chasedputnam/okf-cli/internal/normalize"
	"github.com/chasedputnam/okf-cli/internal/summarize"
	"github.com/chasedputnam/okf-cli/internal/types"
	"github.com/chasedputnam/okf-cli/internal/util"
	"github.com/chasedputnam/okf-cli/internal/writer"
)

// UpdateOptions configures the update operation.
type UpdateOptions struct {
	BundlePath  string
	Source      string   // optional: override source from changelog
	Force       bool     // skip prompts, apply all changes
	DryRun      bool     // show changes without applying
	Include     []string // patterns for crawl/import
	Exclude     []string
	MaxPages    int // for crawl
	MaxDepth    int // for crawl
	Concurrency int // for crawl

	// Callback for prompting user about changes
	// Returns: apply (apply this change), applyAll (apply all remaining), cancel (cancel update)
	OnPrompt func(changeType differ.ChangeType, files []differ.FileChange) (apply bool, applyAll bool, cancel bool)

	// Callback for progress updates
	OnProgress func(phase string, message string)
}

// UpdateResult contains the result of an update operation.
type UpdateResult struct {
	Added         int
	Modified      int
	Deleted       int
	Skipped       int
	DryRun        bool
	AddedFiles    []string
	ModifiedFiles []string
	DeletedFiles  []string
}

// isURL checks if a string looks like a URL.
func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// Update performs an incremental update of an existing bundle.
func Update(ctx context.Context, opts UpdateOptions) (*UpdateResult, error) {
	// Validate bundle exists
	if _, err := os.Stat(opts.BundlePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("bundle not found: %s", opts.BundlePath)
	}

	// Determine source
	source := opts.Source
	if source == "" {
		var err error
		source, err = changelog.GetSource(opts.BundlePath)
		if err != nil {
			return nil, fmt.Errorf("no source specified and %v. Use --source to specify the source location", err)
		}
	}

	if opts.OnProgress != nil {
		opts.OnProgress("fetching", fmt.Sprintf("Fetching content from %s", source))
	}

	// Fetch new content
	var newDocs []types.NormalizedDocument
	var err error

	if isURL(source) {
		newDocs, err = fetchFromURL(ctx, source, opts)
	} else {
		newDocs, err = fetchFromPath(source, opts)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch content: %w", err)
	}

	if opts.OnProgress != nil {
		opts.OnProgress("diffing", fmt.Sprintf("Comparing %d documents against existing bundle", len(newDocs)))
	}

	// Assign output paths to new docs (needed for diffing)
	assignOutputPaths(newDocs)

	// Diff against existing bundle
	diffResult, err := differ.DiffBundles(opts.BundlePath, newDocs)
	if err != nil {
		return nil, fmt.Errorf("failed to diff bundles: %w", err)
	}

	// Dry run mode - just report and exit
	if opts.DryRun {
		var addedFiles, modifiedFiles, deletedFiles []string
		for _, c := range diffResult.Added {
			addedFiles = append(addedFiles, c.Path)
		}
		for _, c := range diffResult.Modified {
			modifiedFiles = append(modifiedFiles, c.Path)
		}
		for _, c := range diffResult.Deleted {
			deletedFiles = append(deletedFiles, c.Path)
		}
		return &UpdateResult{
			Added:         len(diffResult.Added),
			Modified:      len(diffResult.Modified),
			Deleted:       len(diffResult.Deleted),
			DryRun:        true,
			AddedFiles:    addedFiles,
			ModifiedFiles: modifiedFiles,
			DeletedFiles:  deletedFiles,
		}, nil
	}

	if !diffResult.HasChanges() {
		return &UpdateResult{}, nil
	}

	result := &UpdateResult{}

	// Process additions (always apply without prompting)
	if len(diffResult.Added) > 0 {
		if opts.OnProgress != nil {
			opts.OnProgress("applying", fmt.Sprintf("Adding %d new files", len(diffResult.Added)))
		}
		for _, change := range diffResult.Added {
			if err := applyAdd(opts.BundlePath, change, newDocs); err != nil {
				return nil, err
			}
			result.Added++
			result.AddedFiles = append(result.AddedFiles, change.Path)
		}
	}

	// Process modifications
	if len(diffResult.Modified) > 0 {
		applyAll := opts.Force
		for _, change := range diffResult.Modified {
			if !applyAll && opts.OnPrompt != nil {
				apply, all, cancel := opts.OnPrompt(differ.ChangeModified, []differ.FileChange{change})
				if cancel {
					return result, nil
				}
				if all {
					applyAll = true
				}
				if !apply && !all {
					result.Skipped++
					continue
				}
			}
			if err := applyModify(opts.BundlePath, change, newDocs); err != nil {
				return nil, err
			}
			result.Modified++
			result.ModifiedFiles = append(result.ModifiedFiles, change.Path)
		}
	}

	// Process deletions
	if len(diffResult.Deleted) > 0 {
		applyAll := opts.Force
		for _, change := range diffResult.Deleted {
			if !applyAll && opts.OnPrompt != nil {
				apply, all, cancel := opts.OnPrompt(differ.ChangeDeleted, []differ.FileChange{change})
				if cancel {
					return result, nil
				}
				if all {
					applyAll = true
				}
				if !apply && !all {
					result.Skipped++
					continue
				}
			}
			if err := applyDelete(opts.BundlePath, change); err != nil {
				return nil, err
			}
			result.Deleted++
			result.DeletedFiles = append(result.DeletedFiles, change.Path)
		}
	}

	// Update changelog
	if result.Added > 0 || result.Modified > 0 || result.Deleted > 0 {
		msg := fmt.Sprintf("Updated: %d added, %d modified, %d deleted", result.Added, result.Modified, result.Deleted)
		if err := changelog.AppendEntry(opts.BundlePath, msg); err != nil {
			// Non-fatal, just log
			if opts.OnProgress != nil {
				opts.OnProgress("warning", fmt.Sprintf("Failed to update changelog: %v", err))
			}
		}

		// Regenerate backlinks and index after changes
		if opts.OnProgress != nil {
			opts.OnProgress("backlinks", "Regenerating backlinks and index")
		}
		if err := regenerateBacklinksAndIndex(opts.BundlePath); err != nil {
			if opts.OnProgress != nil {
				opts.OnProgress("warning", fmt.Sprintf("Failed to regenerate backlinks: %v", err))
			}
		}
	}

	return result, nil
}

// fetchFromURL fetches content from a URL using the crawler.
func fetchFromURL(ctx context.Context, url string, opts UpdateOptions) ([]types.NormalizedDocument, error) {
	maxPages := opts.MaxPages
	if maxPages <= 0 {
		maxPages = 100
	}
	maxDepth := opts.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 4
	}
	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = 4
	}

	// Create a temporary directory for crawl output
	tmpDir, err := os.MkdirTemp("", "okf-update-*")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	result, err := crawler.Crawl(ctx, crawler.CrawlOptions{
		SeedURL:       url,
		OutDir:        tmpDir,
		MaxPages:      maxPages,
		MaxDepth:      maxDepth,
		Include:       opts.Include,
		Exclude:       opts.Exclude,
		SameOrigin:    true,
		RespectRobots: true,
		Concurrency:   concurrency,
	})
	if err != nil {
		return nil, err
	}

	return result.Documents, nil
}

// fetchFromPath fetches content from a local path using the importer.
func fetchFromPath(path string, opts UpdateOptions) ([]types.NormalizedDocument, error) {
	// Check path exists
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("source path not found: %s", path)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("source path is not a directory: %s", path)
	}

	// Create temporary output for import
	tmpDir, err := os.MkdirTemp("", "okf-update-*")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	result, err := importer.Import(importer.ImportOptions{
		InputPath: path,
		OutDir:    tmpDir,
		Include:   opts.Include,
		Exclude:   opts.Exclude,
	})
	if err != nil {
		return nil, err
	}

	return result.Documents, nil
}

// assignOutputPaths assigns output paths to documents.
func assignOutputPaths(docs []types.NormalizedDocument) {
	used := make(map[string]bool)
	for i := range docs {
		doc := &docs[i]
		var base string
		if doc.Resource != "" {
			base = util.URLToOutputPath(doc.Resource)
		} else {
			base = util.EnsureMarkdownPath(doc.SourcePath)
		}
		
		candidate := base
		index := 2
		for used[candidate] {
			ext := filepath.Ext(base)
			name := strings.TrimSuffix(base, ext)
			candidate = fmt.Sprintf("%s-%d%s", name, index, ext)
			index++
		}
		used[candidate] = true
		doc.OutputPath = candidate
	}
}

// applyAdd adds a new file to the bundle.
func applyAdd(bundlePath string, change differ.FileChange, newDocs []types.NormalizedDocument) error {
	doc := findDoc(newDocs, change.Path)
	if doc == nil {
		return fmt.Errorf("document not found for path: %s", change.Path)
	}

	outPath := filepath.Join(bundlePath, change.Path)
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return err
	}

	content := generateContent(*doc)
	return os.WriteFile(outPath, []byte(content), 0644)
}

// applyModify modifies an existing file in the bundle.
func applyModify(bundlePath string, change differ.FileChange, newDocs []types.NormalizedDocument) error {
	doc := findDoc(newDocs, change.Path)
	if doc == nil {
		return fmt.Errorf("document not found for path: %s", change.Path)
	}

	outPath := filepath.Join(bundlePath, change.Path)
	content := generateContent(*doc)
	return os.WriteFile(outPath, []byte(content), 0644)
}

// applyDelete deletes a file from the bundle and cleans up empty parent directories.
func applyDelete(bundlePath string, change differ.FileChange) error {
	outPath := filepath.Join(bundlePath, change.Path)
	if err := os.Remove(outPath); err != nil {
		return err
	}

	// Clean up empty parent directories
	dir := filepath.Dir(outPath)
	for dir != bundlePath && dir != "." {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			break
		}
		if err := os.Remove(dir); err != nil {
			break
		}
		dir = filepath.Dir(dir)
	}

	return nil
}

// findDoc finds a document by output path.
func findDoc(docs []types.NormalizedDocument, path string) *types.NormalizedDocument {
	for i := range docs {
		if docs[i].OutputPath == path {
			return &docs[i]
		}
	}
	return nil
}

// generateContent generates the full markdown content with frontmatter and summary callout.
func generateContent(doc types.NormalizedDocument) string {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	
	// Generate summary from document content
	description := normalize.DescriptionFromMarkdown(doc.Markdown)
	sum := summarize.Extract(description, doc.Markdown, doc.Title)
	
	// Build markdown with title
	markdown := writer.WithTitle(doc.Title, doc.Markdown)
	
	// Inject summary callout after title
	if sum.Text != "" {
		callout := summarize.FormatCallout(sum)
		markdown = injectSummaryCallout(markdown, callout)
	}
	
	// Generate frontmatter (backlinks handled separately in regenerateBacklinks)
	fm := writer.GenerateFrontmatter(doc, timestamp)
	return fm + markdown
}

// injectSummaryCallout inserts a summary callout after the first heading.
func injectSummaryCallout(markdown, callout string) string {
	if callout == "" {
		return markdown
	}

	lines := strings.Split(markdown, "\n")

	// Find the first heading and insert callout after it
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "# ") {
			// Insert callout after heading with blank line
			before := strings.Join(lines[:i+1], "\n")
			after := strings.Join(lines[i+1:], "\n")
			return before + "\n\n" + callout + "\n" + after
		}
	}

	// No heading found, prepend callout
	return callout + "\n\n" + markdown
}

// regenerateBacklinksAndIndex reads all concepts, recomputes backlinks, and regenerates index files.
func regenerateBacklinksAndIndex(bundlePath string) error {
	// Read all markdown files in the bundle
	concepts, err := readBundleConcepts(bundlePath)
	if err != nil {
		return fmt.Errorf("reading bundle concepts: %w", err)
	}

	if len(concepts) == 0 {
		return nil
	}

	// Build source to output path mapping (identity for existing bundle)
	sourceToOutput := make(map[string]string)
	for _, c := range concepts {
		sourceToOutput[c.path] = c.path
	}

	// Compute backlinks from all outbound links
	backlinks := computeBacklinksFromConcepts(concepts)

	// Update each concept's frontmatter with backlinks
	timestamp := time.Now().UTC().Format(time.RFC3339)
	conceptsByDir := make(map[string][]indexEntry)

	for _, c := range concepts {
		// Parse existing frontmatter to preserve fields
		docBacklinks := backlinks[c.path]

		// Only rewrite if backlinks changed
		needsUpdate := !backlinksSame(c.backlinks, docBacklinks)
		if needsUpdate {
			newContent := updateFrontmatterBacklinks(c.content, docBacklinks, timestamp)
			outPath := filepath.Join(bundlePath, c.path)
			if err := os.WriteFile(outPath, []byte(newContent), 0644); err != nil {
				return fmt.Errorf("writing %s: %w", c.path, err)
			}
		}

		// Track for index generation
		dir := filepath.Dir(c.path)
		if dir == "." || dir == "" {
			dir = "."
		}
		conceptsByDir[dir] = append(conceptsByDir[dir], indexEntry{
			path:    c.path,
			title:   c.title,
			summary: c.summary,
			docType: c.docType,
			tags:    c.tags,
		})
	}

	// Regenerate index files
	return regenerateIndexFiles(bundlePath, conceptsByDir, timestamp)
}

// bundleConcept holds parsed info about an existing concept.
type bundleConcept struct {
	path      string
	content   string
	title     string
	summary   string
	docType   string
	tags      []string
	backlinks []string
	outbound  []string
}

// indexEntry holds info needed to generate index entries.
type indexEntry struct {
	path    string
	title   string
	summary string
	docType string
	tags    []string
}

// readBundleConcepts reads all markdown concept files from a bundle.
func readBundleConcepts(bundlePath string) ([]bundleConcept, error) {
	var concepts []bundleConcept

	err := filepath.Walk(bundlePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}

		relPath, err := filepath.Rel(bundlePath, path)
		if err != nil {
			return err
		}

		// Skip reserved files
		base := filepath.Base(relPath)
		if base == "index.md" || base == "log.md" || base == "changelog.txt" {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil // Skip files we can't read
		}

		concept := parseBundleConcept(relPath, string(content))
		concepts = append(concepts, concept)
		return nil
	})

	return concepts, err
}

// parseBundleConcept extracts metadata from a concept file.
func parseBundleConcept(path, content string) bundleConcept {
	c := bundleConcept{
		path:    path,
		content: content,
	}

	// Extract frontmatter
	if strings.HasPrefix(content, "---\n") {
		end := strings.Index(content[4:], "\n---")
		if end > 0 {
			fm := content[4 : 4+end]
			// Parse simple YAML fields
			for _, line := range strings.Split(fm, "\n") {
				if strings.HasPrefix(line, "title:") {
					c.title = strings.TrimSpace(strings.TrimPrefix(line, "title:"))
					c.title = strings.Trim(c.title, "\"")
				} else if strings.HasPrefix(line, "type:") {
					c.docType = strings.TrimSpace(strings.TrimPrefix(line, "type:"))
					c.docType = strings.Trim(c.docType, "\"")
				} else if strings.HasPrefix(line, "  - ") && len(c.tags) < 10 {
					// Could be tags or backlinks, handled separately
				}
			}

			// Parse backlinks array
			c.backlinks = parseYAMLArray(fm, "backlinks:")
			c.tags = parseYAMLArray(fm, "tags:")
		}
	}

	// Extract summary from callout
	if idx := strings.Index(content, "> [!summary]"); idx >= 0 {
		lines := strings.Split(content[idx:], "\n")
		var summaryParts []string
		for i, line := range lines {
			if i == 0 {
				continue // Skip the [!summary] line
			}
			if strings.HasPrefix(line, "> ") {
				summaryParts = append(summaryParts, strings.TrimPrefix(line, "> "))
			} else {
				break
			}
		}
		c.summary = strings.Join(summaryParts, " ")
	}

	// Extract outbound links
	c.outbound = extractOutboundLinks(content)

	return c
}

// parseYAMLArray extracts a simple YAML array from frontmatter.
func parseYAMLArray(fm, prefix string) []string {
	var result []string
	inArray := false
	for _, line := range strings.Split(fm, "\n") {
		if strings.HasPrefix(line, prefix) {
			inArray = true
			continue
		}
		if inArray {
			if strings.HasPrefix(line, "  - ") {
				val := strings.TrimPrefix(line, "  - ")
				val = strings.Trim(val, "\"")
				result = append(result, val)
			} else if !strings.HasPrefix(line, "  ") {
				break
			}
		}
	}
	return result
}

// extractOutboundLinks extracts internal markdown links from content.
func extractOutboundLinks(content string) []string {
	var links []string
	// Match [text](path) but not external URLs
	re := regexp.MustCompile(`\[([^\]]*)\]\(([^)]+)\)`)
	matches := re.FindAllStringSubmatch(content, -1)
	seen := make(map[string]bool)
	for _, match := range matches {
		if len(match) >= 3 {
			href := match[2]
			// Skip external links and anchors
			if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") ||
				strings.HasPrefix(href, "//") || strings.HasPrefix(href, "#") {
				continue
			}
			// Normalize path
			href = strings.Split(href, "#")[0] // Remove anchor
			if !seen[href] {
				seen[href] = true
				links = append(links, href)
			}
		}
	}
	return links
}

// computeBacklinksFromConcepts builds backlink map from concept outbound links.
func computeBacklinksFromConcepts(concepts []bundleConcept) map[string][]string {
	backlinks := make(map[string][]string)

	for _, c := range concepts {
		for _, link := range c.outbound {
			// Resolve relative link to absolute path in bundle
			target := resolveLink(c.path, link)
			if target != "" && target != c.path {
				// Add source (without .md) as backlink to target
				source := strings.TrimSuffix(c.path, ".md")
				backlinks[target] = append(backlinks[target], source)
			}
		}
	}

	// Sort backlinks for deterministic output
	for target := range backlinks {
		sortStrings(backlinks[target])
	}

	return backlinks
}

// resolveLink resolves a relative link to an absolute path.
func resolveLink(fromPath, link string) string {
	if link == "" {
		return ""
	}
	dir := filepath.Dir(fromPath)
	resolved := filepath.Join(dir, link)
	resolved = filepath.Clean(resolved)
	// Ensure .md extension
	if !strings.HasSuffix(resolved, ".md") {
		resolved += ".md"
	}
	return resolved
}

// sortStrings sorts a string slice in place.
func sortStrings(s []string) {
	for i := 0; i < len(s)-1; i++ {
		for j := i + 1; j < len(s); j++ {
			if s[i] > s[j] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

// backlinksSame checks if two backlink slices are equal.
func backlinksSame(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// updateFrontmatterBacklinks updates the backlinks field in frontmatter.
func updateFrontmatterBacklinks(content string, backlinks []string, timestamp string) string {
	if !strings.HasPrefix(content, "---\n") {
		return content
	}

	endIdx := strings.Index(content[4:], "\n---")
	if endIdx < 0 {
		return content
	}
	endIdx += 4

	fm := content[4:endIdx]
	body := content[endIdx+4:] // Skip "\n---"

	// Remove existing backlinks and timestamp
	var newFMLines []string
	inBacklinks := false
	for _, line := range strings.Split(fm, "\n") {
		if strings.HasPrefix(line, "backlinks:") {
			inBacklinks = true
			continue
		}
		if inBacklinks {
			if strings.HasPrefix(line, "  - ") {
				continue
			}
			inBacklinks = false
		}
		if strings.HasPrefix(line, "timestamp:") {
			continue
		}
		newFMLines = append(newFMLines, line)
	}

	// Add timestamp
	newFMLines = append(newFMLines, fmt.Sprintf("timestamp: \"%s\"", timestamp))

	// Add backlinks if any
	if len(backlinks) > 0 {
		newFMLines = append(newFMLines, "backlinks:")
		for _, bl := range backlinks {
			newFMLines = append(newFMLines, fmt.Sprintf("  - \"%s\"", bl))
		}
	}

	return "---\n" + strings.Join(newFMLines, "\n") + "\n---" + body
}

// regenerateIndexFiles regenerates index.md files for each directory.
func regenerateIndexFiles(bundlePath string, conceptsByDir map[string][]indexEntry, timestamp string) error {
	// Count total concepts
	totalConcepts := 0
	for _, entries := range conceptsByDir {
		totalConcepts += len(entries)
	}

	for dir, entries := range conceptsByDir {
		// Sort entries by title
		for i := 0; i < len(entries)-1; i++ {
			for j := i + 1; j < len(entries); j++ {
				if entries[i].title > entries[j].title {
					entries[i], entries[j] = entries[j], entries[i]
				}
			}
		}

		var content strings.Builder

		// Root index has enhanced frontmatter
		if dir == "." {
			content.WriteString("---\n")
			content.WriteString("okf_version: \"0.1\"\n")
			content.WriteString(fmt.Sprintf("total_concepts: %d\n", totalConcepts))
			content.WriteString(fmt.Sprintf("generated: %s\n", timestamp))
			content.WriteString("---\n\n")
		}

		// Determine title
		title := "Index"
		if dir == "." {
			title = filepath.Base(bundlePath)
		} else {
			title = filepath.Base(dir)
		}

		content.WriteString("# " + title + "\n\n")
		content.WriteString(fmt.Sprintf("## Concepts (%d)\n\n", len(entries)))

		for _, entry := range entries {
			relLink := filepath.Base(entry.path)
			typeAndTags := entry.docType
			if len(entry.tags) > 0 {
				typeAndTags += ", " + strings.Join(entry.tags, ", ")
			}

			_, _ = fmt.Fprintf(&content, "- [[%s]] · %s\n", strings.TrimSuffix(relLink, ".md"), typeAndTags)
			if entry.summary != "" {
				_, _ = fmt.Fprintf(&content, "  %s\n", entry.summary)
			}
			content.WriteString("\n")
		}

		indexPath := filepath.Join(bundlePath, dir, "index.md")
		if err := os.WriteFile(indexPath, []byte(content.String()), 0644); err != nil {
			return fmt.Errorf("writing index %s: %w", indexPath, err)
		}
	}

	return nil
}
