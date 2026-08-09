package git

import "testing"

func TestExpandTemplate(t *testing.T) {
	vars := TemplateVars{
		Key:     "PROJ-123",
		Project: "PROJ",
		Number:  "123",
		Board:   "42",
		Summary: "Add stale detection",
	}

	cases := []struct {
		name    string
		tmpl    string
		want    string
		wantErr bool
	}{
		{"default key", "feature-v5/{Key}", "feature-v5/PROJ-123", false},
		{"ticket key alias", "{Project}/{TicketKey}", "PROJ/PROJ-123", false},
		{"project", "hotfix/{Project}", "hotfix/PROJ", false},
		{"number", "{Number}-work", "123-work", false},
		{"board", "board-{Board}/{Key}", "board-42/PROJ-123", false},
		{"summary slugged", "feat/{Summary}", "feat/add-stale-detection", false},
		{"mixed", "{Project}/{Number}/{Summary}", "PROJ/123/add-stale-detection", false},
		{"no placeholders", "release-branch", "release-branch", false},
		{"unknown placeholder", "{Project}/{Nope}", "", true},
		{"empty template", "", "", true},
		{"whitespace collapsed", "feature v5 {Key}", "feature-v5-PROJ-123", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ExpandTemplate(tc.tmpl, vars)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ExpandTemplate(%q) err = %v, wantErr=%v", tc.tmpl, err, tc.wantErr)
			}
			if err == nil && got != tc.want {
				t.Errorf("ExpandTemplate(%q) = %q, want %q", tc.tmpl, got, tc.want)
			}
		})
	}
}

func TestExpandTemplateEmptyRequiredKey(t *testing.T) {
	vars := TemplateVars{Project: "PROJ", Number: "123"}
	if _, err := ExpandTemplate("feature-v5/{Key}", vars); err == nil {
		t.Error("expected error when {Key} placeholder value is empty")
	}
}

func TestSlugify(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Add stale detection", "add-stale-detection"},
		{"Merge requests!", "merge-requests"},
		{"  trim  spaces  ", "trim-spaces"},
		{"ALLCAPS", "allcaps"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := Slugify(tc.in); got != tc.want {
			t.Errorf("Slugify(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
