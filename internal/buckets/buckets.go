package buckets

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/sreckoskocilic/envocabulary/internal/model"
)

const launchctlTimeout = 3 * time.Second

var terminalVars = map[string]bool{
	"TERM":                    true,
	"TERM_PROGRAM":            true,
	"TERM_PROGRAM_VERSION":    true,
	"TERM_SESSION_ID":         true,
	"COLORTERM":               true,
	"COLORFGBG":               true,
	"TERMINFO_DIRS":           true,
	"LC_TERMINAL":             true,
	"LC_TERMINAL_VERSION":     true,
	"__CFBundleIdentifier":    true,
	"__CF_USER_TEXT_ENCODING": true,
}

var sshVars = map[string]bool{
	"SSH_AUTH_SOCK":  true,
	"SSH_TTY":        true,
	"SSH_CONNECTION": true,
	"SSH_CLIENT":     true,
}

var systemVars = map[string]bool{
	"USER":         true,
	"LOGNAME":      true,
	"HOME":         true,
	"SHELL":        true,
	"TMPDIR":       true,
	"PWD":          true,
	"OLDPWD":       true,
	"COMMAND_MODE": true,
	"_":            true,
}

var shellManagedVars = map[string]bool{
	"SHLVL":         true,
	"PAGER":         true,
	"LESS":          true,
	"LESSCHARSET":   true,
	"LINES":         true,
	"COLUMNS":       true,
	"OPTIND":        true,
	"PPID":          true,
	"HOSTNAME":      true,
	"HOSTTYPE":      true,
	"MACHTYPE":      true,
	"OSTYPE":        true,
	"CPUTYPE":       true,
	"VENDOR":        true,
	"EUID":          true,
	"UID":           true,
	"GROUPS":        true,
	"HISTFILE":      true,
	"HISTSIZE":      true,
	"HISTFILESIZE":  true,
	"HISTCONTROL":   true,
	"SAVEHIST":      true,
	"TERM_FEATURES": true,
	"ZSH":           true,
	"ZSH_NAME":      true,
	"ZSH_VERSION":   true,
	"BASH":          true,
	"BASH_VERSION":  true,
}

func Classify(name, value string) (origin model.Origin, source string) {
	if strings.HasPrefix(name, "ITERM_") || strings.HasPrefix(name, "ITERM2_") {
		return model.OriginTerminal, "iTerm"
	}
	if strings.HasPrefix(name, "XPC_") {
		return model.OriginSystem, "launchd xpc"
	}
	if strings.HasPrefix(name, "CLAUDE_CODE_") || name == "CLAUDECODE" {
		return model.OriginSystem, "parent-injected (Claude Code)"
	}
	if terminalVars[name] {
		return model.OriginTerminal, ""
	}
	if sshVars[name] {
		return model.OriginSSH, ""
	}
	if systemVars[name] {
		return model.OriginSystem, ""
	}
	if strings.HasPrefix(name, "LC_") || name == "LANG" {
		return model.OriginSystem, "locale"
	}
	if shellManagedVars[name] {
		return model.OriginSystem, "shell-managed"
	}
	ctx, cancel := context.WithTimeout(context.Background(), launchctlTimeout)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "launchctl", "getenv", name).Output(); err == nil {
		if strings.TrimRight(string(out), "\n") == value {
			return model.OriginLaunchd, "launchctl setenv"
		}
	}
	return model.OriginUnknown, ""
}
