// Package artifacts defines the typed Canon artifact schemas, ported from
// rac-core's artifacts.py (ARTIFACT_SPECS). It is the single source of truth for
// required/recommended/optional sections, section synonyms, constrained metadata
// enums, and retired-status values, consumed by both the classifier
// (canon/classify) and the validator (canon/validate).
//
// Section names and synonyms are normalized (lower-cased, whitespace collapsed)
// to match parse.Normalize. Metadata enum values keep rac-core's display casing
// (e.g. "Accepted"); comparison is case-insensitive.
package artifacts

// Type identifiers for the five Canon artifact types.
const (
	TypeRequirement = "requirement"
	TypeDecision    = "decision"
	TypeRoadmap     = "roadmap"
	TypePrompt      = "prompt"
	TypeDesign      = "design"
	TypeUnknown     = "unknown"
)

// declarationOrder is rac-core's ARTIFACT_SPECS order, used for classification
// tie-breaks.
var declarationOrder = []string{TypeRequirement, TypeDecision, TypeRoadmap, TypePrompt, TypeDesign}

// Section is a logical section: a canonical normalized name plus synonymous
// headings that also satisfy it.
type Section struct {
	Name     string
	Synonyms []string
}

// ArtifactSpec describes one artifact type (mirrors rac-core's ArtifactSpec).
type ArtifactSpec struct {
	Type        string
	Display     string
	Required    []Section
	Recommended []Section
	Optional    []string // normalized names; recognized but never scored
	// Metadata maps a normalized section name to its allowed values (display
	// casing). A present value not in the set is an error; a missing section is ok.
	Metadata map[string][]string
	// RetiredStatus is the subset of status values that mark an artifact retired
	// (consumed by the relationship status-consistency rule).
	RetiredStatus []string
}

// Matches reports whether a normalized section name satisfies this Section.
func (s Section) Matches(normalized string) bool {
	if normalized == s.Name {
		return true
	}
	for _, syn := range s.Synonyms {
		if normalized == syn {
			return true
		}
	}
	return false
}

// Present reports whether any of the artifact's sections satisfies s.
func (s Section) Present(sections map[string]string) bool {
	for key := range sections {
		if s.Matches(key) {
			return true
		}
	}
	return false
}

// StatusValues returns the allowed status enum (or nil) for convenience.
func (a ArtifactSpec) StatusValues() []string { return a.Metadata["status"] }

// Registry maps a type identifier to its spec.
type Registry map[string]ArtifactSpec

// Ordered returns the registry's types in rac-core declaration order (used for
// deterministic classification tie-breaks).
func (r Registry) Ordered() []string {
	out := make([]string, 0, len(r))
	for _, t := range declarationOrder {
		if _, ok := r[t]; ok {
			out = append(out, t)
		}
	}
	return out
}

// Types returns the registry's type identifiers (declaration order).
func (r Registry) Types() []string { return r.Ordered() }

func sec(name string, synonyms ...string) Section {
	return Section{Name: name, Synonyms: synonyms}
}

// IsRetired reports whether a status value retires an artifact of the given type.
func (r Registry) IsRetired(typ, status string) bool {
	spec, ok := r[typ]
	if !ok {
		return false
	}
	for _, s := range spec.RetiredStatus {
		if equalFold(s, status) {
			return true
		}
	}
	return false
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// Default returns the built-in registry, ported from rac-core ARTIFACT_SPECS.
func Default() Registry {
	statusSpine := []string{"Proposed", "Accepted", "Superseded", "Deprecated"}
	return Registry{
		TypeRequirement: {
			Type:        TypeRequirement,
			Display:     "Requirement",
			Required:    []Section{sec("problem"), sec("requirements")},
			Recommended: []Section{sec("success metrics", "success criteria", "kpis", "kpi"), sec("risks"), sec("assumptions")},
			Optional: []string{
				"related decisions", "related roadmaps", "related prompts",
				"related designs", "related requirements", "related tickets",
			},
			Metadata:      map[string][]string{"status": statusSpine},
			RetiredStatus: []string{"Superseded", "Deprecated"},
		},
		TypeDecision: {
			Type:        TypeDecision,
			Display:     "Decision",
			Required:    []Section{sec("context"), sec("decision"), sec("consequences")},
			Recommended: []Section{sec("status"), sec("category"), sec("alternatives considered", "alternatives", "options considered")},
			Optional: []string{
				"supersedes", "related requirements", "related roadmaps",
				"related designs", "related decisions", "related tickets",
			},
			Metadata: map[string][]string{
				"status":   statusSpine,
				"category": {"Architecture", "Product", "Process", "Technical", "Other"},
			},
			RetiredStatus: []string{"Superseded", "Deprecated"},
		},
		TypeRoadmap: {
			Type:        TypeRoadmap,
			Display:     "Roadmap",
			Required:    []Section{sec("outcomes"), sec("initiatives")},
			Recommended: []Section{sec("success measures", "success metrics"), sec("assumptions"), sec("risks")},
			Optional: []string{
				"related decisions", "related requirements", "related prompts",
				"related designs", "related roadmaps", "related tickets",
			},
			Metadata:      map[string][]string{"status": {"Planned", "Achieved", "Superseded", "Abandoned"}},
			RetiredStatus: []string{"Superseded", "Abandoned"},
		},
		TypePrompt: {
			Type:        TypePrompt,
			Display:     "Prompt",
			Required:    []Section{sec("objective"), sec("input", "input specification"), sec("instructions"), sec("output", "expected output", "output specification")},
			Recommended: []Section{sec("constraints"), sec("examples"), sec("evaluation")},
			Optional: []string{
				"related requirements", "related decisions", "related roadmaps",
				"related designs", "related tickets",
			},
			Metadata:      map[string][]string{"status": {"Active", "Deprecated"}},
			RetiredStatus: []string{"Deprecated"},
		},
		TypeDesign: {
			Type:        TypeDesign,
			Display:     "Design",
			Required:    []Section{sec("context"), sec("user need"), sec("design"), sec("constraints")},
			Recommended: []Section{sec("rationale"), sec("alternatives"), sec("accessibility"), sec("style guidance"), sec("open questions")},
			Optional: []string{
				"related requirements", "related decisions", "related roadmaps",
				"related prompts", "related tickets",
			},
			Metadata:      map[string][]string{"status": statusSpine},
			RetiredStatus: []string{"Superseded", "Deprecated"},
		},
	}
}
