package command

import (
	"fmt"
	"net/url"
	"strings"
)

// ParseTicketRef resolves a user-supplied ticket reference into a project and
// ticket number. It accepts any of:
//
//	123                -> uses the given default project
//	MERADIO-123        -> project "MERADIO", number "123"
//	https://.../browse/MERADIO-123 -> extracts the key from the URL
func ParseTicketRef(ref, defaultProject string) (project, number string, err error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", "", fmt.Errorf("empty ticket reference")
	}

	if strings.Contains(ref, "://") {
		u, uerr := url.Parse(ref)
		if uerr != nil {
			return "", "", fmt.Errorf("invalid ticket URL %q: %w", ref, uerr)
		}
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		ref = parts[len(parts)-1]
	}

	ref = strings.ToUpper(strings.TrimSpace(ref))
	idx := strings.LastIndex(ref, "-")
	if idx > 0 {
		project = ref[:idx]
		number = ref[idx+1:]
	} else {
		project = defaultProject
		number = ref
	}

	if project == "" || number == "" {
		return "", "", fmt.Errorf("invalid ticket reference %q", ref)
	}
	for _, r := range number {
		if r < '0' || r > '9' {
			return "", "", fmt.Errorf("invalid ticket reference %q: number must be digits", ref)
		}
	}
	return project, number, nil
}
