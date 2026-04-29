// Filesystem-dependent path helpers; excluded from gated coverage.
package report

import (
	"os"
	"strings"
)

func tildePath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}
