# envocabulary

[![CI](https://github.com/sreckoskocilic/envocabulary/actions/workflows/ci.yml/badge.svg)](https://github.com/sreckoskocilic/envocabulary/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/sreckoskocilic/envocabulary/branch/main/graph/badge.svg)](https://codecov.io/gh/sreckoskocilic/envocabulary)
[![Go Report Card](https://goreportcard.com/badge/github.com/sreckoskocilic/envocabulary)](https://goreportcard.com/report/github.com/sreckoskocilic/envocabulary)
[![Release](https://img.shields.io/github/v/release/sreckoskocilic/envocabulary)](https://github.com/sreckoskocilic/envocabulary/releases/latest)

For every variable in your current shell, find the file and line that set it — or which subsystem (direnv, launchd, terminal, SSH, system) injected it. Plus a few static-file commands for the moment your shell config has sprawled across N files and backups and you've lost the plot.

I built this because I kept losing the same hour every few months tracing why some `JAVA_HOME` or `PATH` was pointing somewhere I didn't expect. `which` knows commands, `direnv status` knows direnv, `launchctl getenv` knows launchd — none of them tell you that `~/.zshrc:42` is the actual writer.

Works with zsh and bash on macOS, Linux, and FreeBSD.

## The "ah" moment

A `grep -r JAVA_HOME ~` will show every file that mentions the variable, but it's not clear from the output which assignment is active in the current shell:

    $ envocabulary explain --chain JAVA_HOME
    JAVA_HOME
        origin:  shell-file
        winner:  ~/helpers.sh:3
        chain:   ~/.zshrc → ~/helpers.sh
        also written at:  ~/.zshenv:8

## Install

One-liner (detects OS/arch, drops the binary on your `$PATH`, clears Gatekeeper on macOS):

```sh
curl -fsSL https://raw.githubusercontent.com/sreckoskocilic/envocabulary/main/install.sh | sh
```

Or `go install github.com/sreckoskocilic/envocabulary/cmd/envocabulary@latest`. Pre-built binaries and Linux packages (`.deb` / `.rpm` / `.apk` / `.pkg.tar.zst`) are on the [releases page](https://github.com/sreckoskocilic/envocabulary/releases/latest).

## Commands

Live env (introspects your running shell):

- `scan` *(default)* — prints all variables in the current env grouped by origin
- `explain NAME` — prints full attribution for provided variable
- `path [VARNAME...]` — per-entry attribution for colon-separated path variables; `--check` finds dead entries

Static files:

- `inventory` — lists all shell config files and assigned types variables count
- `catalog` — prints entire shell configuration by merging all its config files
- `dedup` — cross-file duplicate report for exports, assigns, aliases, functions
- `dangling` — lists config file entries that no longer reference a valid target
- `lost` — lists orphaned files (not sourced by any canonical config)
- `report` — combined audit: safe-to-delete, dedup, dangling, lost results
- `clean FILE` — prints safe-to-remove lines of provided file

`envocabulary <cmd> -h` for flags.

## Another example: finding broken references

`dangling` lists config file entries that no longer reference a valid target — the `JAVA_HOME=/opt/jdk-i-uninstalled` and `source ~/dotfiles/work-old.zsh` kind of leftovers:

    $ envocabulary dangling
    ## ~/.zshrc
      ~/.zshrc:14  source   → ~/dotfiles/work-old.zsh  (source target missing)
      ~/.zshrc:42  export JAVA_HOME  → /opt/jdk-11  (path does not exist)


## Another example: tracing PATH entries

`path` shows where each entry in `PATH` (or `MANPATH`, `FPATH`, etc.) was introduced:

    $ envocabulary path PATH
    ## PATH
      /opt/homebrew/bin       ~/.zprofile:21
      /usr/local/bin          ~/.zprofile:1
      /usr/bin                /etc/zprofile:1
      /bin                    /etc/zprofile:1
      /usr/sbin               /etc/zprofile:1
      /sbin                   /etc/zprofile:1
      ~/.cargo/bin            ~/.zshrc:22

Entries not matched to any shell startup assignment are shown as `inherited`.

`--check` filters to entries whose directory no longer exists and traces them back to the config file to edit:

    $ envocabulary path --check PATH
    ## PATH
      /opt/homebrew/Cellar/go/1.25.1/libexec/bin  ~/.zshrc:17  (does not exist)
      /opt/pkg/env/active/bin                      /etc/paths.d/10-pmk-global:1  (does not exist)
      /Applications/VMware                         /etc/paths.d/com.vmware.fusion.public:1  (does not exist)

Exits 1 when dead entries are found (useful in scripts).

## Limits

- **One assignment per line** (`export EDITOR=vim VISUAL=vim` records `EDITOR` only).
- **`dangling` will not resolve PATH-like assignments or expansions** (`export GOPATH=$HOME/go`).
- **`path` attribution is based on xtrace diffs** — the first assignment that includes an entry claims it, even if it was carried forward via `$PATH` expansion rather than explicitly added.
- **Unsupported shells:** fish, nu, csh/tcsh, PowerShell.

## Read-only by design

envocabulary will never `unset`, `rm`, or edit your shell config. An emergency tool shouldn't be the thing that makes the emergency worse. If you want to clean things up, copy the `file:line` pointers and do the edits yourself. `clean` outputs to stdout; you do the redirect.
