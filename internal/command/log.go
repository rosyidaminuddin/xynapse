package command

import (
	"fmt"
	"log/slog"
)

// logStep logs at Debug level so it only appears with -v/--verbose.
func logStep(verbose bool, format string, args ...any) {
	if !verbose {
		return
	}
	slog.Debug("[xynapse] " + fmt.Sprintf(format, args...))
}
