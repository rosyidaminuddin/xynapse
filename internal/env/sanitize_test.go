package env

import (
	"strings"
	"testing"
)

func TestSanitizedEnvScrubsSecrets(t *testing.T) {
	t.Setenv("JIRA_API_TOKEN", "secret-token")
	t.Setenv("JIRA_EMAIL", "me@corp.com")
	t.Setenv("GITHUB_TOKEN", "gh-token")
	t.Setenv("DB_PASSWORD", "pw123")
	t.Setenv("MY_API_KEY", "ak-1")
	t.Setenv("PATH", "/usr/bin:/bin")
	t.Setenv("LANG", "en_US.UTF-8")

	got := SanitizedEnv()
	found := map[string]bool{}
	for _, entry := range got {
		name, _, _ := strings.Cut(entry, "=")
		found[name] = true
	}

	for _, keep := range []string{"JIRA_EMAIL", "PATH", "LANG"} {
		if !found[keep] {
			t.Errorf("expected %s to be kept in sanitized env", keep)
		}
	}
	for _, scrub := range []string{"JIRA_API_TOKEN", "GITHUB_TOKEN", "DB_PASSWORD", "MY_API_KEY"} {
		if found[scrub] {
			t.Errorf("expected %s to be scrubbed from sanitized env", scrub)
		}
	}
}

func TestSanitizedEnvKeepsNonSecretSibling(t *testing.T) {
	t.Setenv("JIRA_URL", "https://corp.atlassian.net")
	t.Setenv("SOMETHING_TOKENX", "not-a-token") // suffix rule is exact trailing
	got := SanitizedEnv()
	for _, entry := range got {
		name, _, _ := strings.Cut(entry, "=")
		if name == "JIRA_URL" {
			return
		}
	}
	t.Error("expected JIRA_URL to be kept")
}