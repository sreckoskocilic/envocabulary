// spawns real shells

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

func TracedStartup() ([]model.TraceEntry, error) {
	t, err := TracerForShell("")
	if err != nil {
		return nil, err
	}
	return TracedStartupWith(t)
}
