// spawns real shells

package main

import (
	"flag"
	"fmt"
	"io"

	"github.com/sreckoskocilic/envocabulary/internal/attribute"
	"github.com/sreckoskocilic/envocabulary/internal/capture"
	"github.com/sreckoskocilic/envocabulary/internal/explain"
)

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
	explain.EmitText(stdout, result, *showValues)
	return 0
}
