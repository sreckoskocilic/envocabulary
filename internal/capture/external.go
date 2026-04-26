// Package boundary: this file contains the only code in `capture` that spawns
// external subprocesses (`env -0`, `zsh -l -i -x -c exit`, `bash -l -i -x -c exit`).
// Exercising it requires a real shell environment, not unit-testable input —
// so it is excluded from coverage reporting via .codecov.yml.
//
// The parsing layer that consumes its output (`parseTrace`, `parseNullSeparated`,
// `envWithPS4`, `TracedStartupWith`, `DetectShell`, `TracerForShell`) lives in
// capture.go and IS fully tested via injected fake tracers.
//
// Convention: any new code that spawns subprocesses or calls non-deterministic
// syscalls belongs here, not in capture.go.

package capture

import (
	"bytes"
	"fmt"
	"os/exec"

	"github.com/sreckoskocilic/envocabulary/internal/model"
)

func CurrentEnv() (map[string]string, error) {
	out, err := exec.Command("env", "-0").Output()
	if err != nil {
		return nil, fmt.Errorf("env -0: %w", err)
	}
	return parseNullSeparated(out), nil
}

// ZshTracer spawns `zsh -l -i -x -c exit` with PS4="+%x:%i> " (zsh prompt-expansion
// for source file and line number) and captures stderr (which contains the xtrace).
type ZshTracer struct{}

func (ZshTracer) RawTrace() (string, error) {
	cmd := exec.Command("zsh", "-l", "-i", "-x", "-c", "exit")
	cmd.Env = envWithPS4("+%x:%i> ")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := stderr.String()
	if err != nil && out == "" {
		return "", fmt.Errorf("zsh trace: %w", err)
	}
	return out, nil
}

// BashTracer spawns `bash -l -i -x -c exit` with PS4="+${BASH_SOURCE}:${LINENO}> "
// (bash variable interpolation expanded at xtrace time) and captures stderr.
// The resulting trace format is identical to zsh's, so parseTrace handles both.
type BashTracer struct{}

func (BashTracer) RawTrace() (string, error) {
	cmd := exec.Command("bash", "-l", "-i", "-x", "-c", "exit")
	cmd.Env = envWithPS4(`+${BASH_SOURCE}:${LINENO}> `)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := stderr.String()
	if err != nil && out == "" {
		return "", fmt.Errorf("bash trace: %w", err)
	}
	return out, nil
}

// TracedStartup runs the user's detected login shell and parses its xtrace output.
// Production callers use this. To force a specific shell, use TracerForShell + TracedStartupWith.
func TracedStartup() ([]model.TraceEntry, error) {
	t, err := TracerForShell("")
	if err != nil {
		return nil, err
	}
	return TracedStartupWith(t)
}
