package main

import (
	"cmp"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/sreckoskocilic/envocabulary/internal/attribute"
	"github.com/sreckoskocilic/envocabulary/internal/capture"
	"github.com/sreckoskocilic/envocabulary/internal/catalog"
	"github.com/sreckoskocilic/envocabulary/internal/cleaner"
	"github.com/sreckoskocilic/envocabulary/internal/dangling"
	"github.com/sreckoskocilic/envocabulary/internal/dedup"
	"github.com/sreckoskocilic/envocabulary/internal/explain"
	"github.com/sreckoskocilic/envocabulary/internal/inventory"
	"github.com/sreckoskocilic/envocabulary/internal/lost"
	"github.com/sreckoskocilic/envocabulary/internal/model"
	"github.com/sreckoskocilic/envocabulary/internal/pathentry"
	"github.com/sreckoskocilic/envocabulary/internal/report"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func versionString() string {
	v, c, d := version, commit, date
	if v == "dev" {
		if bi, ok := debug.ReadBuildInfo(); ok {
			if bi.Main.Version != "" && bi.Main.Version != "(devel)" {
				v = strings.TrimPrefix(bi.Main.Version, "v")
			}
			for _, st := range bi.Settings {
				switch st.Key {
				case "vcs.revision":
					if len(st.Value) >= 7 {
						c = st.Value[:7]
					}
				case "vcs.time":
					d = st.Value
				}
			}
		}
	}
	return fmt.Sprintf("envocabulary %s (commit %s, built %s)", v, c, d)
}

func warnShell(stderr io.Writer, shellFlag string) {
	if shellFlag != "" {
		return
	}
	if w := capture.ShellWarning(); w != "" {
		fmt.Fprintln(stderr, "warning:", w)
	}
}

var createReportFile = func(name string) (io.WriteCloser, error) {
	return os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
}

func createUniqueReport(base string) (string, io.WriteCloser, error) {
	for i := 1; i <= 100; i++ {
		name := base + ".html"
		if i > 1 {
			name = fmt.Sprintf("%s-%d.html", base, i)
		}
		f, err := createReportFile(name)
		if err == nil {
			return name, f, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return "", nil, err
		}
	}
	return "", nil, fmt.Errorf("%s.html and 99 numbered variants already exist", base)
}

var tracedStartup = capture.TracedStartupWith

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "-h", "--help", "help":
			usage(stdout)
			return 0
		case "-V", "--version", "version":
			fmt.Fprintln(stdout, versionString())
			return 0
		}
		if !strings.HasPrefix(args[0], "-") {
			switch args[0] {
			case "scan":
				return runScan(args[1:], stdout, stderr)
			case "explain":
				return runExplain(args[1:], stdout, stderr)
			case "inventory":
				return runInventory(args[1:], stdout, stderr)
			case "clean":
				return runClean(args[1:], stdout, stderr)
			case "catalog":
				return runCatalog(args[1:], stdout, stderr)
			case "dedup":
				return runDedup(args[1:], stdout, stderr)
			case "dangling":
				return runDangling(args[1:], stdout, stderr)
			case "lost":
				return runLost(args[1:], stdout, stderr)
			case "path":
				return runPath(args[1:], stdout, stderr)
			case "report":
				return runReport(args[1:], stdout, stderr)
			default:
				fmt.Fprintf(stderr, "unknown command: %s\n\n", args[0])
				usage(stderr)
				return 2
			}
		}
	}
	return runScan(args, stdout, stderr)
}

func usage(w io.Writer) {
	fmt.Fprint(w, `envocabulary — shell env-var forensics & static config audit (read-only)

Live env (reads the running shell):
  scan [--json] [--values] [--shell SHELL]
      All variables in the current env, grouped by origin.

  explain [--json] [--values] [--chain] [--shell SHELL] NAME
      Full attribution for one variable.

  path [--json] [--chain] [--check] [--shell SHELL] [VARNAME...]
      Per-entry attribution for colon-separated path variables.

Static files (the dotfiles in $HOME and their backup variants):
  inventory
      Config files found and definition counts by type.

  catalog [--orphans] [--bash] [-n] [--dedup]
      All config files concatenated in the order the shell reads them.

  dedup [--bash]
      Duplicate definitions within and across files.

  dangling [--orphans] [--bash]
      Sources and path exports whose target no longer exists.

  lost [--bash]
      Definitions that exist only in orphan/backup files.

  clean [--full] FILE
      Comment lines that would be stripped; --full prints the cleaned file.

  report [--html] [--bash]
      Combined audit: safe-to-delete, review, dangling, orphaned files.

Other:
  -V, --version, version
      Version, commit, and build date.
  -h, --help, help

Exit status: 0 ok; 1 runtime error, or dangling / path --check found something;
2 usage error. Warnings go to stderr, output to stdout.

Run with no arguments for scan. envocabulary <command> -h for per-command help.
`)
}

