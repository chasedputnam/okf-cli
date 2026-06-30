package parse

import "testing"

func TestParse_TitleAndSections(t *testing.T) {
	src := []byte(`---
schema_version: 1
id: OKF-ABCDEFGH1234
type: decision
---

# Use Markdown First

## Status

Accepted

## Context

We need a portable format.
`)
	p := Parse(src)
	if p.Title != "Use Markdown First" {
		t.Errorf("title: got %q", p.Title)
	}
	if !p.Metadata.Present {
		t.Error("expected frontmatter present")
	}
	if p.Metadata.ID != "OKF-ABCDEFGH1234" {
		t.Errorf("id: got %q", p.Metadata.ID)
	}
	if body, ok := p.Section("status"); !ok || body != "Accepted" {
		t.Errorf("status section: ok=%v body=%q", ok, body)
	}
	if body, ok := p.Section("context"); !ok || body != "We need a portable format." {
		t.Errorf("context section: ok=%v body=%q", ok, body)
	}
}

func TestParse_NoFrontmatter(t *testing.T) {
	p := Parse([]byte("# Title\n\n## Status\n\nProposed\n"))
	if p.Metadata.Present {
		t.Error("expected no frontmatter")
	}
	if p.Title != "Title" {
		t.Errorf("title: got %q", p.Title)
	}
}

func TestParse_HeadingInsideCodeFenceIgnored(t *testing.T) {
	src := []byte("# Doc\n\n## Real Section\n\n```\n## Not A Heading\n```\n\nbody text\n")
	p := Parse(src)
	if _, ok := p.Section("not a heading"); ok {
		t.Error("heading inside code fence must not become a section")
	}
	body, ok := p.Section("real section")
	if !ok {
		t.Fatal("real section missing")
	}
	if want := "```\n## Not A Heading\n```\n\nbody text"; body != want {
		t.Errorf("section body mismatch:\n got %q\nwant %q", body, want)
	}
}

func TestParse_SubHeadingsStayInSection(t *testing.T) {
	src := []byte("# Doc\n\n## Context\n\nintro\n\n### Detail\n\nmore\n\n## Decision\n\nchosen\n")
	p := Parse(src)
	ctx, ok := p.Section("context")
	if !ok {
		t.Fatal("context missing")
	}
	if want := "intro\n\n### Detail\n\nmore"; ctx != want {
		t.Errorf("context body: got %q want %q", ctx, want)
	}
	if _, ok := p.Section("decision"); !ok {
		t.Error("decision section missing")
	}
}

func TestParse_Requirements(t *testing.T) {
	src := []byte(`# Spec

## Requirements

[REQ-001] The system SHALL do a thing.
[REQ-002] The system SHALL do another.
`)
	p := Parse(src)
	if len(p.Requirements) != 2 {
		t.Fatalf("expected 2 requirements, got %d: %+v", len(p.Requirements), p.Requirements)
	}
	if p.Requirements[0].ID != "REQ-001" {
		t.Errorf("id: got %q", p.Requirements[0].ID)
	}
	if p.Requirements[0].Text != "The system SHALL do a thing." {
		t.Errorf("text: got %q", p.Requirements[0].Text)
	}
	// Line numbers must point into the original file (frontmatter-aware).
	if p.Requirements[0].Line != 5 {
		t.Errorf("line: got %d want 5", p.Requirements[0].Line)
	}
}

func TestParse_MalformedRequirements(t *testing.T) {
	src := []byte(`# Spec

## Requirements

- [REQ-001] valid one.
- [REQ-] missing number.
- [REQ-002]
- a plain line with no bracket
- [REQ-001] duplicate id text.
`)
	p := Parse(src)
	// Duplicates are kept (validation reports them by counting), so REQ-001 x2 are valid.
	if len(p.Requirements) != 2 {
		t.Fatalf("expected 2 valid requirements (dup kept), got %d: %+v", len(p.Requirements), p.Requirements)
	}
	reasons := map[string]bool{}
	for _, m := range p.Malformed {
		reasons[m.Reason] = true
	}
	for _, want := range []string{"bad-id", "empty-text", "missing-id"} {
		if !reasons[want] {
			t.Errorf("expected malformed reason %q in %+v", want, p.Malformed)
		}
	}
}

func TestParse_RequirementListMarkersStripped(t *testing.T) {
	p := Parse([]byte("# Spec\n\n## Requirements\n\n- [REQ-001] The system SHALL do a thing.\n"))
	if len(p.Requirements) != 1 || p.Requirements[0].ID != "REQ-001" {
		t.Fatalf("list-marked requirement not parsed: %+v", p.Requirements)
	}
}

func TestParse_RequirementsOutsideSectionIgnored(t *testing.T) {
	// A [REQ-001] line not under ## Requirements is not parsed as a requirement.
	src := []byte("# Spec\n\n## Overview\n\n[REQ-001] not in requirements section.\n")
	p := Parse(src)
	if len(p.Requirements) != 0 {
		t.Errorf("expected 0 requirements, got %d", len(p.Requirements))
	}
}

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"Related Decisions":   "related decisions",
		"  STATUS  ":          "status",
		"Related   Tickets":   "related tickets",
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q)=%q want %q", in, got, want)
		}
	}
}
