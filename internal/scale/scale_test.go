package scale

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyze_EmptyBundle(t *testing.T) {
	dir := t.TempDir()

	metrics, ceiling, err := Analyze(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if metrics.ConceptCount != 0 {
		t.Errorf("expected 0 concepts, got %d", metrics.ConceptCount)
	}
	if ceiling.Status != StatusHealthy {
		t.Errorf("expected healthy status, got %s", ceiling.Status)
	}
}

func TestAnalyze_SmallBundle(t *testing.T) {
	dir := t.TempDir()

	// Create 10 small concepts
	for i := 0; i < 10; i++ {
		content := "---\ntype: Guide\n---\n\nSome content here."
		path := filepath.Join(dir, "concepts", "concept-"+string(rune('a'+i))+".md")
		os.MkdirAll(filepath.Dir(path), 0755)
		os.WriteFile(path, []byte(content), 0644)
	}

	metrics, ceiling, err := Analyze(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if metrics.ConceptCount != 10 {
		t.Errorf("expected 10 concepts, got %d", metrics.ConceptCount)
	}
	if ceiling.Status != StatusHealthy {
		t.Errorf("expected healthy status, got %s", ceiling.Status)
	}
}

func TestAnalyze_WarningThreshold(t *testing.T) {
	dir := t.TempDir()

	// Create 101 concepts to trigger warning
	for i := 0; i < 101; i++ {
		content := "---\ntype: Guide\n---\n\nSome content here."
		path := filepath.Join(dir, "concepts", "concept-"+string(rune('a'+i%26))+"-"+string(rune('0'+i/26))+".md")
		os.MkdirAll(filepath.Dir(path), 0755)
		os.WriteFile(path, []byte(content), 0644)
	}

	metrics, ceiling, err := Analyze(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if metrics.ConceptCount != 101 {
		t.Errorf("expected 101 concepts, got %d", metrics.ConceptCount)
	}
	if ceiling.Status != StatusWarning {
		t.Errorf("expected warning status, got %s", ceiling.Status)
	}
}

func TestAnalyze_ExceededThreshold(t *testing.T) {
	dir := t.TempDir()

	// Create 151 concepts to trigger exceeded
	for i := 0; i < 151; i++ {
		content := "---\ntype: Guide\n---\n\nSome content here."
		name := "concept-" + string(rune('a'+i%26)) + "-" + string(rune('a'+i/26%26)) + ".md"
		path := filepath.Join(dir, "concepts", name)
		os.MkdirAll(filepath.Dir(path), 0755)
		os.WriteFile(path, []byte(content), 0644)
	}

	metrics, ceiling, err := Analyze(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if metrics.ConceptCount != 151 {
		t.Errorf("expected 151 concepts, got %d", metrics.ConceptCount)
	}
	if ceiling.Status != StatusExceeded {
		t.Errorf("expected exceeded status, got %s", ceiling.Status)
	}
}

func TestAnalyze_TokenThreshold(t *testing.T) {
	dir := t.TempDir()

	// Create a few concepts with lots of content to exceed token threshold
	// ~400K tokens ≈ 1.6M characters at 4 chars/token
	largeContent := "---\ntype: Guide\n---\n\n" + strings.Repeat("word ", 100000)
	for i := 0; i < 5; i++ {
		path := filepath.Join(dir, "concepts", "large-"+string(rune('a'+i))+".md")
		os.MkdirAll(filepath.Dir(path), 0755)
		os.WriteFile(path, []byte(largeContent), 0644)
	}

	metrics, ceiling, err := Analyze(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ceiling.Status != StatusWarning && ceiling.Status != StatusExceeded {
		t.Errorf("expected warning or exceeded for large token count, got %s (tokens: %d)", ceiling.Status, metrics.TotalTokens)
	}
}

func TestAnalyze_SkipsIndexAndLog(t *testing.T) {
	dir := t.TempDir()

	// Create index.md and log.md (should be skipped)
	os.WriteFile(filepath.Join(dir, "index.md"), []byte("# Index"), 0644)
	os.WriteFile(filepath.Join(dir, "log.md"), []byte("# Log"), 0644)

	// Create one real concept
	os.MkdirAll(filepath.Join(dir, "concepts"), 0755)
	os.WriteFile(filepath.Join(dir, "concepts", "test.md"), []byte("---\ntype: Guide\n---\n"), 0644)

	metrics, _, err := Analyze(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if metrics.ConceptCount != 1 {
		t.Errorf("expected 1 concept (excluding index/log), got %d", metrics.ConceptCount)
	}
}

func TestAnalyze_IndexTokens(t *testing.T) {
	dir := t.TempDir()

	// Create index with some content
	indexContent := "---\nokf_version: \"0.1\"\n---\n# Bundle\n\n- [[concepts/a]]\n- [[concepts/b]]"
	os.WriteFile(filepath.Join(dir, "index.md"), []byte(indexContent), 0644)

	// Create concepts
	os.MkdirAll(filepath.Join(dir, "concepts"), 0755)
	os.WriteFile(filepath.Join(dir, "concepts", "a.md"), []byte("---\ntype: Guide\n---\nContent A"), 0644)
	os.WriteFile(filepath.Join(dir, "concepts", "b.md"), []byte("---\ntype: Guide\n---\nContent B"), 0644)

	metrics, _, err := Analyze(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if metrics.IndexTokens == 0 {
		t.Error("expected non-zero index tokens")
	}
	if metrics.IndexRatio <= 0 {
		t.Error("expected positive index ratio")
	}
}

func TestDetermineCeiling_Boundaries(t *testing.T) {
	tests := []struct {
		name     string
		concepts int
		tokens   int
		expected Status
	}{
		{"healthy_low", 50, 200000, StatusHealthy},
		{"healthy_at_boundary", 100, 400000, StatusHealthy},
		{"warning_concepts", 101, 200000, StatusWarning},
		{"warning_tokens", 50, 400001, StatusWarning},
		{"warning_both", 101, 400001, StatusWarning},
		{"exceeded_concepts", 151, 200000, StatusExceeded},
		{"exceeded_tokens", 50, 600001, StatusExceeded},
		{"exceeded_both", 151, 600001, StatusExceeded},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := &Metrics{ConceptCount: tc.concepts, TotalTokens: tc.tokens}
			ceiling := determineCeiling(m)
			if ceiling.Status != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, ceiling.Status)
			}
		})
	}
}

func TestRAGGuidance(t *testing.T) {
	guidance := RAGGuidance()

	// Check key sections exist
	if !strings.Contains(guidance, "WHY THIS MATTERS") {
		t.Error("guidance missing WHY THIS MATTERS section")
	}
	if !strings.Contains(guidance, "header-based chunking") {
		t.Error("guidance missing chunking recommendation")
	}
	if !strings.Contains(guidance, "DuckDB") {
		t.Error("guidance missing vector store recommendations")
	}
	if !strings.Contains(guidance, "ChromaDB") {
		t.Error("guidance missing ChromaDB mention")
	}
}
