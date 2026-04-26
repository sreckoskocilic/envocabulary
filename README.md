# envocabulary

[![CI](https://github.com/sreckoskocilic/envocabulary/actions/workflows/ci.yml/badge.svg)](https://github.com/sreckoskocilic/envocabulary/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/sreckoskocilic/envocabulary/branch/main/graph/badge.svg)](https://codecov.io/gh/sreckoskocilic/envocabulary)
[![Go Report Card](https://goreportcard.com/badge/github.com/sreckoskocilic/envocabulary)](https://goreportcard.com/report/github.com/sreckoskocilic/envocabulary)
[![Release](https://img.shields.io/github/v/release/sreckoskocilic/envocabulary)](https://github.com/sreckoskocilic/envocabulary/releases/latest)
[![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20FreeBSD-lightgrey)](#install)

Forensics and audit toolkit for shell environments. Two layers, one CLI:

- **Live env** — for every variable in your current shell, find out where it came from: a specific `file:line`, direnv, launchd, terminal app, SSH, or system.
- **Static config** — inventory, catalog, dedup, and clean your sprawling shell config files without running them.

Read-only. It will never `unset`, `rm`, or edit your shell config. Designed for the moment you ask *"why on earth is `ENV_VAR` set to that?"* and your shell config sprawls across five files.

> **Shell support.** Live-env commands (`scan`, `explain`) currently require zsh. Static-file commands (`inventory`, `catalog`, `dedup`, `clean`) work with both zsh and bash. fish, nu, csh/tcsh, and PowerShell are not supported. Platforms: macOS, Linux, FreeBSD (amd64, arm64, plus 386/armv7 for Linux).

## Install

### One-liner (recommended)

Detects your OS and architecture, downloads the matching release binary, places it on your `$PATH`. On macOS it also clears the Gatekeeper quarantine so the first run works.

```sh
curl -fsSL https://raw.githubusercontent.com/sreckoskocilic/envocabulary/main/install.sh | sh
```

Pin a specific version or change the install directory:

```sh
curl -fsSL https://raw.githubusercontent.com/sreckoskocilic/envocabulary/main/install.sh \
    | sh -s -- --version v0.1.0 --bin-dir /usr/local/bin
```

### Native Linux packages

For each release we publish `.deb`, `.rpm`, `.apk`, and `.pkg.tar.zst` files alongside the tarballs. Download the right one from the [latest release](https://github.com/sreckoskocilic/envocabulary/releases/latest) page and:

```sh
# Debian, Ubuntu, Mint
sudo dpkg -i envocabulary_*_amd64.deb

# Fedora, RHEL, openSUSE
sudo rpm -i envocabulary_*_amd64.rpm

# Alpine
sudo apk add --allow-untrusted envocabulary_*_amd64.apk

# Arch, Manjaro
sudo pacman -U envocabulary_*_amd64.pkg.tar.zst
```

### Manual download

Pre-built binaries for darwin/linux/freebsd × amd64/arm64 (plus linux 386/armv7) are on the [releases page](https://github.com/sreckoskocilic/envocabulary/releases/latest). Pick the right tarball, extract, place the binary on your `$PATH`.

### From source (Go 1.22+)

```sh
go install github.com/sreckoskocilic/envocabulary/cmd/envocabulary@latest
```

Make sure `$(go env GOBIN)` (or `$(go env GOPATH)/bin`) is on your `$PATH`.

## Usage

For the full flag-by-flag reference, arguments, and example invocations per subcommand, run:

- `envocabulary help` — directory of all subcommands
- `envocabulary <command> -h` — detailed help for one subcommand (e.g. `envocabulary catalog -h`)
- `envocabulary --version` — version and build info

### Subcommands at a glance

**Live env (introspect the running shell — currently zsh-only):**

- **`scan`** *(default)* — one-shot view of every variable in your current shell, grouped by where it came from (shell-file with `file:line`, direnv, launchd, terminal, ssh, system, unknown). Reach for this when something feels wrong about your env and you don't yet know *which* variable is the problem.

- **`explain NAME`** — full attribution for a single variable: every writer in startup order, the winner marked. Reach for this when you already know the misbehaving variable and need to spot which file:line ultimately won — the typical "I set this in two places and forgot" finding.

**Static config (audit shell config files without running them — zsh + bash):**

- **`inventory`** — counts and names per file (exports, assigns, aliases, functions, sources). Reach for this before any cleanup, to refresh your mental model of what each of your `~/.zshrc`/`~/.bashrc`/orphan files actually defines.

- **`catalog`** — concatenates all canonical config files in startup order to stdout. Reach for this when you want to read your config in execution order without opening five files in five tabs. With `--dedup`, it inline-annotates which lines are dead because something later overrides them (highlighted in red on a terminal).

- **`dedup`** — focused list of duplicate exports/aliases/functions across files, with the winning writer marked. Reach for this when you suspect conflicting writers and want only the conflicts, not the surrounding code.

- **`clean FILE`** — strip default/template comments (oh-my-zsh boilerplate, commented-out examples). Default mode is a **dry-run preview** that shows which lines *would* be stripped. Pass `--full` to emit the cleaned content to stdout for you to redirect into a new file. Read-only — never mutates the input.

## Origin taxonomy

| Origin              | Meaning                                                                                      |
| ------------------- | -------------------------------------------------------------------------------------------- |
| `shell-file`        | A literal assignment was traced to a specific file and line in your shell startup.           |
| `direnv`            | A `DIRENV_*` variable.                                                                       |
| `launchd`           | Confirmed by `launchctl getenv NAME` (typically set via `launchctl setenv` in a plist).      |
| `terminal`          | Terminal app injection (iTerm, Apple Terminal, etc.).                                        |
| `ssh`               | SSH session variables (`SSH_AUTH_SOCK`, `SSH_TTY`, ...).                                     |
| `system`            | System basics (`HOME`, `USER`, `PWD`, ...), locale (`LANG`, `LC_*`), shell internals.        |
| `deferred-list-var` | Colon-accumulated vars (`PATH`, `MANPATH`, `DYLD_*`). Last-writer attribution lies here — a future `envocabulary path` subcommand will split these per entry. |
| `unknown`           | None of the above. Usually parent-process injection or a name the classifier doesn't know.   |

## Limits

- Attribution is shallow. If a config file sources a helper that sets a variable, you'll see the helper's `file:line` — not the chain that led there.
- `export A=1 B=2` on one line is recorded as `A` only.
- If startup tracing fails on your machine, the scan warns and falls back to classify-only attribution.
- Some variables land in `unknown` — typically parent-process injection (a tool that `exec`s your CLI with extra env).

Bug reports for breakage on a new zsh release: open a [GitHub issue](https://github.com/sreckoskocilic/envocabulary/issues) with your `zsh --version` and a snippet of the broken output.

## Scope

envocabulary is intentionally read-only. It will never `unset`, `rm`, or edit your shell config. An emergency tool must not be the thing that makes the emergency worse. If you want to clean things up, copy the `file:line` pointers and do the edits yourself.

This applies to subcommands that look like mutations too: `clean` outputs to stdout only and never touches the input file. You do the redirect and the replace.
