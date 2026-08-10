package command

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
)

// Confirmation is a question in a plan's `## Confirmations` section that the
// user must answer before the plan is implemented.
type Confirmation struct {
	Number   int
	Question string
	Default  string
}

// Decision is a recorded answer for a numbered confirmation, stored in the
// plan's `## Decisions` section.
type Decision struct {
	Number int
	Answer string
}

var (
	confirmationRe = regexp.MustCompile(`^(\d+)\.\s+(.+?)(?:\s+\(default:\s*(.*)\))?$`)
	decisionRe     = regexp.MustCompile(`^-\s*(\d+)\.\s*(.*)$`)
)

// parseConfirmations extracts the numbered questions from a plan's
// `## Confirmations` section. A section that only says "None." yields no
// confirmations.
func parseConfirmations(body string) []Confirmation {
	lines := sectionLines(body, "Confirmations")
	var out []Confirmation
	for _, ln := range lines {
		m := confirmationRe.FindStringSubmatch(strings.TrimSpace(ln))
		if m == nil {
			continue
		}
		var n int
		if _, err := fmt.Sscanf(m[1], "%d", &n); err != nil {
			continue
		}
		out = append(out, Confirmation{
			Number:   n,
			Question: strings.TrimSpace(m[2]),
			Default:  strings.TrimSpace(m[3]),
		})
	}
	return out
}

// parseDecisions extracts recorded answers from a plan's `## Decisions`
// section, keyed by confirmation number.
func parseDecisions(body string) map[int]string {
	decisions := map[int]string{}
	for _, ln := range sectionLines(body, "Decisions") {
		m := decisionRe.FindStringSubmatch(strings.TrimSpace(ln))
		if m == nil {
			continue
		}
		var n int
		if _, err := fmt.Sscanf(m[1], "%d", &n); err != nil {
			continue
		}
		decisions[n] = strings.TrimSpace(m[2])
	}
	return decisions
}

// unanswered returns the confirmations that have no recorded decision.
func unanswered(confirmations []Confirmation, decisions map[int]string) []Confirmation {
	var out []Confirmation
	for _, c := range confirmations {
		if _, ok := decisions[c.Number]; !ok {
			out = append(out, c)
		}
	}
	return out
}

// promptForAnswers asks the user for a decision on each confirmation, reading
// lines from r and printing prompts to w. An empty input accepts the suggested
// default (or records an empty answer when there is none). It returns an error
// if input ends before all questions are answered.
func promptForAnswers(confirmations []Confirmation, r io.Reader, w io.Writer) ([]Decision, error) {
	scanner := bufio.NewScanner(r)
	var decisions []Decision
	for _, c := range confirmations {
		if c.Default != "" {
			fmt.Fprintf(w, "  %d. %s [%s]: ", c.Number, c.Question, c.Default)
		} else {
			fmt.Fprintf(w, "  %d. %s: ", c.Number, c.Question)
		}
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return nil, fmt.Errorf("cancelled: %w", err)
			}
			return nil, fmt.Errorf("cancelled: no answer provided")
		}
		answer := strings.TrimSpace(scanner.Text())
		if answer == "" {
			answer = c.Default
		}
		decisions = append(decisions, Decision{Number: c.Number, Answer: answer})
	}
	return decisions, nil
}

// writeDecisions appends a `## Decisions` section to the plan body, replacing
// any existing section. Existing decisions are preserved; new ones are merged
// in by number.
func writeDecisions(body string, decisions []Decision) string {
	existing := parseDecisions(body)
	for _, d := range decisions {
		existing[d.Number] = d.Answer
	}

	var nums []int
	for n := range existing {
		nums = append(nums, n)
	}
	sort.Ints(nums)

	var b strings.Builder
	b.WriteString("## Decisions\n")
	if len(nums) == 0 {
		b.WriteString("\nNone.\n")
	} else {
		b.WriteString("\n")
		for _, n := range nums {
			fmt.Fprintf(&b, "- %d. %s\n", n, existing[n])
		}
	}
	return replaceSection(body, "Decisions", b.String())
}

// sectionLines returns the non-heading lines under a `## <heading>` section.
func sectionLines(body, heading string) []string {
	start, end, ok := sectionRange(body, heading)
	if !ok {
		return nil
	}
	lines := strings.Split(body, "\n")
	return lines[start+1 : end]
}

// replaceSection swaps the `## <heading>` section in body for newSection,
// appending it when the section does not exist.
func replaceSection(body, heading, newSection string) string {
	start, end, ok := sectionRange(body, heading)
	if ok {
		lines := strings.Split(body, "\n")
		replaced := append(append([]string{}, lines[:start]...), newSection)
		replaced = append(replaced, lines[end:]...)
		return strings.Join(replaced, "\n")
	}
	return strings.TrimRight(body, "\n") + "\n\n" + newSection + "\n"
}

// sectionRange locates the line range [start, end) of a `## <heading>` section
// in body. end is the line of the next `## ` heading or the last line.
func sectionRange(body, heading string) (start, end int, ok bool) {
	prefix := "## " + heading
	lines := strings.Split(body, "\n")
	start = -1
	for i, ln := range lines {
		if strings.TrimSpace(ln) == prefix {
			start = i
			break
		}
	}
	if start < 0 {
		return 0, 0, false
	}
	end = len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
			end = i
			break
		}
	}
	return start, end, true
}
