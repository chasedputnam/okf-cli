package validate

import "regexp"

// Ticket format validators, ported from rac-core validation.py TICKETING_PROVIDERS.
// Each accepts the provider's key format or any http(s) URL. The check is pure and
// offline — the engine never contacts the ticketing system.

var (
	ticketURLRe        = regexp.MustCompile(`^https?://\S+$`)
	ticketJiraRe       = regexp.MustCompile(`^[A-Z][A-Z0-9]+-\d+$`)
	ticketLinearRe     = regexp.MustCompile(`^[A-Z][A-Z0-9]*-\d+$`)
	ticketGitHubRe     = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+#\d+$`)
	ticketADORe        = regexp.MustCompile(`^(?:AB#)?\d+$`)
	ticketServiceNowRe = regexp.MustCompile(`^[A-Z]{2,}\d{5,}$`)
)

type ticketValidator struct {
	re    *regexp.Regexp
	label string
}

func (t ticketValidator) valid(entry string) bool {
	return ticketURLRe.MatchString(entry) || t.re.MatchString(entry)
}

// ticketValidators is the recognized provider vocabulary (ADR-088). A provider
// not in this set (or "none"/empty) skips the format-lint.
var ticketValidators = map[string]ticketValidator{
	"jira":         {ticketJiraRe, "Jira key (e.g. PROJ-1234) or URL"},
	"github":       {ticketGitHubRe, "GitHub issue (e.g. owner/repo#123) or URL"},
	"linear":       {ticketLinearRe, "Linear key (e.g. ENG-123) or URL"},
	"azure-devops": {ticketADORe, "Azure DevOps work item (e.g. 1234 or AB#1234) or URL"},
	"servicenow":   {ticketServiceNowRe, "ServiceNow record (e.g. INC0010023) or URL"},
}
