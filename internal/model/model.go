package model

import "strings"

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
	File string `json:"file"`
	Line int    `json:"line"`
	Name string `json:"name"`
	Raw  string `json:"raw,omitempty"`
}

func IsDeferredListVar(name string) bool {
	switch name {
	case "PATH", "MANPATH", "INFOPATH", "FPATH", "CDPATH":
		return true
	}
	return strings.HasPrefix(name, "DYLD_")
}

func IsDirenvVar(name string) bool {
	switch name {
	case "DIRENV_DIR", "DIRENV_FILE", "DIRENV_DIFF", "DIRENV_WATCHES":
		return true
	}
	return false
}
