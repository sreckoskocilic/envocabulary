// Package boundary: runScan and runExplain depend on capture.CurrentEnv() and
// capture.TracedStartup(), both of which spawn external subprocesses. Their happy
// paths can't be exercised in unit tests without a real shell environment + trace
// — so this file is excluded from coverage reporting via .codecov.yml.
//
// All testable CLI logic (run dispatch, flag parsing, helpX, emitX) lives in
// main.go and IS fully tested.
//
// Convention: any new run* function that depends on subprocess-derived input
// belongs here, not in main.go.

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