func helpScan(w io.Writer) {
	fmt.Fprint(w, `envocabulary scan — all variables in the current env, grouped by origin

Usage:
  envocabulary scan [--json] [--values] [--shell SHELL]
  envocabulary [--json] [--values]                (scan is the default command)

Origins: shell-file (file:line from tracing a login shell), direnv, terminal,
ssh, launchd, system, deferred-list-var (PATH-like; use envocabulary path),
unknown.

Flags:
  --json          emit JSON: [{name, value?, origin, source?, notes?}]
  --values        include values (may expose secrets); text output truncates
                  them to 60 characters, --json prints them whole
  --shell SHELL   force tracer (zsh|bash); default: bash if $SHELL is bash,
                  otherwise zsh

Examples:
  envocabulary scan
  envocabulary scan --shell bash
  envocabulary scan --json | jq '.[] | select(.origin=="shell-file")'
  envocabulary scan --values --json | jq -r '.[] | select(.value|test("token";"i")) | .name'
`)
}

func helpExplain(w io.Writer) {
	fmt.Fprint(w, `envocabulary explain — full attribution for one variable

Usage:
  envocabulary explain [--json] [--values] [--chain] [--shell SHELL] NAME

Shows the origin, the winning writer (primary) and every file:line that
assigned the variable during login, in execution order. Variables set through
eval "$(...)" or inside a function resolve to the eval or the function body
line in the file.

Arguments:
  NAME            the env variable name (e.g. JAVA_HOME, EDITOR)

Flags:
  --json          emit JSON: {name, present, value?, origin, primary?,
                  writers: [{file, line, name, raw?, chain?}], notes?}
  --values        include value and raw assignment lines (may expose secrets)
  --chain         show source chain (which file sourced the file that set the var)
  --shell SHELL   force tracer (zsh|bash); default: bash if $SHELL is bash,
                  otherwise zsh

Examples:
  envocabulary explain JAVA_HOME
  envocabulary explain --values EDITOR
  envocabulary explain --chain EDITOR
  envocabulary explain --shell bash EDITOR
  envocabulary explain --json EDITOR | jq
`)
}

func helpInventory(w io.Writer) {
	fmt.Fprint(w, `envocabulary inventory — config files found and definition counts by type

Usage:
  envocabulary inventory

Scans $HOME only: .zshenv .zprofile .zshrc .zlogin .zlogout .bashrc
.bash_profile .profile, plus variants of those names (NAME.*, NAME_*, NAME-*,
e.g. .zshrc.backup) as orphans. $ZDOTDIR, ~/.config/zsh, /etc and files you
source are not scanned. Counts cover exports, assigns, aliases, functions and
source lines; one definition per line.

Examples:
  envocabulary inventory
  envocabulary inventory | less
`)
}

func helpCatalog(w io.Writer) {
	fmt.Fprint(w, `envocabulary catalog — all config files concatenated in the order the shell reads them

Usage:
  envocabulary catalog [--orphans] [--bash] [-n] [--dedup]

Prints .zshenv, .zprofile, .zshrc, .zlogin, .zlogout in login order, each under
a header with its path. Exits 1 if a file could not be read; the output is then
incomplete.

Flags:
  --orphans       also include zsh backup/variant files (.zshrc.backup, ...); with --bash, bash variants too. Never annotated by --dedup
  --bash          also include .bashrc / .bash_profile / .profile
  -n              prefix each line with its source line number
  --dedup         comment out lines overridden by a later writer,
                  annotated as: # [overridden by file:line] ...

Examples:
  envocabulary catalog | less
  envocabulary catalog -n
  envocabulary catalog --bash --orphans
  envocabulary catalog --dedup
  envocabulary catalog --dedup | grep '# \[overridden'
`)
}

