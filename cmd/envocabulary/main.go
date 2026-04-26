package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/sreckoskocilic/envocabulary/internal/attribute"
	"github.com/sreckoskocilic/envocabulary/internal/capture"
	"github.com/sreckoskocilic/envocabulary/internal/catalog"
	"github.com/sreckoskocilic/envocabulary/internal/cleaner"
	"github.com/sreckoskocilic/envocabulary/internal/color"
	"github.com/sreckoskocilic/envocabulary/internal/dedup"
	"github.com/sreckoskocilic/envocabulary/internal/explain"
	"github.com/sreckoskocilic/envocabulary/internal/inventory"
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
	fmt.Fprintln(w, "envocabulary — shell env-var forensics & static config audit (read-only)")
	fmt.Fprintln(w, "Supported shells: zsh (live-env + static-file), bash (static-file).")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Live-env commands (introspect the running shell):")
	fmt.Fprintln(w, "  scan [--json] [--values]")
	fmt.Fprintln(w, "      Group every variable in the current env by origin (shell-file,")
	fmt.Fprintln(w, "      direnv, launchd, terminal, ssh, system, ...). Default command.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  explain [--json] [--values] NAME")
	fmt.Fprintln(w, "      Show full attribution for one variable: origin, primary writer,")
	fmt.Fprintln(w, "      and every other writer in startup order.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Static-file commands (audit shell config without running it):")
	fmt.Fprintln(w, "  inventory")
	fmt.Fprintln(w, "      List counts and names per shell config file.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  catalog [--orphans] [--bash] [-n] [--dedup]")
	fmt.Fprintln(w, "      Concatenate all canonical zsh config files in startup order to stdout.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  dedup [--orphans] [--bash]")
	fmt.Fprintln(w, "      Cross-file duplicate report for exports/assigns/aliases/functions.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  clean FILE")
	fmt.Fprintln(w, "      Strip default/template comments from FILE to stdout (never mutates).")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Run with no arguments to default to `scan`.")
	fmt.Fprintln(w, "Run `envocabulary <command> -h` for command-specific help with flags & examples.")
	fmt.Fprintln(w, "Run `envocabulary --version` to print the version.")
}

func helpScan(w io.Writer) {
	fmt.Fprintln(w, "envocabulary scan — group every variable in the current env by origin")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  envocabulary scan [--json] [--values]")
	fmt.Fprintln(w, "  envocabulary [--json] [--values]                (scan is the default command)")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Description:")
	fmt.Fprintln(w, "  For each variable in the current shell env, attribute its origin to one of:")
	fmt.Fprintln(w, "  shell-file (file:line), direnv, launchd, terminal, ssh, system, deferred-list-var,")
	fmt.Fprintln(w, "  or unknown. Output is grouped by origin.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  --json     emit JSON instead of grouped text")
	fmt.Fprintln(w, "  --values   include values in output (may expose secrets)")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  envocabulary scan")
	fmt.Fprintln(w, "  envocabulary scan --json | jq '.[] | select(.origin==\"shell-file\")'")
	fmt.Fprintln(w, "  envocabulary scan --values | grep -i token")
}

func helpExplain(w io.Writer) {
	fmt.Fprintln(w, "envocabulary explain — show full attribution for one variable")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  envocabulary explain [--json] [--values] NAME")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Description:")
	fmt.Fprintln(w, "  Lists every writer (file:line) for NAME in startup order, marks the winner")
	fmt.Fprintln(w, "  (the assignment that set the final value), and reports the origin bucket.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Arguments:")
	fmt.Fprintln(w, "  NAME       the env variable name as it appears in `env` (e.g. JAVA_HOME, EDITOR)")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  --json     emit JSON")
	fmt.Fprintln(w, "  --values   include value and raw assignment lines (may expose secrets)")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  envocabulary explain JAVA_HOME")
	fmt.Fprintln(w, "  envocabulary explain --values EDITOR")
	fmt.Fprintln(w, "  envocabulary explain --json PATH | jq")
}

func helpInventory(w io.Writer) {
	fmt.Fprintln(w, "envocabulary inventory — list counts and names per shell config file")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  envocabulary inventory")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Description:")
	fmt.Fprintln(w, "  Walks your shell config files: canonical zsh (.zshenv, .zprofile, .zshrc,")
	fmt.Fprintln(w, "  .zlogin, .zlogout in $ZDOTDIR or $HOME), canonical bash (.bashrc, .bash_profile,")
	fmt.Fprintln(w, "  .profile), and orphan/backup variants. Reports counts and names per file,")
	fmt.Fprintln(w, "  grouped by kind (exports, assigns, aliases, functions, sources).")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  envocabulary inventory")
	fmt.Fprintln(w, "  envocabulary inventory | less")
}

func helpCatalog(w io.Writer) {
	fmt.Fprintln(w, "envocabulary catalog — concatenate shell config files in startup order")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  envocabulary catalog [--orphans] [--bash] [-n] [--dedup] [--color=MODE]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Description:")
	fmt.Fprintln(w, "  Emits all canonical zsh config files concatenated to stdout, separated by")
	fmt.Fprintln(w, "  banner headers, in zsh login order. Reading top-to-bottom mirrors execution.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  --orphans       also include backup/variant files (.zshrc.backup, .bashrc.old, ...)")
	fmt.Fprintln(w, "  --bash          also include .bashrc / .bash_profile / .profile")
	fmt.Fprintln(w, "  -n              prefix each line with its source line number")
	fmt.Fprintln(w, "  --dedup         comment out lines overridden by a later writer,")
	fmt.Fprintln(w, "                  annotated as `# [overridden by file:line] ...`")
	fmt.Fprintln(w, "  --color=MODE    color override-annotated lines: auto|always|never (default auto)")
	fmt.Fprintln(w, "                  auto = color only when stdout is a terminal; honors NO_COLOR")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  envocabulary catalog | less")
	fmt.Fprintln(w, "  envocabulary catalog -n")
	fmt.Fprintln(w, "  envocabulary catalog --bash --orphans")
	fmt.Fprintln(w, "  envocabulary catalog --dedup                       # red highlight on dead lines")
	fmt.Fprintln(w, "  envocabulary catalog --dedup --color=never | grep '# \\[overridden'")
}

