package relate

import "testing"

// A small corpus: DEC-1 -> DEC-2 (related decisions), REQ-3 -> DEC-1 (related
// decisions), DEC-2 -> GHOST (not found), DEC-1 -> DEC-1 (self).
func sampleCorpus() []Entry {
	return []Entry{
		entry("DEC-1", "decision", "accepted", "# One\n\n## Related Decisions\n\n- DEC-2\n- DEC-1\n"),
		entry("DEC-2", "decision", "accepted", "# Two\n\n## Related Decisions\n\n- GHOST\n"),
		entry("REQ-3", "requirement", "accepted", "# Three\n\n## Problem\n\np\n\n## Requirements\n\n- [REQ-001] x.\n\n## Related Decisions\n\n- DEC-1\n"),
	}
}

func TestRelationships_ResolutionOutcomes(t *testing.T) {
	rels := Relationships(sampleCorpus())
	got := map[string]string{} // "source->target" -> issue/resolved
	for _, r := range rels {
		key := r.SourcePath + "->" + r.Target
		if r.ResolvedPath != "" {
			got[key] = "resolved:" + r.ResolvedPath
		} else {
			got[key] = r.Issue
		}
	}
	if got["DEC-1.md->DEC-2"] != "resolved:DEC-2.md" {
		t.Errorf("DEC-1->DEC-2: %q", got["DEC-1.md->DEC-2"])
	}
	if got["DEC-1.md->DEC-1"] != CodeSelfReference {
		t.Errorf("DEC-1->DEC-1 should be self-reference: %q", got["DEC-1.md->DEC-1"])
	}
	if got["DEC-2.md->GHOST"] != CodeTargetNotFound {
		t.Errorf("DEC-2->GHOST should be not-found: %q", got["DEC-2.md->GHOST"])
	}
	if got["REQ-3.md->DEC-1"] != "resolved:DEC-1.md" {
		t.Errorf("REQ-3->DEC-1: %q", got["REQ-3.md->DEC-1"])
	}
}

func TestInboundCounts(t *testing.T) {
	counts := InboundCounts(sampleCorpus())
	// DEC-1 is referenced by REQ-3 (resolved). DEC-2 by DEC-1 (resolved). Self and
	// not-found do not count.
	if counts["DEC-1.md"] != 1 {
		t.Errorf("DEC-1 inbound: got %d want 1", counts["DEC-1.md"])
	}
	if counts["DEC-2.md"] != 1 {
		t.Errorf("DEC-2 inbound: got %d want 1", counts["DEC-2.md"])
	}
}

func TestOutgoing(t *testing.T) {
	rels := Relationships(sampleCorpus())
	out := Outgoing(rels, "DEC-1.md", 0)
	if out.Total != 2 {
		t.Errorf("DEC-1 outgoing total: got %d want 2", out.Total)
	}
	if len(out.BySection["related_decisions"]) != 2 {
		t.Errorf("DEC-1 related_decisions: %+v", out.BySection)
	}
}

func TestOutgoing_Cap(t *testing.T) {
	rels := Relationships(sampleCorpus())
	out := Outgoing(rels, "DEC-1.md", 1)
	if out.Total != 2 {
		t.Errorf("total should report full count 2, got %d", out.Total)
	}
	kept := 0
	for _, ts := range out.BySection {
		kept += len(ts)
	}
	if kept != 1 {
		t.Errorf("cap 1 should keep 1 edge, kept %d", kept)
	}
}

func TestIncoming(t *testing.T) {
	entries := sampleCorpus()
	rels := Relationships(entries)
	in := Incoming(rels, IdentityByPath(entries), "DEC-1.md", 0)
	if in.Total != 1 {
		t.Fatalf("DEC-1 incoming total: got %d want 1", in.Total)
	}
	if in.Items[0].ID != "REQ-3" || in.Items[0].Section != "related_decisions" {
		t.Errorf("incoming item wrong: %+v", in.Items[0])
	}
}

func TestNeighborhoodWalk(t *testing.T) {
	entries := sampleCorpus()
	rels := Relationships(entries)
	idByPath := IdentityByPath(entries)

	// From DEC-2: depth 1 reaches DEC-1 (DEC-1->DEC-2 edge, undirected).
	n1 := Neighborhood(rels, idByPath, "DEC-2.md", 1, 0, 0)
	if len(n1.Nodes) != 1 || n1.Nodes[0].ID != "DEC-1" || n1.Nodes[0].Hops != 1 {
		t.Fatalf("depth 1 from DEC-2: %+v", n1.Nodes)
	}
	// Depth 2 also reaches REQ-3 (REQ-3->DEC-1).
	n2 := Neighborhood(rels, idByPath, "DEC-2.md", 2, 0, 0)
	ids := map[string]int{}
	for _, node := range n2.Nodes {
		ids[node.ID] = node.Hops
	}
	if ids["DEC-1"] != 1 || ids["REQ-3"] != 2 {
		t.Errorf("depth 2 from DEC-2: %+v", n2.Nodes)
	}
	if n2.Truncated {
		t.Error("should not be truncated")
	}
}

func TestNeighborhood_DepthClampAndExcludesOrigin(t *testing.T) {
	entries := sampleCorpus()
	rels := Relationships(entries)
	n := Neighborhood(rels, IdentityByPath(entries), "DEC-1.md", 99, 0, 0)
	for _, node := range n.Nodes {
		if node.Path == "DEC-1.md" {
			t.Error("origin must be excluded from neighborhood")
		}
		if node.Hops > MaxTraversalDepth {
			t.Errorf("hops %d exceeds clamp %d", node.Hops, MaxTraversalDepth)
		}
	}
}

func TestSummarize(t *testing.T) {
	s := Summarize(sampleCorpus())
	// Non-external checked refs: DEC-1->DEC-2 (ok), DEC-1->DEC-1 (self/broken),
	// DEC-2->GHOST (broken), REQ-3->DEC-1 (ok) = 4 checked, 2 broken.
	if s.Total != 4 {
		t.Errorf("total checked: got %d want 4", s.Total)
	}
	if s.Broken != 2 {
		t.Errorf("broken: got %d want 2", s.Broken)
	}
	if s.Valid != 2 {
		t.Errorf("valid: got %d want 2", s.Valid)
	}
	// Known artifacts: DEC-1, DEC-2, REQ-3. Resolved targets: DEC-2, DEC-1.
	// Orphan = REQ-3 (never a resolved target).
	if s.Orphaned != 1 {
		t.Errorf("orphaned: got %d want 1", s.Orphaned)
	}
	// All three declare relationships -> coverage 1.0.
	if s.Coverage != 1.0 {
		t.Errorf("coverage: got %.4f want 1.0", s.Coverage)
	}
}

func TestSummarize_Empty(t *testing.T) {
	s := Summarize(nil)
	if s.Coverage != 1.0 || s.Total != 0 {
		t.Errorf("empty summary: %+v", s)
	}
}
