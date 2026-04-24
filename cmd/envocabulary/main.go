package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"envocabulary/internal/attribute"
	"envocabulary/internal/capture"
	"envocabulary/internal/model"
)

func main() {
	jsonOut := flag.Bool("json", false, "emit JSON instead of grouped text")
	showValues := flag.Bool("values", false, "include values in output (may contain secrets)")
	flag.Parse()

	current, err := capture.CurrentEnv()
	if err != nil {
		die(err)
	}

	trace, err := capture.TracedStartup()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: trace unavailable, falling back to classification-only: %v\n", err)
		trace = map[string]model.TraceEntry{}
	}

	words := attribute.Attribute(current, trace)

	if *jsonOut {
		emitJSON(words, *showValues)
		return
	}
	emitText(words, *showValues)
}

func emitJSON(words []model.EnWord, showValues bool) {
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

func emitText(words []model.EnWord, showValues bool) {
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
