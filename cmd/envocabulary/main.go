package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"envocabulary/internal/attribute"
	"envocabulary/internal/capture"
	"envocabulary/internal/catalog"
	"envocabulary/internal/cleaner"
	"envocabulary/internal/dedup"
	"envocabulary/internal/explain"
	"envocabulary/internal/inventory"
	"envocabulary/internal/model"
)

func main() {
	args := os.Args[1:]
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		switch args[0] {
		case "scan":
			runScan(args[1:])
			return
		case "explain":
			runExplain(args[1:])
			return
		case "inventory":
			runInventory(args[1:])
			return
		case "clean":
			runClean(args[1:])
			return
		case "catalog":
			runCatalog(args[1:])
			return
		case "dedup":
			runDedup(args[1:])
			return
		case "help", "-h", "--help":
			usage(os.Stdout)
			return
		default:
			fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", args[0])
			usage(os.Stderr)
			os.Exit(2)
		}
	}
	runScan(args)
}

func usage(w *os.File) {
	fmt.Fprintln(w, "usage: envocabulary [scan] [--json] [--values]")
	fmt.Fprintln(w, "       envocabulary explain [--json] [--values] NAME")
	fmt.Fprintln(w, "       envocabulary inventory")
	fmt.Fprintln(w, "       envocabulary clean FILE  (writes cleaned output to stdout)")
	fmt.Fprintln(w, "       envocabulary catalog [--orphans] [--bash] [-n] [--dedup]")
	fmt.Fprintln(w, "       envocabulary dedup [--orphans] [--bash]")
}

func runScan(args []string) {
	fs := flag.NewFlagSet("scan", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "emit JSON instead of grouped text")
	showValues := fs.Bool("values", false, "include values in output (may expose secrets)")
	_ = fs.Parse(args)

	current, err := capture.CurrentEnv()
	if err != nil {
		die(err)
	}

	trace, err := capture.TracedStartup()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: trace unavailable, falling back to classification-only: %v\n", err)
		trace = nil
	}

	words := attribute.Attribute(current, trace)

	if *jsonOut {
		emitScanJSON(words, *showValues)
		return
	}
	emitScanText(words, *showValues)
}

func runExplain(args []string) {
	fs := flag.NewFlagSet("explain", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "emit JSON")
	showValues := fs.Bool("values", false, "include value and raw traced commands (may expose secrets)")
	_ = fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: envocabulary explain [--json] [--values] NAME")
		os.Exit(2)
	}
	name := fs.Arg(0)

	current, err := capture.CurrentEnv()
	if err != nil {
		die(err)
	}

	trace, err := capture.TracedStartup()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: trace unavailable: %v\n", err)
		trace = nil
	}

	result := explain.Explain(name, current, trace)

	if *jsonOut {
		if err := explain.EmitJSON(os.Stdout, result, *showValues); err != nil {
			die(err)
		}
		return
	}
	explain.EmitText(os.Stdout, result, *showValues)
}

func runCatalog(args []string) {
	fs := flag.NewFlagSet("catalog", flag.ExitOnError)
	orphans := fs.Bool("orphans", false, "include orphan/backup files")
	bash := fs.Bool("bash", false, "include bash config files")
	lineNums := fs.Bool("n", false, "prefix each line with its line number")
	dedupFlag := fs.Bool("dedup", false, "comment out lines overridden by a later writer")
	_ = fs.Parse(args)

	opts := catalog.Options{
		IncludeOrphans: *orphans,
		IncludeBash:    *bash,
		LineNumbers:    *lineNums,
		Dedup:          *dedupFlag,
	}
	if err := catalog.Write(os.Stdout, opts); err != nil {
		die(err)
	}
}

func runDedup(args []string) {
	fs := flag.NewFlagSet("dedup", flag.ExitOnError)
	orphans := fs.Bool("orphans", false, "include orphan/backup files")
	bash := fs.Bool("bash", false, "include bash config files")
	_ = fs.Parse(args)

	files := inventory.Discover()
	var keep []inventory.File
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
		fmt.Println("no duplicates found")
		return
	}
	emitDedupText(groups)
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

func emitDedupText(groups []dedup.Group) {
	currentKind := inventory.Kind("")
	for _, g := range groups {
		if g.Kind != currentKind {
			if currentKind != "" {
				fmt.Println()
			}
			fmt.Printf("## %s\n", g.Kind)
			currentKind = g.Kind
		}
		fmt.Printf("  %s\n", g.Name)
		fmt.Printf("    winner  %s:%d\n", g.Winner.File, g.Winner.Line)
		for _, l := range g.Losers {
			fmt.Printf("    loser   %s:%d\n", l.File, l.Line)
		}
	}
}

func runClean(args []string) {
	fs := flag.NewFlagSet("clean", flag.ExitOnError)
	_ = fs.Parse(args)
	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: envocabulary clean FILE")
		os.Exit(2)
	}
	path := fs.Arg(0)
	f, err := os.Open(path)
	if err != nil {
		die(err)
	}
	defer f.Close()
	cleaned, stats, err := cleaner.Clean(f)
	if err != nil {
		die(err)
	}
	fmt.Print(cleaned)
	fmt.Fprintf(os.Stderr, "# %d kept, %d stripped — review, then redirect yourself: envocabulary clean %s > %s.cleaned\n", stats.Kept, stats.Stripped, path, path)
}

func runInventory(args []string) {
	fs := flag.NewFlagSet("inventory", flag.ExitOnError)
	_ = fs.Parse(args)

	files := inventory.Discover()
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "no shell config files found")
		return
	}
	emitInventoryText(files)
}

func emitInventoryText(files []inventory.File) {
	for i, f := range files {
		if i > 0 {
			fmt.Println()
		}
		suffix := ""
		if f.Role == inventory.RoleOrphan {
			suffix = "  (orphan)"
		}
		fmt.Printf("## %s%s\n", f.Path, suffix)
		if f.Err != nil {
			fmt.Printf("  error: %v\n", f.Err)
			continue
		}
		groups := groupItems(f.Items)
		printGroup("exports", groups[inventory.KindExport])
		printGroup("assigns", groups[inventory.KindAssign])
		printGroup("aliases", groups[inventory.KindAlias])
		printGroup("functions", groups[inventory.KindFunction])
		printGroup("sources", groups[inventory.KindSource])
	}
}

func groupItems(items []inventory.Item) map[inventory.Kind][]inventory.Item {
	g := map[inventory.Kind][]inventory.Item{}
	for _, it := range items {
		g[it.Kind] = append(g[it.Kind], it)
	}
	return g
}

func printGroup(label string, items []inventory.Item) {
	if len(items) == 0 {
		return
	}
	names := make([]string, 0, len(items))
	for _, it := range items {
		names = append(names, it.Name)
	}
	fmt.Printf("  %-10s %3d  %s\n", label, len(items), strings.Join(names, ", "))
}

func emitScanJSON(words []model.EnWord, showValues bool) {
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
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(list); err != nil {
		die(err)
	}
}

func emitScanText(words []model.EnWord, showValues bool) {
	current := model.Origin("")
	for _, w := range words {
		if w.Origin != current {
			if current != "" {
				fmt.Println()
			}
			fmt.Printf("## %s\n", w.Origin)
			current = w.Origin
		}
		line := fmt.Sprintf("%-32s", w.Name)
		if w.Source != "" {
			line += "  " + w.Source
		}
		if w.Notes != "" {
			line += "  (" + w.Notes + ")"
		}
		if showValues {
			line += "  = " + truncate(w.Value, 60)
		}
		fmt.Println(line)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
