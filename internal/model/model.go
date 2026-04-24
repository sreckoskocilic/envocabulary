package model

type Origin string

const (
	OriginLaunchd   Origin = "launchd"
	OriginTerminal  Origin = "terminal"
	OriginSSH       Origin = "ssh"
	OriginSystem    Origin = "system"
	OriginShellFile Origin = "shell-file"
	OriginDirenv    Origin = "direnv"
	OriginDeferred  Origin = "deferred-list-var"
	OriginUnknown   Origin = "unknown"
)

type EnWord struct {
	Name   string
	Value  string
	Origin Origin
	Source string
	Notes  string
}

type TraceEntry struct {
	File string
	Line int
	Name string
	Raw  string
}

func IsDeferredListVar(name string) bool {
	switch name {
	case "PATH", "MANPATH", "INFOPATH", "FPATH", "CDPATH",
		"DYLD_LIBRARY_PATH", "DYLD_FALLBACK_LIBRARY_PATH", "DYLD_FRAMEWORK_PATH":
		return true
	}
	return false
}

func IsDirenvVar(name string) bool {
	switch name {
	case "DIRENV_DIR", "DIRENV_FILE", "DIRENV_DIFF", "DIRENV_WATCHES":
		return true
	}
	return false
}
