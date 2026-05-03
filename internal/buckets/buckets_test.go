package buckets

import (
	"errors"
	"runtime"
	"testing"

	"github.com/sreckoskocilic/envocabulary/internal/model"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name       string
		varName    string
		value      string
		wantOrigin model.Origin
		wantSource string
	}{
		{"iTerm prefix", "ITERM_PROFILE", "Default", model.OriginTerminal, "iTerm"},
		{"iTerm2 prefix", "ITERM2_FOO", "x", model.OriginTerminal, "iTerm"},
		{"TERM_PROGRAM", "TERM_PROGRAM", "Apple_Terminal", model.OriginTerminal, ""},
		{"COLORTERM", "COLORTERM", "truecolor", model.OriginTerminal, ""},
		{"__CFBundleIdentifier", "__CFBundleIdentifier", "com.apple.Terminal", model.OriginTerminal, ""},
		{"XPC_ prefix", "XPC_FLAGS", "0x0", model.OriginSystem, "launchd xpc"},
		{"CLAUDE_CODE_ prefix", "CLAUDE_CODE_ENTRYPOINT", "cli", model.OriginSystem, "parent-injected (Claude Code)"},
		{"CLAUDECODE bare", "CLAUDECODE", "1", model.OriginSystem, "parent-injected (Claude Code)"},
		{"SSH_AUTH_SOCK", "SSH_AUTH_SOCK", "/tmp/ssh", model.OriginSSH, ""},
		{"SSH_TTY", "SSH_TTY", "/dev/ttys000", model.OriginSSH, ""},
		{"SSH_CONNECTION", "SSH_CONNECTION", "1.2.3.4 22 5.6.7.8 22", model.OriginSSH, ""},
		{"USER", "USER", "alice", model.OriginSystem, ""},
		{"HOME", "HOME", "/Users/alice", model.OriginSystem, ""},
		{"PWD", "PWD", "/tmp", model.OriginSystem, ""},
		{"LANG", "LANG", "en_US.UTF-8", model.OriginSystem, "locale"},
		{"LC_ALL", "LC_ALL", "en_US.UTF-8", model.OriginSystem, "locale"},
		{"LC_CTYPE", "LC_CTYPE", "UTF-8", model.OriginSystem, "locale"},
		{"SHLVL", "SHLVL", "1", model.OriginSystem, "shell-managed"},
		{"PAGER", "PAGER", "less", model.OriginSystem, "shell-managed"},
		{"ZSH_VERSION", "ZSH_VERSION", "5.9", model.OriginSystem, "shell-managed"},
		{"BASH", "BASH", "/bin/bash", model.OriginSystem, "shell-managed"},
		{"unknown random", "ZZ_NOT_A_REAL_VAR_FOR_TESTS", "x", model.OriginUnknown, ""},
		{"COMMAND_MODE", "COMMAND_MODE", "unix2003", model.OriginSystem, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotOrigin, gotSource := Classify(tc.varName, tc.value)
			if gotOrigin != tc.wantOrigin {
				t.Errorf("origin: got %q, want %q", gotOrigin, tc.wantOrigin)
			}
			if gotSource != tc.wantSource {
				t.Errorf("source: got %q, want %q", gotSource, tc.wantSource)
			}
		})
	}
}

func TestClassify_LaunchdMatch(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("launchctl only on macOS")
	}
	orig := launchctlGetenv
	t.Cleanup(func() { launchctlGetenv = orig })
	launchctlGetenv = func(name string) (string, error) {
		if name == "MY_LAUNCHD_VAR" {
			return "expected-value", nil
		}
		return "", errors.New("not found")
	}

	origin, source := Classify("MY_LAUNCHD_VAR", "expected-value")
	if origin != model.OriginLaunchd {
		t.Errorf("origin: got %q, want %q", origin, model.OriginLaunchd)
	}
	if source != "launchctl setenv" {
		t.Errorf("source: got %q, want %q", source, "launchctl setenv")
	}
}

func TestClassify_LaunchdMismatch(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("launchctl only on macOS")
	}
	orig := launchctlGetenv
	t.Cleanup(func() { launchctlGetenv = orig })
	launchctlGetenv = func(name string) (string, error) {
		return "different-value", nil
	}

	origin, _ := Classify("MY_LAUNCHD_VAR", "expected-value")
	if origin != model.OriginUnknown {
		t.Errorf("origin: got %q, want %q", origin, model.OriginUnknown)
	}
}

func TestClassify_LaunchdError(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("launchctl only on macOS")
	}
	orig := launchctlGetenv
	t.Cleanup(func() { launchctlGetenv = orig })
	launchctlGetenv = func(name string) (string, error) {
		return "", errors.New("timeout")
	}

	origin, _ := Classify("MY_LAUNCHD_VAR", "val")
	if origin != model.OriginUnknown {
		t.Errorf("origin: got %q, want %q", origin, model.OriginUnknown)
	}
}
