// Package model defines the Canon "Product AST": the typed in-memory
// representation of a normative artifact (requirement, decision, design,
// roadmap, prompt). Everything downstream in the authority path reads this AST,
// never raw Markdown text.
//
// model holds only data types. Parsing lives in canon/parse, frontmatter
// hardening in canon/frontmatter, and validation in canon/validate, all of which
// depend on this package — never the reverse.
package model

// Severity levels for issues. They are wire-compatible with the existing
// types.ValidationIssue severities.
const (
	SeverityError   = "error"
	SeverityWarning = "warning"
)

// Issue is a single parse or validation finding. It is a superset of
// types.ValidationIssue (it adds Line) so the gate and the Reference validator
// can share one finding shape and one JSON contract.
type Issue struct {
	Severity string `json:"severity"` // "error" | "warning"
	Code     string `json:"code"`
	Message  string `json:"message"`
	Path     string `json:"path,omitempty"`
	Line     int    `json:"line,omitempty"`
}

// Frontmatter is the strict Canon metadata envelope. It is defined here (rather
// than in canon/frontmatter) so Product can reference it without an import
// cycle; the frontmatter package owns parsing and hardening.
type Frontmatter struct {
	SchemaVersion int                 `yaml:"schema_version" json:"schema_version"`
	ID            string              `yaml:"id" json:"id"`
	Type          string              `yaml:"type" json:"type,omitempty"`
	Relationships map[string][]string `yaml:"relationships,omitempty" json:"relationships,omitempty"`
	Tags          []string            `yaml:"tags,omitempty" json:"tags,omitempty"`
	// Present reports whether a frontmatter block existed at all.
	Present bool `yaml:"-" json:"-"`
}

// Requirement is the most granular RaC unit: a single "[REQ-NNN] <text>" line
// under the Requirements section.
type Requirement struct {
	ID   string `json:"id"`
	Text string `json:"text"`
	Line int    `json:"line"`
}

// MalformedRequirement captures a requirement-like line that could not be parsed
// into a valid Requirement, with a machine-stable reason code, so validation can
// explain why rather than silently dropping it.
//
// Reason is one of: "missing-id" (no [REQ-NNN] token), "empty-text" (valid ID but
// no description), "bad-id" (a bracket ID that is not the canonical REQ-NNN shape).
// BadID holds the offending identifier where one was present.
type MalformedRequirement struct {
	Raw    string `json:"raw"`
	Line   int    `json:"line"`
	Reason string `json:"reason"`
	BadID  string `json:"bad_id,omitempty"`
}

// Product is the parsed AST of one Canon artifact.
type Product struct {
	Title        string                 `json:"title"`
	TitleCount   int                    `json:"title_count"` // number of level-1 (#) headings
	Sections     map[string]string      `json:"sections"`    // normalized heading -> body text
	Order        []string               `json:"order"`    // section headings in document order
	Requirements []Requirement          `json:"requirements,omitempty"`
	Malformed    []MalformedRequirement `json:"malformed,omitempty"`
	Metadata     Frontmatter            `json:"metadata"`
	ParseIssues  []Issue                `json:"parse_issues,omitempty"`
}

// Section returns the body for a normalized heading and whether it exists.
func (p *Product) Section(normalizedHeading string) (string, bool) {
	body, ok := p.Sections[normalizedHeading]
	return body, ok
}
