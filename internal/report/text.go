package report

import (
	"fmt"
	"io"
	"text/tabwriter"
)

func WriteText(w io.Writer, r Report) {
	fmt.Fprintf(w, "envocabulary audit report\n")
	fmt.Fprintf(w, "%s · %d files scanned\n", r.Generated.Format("2006-01-02 15:04"), r.FilesScanned)

	writeTextSection(w, "SAFE TO DELETE", len(r.Safe), []string{"DEFINITION", "LOCATION", "SUPERSEDED BY"}, func(tw *tabwriter.Writer) {
		for _, e := range r.Safe {
			fmt.Fprintf(tw, "%s\t%s\t%s\n", e.Definition, e.Location, e.Reference)
		}
	})

	writeTextSection(w, "REVIEW", len(r.Review), []string{"DEFINITION", "LOCATION", "SUPERSEDED BY"}, func(tw *tabwriter.Writer) {
		for _, e := range r.Review {
			ref := e.Reference
			if e.ActiveValue != "" {
				ref += " → " + e.ActiveValue
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\n", e.Definition, e.Location, ref)
		}
	})

	writeTextSection(w, "DANGLING", len(r.Dangling), []string{"DEFINITION", "LOCATION", "MISSING TARGET"}, func(tw *tabwriter.Writer) {
		for _, e := range r.Dangling {
			fmt.Fprintf(tw, "%s\t%s\t%s\n", e.Definition, e.Location, e.Reference)
		}
	})

	writeTextSection(w, "ORPHANED FILES", len(r.Orphans), []string{"FILE", "CONTENTS"}, func(tw *tabwriter.Writer) {
		for _, o := range r.Orphans {
			fmt.Fprintf(tw, "%s\t%s\n", o.Path, o.Summary)
		}
	})
}

func writeTextSection(w io.Writer, title string, count int, headers []string, writeRows func(*tabwriter.Writer)) {
	fmt.Fprintf(w, "\n%s (%d)\n", title, count)
	if count == 0 {
		fmt.Fprintln(w, "  (none)")
		return
	}
	fmt.Fprintln(w, "─────────────────────────────────────────────────────────────────────")
	tw := tabwriter.NewWriter(w, 0, 0, 4, ' ', 0)
	for i, h := range headers {
		if i > 0 {
			fmt.Fprint(tw, "\t")
		}
		fmt.Fprint(tw, h)
	}
	fmt.Fprintln(tw)
	writeRows(tw)
	tw.Flush()
}
