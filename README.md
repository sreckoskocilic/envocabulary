# envocabulary

Emergency forensics for macOS env variables. For every variable `env` prints in your current shell, it tells you where that variable came from: a specific shell file and line, direnv, launchd, terminal app, SSH, or system.

Read-only. Designed for the moment you ask *"why on earth is `FOO` set to that?"* and your shell config sprawls across five files.

## Install

Requires Go 1.22+ and zsh as your login shell (the tracer invokes `zsh -l -i`).

```
git clone <repo>
cd envocabulary
go build -o envocabulary ./cmd/envocabulary
```

Drop the resulting `envocabulary` binary anywhere on your `$PATH`.

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
