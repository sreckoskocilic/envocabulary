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
