package capture

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/sreckoskocilic/envocabulary/internal/model"
)

type Tracer interface {
	RawTrace() (string, error)
}

func TracedStartupWith(t Tracer) ([]model.TraceEntry, error) {
	raw, err := t.RawTrace()
	if err != nil {
		return nil, err
	}
	return parseTrace(raw), nil
}

func DetectShell() string {
	base := filepath.Base(os.Getenv("SHELL"))
	switch base {
	case "bash":
		return "bash"
	case "zsh", "":
		return "zsh"
	}
	return "zsh"
}

func TracerForShell(name string) (Tracer, error) {
	if name == "" {
		name = DetectShell()
	}
	switch name {
	case "zsh":
		return ZshTracer{}, nil
	case "bash":
		return BashTracer{}, nil
	}
	return nil, fmt.Errorf("unsupported shell %q (want zsh or bash)", name)
}

func envWithPS4(ps4 string) []string {
	e := os.Environ()
	out := make([]string, 0, len(e)+1)
	for _, kv := range e {
		if strings.HasPrefix(kv, "PS4=") {
			continue
		}
		out = append(out, kv)
	}
	out = append(out, "PS4="+ps4)
	return out
}

func parseNullSeparated(b []byte) map[string]string {
	m := make(map[string]string)
	for _, entry := range bytes.Split(b, []byte{0}) {
		if len(entry) == 0 {
			continue
		}
		i := bytes.IndexByte(entry, '=')
		if i < 0 {
			continue
		}
		m[string(entry[:i])] = string(entry[i+1:])
	}
	return m
}

var (
	traceLineRe = regexp.MustCompile(`^\++(.+?):(\d+)> (.*)$`)
	assignRe    = regexp.MustCompile(`(?:^|\s)(?:export\s+|typeset(?:\s+-[a-zA-Z]+)*\s+|declare(?:\s+-[a-zA-Z]+)*\s+|local(?:\s+-[a-zA-Z]+)*\s+)?([A-Za-z_][A-Za-z0-9_]*)=`)
)

func parseTrace(s string) []model.TraceEntry {
	lines := strings.Split(s, "\n")
	var entries []model.TraceEntry //nolint:prealloc // most lines are non-trace noise
	for _, line := range lines {
		m := traceLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		file, lineStr, rest := m[1], m[2], m[3]
		ln, _ := strconv.Atoi(lineStr)
		am := assignRe.FindStringSubmatch(rest)
		if am == nil {
			continue
		}
		entries = append(entries, model.TraceEntry{File: file, Line: ln, Name: am[1], Raw: rest})
	}
	return entries
}
