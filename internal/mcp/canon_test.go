package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func canonStore(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "canon/adr-001.md", `---
schema_version: 1
id: OKF-0123456789AB
type: decision
---

# Caching Decision

## Status

Accepted

## Context

We need caching for documents.

## Decision

We SHALL cache documents in memory.

## Consequences

Faster lookups.
`)
	writeFile(t, root, "guides/cache.md", "---\ntype: Guide\ntitle: Cache Guide\n---\n\n# Cache Guide\n\nHow document caching works.\n")
	return root
}

func parseResult(t *testing.T, r *mcp.CallToolResult) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(r.Content[0].(mcp.TextContent).Text), &m); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	return m
}

func TestHandleGetArtifact_AuthorityMetadata(t *testing.T) {
	srv := createTestServer(t, canonStore(t))
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"id": "OKF-0123456789AB"}

	res, err := srv.handleGetArtifact(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	m := parseResult(t, res)
	if m["type"] != "decision" {
		t.Errorf("type: %v", m["type"])
	}
	if m["authoritative"] != true {
		t.Errorf("expected authoritative=true, got %v", m["authoritative"])
	}
	if m["citation"] == nil {
		t.Error("missing citation")
	}
}

func TestHandleGetRelated_IncomingOutgoingNeighborhood(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "canon/adr-001.md", `---
schema_version: 1
id: OKF-0000000000AA
type: decision
---

# One

## Status

Accepted

## Context

c

## Decision

d

## Consequences

x

## Related Decisions

- adr-002
`)
	writeFile(t, root, "canon/adr-002.md", "---\nschema_version: 1\nid: OKF-1111111111BB\ntype: decision\n---\n\n# Two\n\n## Status\n\nAccepted\n\n## Context\n\nc\n\n## Decision\n\nd\n\n## Consequences\n\nx\n")
	srv := createTestServer(t, root)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"id": "OKF-0000000000AA", "depth": float64(1)}
	res, err := srv.handleGetRelated(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	m := parseResult(t, res)
	outgoing := m["outgoing"].(map[string]any)
	if int(outgoing["total"].(float64)) != 1 {
		t.Errorf("expected 1 outgoing, got %v", outgoing["total"])
	}
	nb := m["neighborhood"].(map[string]any)
	nodes := nb["nodes"].([]any)
	if len(nodes) != 1 || nodes[0].(map[string]any)["id"] != "OKF-1111111111BB" {
		t.Errorf("expected neighborhood to reach OKF-1111111111BB, got %+v", nodes)
	}

	// adr-002 should see adr-001 as an incoming reference.
	req2 := mcp.CallToolRequest{}
	req2.Params.Arguments = map[string]any{"id": "OKF-1111111111BB"}
	res2, err := srv.handleGetRelated(context.Background(), req2)
	if err != nil {
		t.Fatal(err)
	}
	incoming := parseResult(t, res2)["incoming"].(map[string]any)
	if int(incoming["total"].(float64)) != 1 {
		t.Errorf("expected 1 incoming for adr-002, got %v", incoming["total"])
	}
}

func TestHandleGetSummary(t *testing.T) {
	srv := createTestServer(t, canonStore(t))
	res, err := srv.handleGetSummary(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatal(err)
	}
	m := parseResult(t, res)
	if int(m["canon_count"].(float64)) != 1 {
		t.Errorf("canon_count: %v", m["canon_count"])
	}
}

func TestHandleFindDecisions(t *testing.T) {
	srv := createTestServer(t, canonStore(t))
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"topic": "caching"}
	res, err := srv.handleFindDecisions(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	m := parseResult(t, res)
	decisions, _ := m["decisions"].([]any)
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
}

func TestHandleGetContext_UnifiedHasCitationAndBudget(t *testing.T) {
	srv := createTestServer(t, canonStore(t))
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"query": "caching documents", "token_budget": float64(500)}
	res, err := srv.handleGetContext(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	m := parseResult(t, res)
	items, _ := m["items"].([]any)
	if len(items) == 0 {
		t.Fatal("no items")
	}
	first := items[0].(map[string]any)
	if first["tier"] != "canon" {
		t.Errorf("expected canon ranked first, got %v", first["tier"])
	}
	if first["citation"] == nil {
		t.Error("canon item missing citation")
	}
	if int(m["total_tokens"].(float64)) > 500 {
		t.Errorf("total_tokens exceeds budget: %v", m["total_tokens"])
	}
}

func TestCanonTools_ReadOnly(t *testing.T) {
	root := canonStore(t)
	before := snapshot(t, root)

	srv := createTestServer(t, root)
	ctx := context.Background()
	calls := []func(){
		func() {
			req := mcp.CallToolRequest{}
			req.Params.Arguments = map[string]any{"id": "OKF-0123456789AB"}
			_, _ = srv.handleGetArtifact(ctx, req)
		},
		func() {
			req := mcp.CallToolRequest{}
			req.Params.Arguments = map[string]any{"topic": "caching"}
			_, _ = srv.handleFindDecisions(ctx, req)
		},
		func() {
			req := mcp.CallToolRequest{}
			req.Params.Arguments = map[string]any{"id": "OKF-0123456789AB", "depth": float64(2)}
			_, _ = srv.handleGetRelated(ctx, req)
		},
		func() { _, _ = srv.handleGetSummary(ctx, mcp.CallToolRequest{}) },
		func() {
			req := mcp.CallToolRequest{}
			req.Params.Arguments = map[string]any{"query": "caching"}
			_, _ = srv.handleGetContext(ctx, req)
		},
	}
	for _, c := range calls {
		c()
	}

	after := snapshot(t, root)
	if len(before) != len(after) {
		t.Fatalf("file set changed: %d -> %d", len(before), len(after))
	}
	for i := range before {
		if before[i] != after[i] {
			t.Errorf("file changed by a read-only tool: %q -> %q", before[i], after[i])
		}
	}
}

// snapshot returns a sorted list of "path|size|modtime" for every file under root.
func snapshot(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		out = append(out, p+"|"+strconv.FormatInt(info.Size(), 10)+"|"+info.ModTime().String())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(out)
	return out
}