func helpDedup(w io.Writer) {
	fmt.Fprintln(w, "envocabulary dedup — find duplicate exports/aliases across config files")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  envocabulary dedup [--orphans] [--bash]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Description:")
	fmt.Fprintln(w, "  Cross-file duplicate report: groups duplicate exports/assigns/aliases/functions")
	fmt.Fprintln(w, "  by name, marks the winning writer (last in execution order) and shadowed losers.")
	fmt.Fprintln(w, "  PATH-like accumulating vars (PATH, MANPATH, FPATH, INFOPATH, CDPATH, DYLD_*) and")
	fmt.Fprintln(w, "  `source` lines are deliberately excluded — they extend rather than override.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  --orphans  include orphan/backup files in the search")
	fmt.Fprintln(w, "  --bash     include bash config files")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  envocabulary dedup")
	fmt.Fprintln(w, "  envocabulary dedup --bash --orphans")
}

func helpClean(w io.Writer) {
	fmt.Fprintln(w, "envocabulary clean — strip template/boilerplate comments from a config file")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  envocabulary clean [--full] [--color=MODE] FILE")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Description:")
	fmt.Fprintln(w, "  Default mode is dry-run: prints a preview of the lines that would be stripped,")
	fmt.Fprintln(w, "  one per output line, prefixed with `- LINENO  ...`. The input FILE is never")
	fmt.Fprintln(w, "  modified by either mode.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  Pass --full to emit the full cleaned file content to stdout instead. Then")
	fmt.Fprintln(w, "  redirect to a new file and replace at your own discretion.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  Heuristic errs on keeping — if a comment doesn't clearly match a strip rule,")
	fmt.Fprintln(w, "  it stays. Real code is never removed. Works on any shell config file.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Arguments:")
	fmt.Fprintln(w, "  FILE         path to the shell config file (e.g. ~/.zshrc, ~/.bashrc)")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  --full       emit full cleaned content (instead of dry-run preview)")
	fmt.Fprintln(w, "  --color=MODE color stripped lines in dry-run output: auto|always|never (default auto)")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  envocabulary clean ~/.zshrc                            # preview what would be stripped")
	fmt.Fprintln(w, "  envocabulary clean ~/.bashrc                           # bash works the same")
	fmt.Fprintln(w, "  envocabulary clean --full ~/.zshrc > ~/.zshrc.cleaned  # save cleaned copy")
	fmt.Fprintln(w, "  diff ~/.zshrc ~/.zshrc.cleaned                         # review before replacing")
}

func runScan(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { helpScan(stdout) }
	jsonOut := fs.Bool("json", false, "emit JSON instead of grouped text")
	showValues := fs.Bool("values", false, "include values in output (may expose secrets)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	current, err := capture.CurrentEnv()
	if err != nil {
		return die(stderr, err)
	}

	trace, err := capture.TracedStartup()
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
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if fs.NArg() < 1 {
		helpExplain(stderr)
		return 2
	}
	name := fs.Arg(0)

	current, err := capture.CurrentEnv()
	if err != nil {
		return die(stderr, err)
	}

	trace, err := capture.TracedStartup()
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
	explain.EmitText(stdout, result, *showValues)
	return 0
}

func runCatalog(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("catalog", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { helpCatalog(stdout) }
	orphans := fs.Bool("orphans", false, "include orphan/backup files")
	bash := fs.Bool("bash", false, "include bash config files")
	lineNums := fs.Bool("n", false, "prefix each line with its line number")
	dedupFlag := fs.Bool("dedup", false, "comment out lines overridden by a later writer")
	colorFlag := fs.String("color", "auto", "color override-annotated lines (auto|always|never)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	mode, err := color.ParseMode(*colorFlag)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 2
	}

	opts := catalog.Options{
		IncludeOrphans: *orphans,
		IncludeBash:    *bash,
		LineNumbers:    *lineNums,
		Dedup:          *dedupFlag,
		Color:          mode,
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

func runClean(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("clean", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { helpClean(stdout) }
	full := fs.Bool("full", false, "emit full cleaned content (default is dry-run preview of stripped lines)")
	colorFlag := fs.String("color", "auto", "color stripped lines in dry-run output (auto|always|never)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() < 1 {
		helpClean(stderr)
		return 2
	}

	mode, err := color.ParseMode(*colorFlag)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
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
	colorOn := mode.Enabled(stdout)
	for _, d := range decisions {
		if d.Kept {
			continue
		}
		line := fmt.Sprintf("- %5d  %s", d.LineNum, d.Content)
		fmt.Fprintln(stdout, color.Wrap(line, color.LightRed, colorOn))
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
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func die(stderr io.Writer, err error) int {
	fmt.Fprintln(stderr, "error:", err)
	return 1
}
