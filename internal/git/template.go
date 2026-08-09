package git

import (
	"fmt"
	"regexp"
	"strings"
)

// TemplateVars supplies the placeholder values used to expand a branch
// template. Key is the full ticket key (e.g. "PROJ-123"), Project the
// project key ("PROJ"), Number the numeric part ("123"), Board the board id
// or name, and Summary the slugified ticket summary.
type TemplateVars struct {
	Key     string
	Project string
	Number  string
	Board   string
	Summary string
}

var placeholderRe = regexp.MustCompile(`\{[^}]*\}`)

// ResolveTemplate selects the branch template for an issue type. When
// perType contains a template for ticketType (both matched
// case-insensitively) it wins; otherwise the fallback is returned.
func ResolveTemplate(fallback string, perType map[string]string, ticketType string) string {
	key := strings.ToLower(strings.TrimSpace(ticketType))
	if v, ok := perType[key]; ok {
		return v
	}
	for k, v := range perType {
		if strings.ToLower(strings.TrimSpace(k)) == key {
			return v
		}
	}
	return fallback
}

// ExpandTemplate replaces placeholders in a branch template. Supported
// placeholders: {Key} or {TicketKey}, {Project}, {Number}, {Board}, {Summary}.
// {Summary} is slugified automatically. Unknown placeholders and missing
// required values produce an error. Runs of whitespace in the result are
// collapsed to a single dash.
func ExpandTemplate(tmpl string, vars TemplateVars) (string, error) {
	if vars.Summary != "" {
		vars.Summary = Slugify(vars.Summary)
	}

	required := []struct {
		placeholder string
		value       string
	}{
		{"{Key}", vars.Key},
		{"{TicketKey}", vars.Key},
		{"{Project}", vars.Project},
		{"{Number}", vars.Number},
	}
	for _, r := range required {
		if strings.Contains(tmpl, r.placeholder) && r.value == "" {
			return "", fmt.Errorf("branch template %q requires %s, which is empty", tmpl, r.placeholder)
		}
	}

	values := map[string]string{
		"{Key}":       vars.Key,
		"{TicketKey}": vars.Key,
		"{Project}":   vars.Project,
		"{Number}":    vars.Number,
		"{Board}":     vars.Board,
		"{Summary}":   vars.Summary,
	}

	out := placeholderRe.ReplaceAllStringFunc(tmpl, func(ph string) string {
		v, ok := values[ph]
		if !ok {
			return ph // reported below
		}
		return v
	})

	unknown := placeholderRe.FindAllString(out, -1)
	if len(unknown) > 0 {
		return "", fmt.Errorf("unknown branch template placeholder(s) %v (supported: {Key}, {TicketKey}, {Project}, {Number}, {Board}, {Summary})", unknown)
	}

	out = strings.Join(strings.Fields(out), "-")
	if out == "" {
		return "", fmt.Errorf("branch template %q produced an empty branch name", tmpl)
	}
	if strings.Contains(out, "{") || strings.Contains(out, "}") {
		return "", fmt.Errorf("branch template %q contains an unresolved placeholder", tmpl)
	}
	return out, nil
}

// Slugify lowercases s and replaces runs of non-alphanumeric characters with
// a single dash, trimming leading/trailing dashes.
func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	re := regexp.MustCompile(`[^a-z0-9]+`)
	return strings.Trim(re.ReplaceAllString(s, "-"), "-")
}
