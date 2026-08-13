package command

import (
	"fmt"
	"regexp"
	"strings"
)

// ACResult is one acceptance criterion result from an implement report.
type ACResult struct {
	// Pass reports whether the agent marked the criterion as met ([x]).
	Pass bool
	// Text is the criterion text following the checkbox.
	Text string
}

// acCheckboxRe matches a task-list checkbox line under the `## AC Results`
// section. Both `[x]` and `[X]` count as marked.
var acCheckboxRe = regexp.MustCompile(`^\s*-\s*\[(x|X| )\]\s*(.*)$`)

// aceResultsHeading is the markdown section the implement-plan skill must emit
// with one - [x]/[ ] line per acceptance criterion.
const aceResultsHeading = "AC Results"

// parseACResults extracts the acceptance-criteria results from an implement
// report. It returns the parsed results and whether a `## AC Results` section
// was found at all. An absent section yields (nil, false) so callers can
// distinguish "no results reported" from "results reported but unparseable".
func parseACResults(md string) ([]ACResult, bool) {
	lines := sectionLines(md, aceResultsHeading)
	if lines == nil {
		return nil, false
	}

	var out []ACResult
	for _, ln := range lines {
		m := acCheckboxRe.FindStringSubmatch(strings.TrimSpace(ln))
		if m == nil {
			continue
		}
		out = append(out, ACResult{
			Pass: strings.EqualFold(m[1], "x"),
			Text: strings.TrimSpace(m[2]),
		})
	}
	return out, true
}

// acResultsSummary renders a human-readable one-line summary of AC results.
func acResultsSummary(results []ACResult) string {
	if len(results) == 0 {
		return "no acceptance criteria results reported"
	}
	var pass, fail int
	for _, r := range results {
		if r.Pass {
			pass++
		} else {
			fail++
		}
	}
	switch {
	case fail == 0:
		return "all acceptance criteria met"
	case pass == 0:
		return "no acceptance criteria met"
	default:
		return fmt.Sprintf("%d met, %d not met", pass, fail)
	}
}