func helpDangling(w io.Writer) {
	fmt.Fprint(w, `envocabulary dangling — sources and path exports whose target no longer exists

Usage:
  envocabulary dangling [--orphans] [--bash]

Checks source/. targets and export/assign values that look like a literal
absolute or ~/ path. Values with $VAR, $(...) or colons (PATH-like) cannot be
resolved statically and are skipped. Exits 1 when anything is found, so it
works as a check in scripts.

Flags:
  --orphans  also check zsh backup/variant files; with --bash, bash variants too
  --bash     include bash config files

Examples:
  envocabulary dangling
  envocabulary dangling --orphans --bash
`)
}

func helpDedup(w io.Writer) {
	fmt.Fprint(w, `envocabulary dedup — duplicate report for exports, assigns, aliases, functions

Usage:
  envocabulary dedup [--bash]

Groups definitions of the same name within and across the config files the
shell actually reads; the last one in login order wins. Backup/variant files
are never grouped (they are not executed), see lost for those. .zlogout is
skipped for the same reason.

Flags:
  --bash     include bash config files

Examples:
  envocabulary dedup
  envocabulary dedup --bash
`)
}

func helpClean(w io.Writer) {
	fmt.Fprint(w, `envocabulary clean — comment lines that would be stripped from a config file

Usage:
  envocabulary clean [--full] FILE

Strips commented-out code (# export FOO=..., # alias x=..., # source ...),
decoration bars (# -----) and multi-line comment blocks of prose such as
template boilerplate. Single-line comments and short section labels are kept.
Multi-line prose you wrote yourself is also stripped: review the preview
before redirecting. Never touches non-comment lines and never writes to FILE.

Arguments:
  FILE         path to the shell config file (e.g. ~/.zshrc, ~/.bashrc)

Flags:
  --full       print the cleaned file to stdout instead of the preview;
               a "# N kept, M stripped" summary goes to stderr

Examples:
  envocabulary clean ~/.zshrc
  envocabulary clean --full ~/.zshrc > ~/.zshrc.cleaned
  diff ~/.zshrc ~/.zshrc.cleaned
`)
}

