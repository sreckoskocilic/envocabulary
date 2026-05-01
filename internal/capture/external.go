// spawns real shells

package capture

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/sreckoskocilic/envocabulary/internal/model"
)

const traceTimeout = 30 * time.Second

func CurrentEnv() (map[string]string, error) {
	out, err := exec.Command("env", "-0").Output()
	if err != nil {
		return nil, fmt.Errorf("env -0: %w", err)
	}
	return parseNullSeparated(out), nil
}

type ZshTracer struct{}

func (ZshTracer) RawTrace() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), traceTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "zsh", "-l", "-i", "-x", "-c", "exit")
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

type BashTracer struct{}

func (BashTracer) RawTrace() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), traceTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "-l", "-i", "-x", "-c", "exit")
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

func TracedStartup() ([]model.TraceEntry, error) {
	t, err := TracerForShell("")
	if err != nil {
		return nil, err
	}
	return TracedStartupWith(t)
}
