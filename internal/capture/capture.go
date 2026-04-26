package capture

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/sreckoskocilic/envocabulary/internal/model"
)

func CurrentEnv() (map[string]string, error) {
	out, err := exec.Command("env", "-0").Output()
	if err != nil {
		return nil, fmt.Errorf("env -0: %w", err)
	}
	return parseNullSeparated(out), nil
}

// Tracer abstracts the source of raw zsh xtrace output so that the trace-parsing
// pipeline can be tested with synthetic input instead of spawning a real zsh process.
type Tracer interface {
	RawTrace() (string, error)
}

// ZshTracer is the production Tracer that spawns `zsh -l -i -x -c exit` and
// captures its stderr (which contains the xtrace output).
type ZshTracer struct{}

func (ZshTracer) RawTrace() (string, error) {
	cmd := exec.Command("zsh", "-l", "-i", "-x", "-c", "exit")
	cmd.Env = envWithPS4()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := stderr.String()
	if err != nil && out == "" {
		return "", fmt.Errorf("zsh trace: %w", err)
	}
	return out, nil
}

// TracedStartup runs zsh and parses its xtrace output. Production callers use this.
func TracedStartup() ([]model.TraceEntry, error) {
	return TracedStartupWith(ZshTracer{})
}

// TracedStartupWith runs the given Tracer and parses its raw output.
// Tests inject a fake Tracer to drive the parser without spawning any subprocess.
func TracedStartupWith(t Tracer) ([]model.TraceEntry, error) {
	raw, err := t.RawTrace()
	if err != nil {
		return nil, err
	}
	return parseTrace(raw), nil
}

func envWithPS4() []string {
	e := os.Environ()
	out := make([]string, 0, len(e)+1)
	for _, kv := range e {
		if strings.HasPrefix(kv, "PS4=") {
			continue
		}
		out = append(out, kv)
	}
	out = append(out, `PS4=+%x:%i> `)
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
	var entries []model.TraceEntry //nolint:prealloc // most lines are non-trace noise; allocating len(lines) wastes memory
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
