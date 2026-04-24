package buckets

import (
	"os/exec"
	"strings"

	"envocabulary/internal/model"
)

var terminalVars = map[string]bool{
	"TERM":                    true,
	"TERM_PROGRAM":            true,
	"TERM_PROGRAM_VERSION":    true,
	"TERM_SESSION_ID":         true,
	"COLORTERM":               true,
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

func Classify(name, value string) (model.Origin, string) {
	if strings.HasPrefix(name, "ITERM_") || strings.HasPrefix(name, "ITERM2_") {
		return model.OriginTerminal, "iTerm"
	}
	if strings.HasPrefix(name, "XPC_") {
		return model.OriginSystem, "launchd xpc"
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
	if out, err := exec.Command("launchctl", "getenv", name).Output(); err == nil {
		if strings.TrimRight(string(out), "\n") == value {
			return model.OriginLaunchd, "launchctl setenv"
		}
	}
	return model.OriginUnknown, ""
}
