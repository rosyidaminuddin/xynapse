package client

import (
	"strings"
	"testing"
)

func TestBuildSprintJQL(t *testing.T) {
	cases := []struct {
		name     string
		project  string
		sprintID int
		types    []string
		wantSub  []string
	}{
		{
			name:     "no sprint id",
			project:  "MERADIO",
			sprintID: 0,
			types:    nil,
			wantSub:  []string{`project = "MERADIO"`, "sprint in openSprints()", "assignee = currentUser()"},
		},
		{
			name:     "with sprint id",
			project:  "MERADIO",
			sprintID: 42,
			types:    nil,
			wantSub:  []string{`project = "MERADIO"`, "sprint = 42", "assignee = currentUser()"},
		},
		{
			name:     "single type",
			project:  "MERADIO",
			sprintID: 0,
			types:    []string{"Story"},
			wantSub:  []string{`issuetype in ("Story")`},
		},
		{
			name:     "multiple types",
			project:  "MERADIO",
			sprintID: 0,
			types:    []string{"Story", "Bug", "Epic"},
			wantSub:  []string{`issuetype in ("Story","Bug","Epic")`},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			jql := BuildSprintJQL(tc.project, tc.sprintID, tc.types)
			for _, sub := range tc.wantSub {
				if !strings.Contains(jql, sub) {
					t.Errorf("JQL %q does not contain %q", jql, sub)
				}
			}
		})
	}
}

func TestBuildSprintJQLTypeExcludesOpenSprints(t *testing.T) {
	// With a sprint id present, the openSprints() clause must not appear.
	jql := BuildSprintJQL("MERADIO", 42, []string{"Story"})
	if strings.Contains(jql, "openSprints()") {
		t.Errorf("JQL %q should not contain openSprints() when sprint id given", jql)
	}
}
