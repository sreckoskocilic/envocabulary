package main

import (
	"cmp"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
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

var createReportFile = func(name string) (io.WriteCloser, error) {
	return os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
}

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
			fmt.Fprintf(stdout, "envocabulary %s (commit %s, built %s)\n", version, commit, date)
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

Live-env (introspects the running shell):
  scan [--json] [--values] [--shell SHELL]
      Prints all variables in the current env grouped by origin.

  explain [--json] [--values] [--chain] [--shell SHELL] NAME
      Prints full attribution for provided variable.

  path [--json] [--chain] [--check] [--shell SHELL] [VARNAME...]
      Per-entry attribution for colon-separated path variables.

Static-file:
  inventory
      Lists all shell config files and assigned types variables count.

  catalog [--orphans] [--bash] [-n] [--dedup]
      Prints entire shell configuration by merging all its config files.

  dedup [--orphans] [--bash]
      Cross-file duplicate report for exports, assigns, aliases, functions.

  dangling [--orphans] [--bash]
      Lists config file entries that no longer reference a valid target.

  lost [--bash]
      Lists orphaned files (not sourced by any canonical config).

  clean FILE
      Prints safe-to-remove lines of provided file.

  report [--html] [--bash]
      Combined audit: safe-to-delete, dedup, dangling, lost results.

Run with no arguments for scan. envocabulary <command> -h for per-command help.
`)
}

func helpScan(w io.Writer) {
	fmt.Fprint(w, `envocabulary scan — prints all variables in the current env grouped by origin

Usage:
  envocabulary scan [--json] [--values] [--shell SHELL]
  envocabulary [--json] [--values]                (scan is the default command)

Flags:
  --json          emit JSON instead of grouped text
  --values        include values in output (may expose secrets)
  --shell SHELL   force tracer (zsh|bash); default auto-detects

Examples:
  envocabulary scan
  envocabulary scan --shell bash
  envocabulary scan --json | jq '.[] | select(.origin=="shell-file")'
  envocabulary scan --values | grep -i token
`)
}

func helpExplain(w io.Writer) {
	fmt.Fprint(w, `envocabulary explain — prints full attribution for provided variable

Usage:
  envocabulary explain [--json] [--values] [--chain] [--shell SHELL] NAME

Arguments:
  NAME            the env variable name (e.g. JAVA_HOME, EDITOR)

Flags:
  --json          emit JSON
  --values        include value and raw assignment lines (may expose secrets)
  --chain         show source chain (which file sourced the file that set the var)
  --shell SHELL   force tracer (zsh|bash); default auto-detects

Examples:
  envocabulary explain JAVA_HOME
  envocabulary explain --values EDITOR
  envocabulary explain --chain EDITOR
  envocabulary explain --shell bash PATH
  envocabulary explain --json EDITOR | jq
`)
}

func helpInventory(w io.Writer) {
	fmt.Fprint(w, `envocabulary inventory — lists all shell config files and assigned types variables count

Usage:
  envocabulary inventory

Examples:
  envocabulary inventory
  envocabulary inventory | less
`)
}

func helpCatalog(w io.Writer) {
	fmt.Fprint(w, `envocabulary catalog — prints entire shell configuration by merging all its config files

Usage:
  envocabulary catalog [--orphans] [--bash] [-n] [--dedup]

Prints your zsh config files (.zshenv, .zprofile, .zshrc, .zlogin, .zlogout)

Flags:
  --orphans       also include backup/variant files (.zshrc.backup, .bashrc.old, ...)
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
	fmt.Fprint(w, `envocabulary dangling — lists config file entries that no longer reference a valid target

Usage:
  envocabulary dangling [--orphans] [--bash]

Flags:
  --orphans  include orphan/backup files in the search
  --bash     include bash config files

Examples:
  envocabulary dangling
  envocabulary dangling --orphans --bash
`)
}

func helpDedup(w io.Writer) {
	fmt.Fprint(w, `envocabulary dedup — cross-file duplicate report for exports, assigns, aliases, functions

Usage:
  envocabulary dedup [--orphans] [--bash]

Flags:
  --orphans  include orphan/backup files in the search
  --bash     include bash config files

Examples:
  envocabulary dedup
  envocabulary dedup --bash --orphans
`)
}

