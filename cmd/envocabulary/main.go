package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"envocabulary/internal/attribute"
	"envocabulary/internal/capture"
	"envocabulary/internal/explain"
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
