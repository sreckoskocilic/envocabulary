# envocabulary

[![CI](https://github.com/sreckoskocilic/envocabulary/actions/workflows/ci.yml/badge.svg)](https://github.com/sreckoskocilic/envocabulary/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/go-1.22+-00ADD8?logo=go)](https://go.dev/)
[![Platform](https://img.shields.io/badge/platform-macOS-lightgrey)](https://www.apple.com/macos/)

Emergency forensics for macOS env variables. For every variable `env` prints in your current shell, it tells you where that variable came from: a specific shell file and line, direnv, launchd, terminal app, SSH, or system.

Read-only. Designed for the moment you ask *"why on earth is `FOO` set to that?"* and your shell config sprawls across five files.

There's also a complementary set of static-file subcommands — `inventory`, `catalog`, `dedup`, `clean` — for auditing shell config drift without running it. See [Usage](#usage).

## Install

Requires Go 1.22+ and zsh as your login shell (the tracer invokes `zsh -l -i`).

```
go install github.com/sreckoskocilic/envocabulary/cmd/envocabulary@latest
```

Make sure `$(go env GOBIN)` (or `$(go env GOPATH)/bin`) is on your `$PATH`.

## Usage

### Scan all variables

```
envocabulary                # grouped text output (default)
envocabulary scan           # same thing, explicit subcommand
envocabulary --json         # machine-readable
envocabulary --values       # include values (may expose secrets)
```

Output is grouped by origin:

```
## shell-file
JAVA_HOME                       /Users/you/.zprofile:46
EDITOR                          /Users/you/.zshrc:12

## direnv
DIRENV_DIR

## launchd
HOMEBREW_PREFIX                 (launchctl setenv)

## terminal
TERM_PROGRAM
ITERM_PROFILE                   (iTerm)

## system
HOME
LANG                            (locale)
SHLVL                           (shell-managed)

## deferred-list-var
PATH                            multi-source; `envocabulary path` (TODO)

## unknown
SOMETHING_WEIRD
```

### Explain one variable

```
envocabulary explain JAVA_HOME
envocabulary explain --values JAVA_HOME     # include value and raw assignment lines
envocabulary explain --json JAVA_HOME       # JSON for piping into jq
```

Example:

```
JAVA_HOME
  origin   shell-file
  primary  /Users/you/.zprofile:46
  writers
    /Users/you/.zprofile:37
    /Users/you/.zprofile:46  (winner)
  value    [hidden, use --values]
```

The `winner` marker is the assignment that set the final value. Earlier writers are shown so you can spot conflicting config.

### Inventory shell config files

```
envocabulary inventory
```

Walks your canonical zsh files (`.zshenv`, `.zprofile`, `.zshrc`, `.zlogin`, `.zlogout` in `$ZDOTDIR` or `$HOME`) plus bash files and orphan variants (`.zshrc.backup`, `.zshrc.old`, etc.). Reports counts and names per file, grouped by kind:

```
## /Users/you/.zshrc
  exports     12  ZSH, PATH, PATH, LANG, LC_ALL, ...
  assigns      5  ZSH_THEME, plugins, DISABLE_MAGIC_FUNCTIONS, fpath, output
  aliases      1  claude-mem
  functions    1  pip
  sources      2  $ZSH/oh-my-zsh.sh, ~/.zprofile
```

Good for a quick "what's defined where."

### Catalog all contents into one scrollable stream

```
envocabulary catalog                  # canonical zsh files, login order
envocabulary catalog -n               # with line numbers
envocabulary catalog --orphans        # include .zshrc.backup.* files
envocabulary catalog --bash           # include .bashrc / .bash_profile / .profile
envocabulary catalog --dedup          # annotate lines overridden by a later writer
envocabulary catalog | less
```

One concatenated stream instead of opening ten files. Files are emitted in zsh login order, separated by banner headers, so reading top-to-bottom mirrors execution order.

`--dedup` turns each duplicate-earlier-writer line into a shell comment prefixed with `# [overridden by file:line]`, so the output is still valid shell and you can see exactly what gets shadowed:

```
# [overridden by /Users/you/.zshrc:42] export JAVA_HOME=$(/usr/libexec/java_home)
```

### Dedup: find duplicate exports/aliases/functions across files

```
envocabulary dedup
envocabulary dedup --orphans          # include orphan files in the search
envocabulary dedup --bash             # include bash config files
```

Groups duplicates by kind + name and shows which occurrence wins (last writer in execution order) and which are shadowed:

```
## export
  JAVA_HOME
    winner  /Users/you/.zprofile:46
    loser   /Users/you/.zprofile:37
```

Colon-accumulated vars (`PATH`, `MANPATH`, `FPATH`, `INFOPATH`, `CDPATH`, `DYLD_*`) are deliberately excluded — multiple `export PATH=...` lines extend rather than override, so calling them duplicates would lie. Sources are also excluded (re-sourcing the same file from multiple places is usually intentional).

### Clean boilerplate comments from a config file

```
envocabulary clean ~/.zshrc                           # outputs cleaned content to stdout
envocabulary clean ~/.zshrc > ~/.zshrc.cleaned        # you apply it yourself
diff ~/.zshrc ~/.zshrc.cleaned                        # review before replacing
```

Strips default/template comments while preserving your own:

- **Strips**: multi-line prose blocks (oh-my-zsh template help text), commented-out example code (`# export ZSH_THEME=...`, `# plugins=(...)`).
- **Keeps**: single-line section headers (`# aliases`), decorated header blocks (`# --- env vars ---`), shebangs, all real code, any comment that doesn't clearly match a strip rule.

**Writes to stdout only.** The tool will not modify the input file — you do the redirect and the replace yourself. See [Scope](#scope).

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

## How attribution works

1. Capture current env via `env -0` (null-separated to survive newlines in values).
2. Spawn `zsh -l -i -x -c exit` with `PS4='+%x:%i> '` — zsh's xtrace prints every executed line to stderr prefixed with the source file and line number.
3. Parse the trace for assignments (`export FOO=`, `typeset ... FOO=`, bare `FOO=`). Record every writer per name, in order.
4. For each variable:
   - If it's a deferred list var → stub.
   - If it's a direnv var → tag and move on.
   - Else if it appeared in the trace → attribute to the *last* writer's file:line.
   - Else consult the classifier (prefix matches + name tables + `launchctl getenv` probe).
   - Else `unknown`.

## Known gaps

- Attribution is shallow. If `.zshrc` sources a helper that sets the variable, you'll see the helper's `file:line` — not the chain that led there. A `--chain` flag is plausible future work.
- `export A=1 B=2` on one line captures only `A`.
- If `zsh -l -i` fails on your machine (e.g., `zshrc` assumes a TTY), the trace comes back empty; the scan warns and falls back to classify-only attribution.
- The remaining `unknown` bucket is where parent-process-injected vars (tools that `exec` your CLI with extra env) end up. Scope creep warning: don't conflate this with missing functionality.

## Scope

envocabulary is intentionally read-only. It will never `unset`, `rm`, or edit your shell config. An emergency tool must not be the thing that makes the emergency worse. If you want to clean things up, copy the file:line pointers and do the edits yourself.

This applies to subcommands that look like mutations too: `clean` outputs to stdout only and never touches the input file. You do the redirect and the replace.
