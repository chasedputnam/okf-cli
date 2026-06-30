package recency

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGitProvider_MtimeFallbackWhenNotARepo(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.md")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	want := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(file, want, want); err != nil {
		t.Fatal(err)
	}

	p := NewGitProvider(dir)
	if !p.Degraded() {
		t.Skip("temp dir is unexpectedly inside a git work tree; skipping fallback assertion")
	}
	if _, ok := p.Advisory(); !ok {
		t.Error("expected advisory when degraded")
	}
	got, ok := p.LastModified(file)
	if !ok {
		t.Fatal("expected mtime fallback to succeed")
	}
	if !got.Truncate(time.Second).Equal(want.Truncate(time.Second)) {
		t.Errorf("mtime: got %v want %v", got, want)
	}
}

func TestGitProvider_MissingFile(t *testing.T) {
	dir := t.TempDir()
	p := NewGitProvider(dir)
	if _, ok := p.LastModified(filepath.Join(dir, "nope.md")); ok {
		t.Error("expected false for missing file")
	}
}
