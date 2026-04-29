package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/sreckoskocilic/envocabulary/internal/catalog"
	"github.com/sreckoskocilic/envocabulary/internal/cleaner"
	"github.com/sreckoskocilic/envocabulary/internal/dangling"
	"github.com/sreckoskocilic/envocabulary/internal/dedup"
	"github.com/sreckoskocilic/envocabulary/internal/inventory"
	"github.com/sreckoskocilic/envocabulary/internal/lost"
	"github.com/sreckoskocilic/envocabulary/internal/model"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

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
      Group every variable in the current env by origin (shell-file,
      direnv, launchd, terminal, ssh, system...), default command.

  explain [--json] [--values] [--shell SHELL] NAME
      Full attribution for one variable: origin, primary writer,
      and every other writer in startup order.

Static-file (parses config without running it):
  inventory
      Counts and names per shell config file.

  catalog [--orphans] [--bash] [-n] [--dedup]
      Concatenate canonical zsh config files.

  dedup [--orphans] [--bash]
      Cross-file duplicate report for exports/assigns/aliases/functions.

  dangling [--orphans] [--bash]
      Source lines and path-like exports of missing targets.

  lost [--bash]
      Config in orphan files not found in canonical.

  clean FILE
      Strip boilerplate comments from FILE to stdout (never mutates).

Run with no arguments for scan. envocabulary <command> -h for per-command help.
`)
}

func helpScan(w io.Writer) {
	fmt.Fprint(w, `envocabulary scan — group every variable in the current env by origin

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
	fmt.Fprint(w, `envocabulary explain — show full attribution for one variable

Usage:
  envocabulary explain [--json] [--values] [--shell SHELL] NAME

Arguments:
  NAME            the env variable name (e.g. JAVA_HOME, EDITOR)

Flags:
  --json          emit JSON
  --values        include value and raw assignment lines (may expose secrets)
  --shell SHELL   force tracer (zsh|bash); default auto-detects

Examples:
  envocabulary explain JAVA_HOME
  envocabulary explain --values EDITOR
  envocabulary explain --shell bash PATH
  envocabulary explain --json EDITOR | jq
`)
}

func helpInventory(w io.Writer) {
	fmt.Fprint(w, `envocabulary inventory — counts and names per shell config file

Usage:
  envocabulary inventory

Examples:
  envocabulary inventory
  envocabulary inventory | less
`)
}

func helpCatalog(w io.Writer) {
	fmt.Fprint(w, `envocabulary catalog — concatenate shell config files in startup order

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
	fmt.Fprint(w, `envocabulary dangling — source lines and path-like exports whose target is gone

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
	fmt.Fprint(w, `envocabulary dedup — cross-file duplicate report for exports/assigns/aliases/functions

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
	fmt.Fprint(w, `envocabulary clean — strip boilerplate comments from a config file

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

	files := inventory.Discover()
	keep := make([]inventory.File, 0, len(files))
	for _, f := range files {
		switch f.Role {
		case inventory.RoleCanonicalZsh:
			keep = append(keep, f)
		case inventory.RoleCanonicalBash:
			if *bash {
				keep = append(keep, f)
			}
		case inventory.RoleOrphan:
			if *orphans {
				keep = append(keep, f)
			}
		}
	}
	sort.SliceStable(keep, func(i, j int) bool {
		return dedupFileRank(keep[i]) < dedupFileRank(keep[j])
	})

	groups := dedup.Find(keep)
	if len(groups) == 0 {
		fmt.Fprintln(stdout, "no duplicates found")
		return 0
	}
	emitDedupText(stdout, groups)
	return 0
}

var zshLoginRank = map[string]int{
	".zshenv": 0, ".zprofile": 1, ".zshrc": 2, ".zlogin": 3, ".zlogout": 4,
}

func dedupFileRank(f inventory.File) int {
	base := f.Path
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	switch f.Role {
	case inventory.RoleCanonicalZsh:
		return zshLoginRank[base]
	case inventory.RoleCanonicalBash:
		return 100
	case inventory.RoleOrphan:
		return 200
	}
	return 999
}

func emitDedupText(w io.Writer, groups []dedup.Group) {
	currentKind := inventory.Kind("")
	for _, g := range groups {
		if g.Kind != currentKind {
			if currentKind != "" {
				fmt.Fprintln(w)
			}
			fmt.Fprintf(w, "## %s\n", g.Kind)
			currentKind = g.Kind
		}
		fmt.Fprintf(w, "  %s\n", g.Name)
		fmt.Fprintf(w, "    winner  %s:%d\n", g.Winner.File, g.Winner.Line)
		for _, l := range g.Losers {
			fmt.Fprintf(w, "    loser   %s:%d\n", l.File, l.Line)
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

	files := inventory.Discover()
	keep := make([]inventory.File, 0, len(files))
	for _, f := range files {
		switch f.Role {
		case inventory.RoleCanonicalZsh:
			keep = append(keep, f)
		case inventory.RoleCanonicalBash:
			if *bash {
				keep = append(keep, f)
			}
		case inventory.RoleOrphan:
			if *orphans {
				keep = append(keep, f)
			}
		}
	}

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
	fmt.Fprint(w, `envocabulary lost — config in orphan files that no canonical file defines

Usage:
  envocabulary lost [--bash]

Config from orphan files (.zshrc.backup, .zshrc.old, ...) not in any canonical file.

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

	files := inventory.Discover()
	keep := make([]inventory.File, 0, len(files))
	for _, f := range files {
		switch f.Role {
		case inventory.RoleCanonicalZsh:
			keep = append(keep, f)
		case inventory.RoleCanonicalBash:
			if *bash {
				keep = append(keep, f)
			}
		case inventory.RoleOrphan:
			keep = append(keep, f)
		}
	}

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

	files := inventory.Discover()
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

func die(stderr io.Writer, err error) int {
	fmt.Fprintln(stderr, "error:", err)
	return 1
}