func runCatalog(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("catalog", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { helpCatalog(stdout) }
	orphans := fs.Bool("orphans", false, "include orphan/backup files")
	bash := fs.Bool("bash", false, "include bash config files")
	lineNums := fs.Bool("n", false, "prefix each line with its line number")
	dedupFlag := fs.Bool("dedup", false, "comment out lines overridden by a later writer")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	opts := catalog.Options{
		IncludeOrphans: *orphans,
		IncludeBash:    *bash,
		LineNumbers:    *lineNums,
		Dedup:          *dedupFlag,
	}
	if err := catalog.Write(stdout, opts); err != nil {
		return die(stderr, err)
	}
	return 0
}

func runDedup(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("dedup", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { helpDedup(stdout) }
	bash := fs.Bool("bash", false, "include bash config files")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	files, err := inventory.Discover()
	if err != nil {
		return die(stderr, err)
	}
	keep := inventory.FilterFiles(files, *bash, false)
	slices.SortStableFunc(keep, func(a, b inventory.File) int {
		return cmp.Compare(inventory.FileRank(a), inventory.FileRank(b))
	})
	warnUnreadable(stderr, keep)

	groups := dedup.Find(keep)
	if len(groups) == 0 {
		fmt.Fprintln(stdout, "no duplicates found")
		return 0
	}
	emitDedupText(stdout, groups)
	return 0
}

func emitDedupText(w io.Writer, groups []dedup.Group) {
	currentKind := inventory.Kind("")
	for i := range groups {
		if groups[i].Kind != currentKind {
			if currentKind != "" {
				fmt.Fprintln(w)
			}
			fmt.Fprintf(w, "## %s\n", groups[i].Kind)
			currentKind = groups[i].Kind
		}
		fmt.Fprintf(w, "  %s\n", groups[i].Name)
		fmt.Fprintf(w, "    winner  %s:%d\n", groups[i].Winner.File, groups[i].Winner.Line)
		for j := range groups[i].Losers {
			fmt.Fprintf(w, "    loser   %s:%d\n", groups[i].Losers[j].File, groups[i].Losers[j].Line)
		}
	}
}

func runDangling(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("dangling", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { helpDangling(stdout) }
	orphans := fs.Bool("orphans", false, "include orphan/backup files")
	bash := fs.Bool("bash", false, "include bash config files")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	files, err := inventory.Discover()
	if err != nil {
		return die(stderr, err)
	}
	keep := inventory.FilterFiles(files, *bash, *orphans)
	warnUnreadable(stderr, keep)

	findings := dangling.Find(keep)
	if len(findings) == 0 {
		fmt.Fprintln(stdout, "no dangling references found")
		return 0
	}
	emitDanglingText(stdout, findings)
	return 1
}

func emitDanglingText(w io.Writer, findings []dangling.Finding) {
	currentFile := ""
	for _, f := range findings {
		if f.File != currentFile {
			if currentFile != "" {
				fmt.Fprintln(w)
			}
			fmt.Fprintf(w, "## %s\n", f.File)
			currentFile = f.File
		}
		if f.Kind == inventory.KindSource {
			fmt.Fprintf(w, "  %s:%d  %s  → %s  (%s)\n", f.File, f.Line, f.Kind, f.Value, f.Reason)
		} else {
			fmt.Fprintf(w, "  %s:%d  %s %s  → %s  (%s)\n", f.File, f.Line, f.Kind, f.Name, f.Value, f.Reason)
		}
	}
}

func helpLost(w io.Writer) {
	fmt.Fprint(w, `envocabulary lost — lists definitions unique to orphan/backup config files

Usage:
  envocabulary lost [--bash]

Scans orphan/backup config files (.zshrc.backup, .zprofile.old, ...) for
definitions whose name does not appear in any file the shell reads. export and
plain assignment of the same name count as one definition.

Flags:
  --bash  include bash config files

Examples:
  envocabulary lost
  envocabulary lost --bash
`)
}

func runLost(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lost", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { helpLost(stdout) }
	bash := fs.Bool("bash", false, "include bash config files")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	files, err := inventory.Discover()
	if err != nil {
		return die(stderr, err)
	}
	keep := inventory.FilterFiles(files, *bash, true)
	warnUnreadable(stderr, keep)

	findings := lost.Find(keep)
	if len(findings) == 0 {
		fmt.Fprintln(stdout, "no lost items found")
		return 0
	}
	emitLostText(stdout, findings)
	return 0
}

func emitLostText(w io.Writer, findings []lost.Finding) {
	currentFile := ""
	for _, f := range findings {
		if f.File != currentFile {
			if currentFile != "" {
				fmt.Fprintln(w)
			}
			fmt.Fprintf(w, "## %s\n", f.File)
			currentFile = f.File
		}
		fmt.Fprintf(w, "  %-10s %-24s :%d\n", f.Kind, f.Name, f.Line)
	}
}

func runClean(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("clean", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { helpClean(stdout) }
	full := fs.Bool("full", false, "emit full cleaned content (default is dry-run preview of stripped lines)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() < 1 {
		helpClean(stderr)
		return 2
	}

	path := fs.Arg(0)
	f, err := os.Open(path)
	if err != nil {
		return die(stderr, err)
	}
	defer f.Close()

	if *full {
		cleaned, stats, err := cleaner.Clean(f)
		if err != nil {
			return die(stderr, err)
		}
		fmt.Fprint(stdout, cleaned)
		fmt.Fprintf(stderr, "# %d kept, %d stripped (--full mode)\n", stats.Kept, stats.Stripped)
		return 0
	}

	decisions, _, err := cleaner.Process(f)
	if err != nil {
		return die(stderr, err)
	}
	for _, d := range decisions {
		if d.Kept {
			continue
		}
		fmt.Fprintf(stdout, "- %5d  %s\n", d.LineNum, d.Content)
	}
	return 0
}

func runInventory(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("inventory", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { helpInventory(stdout) }
	if err := fs.Parse(args); err != nil {
		return 2
	}

	files, err := inventory.Discover()
	if err != nil {
		return die(stderr, err)
	}
	if len(files) == 0 {
		fmt.Fprintln(stderr, "no shell config files found")
		return 0
	}
	emitInventoryText(stdout, files)
	return 0
}

func emitInventoryText(w io.Writer, files []inventory.File) {
	for i, f := range files {
		if i > 0 {
			fmt.Fprintln(w)
		}
		suffix := ""
		if f.Role == inventory.RoleOrphan {
			suffix = "  (orphan)"
		}
		fmt.Fprintf(w, "## %s%s\n", f.Path, suffix)
		if f.Err != nil {
			fmt.Fprintf(w, "  error: %v\n", f.Err)
			continue
		}
		groups := groupItems(f.Items)
		printGroup(w, "exports", groups[inventory.KindExport])
		printGroup(w, "assigns", groups[inventory.KindAssign])
		printGroup(w, "aliases", groups[inventory.KindAlias])
		printGroup(w, "functions", groups[inventory.KindFunction])
		printGroup(w, "sources", groups[inventory.KindSource])
	}
}

func groupItems(items []inventory.Item) map[inventory.Kind][]inventory.Item {
	g := map[inventory.Kind][]inventory.Item{}
	for _, it := range items {
		g[it.Kind] = append(g[it.Kind], it)
	}
	return g
}

func printGroup(w io.Writer, label string, items []inventory.Item) {
	if len(items) == 0 {
		return
	}
	names := make([]string, 0, len(items))
	for _, it := range items {
		names = append(names, it.Name)
	}
	fmt.Fprintf(w, "  %-10s %3d  %s\n", label, len(items), strings.Join(names, ", "))
}

func emitScanJSON(stdout, stderr io.Writer, words []model.EnWord, showValues bool) int {
	type out struct {
		Name   string       `json:"name"`
		Value  string       `json:"value,omitempty"`
		Origin model.Origin `json:"origin"`
		Source string       `json:"source,omitempty"`
		Notes  string       `json:"notes,omitempty"`
	}
	list := make([]out, 0, len(words))
	for _, w := range words {
		o := out{Name: w.Name, Origin: w.Origin, Source: w.Source, Notes: w.Notes}
		if showValues {
			o.Value = w.Value
		}
		list = append(list, o)
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(list); err != nil {
		return die(stderr, err)
	}
	return 0
}

func emitScanText(w io.Writer, words []model.EnWord, showValues bool) {
	current := model.Origin("")
	for _, ent := range words {
		if ent.Origin != current {
			if current != "" {
				fmt.Fprintln(w)
			}
			fmt.Fprintf(w, "## %s\n", ent.Origin)
			current = ent.Origin
		}
		line := fmt.Sprintf("%-32s", ent.Name)
		if ent.Source != "" {
			line += "  " + ent.Source
		}
		if ent.Notes != "" {
			line += "  (" + ent.Notes + ")"
		}
		if showValues {
			line += "  = " + truncate(ent.Value, 60)
		}
		fmt.Fprintln(w, line)
	}
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}

func helpReport(w io.Writer) {
	fmt.Fprint(w, `envocabulary report — combined audit report

Usage:
  envocabulary report [--html] [--bash]

Sections: SAFE TO DELETE (duplicates with the same value as the winner),
REVIEW (duplicates whose value differs, plus every duplicate function),
DANGLING, ORPHANED FILES, UNREADABLE FILES. Backup/variant files are always
scanned: they feed ORPHANED FILES and DANGLING but never SAFE TO DELETE or
REVIEW. Definition values are printed as written in the file.

Flags:
  --html   write the report to MM_DD_YYYY_HH_MM.html in the current directory,
           mode 0600, and print the file name; a taken name gets a -2, -3, ...
           suffix, nothing is overwritten
  --bash   include bash config files

Examples:
  envocabulary report
  envocabulary report --html
  envocabulary report --bash --html
`)
}

func runReport(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { helpReport(stdout) }
	htmlFlag := fs.Bool("html", false, "write HTML report to MM_DD_YYYY_HH_MM.html")
	bash := fs.Bool("bash", false, "include bash config files")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	files, err := inventory.Discover()
	if err != nil {
		return die(stderr, err)
	}
	keep := inventory.FilterFiles(files, *bash, true)
	slices.SortStableFunc(keep, func(a, b inventory.File) int {
		return cmp.Compare(inventory.FileRank(a), inventory.FileRank(b))
	})
	warnUnreadable(stderr, keep)

	r := report.Build(keep)

	if *htmlFlag {
		name, f, err := createUniqueReport(r.Generated.Format("01_02_2006_15_04"))
		if err != nil {
			return die(stderr, err)
		}
		if err := report.WriteHTML(f, r); err != nil {
			f.Close()
			return die(stderr, err)
		}
		if err := f.Close(); err != nil {
			return die(stderr, err)
		}
		fmt.Fprintln(stdout, name)
		return 0
	}
	if err := report.WriteText(stdout, r); err != nil {
		return die(stderr, err)
	}
	return 0
}

func die(stderr io.Writer, err error) int {
	fmt.Fprintln(stderr, "error:", err)
	return 1
}

func warnUnreadable(stderr io.Writer, files []inventory.File) {
	for _, f := range files {
		if f.Err != nil {
			fmt.Fprintf(stderr, "warning: %s not parsed, results are incomplete: %v\n", f.Path, f.Err)
		}
	}
}

func runScan(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { helpScan(stdout) }
	jsonOut := fs.Bool("json", false, "emit JSON instead of grouped text")
	showValues := fs.Bool("values", false, "include values in output (may expose secrets)")
	shellFlag := fs.String("shell", "", "force tracer for a specific shell (zsh|bash); default auto-detects from $SHELL")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	tracer, err := capture.TracerForShell(*shellFlag)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 2
	}
	warnShell(stderr, *shellFlag)

	current, err := capture.CurrentEnv()
	if err != nil {
		return die(stderr, err)
	}

	trace, err := tracedStartup(tracer)
	if err != nil {
		fmt.Fprintf(stderr, "warning: %v; falling back to classification where the trace is missing\n", err)
	}

	words := attribute.Attribute(current, trace)

	if *jsonOut {
		return emitScanJSON(stdout, stderr, words, *showValues)
	}
	emitScanText(stdout, words, *showValues)
	return 0
}

func runExplain(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("explain", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { helpExplain(stdout) }
	jsonOut := fs.Bool("json", false, "emit JSON")
	showValues := fs.Bool("values", false, "include value and raw traced commands (may expose secrets)")
	showChain := fs.Bool("chain", false, "show source chain (which file sourced the file that set the var)")
	shellFlag := fs.String("shell", "", "force tracer for a specific shell (zsh|bash); default auto-detects from $SHELL")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if fs.NArg() < 1 {
		helpExplain(stderr)
		return 2
	}
	name := fs.Arg(0)

	tracer, err := capture.TracerForShell(*shellFlag)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 2
	}
	warnShell(stderr, *shellFlag)

	current, err := capture.CurrentEnv()
	if err != nil {
		return die(stderr, err)
	}

	trace, err := tracedStartup(tracer)
	if err != nil {
		fmt.Fprintf(stderr, "warning: %v\n", err)
	}

	result := explain.Explain(name, current, trace)

	if *jsonOut {
		if err := explain.EmitJSON(stdout, result, *showValues); err != nil {
			return die(stderr, err)
		}
		return 0
	}
	explain.EmitText(stdout, result, *showValues, *showChain)
	return 0
}

func helpPath(w io.Writer) {
	fmt.Fprint(w, `envocabulary path — per-entry attribution for colon-separated path variables

Usage:
  envocabulary path [--json] [--chain] [--check] [--shell SHELL] [VARNAME...]

Shows where each entry in PATH, MANPATH, FPATH, etc. was introduced by
replaying the login shell with the list variables reset. Entries the seed
provides (/usr/bin /bin /usr/sbin /sbin) or that were already in the env before
login show as inherited. zsh array forms (path=(...), path+=(...)) are not
recognized yet.

Arguments:
  VARNAME...      PATH MANPATH FPATH INFOPATH CDPATH DYLD_* (default: all present)

Flags:
  --json          emit JSON: [{name, entries: [{dir, file?, line?, chain?, exists?}]}]
  --chain         show source chain
  --check         show only entries whose directory does not exist and exit 1 if
                  any; their source is re-resolved against your dotfiles and
                  /etc/paths, /etc/paths.d so it points at the line to edit
  --shell SHELL   force tracer (zsh|bash); default: bash if $SHELL is bash,
                  otherwise zsh

Examples:
  envocabulary path
  envocabulary path PATH
  envocabulary path --check
  envocabulary path --chain PATH MANPATH
  envocabulary path --json | jq
`)
}

func runPath(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("path", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { helpPath(stdout) }
	jsonOut := fs.Bool("json", false, "emit JSON")
	showChain := fs.Bool("chain", false, "show source chain")
	checkExists := fs.Bool("check", false, "show only entries whose directory does not exist")
	shellFlag := fs.String("shell", "", "force tracer (zsh|bash); default auto-detects from $SHELL")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	tracer, err := capture.TracerForShellBaseline(*shellFlag)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 2
	}
	warnShell(stderr, *shellFlag)

	current, err := capture.CurrentEnv()
	if err != nil {
		return die(stderr, err)
	}

	trace, err := tracedStartup(tracer)
	if err != nil {
		fmt.Fprintf(stderr, "warning: %v\n", err)
	}

	results := collectBreakdowns(resolvePathTargets(fs, current), current, trace)

	if *checkExists {
		filtered := filterDead(results)
		if len(filtered) == 0 {
			fmt.Fprintln(stdout, "no dead path entries found")
			return 0
		}
		if files, err := inventory.Discover(); err == nil {
			shell := *shellFlag
			if shell == "" {
				shell = capture.DetectShell()
			}
			pathentry.OverrideFromConfig(filtered, inventory.FilterFiles(files, shell == "bash", false))
		}
		if code := emitPath(stdout, stderr, filtered, *jsonOut, *showChain); code != 0 {
			return code
		}
		return 1
	}

	if len(results) == 0 {
		fmt.Fprintln(stdout, "no path entries found")
		return 0
	}
	return emitPath(stdout, stderr, results, *jsonOut, *showChain)
}

func resolvePathTargets(fs *flag.FlagSet, current map[string]string) []string {
	if fs.NArg() > 0 {
		return fs.Args()
	}
	var names []string
	for name := range current {
		if model.IsDeferredListVar(name) && current[name] != "" {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}

func collectBreakdowns(varNames []string, current map[string]string, trace []model.TraceEntry) []pathentry.VarBreakdown {
	var results []pathentry.VarBreakdown
	for _, name := range varNames {
		r := pathentry.Attribute(name, current[name], capture.BaselineListValue(name), trace)
		if len(r.Entries) > 0 {
			pathentry.CheckExists(r.Entries)
			results = append(results, r)
		}
	}
	return results
}

func filterDead(results []pathentry.VarBreakdown) []pathentry.VarBreakdown {
	var filtered []pathentry.VarBreakdown
	for _, r := range results {
		var dead []pathentry.Entry
		for _, e := range r.Entries {
			if e.Exists != nil && !*e.Exists {
				dead = append(dead, e)
			}
		}
		if len(dead) > 0 {
			filtered = append(filtered, pathentry.VarBreakdown{Name: r.Name, Entries: dead})
		}
	}
	return filtered
}

func emitPath(stdout, stderr io.Writer, results []pathentry.VarBreakdown, jsonOut, showChain bool) int {
	if jsonOut {
		return emitPathJSON(stdout, stderr, results)
	}
	emitPathText(stdout, results, showChain)
	return 0
}

func emitPathText(w io.Writer, results []pathentry.VarBreakdown, showChain bool) {
	for i, r := range results {
		if i > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "## %s\n", r.Name)

		maxDir := 0
		for _, e := range r.Entries {
			if len(e.Dir) > maxDir {
				maxDir = len(e.Dir)
			}
		}

		for _, e := range r.Entries {
			source := "inherited"
			if e.File != "" {
				source = fmt.Sprintf("%s:%d", e.File, e.Line)
			}
			chainInfo := ""
			if showChain && len(e.Chain) > 0 {
				chainInfo = fmt.Sprintf("  (via %s)", strings.Join(e.Chain, " → "))
			}
			deadInfo := ""
			if e.Exists != nil && !*e.Exists {
				deadInfo = "  (does not exist)"
			}
			fmt.Fprintf(w, "  %-*s  %s%s%s\n", maxDir, e.Dir, source, chainInfo, deadInfo)
		}
	}
}

func emitPathJSON(stdout, stderr io.Writer, results []pathentry.VarBreakdown) int {
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(results); err != nil {
		return die(stderr, err)
	}
	return 0
}
