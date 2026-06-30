package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_MissingFileReturnsDefaults(t *testing.T) {
	dir := t.TempDir()

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RepositoryKey != DefaultRepositoryKey {
		t.Errorf("expected default repo key %q, got %q", DefaultRepositoryKey, cfg.RepositoryKey)
	}
	if len(cfg.CanonRoots) != 1 || cfg.CanonRoots[0] != "canon" {
		t.Errorf("expected default canon roots [canon], got %v", cfg.CanonRoots)
	}
	if cfg.Ticketing.Provider != "github" {
		t.Errorf("expected default provider github, got %q", cfg.Ticketing.Provider)
	}
}

func TestLoad_PartialConfigKeepsDefaults(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "repository_key: PROJ\n")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RepositoryKey != "PROJ" {
		t.Errorf("expected PROJ, got %q", cfg.RepositoryKey)
	}
	// Unspecified fields fall back to defaults.
	if cfg.Ticketing.Provider != "github" {
		t.Errorf("expected default provider preserved, got %q", cfg.Ticketing.Provider)
	}
	if len(cfg.CanonRoots) != 1 || cfg.CanonRoots[0] != "canon" {
		t.Errorf("expected default canon roots preserved, got %v", cfg.CanonRoots)
	}
}

func TestLoad_FullConfig(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `repository_key: RAC
canon_roots: [rac, decisions]
ticketing:
  provider: jira
enforcement:
  blocking: [missing_required_section]
  advisory: [ears_conformance]
  disabled: [iso29148_singular]
`)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RepositoryKey != "RAC" {
		t.Errorf("repo key: got %q", cfg.RepositoryKey)
	}
	if len(cfg.CanonRoots) != 2 || cfg.CanonRoots[1] != "decisions" {
		t.Errorf("canon roots: got %v", cfg.CanonRoots)
	}
	if cfg.Ticketing.Provider != "jira" {
		t.Errorf("provider: got %q", cfg.Ticketing.Provider)
	}
	if len(cfg.Enforcement.Blocking) != 1 || cfg.Enforcement.Blocking[0] != "missing_required_section" {
		t.Errorf("blocking: got %v", cfg.Enforcement.Blocking)
	}
	if len(cfg.Enforcement.Disabled) != 1 || cfg.Enforcement.Disabled[0] != "iso29148_singular" {
		t.Errorf("disabled: got %v", cfg.Enforcement.Disabled)
	}
}

func TestLoad_EmptyCanonRootsFallsBack(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "canon_roots: []\n")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.CanonRoots) != 1 || cfg.CanonRoots[0] != "canon" {
		t.Errorf("expected fallback to [canon], got %v", cfg.CanonRoots)
	}
}

func TestLoad_MalformedYAMLReturnsError(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "repository_key: [unterminated\n")

	if _, err := Load(dir); err == nil {
		t.Fatal("expected error for malformed yaml, got nil")
	}
}

func writeConfig(t *testing.T, dir, content string) {
	t.Helper()
	cfgDir := filepath.Join(dir, ".okf")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
