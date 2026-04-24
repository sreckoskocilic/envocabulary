package capture

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"envocabulary/internal/model"
)

func CurrentEnv() (map[string]string, error) {
	out, err := exec.Command("env", "-0").Output()
	if err != nil {
		return nil, fmt.Errorf("env -0: %w", err)
	}
	return parseNullSeparated(out), nil
}

func TracedStartup() (map[string]model.TraceEntry, error) {
	cmd := exec.Command("zsh", "-l", "-i", "-x", "-c", "exit")
	cmd.Env = envWithPS4()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	trace := parseTrace(stderr.String())
	if err != nil && len(trace) == 0 {
		return nil, fmt.Errorf("zsh trace: %w", err)
	}
	return trace, nil
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

func parseTrace(s string) map[string]model.TraceEntry {
	last := make(map[string]model.TraceEntry)
	for _, line := range strings.Split(s, "\n") {
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
		name := am[1]
		last[name] = model.TraceEntry{File: file, Line: ln, Name: name, Raw: rest}
	}
	return last
}
