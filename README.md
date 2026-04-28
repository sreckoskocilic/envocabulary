# envocabulary

[![CI](https://github.com/sreckoskocilic/envocabulary/actions/workflows/ci.yml/badge.svg)](https://github.com/sreckoskocilic/envocabulary/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/sreckoskocilic/envocabulary/branch/main/graph/badge.svg)](https://codecov.io/gh/sreckoskocilic/envocabulary)
[![Go Report Card](https://goreportcard.com/badge/github.com/sreckoskocilic/envocabulary)](https://goreportcard.com/report/github.com/sreckoskocilic/envocabulary)
[![Release](https://img.shields.io/github/v/release/sreckoskocilic/envocabulary)](https://github.com/sreckoskocilic/envocabulary/releases/latest)

For every variable in your current shell, find the file and line that set it — or which subsystem (direnv, launchd, terminal, SSH, system) injected it. Plus a few static-file commands for the moment your shell config has sprawled across N files and backups and you've lost the plot.

I built this because I kept losing the same hour every few months tracing why some `JAVA_HOME` or `PATH` was pointing somewhere I didn't expect. `which` knows commands, `direnv status` knows direnv, `launchctl getenv` knows launchd — none of them tell you that `~/.zshrc:42` is the actual writer.

Works with zsh and bash on macOS, Linux, and FreeBSD.

## The "ah" moment

    $ envocabulary explain JAVA_HOME
    JAVA_HOME
        origin:  shell-file
        winner:  ~/.zshrc:42
        also written at:  ~/.zshenv:8

`grep -r JAVA_HOME ~` shows both files but can't tell you which one actually won at startup. envocabulary runs your shell with xtrace and reads the trace.

## Install

One-liner (detects OS/arch, drops the binary on your `$PATH`, clears Gatekeeper on macOS):

```sh
curl -fsSL https://raw.githubusercontent.com/sreckoskocilic/envocabulary/main/install.sh | sh
```

Or `go install github.com/sreckoskocilic/envocabulary/cmd/envocabulary@latest`. Pre-built binaries and Linux packages (`.deb` / `.rpm` / `.apk` / `.pkg.tar.zst`) are on the [releases page](https://github.com/sreckoskocilic/envocabulary/releases/latest).

## Commands

Live env (introspects your running shell):

- `scan` *(default)* — every variable grouped by origin
- `explain NAME` — full attribution for one variable

Static files (parses config without running it):

- `inventory` — what each config file defines (exports, aliases, functions, sources)
- `catalog` — concatenates configs in startup order; `--dedup` flags overridden lines
- `dedup` — cross-file duplicate report
- `dangling` — `source` targets and path-like exports whose file no longer exists
- `clean FILE` — strips boilerplate comments to stdout, never mutates the file

`envocabulary <cmd> -h` for flags. `--shell zsh|bash` overrides auto-detection from `$SHELL` (handy when `$SHELL` is stale inside tmux/SSH/sudo).

## Another example: finding broken references

`dangling` walks your config and reports `source` lines and path-like exports whose target is gone — the `JAVA_HOME=/opt/jdk-i-uninstalled` and `source ~/dotfiles/work-old.zsh` kind of leftovers:

    $ envocabulary dangling
    ## ~/.zshrc
      ~/.zshrc:14  source   → ~/dotfiles/work-old.zsh  (source target missing)
      ~/.zshrc:42  export JAVA_HOME  → /opt/jdk-11  (path does not exist)

Exits non-zero when there are findings, so it composes in shell pipelines.

## Limits

- **Attribution is last-writer only.** If `.zshrc` sources a helper that exports the variable, you see the helper's `file:line`, not the chain that led there.
- **Colon-accumulated vars are not attributed.** `PATH`, `MANPATH`, `FPATH`, `DYLD_*` get tagged `deferred-list-var` and skipped — last-writer would lie because these append rather than override. A dedicated subcommand for per-entry attribution is planned.
- **One assignment per line.** `export A=1 B=2` records `A` only.
- **`dangling` only flags literal paths.** Values containing `$` (variable expansion) or `:` (PATH-like) are skipped — we can't resolve those statically without lying.
- **Unsupported shells:** fish, nu, csh/tcsh, PowerShell.

If startup tracing fails on a given machine, `scan` falls back to classify-only and prints a warning. Bug reports welcome — include `zsh --version` / `bash --version` and the broken output.

## Read-only by design

envocabulary will never `unset`, `rm`, or edit your shell config. An emergency tool shouldn't be the thing that makes the emergency worse. If you want to clean things up, copy the `file:line` pointers and do the edits yourself. `clean` outputs to stdout; you do the redirect.
