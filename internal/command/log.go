package command

import (
	"fmt"
	"os"
)

func logStep(verbose bool, format string, args ...any) {
	if !verbose {
		return
	}
	fmt.Fprintf(os.Stderr, "[xynapse] "+format+"\n", args...)
}
