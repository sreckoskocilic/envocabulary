package capture

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sreckoskocilic/envocabulary/internal/model"
)

const (
	traceTimeout  = 30 * time.Second
	waitDelay     = 5 * time.Second
	maxTraceBytes = 100 * 1024 * 1024
)

var errTraceTooLarge = errors.New("trace output exceeded 100 MB; shell startup may contain a loop")

type boundedWriter struct {
	buf bytes.Buffer
	max int
}

func (w *boundedWriter) Write(p []byte) (int, error) {
	if w.buf.Len()+len(p) > w.max {
		return 0, errTraceTooLarge
	}
	return w.buf.Write(p)
}

func (w *boundedWriter) String() string { return w.buf.String() }

var CurrentEnv = currentEnv

func currentEnv() (map[string]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), traceTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "env", "-0").Output()
	if err != nil {
		return nil, fmt.Errorf("env -0: %w", err)
	}
	return parseNullSeparated(out), nil
}

type ZshTracer struct{ BaselineLists bool }

func (t ZshTracer) RawTrace() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), traceTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "zsh", "-l", "-i", "-x", "-c", "exit")
	cmd.Env = buildEnv("+%x:%i> ", t.BaselineLists)
	stderr := &boundedWriter{max: maxTraceBytes}
	cmd.Stderr = stderr
	cmd.WaitDelay = waitDelay
	err := cmd.Run()
	out := stderr.String()
	if err != nil && out == "" {
		return "", fmt.Errorf("zsh trace: %w", err)
	}
	return out, nil
}

type BashTracer struct{ BaselineLists bool }

func (t BashTracer) RawTrace() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), traceTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "-l", "-i", "-x", "-c", "exit")
	cmd.Env = buildEnv(`+${BASH_SOURCE}:${LINENO}> `, t.BaselineLists)
	stderr := &boundedWriter{max: maxTraceBytes}
	cmd.Stderr = stderr
	cmd.WaitDelay = waitDelay
	err := cmd.Run()
	out := stderr.String()
	if err != nil && out == "" {
		return "", fmt.Errorf("bash trace: %w", err)
	}
	return out, nil
}

func TracedStartup() ([]model.TraceEntry, error) {
	t, err := TracerForShell("")
	if err != nil {
		return nil, err
	}
	return TracedStartupWith(t)
}

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

func TracerForShell(name string) (Tracer, error) { return tracerFor(name, false) }

func TracerForShellBaseline(name string) (Tracer, error) { return tracerFor(name, true) }

func tracerFor(name string, baselineLists bool) (Tracer, error) {
	if name == "" {
		name = DetectShell()
	}
	switch name {
	case "zsh":
		return ZshTracer{BaselineLists: baselineLists}, nil
	case "bash":
		return BashTracer{BaselineLists: baselineLists}, nil
	}
	return nil, fmt.Errorf("unsupported shell %q (want zsh or bash)", name)
}

const BaselinePath = "/usr/bin:/bin:/usr/sbin:/sbin"

func BaselineListValue(name string) string {
	if name == "PATH" {
		return BaselinePath
	}
	return ""
}

func envWithPS4(ps4 string) []string { return buildEnv(ps4, false) }

func buildEnv(ps4 string, baselineLists bool) []string {
	e := os.Environ()
	out := make([]string, 0, len(e)+2)
	for _, kv := range e {
		if strings.HasPrefix(kv, "PS4=") {
			continue
		}
		if baselineLists {
			if name, _, ok := strings.Cut(kv, "="); ok && model.IsDeferredListVar(name) {
				continue
			}
		}
		out = append(out, kv)
	}
	if baselineLists {
		out = append(out, "PATH="+BaselinePath)
	}
	out = append(out, "PS4="+ps4)
	return out
}

func parseNullSeparated(b []byte) map[string]string {
	m := make(map[string]string)
	for entry := range bytes.SplitSeq(b, []byte{0}) {
		if len(entry) == 0 {
			continue
		}
		before, after, ok := bytes.Cut(entry, []byte{'='})
		if !ok {
			continue
		}
		m[string(before)] = string(after)
	}
	return m
}

var (
	traceLineRe = regexp.MustCompile(`^(\++)(.+?):(\d+)> (.*)$`)
	assignRe    = regexp.MustCompile(`(?:^|\s)(?:export\s+|typeset(?:\s+-[a-zA-Z]+)*\s+|declare(?:\s+-[a-zA-Z]+)*\s+|local(?:\s+-[a-zA-Z]+)*\s+)?([A-Za-z_][A-Za-z0-9_]*)=`)
	sourceRe    = regexp.MustCompile(`^(?:source|\.)\s+(\S+)`)
)

func sourceTargets(target, file string) bool {
	t := strings.Trim(target, `"'`)
	if t == "" {
		return false
	}
	return filepath.Base(t) == filepath.Base(file)
}

func parseTrace(s string) []model.TraceEntry {
	lines := strings.Split(s, "\n")
	var entries []model.TraceEntry //nolint:prealloc // most lines are non-trace noise
	var stack []string
	currentFile := ""
	pendingSource := ""

	for _, line := range lines {
		m := traceLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		file, lineStr, rest := m[2], m[3], m[4]
		ln, _ := strconv.Atoi(lineStr)

		if file != currentFile {
			if pendingSource != "" && !strings.HasPrefix(file, "(") && sourceTargets(pendingSource, file) {
				if idx := fileIndex(stack, file); idx >= 0 {
					stack = stack[:idx+1]
				} else {
					stack = append(stack, file)
				}
			} else if idx := fileIndex(stack, file); idx >= 0 {
				stack = stack[:idx+1]
			} else {
				stack = []string{file}
			}
			currentFile = file
		}

		if sm := sourceRe.FindStringSubmatch(rest); sm != nil {
			pendingSource = sm[1]
		} else {
			pendingSource = ""
		}

		am := assignRe.FindStringSubmatch(rest)
		if am == nil {
			continue
		}
		entry := model.TraceEntry{File: file, Line: ln, Name: am[1], Raw: rest}
		if len(stack) > 1 {
			entry.Chain = make([]string, len(stack)-1)
			copy(entry.Chain, stack[:len(stack)-1])
		}
		entries = append(entries, entry)
	}
	return entries
}

func fileIndex(stack []string, file string) int {
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i] == file {
			return i
		}
	}
	return -1
}
