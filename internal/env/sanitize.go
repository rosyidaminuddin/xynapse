// Package env provides helpers for building a process environment for
// child processes that should not inherit secrets.
package env

import (
	"os"
	"strings"
)

// secretSuffixes are upper-cased variable name suffixes that mark a value as
// sensitive and therefore excluded from child-process environments (e.g.
// JIRA_API_TOKEN, GITHUB_TOKEN, DB_PASSWORD).
var secretSuffixes = []string{
	"TOKEN",
	"SECRET",
	"PASSWORD",
	"PRIVATE_KEY",
	"API_KEY",
}

// JiraTokenKeys are the literal names that must never reach a child
// process even when they do not match a suffix rule.
var JiraTokenKeys = []string{"JIRA_API_TOKEN"}

// isSensitive reports whether an environment variable name should be
// excluded from sanitized environments.
func isSensitive(name string) bool {
	upper := strings.ToUpper(name)
	for _, lit := range JiraTokenKeys {
		if upper == lit {
			return true
		}
	}
	for _, suffix := range secretSuffixes {
		if strings.HasSuffix(upper, suffix) {
			return true
		}
	}
	return false
}

// SanitizedEnv returns the parent environment with secret-like variables
// removed. It is intended for child processes that must not see credentials
// (opencode agent, workflow test/lint shell commands).
func SanitizedEnv() []string {
	env := os.Environ()
	out := make([]string, 0, len(env))
	for _, entry := range env {
		name, _, ok := strings.Cut(entry, "=")
		if !ok || isSensitive(name) {
			continue
		}
		out = append(out, entry)
	}
	return out
}