func helpClean(w io.Writer) {
	fmt.Fprint(w, `envocabulary clean — prints safe-to-remove lines of provided file

Usage:
  envocabulary clean [--full] FILE

Previews which lines would be stripped (dry-run).

Arguments:
  FILE         path to the shell config file (e.g. ~/.zshrc, ~/.bashrc)

Flags:
  --full       emits full cleaned content

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
	slices.SortStableFunc(keep, func(a, b inventory.File) int {
		return cmp.Compare(inventory.FileRank(a), inventory.FileRank(b))
	})

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
		fmt.Fprintf(w, "  %s:%d  %s %s  → %s  (%s)\n", f.File, f.Line, f.Kind, f.Name, f.Value, f.Reason)
	}
}

func helpLost(w io.Writer) {
	fmt.Fprint(w, `envocabulary lost — lists orphaned files (not sourced by any canonical config)

Usage:
  envocabulary lost [--bash]

Scans for orphaned files (not sourced by any canonical config).

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

Generates aligned text tables summary report containing
safe-to-delete, dedup, dangling, lost results.

Flags:
  --html   write HTML report to MM_DD_YYYY_HH_MM.html in current directory
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

	r := report.Build(keep)

	if *htmlFlag {
		name := r.Generated.Format("01_02_2006_15_04") + ".html"
		f, err := createReportFile(name)
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

	current, err := capture.CurrentEnv()
	if err != nil {
		return die(stderr, err)
	}

	trace, err := capture.TracedStartupWith(tracer)
	if err != nil {
		fmt.Fprintf(stderr, "warning: trace unavailable, falling back to classification-only: %v\n", err)
		trace = nil
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

	current, err := capture.CurrentEnv()
	if err != nil {
		return die(stderr, err)
	}

	trace, err := capture.TracedStartupWith(tracer)
	if err != nil {
		fmt.Fprintf(stderr, "warning: trace unavailable: %v\n", err)
		trace = nil
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

Shows where each entry in PATH, MANPATH, FPATH, etc. was introduced.

Arguments:
  VARNAME...      specific variables (default: all deferred-list vars in env)

Flags:
  --json          emit JSON
  --chain         show source chain
  --check         show only entries whose directory does not exist (exit 1 if any)
  --shell SHELL   force tracer (zsh|bash); default auto-detects

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

	tracer, err := capture.TracerForShell(*shellFlag)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 2
	}

	current, err := capture.CurrentEnv()
	if err != nil {
		return die(stderr, err)
	}

	trace, err := capture.TracedStartupWith(tracer)
	if err != nil {
		fmt.Fprintf(stderr, "warning: trace unavailable: %v\n", err)
		trace = nil
	}

	var varNames []string
	if fs.NArg() > 0 {
		varNames = fs.Args()
	} else {
		for name := range current {
			if model.IsDeferredListVar(name) && current[name] != "" {
				varNames = append(varNames, name)
			}
		}
		slices.Sort(varNames)
	}

	var results []pathentry.VarBreakdown
	for _, name := range varNames {
		r := pathentry.Attribute(name, current[name], trace)
		if len(r.Entries) > 0 {
			pathentry.CheckExists(r.Entries)
			results = append(results, r)
		}
	}

	if *checkExists {
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
		if len(filtered) == 0 {
			fmt.Fprintln(stdout, "no dead path entries found")
			return 0
		}
		if files, err := inventory.Discover(); err == nil {
			overrideFromConfig(filtered, files)
		}
		if *jsonOut {
			if code := emitPathJSON(stdout, stderr, filtered); code != 0 {
				return code
			}
			return 1
		}
		emitPathText(stdout, filtered, *showChain)
		return 1
	}

	if len(results) == 0 {
		fmt.Fprintln(stdout, "no path entries found")
		return 0
	}

	if *jsonOut {
		return emitPathJSON(stdout, stderr, results)
	}
	emitPathText(stdout, results, *showChain)
	return 0
}

func overrideFromConfig(results []pathentry.VarBreakdown, files []inventory.File) {
	pathsFiles := scanPathsD()
	for i, r := range results {
		for j, e := range r.Entries {
			if findConfigRef(&results[i].Entries[j], e.Dir, files) {
				continue
			}
			findPathsDRef(&results[i].Entries[j], e.Dir, pathsFiles)
		}
	}
}

func findConfigRef(entry *pathentry.Entry, dir string, files []inventory.File) bool {
	for _, f := range files {
		for _, item := range f.Items {
			if (item.Kind == inventory.KindExport || item.Kind == inventory.KindAssign) &&
				strings.Contains(item.Value, dir) {
				entry.File = f.Path
				entry.Line = item.Line
				return true
			}
		}
	}
	return false
}

type pathsDEntry struct {
	File string
	Line int
	Dir  string
}

var scanPathsD = scanPathsDFiles

func scanPathsDFiles() []pathsDEntry {
	var entries []pathsDEntry
	readLines := func(path string) {
		data, err := os.ReadFile(path)
		if err != nil {
			return
		}
		for i, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				entries = append(entries, pathsDEntry{File: path, Line: i + 1, Dir: line})
			}
		}
	}
	readLines("/etc/paths")
	dirEntries, err := os.ReadDir("/etc/paths.d")
	if err != nil {
		return entries
	}
	for _, de := range dirEntries {
		if !de.IsDir() {
			readLines("/etc/paths.d/" + de.Name())
		}
	}
	return entries
}

func findPathsDRef(entry *pathentry.Entry, dir string, pathsFiles []pathsDEntry) {
	for _, p := range pathsFiles {
		if p.Dir == dir || strings.HasPrefix(p.Dir, dir+" ") {
			entry.File = p.File
			entry.Line = p.Line
			return
		}
	}
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
