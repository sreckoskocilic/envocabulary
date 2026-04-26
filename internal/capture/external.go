// Package boundary: this file contains the only code in `capture` that spawns
// external subprocesses (`env -0`, `zsh -l -i -x -c exit`). Exercising it requires
// a real shell environment, not unit-testable input — so it is excluded from
// coverage reporting via .codecov.yml.
//
// The parsing layer that consumes its output (`parseTrace`, `parseNullSeparated`,
// `envWithPS4`, `TracedStartupWith`) lives in capture.go and IS fully tested
// via injected fake tracers.
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
