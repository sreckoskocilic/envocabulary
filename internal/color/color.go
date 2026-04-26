package color

import (
	"fmt"
	"io"
	"os"
)

const (
	Reset    = "\x1b[0m"
	Red      = "\x1b[31m"
	LightRed = "\x1b[91m"
	Dim      = "\x1b[2m"
)

type Mode int

const (
	Auto Mode = iota
	Always
	Never
)

func ParseMode(s string) (Mode, error) {
	switch s {
	case "", "auto":
		return Auto, nil
	case "always", "yes", "true":
		return Always, nil
	case "never", "no", "false", "off":
		return Never, nil
	default:
		return Auto, fmt.Errorf("invalid color mode %q (want auto|always|never)", s)
	}
}

// Enabled reports whether ANSI color codes should be emitted for the given writer.
// Honors NO_COLOR (https://no-color.org) and only enables for terminals in Auto mode.
func (m Mode) Enabled(w io.Writer) bool {
	switch m {
	case Always:
		return true
	case Never:
		return false
	case Auto:
		if os.Getenv("NO_COLOR") != "" {
			return false
		}
		return isTerminal(w)
	}
	return false
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// Wrap returns s wrapped in the given ANSI code + Reset, when on is true.
// When on is false, returns s unchanged.
func Wrap(s, code string, on bool) string {
	if !on {
		return s
	}
	return code + s + Reset
}
