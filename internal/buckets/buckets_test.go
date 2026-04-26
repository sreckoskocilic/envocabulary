package buckets

import (
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
		// terminal
		{"iTerm prefix", "ITERM_PROFILE", "Default", model.OriginTerminal, "iTerm"},
		{"iTerm2 prefix", "ITERM2_FOO", "x", model.OriginTerminal, "iTerm"},
		{"TERM_PROGRAM", "TERM_PROGRAM", "Apple_Terminal", model.OriginTerminal, ""},
		{"COLORTERM", "COLORTERM", "truecolor", model.OriginTerminal, ""},
		{"__CFBundleIdentifier", "__CFBundleIdentifier", "com.apple.Terminal", model.OriginTerminal, ""},

		// system / launchd / parent-process
		{"XPC_ prefix", "XPC_FLAGS", "0x0", model.OriginSystem, "launchd xpc"},
		{"CLAUDE_CODE_ prefix", "CLAUDE_CODE_ENTRYPOINT", "cli", model.OriginSystem, "parent-injected (Claude Code)"},
		{"CLAUDECODE bare", "CLAUDECODE", "1", model.OriginSystem, "parent-injected (Claude Code)"},

		// ssh
		{"SSH_AUTH_SOCK", "SSH_AUTH_SOCK", "/tmp/ssh", model.OriginSSH, ""},
		{"SSH_TTY", "SSH_TTY", "/dev/ttys000", model.OriginSSH, ""},
		{"SSH_CONNECTION", "SSH_CONNECTION", "1.2.3.4 22 5.6.7.8 22", model.OriginSSH, ""},

		// system basics
		{"USER", "USER", "alice", model.OriginSystem, ""},
		{"HOME", "HOME", "/Users/alice", model.OriginSystem, ""},
		{"PWD", "PWD", "/tmp", model.OriginSystem, ""},

		// locale
		{"LANG", "LANG", "en_US.UTF-8", model.OriginSystem, "locale"},
		{"LC_ALL", "LC_ALL", "en_US.UTF-8", model.OriginSystem, "locale"},
		{"LC_CTYPE", "LC_CTYPE", "UTF-8", model.OriginSystem, "locale"},

		// shell-managed
		{"SHLVL", "SHLVL", "1", model.OriginSystem, "shell-managed"},
		{"PAGER", "PAGER", "less", model.OriginSystem, "shell-managed"},
		{"ZSH_VERSION", "ZSH_VERSION", "5.9", model.OriginSystem, "shell-managed"},
		{"BASH", "BASH", "/bin/bash", model.OriginSystem, "shell-managed"},

		// unknown — exact arbitrary name (avoid system PATH that launchctl might know)
		{"unknown random", "ZZ_NOT_A_REAL_VAR_FOR_TESTS", "x", model.OriginUnknown, ""},
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
