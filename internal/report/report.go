package report

import (
	"fmt"
	"strings"
	"time"

	"github.com/sreckoskocilic/envocabulary/internal/dangling"
	"github.com/sreckoskocilic/envocabulary/internal/dedup"
	"github.com/sreckoskocilic/envocabulary/internal/inventory"
	"github.com/sreckoskocilic/envocabulary/internal/lost"
)

type Entry struct {
	Definition  string
	Location    string
	Reference   string
	ActiveValue string
}

type OrphanFile struct {
	Path    string
	Summary string
}

type Report struct {
	Generated    time.Time
	FilesScanned int
	Safe         []Entry
	Review       []Entry
	Dangling     []Entry
	Orphans      []OrphanFile
}

func Build(files []inventory.File) Report {
	r := Report{
		Generated:    time.Now(),
		FilesScanned: len(files),
	}

	groups := dedup.Find(files)
	for i := range groups {
		g := &groups[i]
		for j := range g.Losers {
			l := &g.Losers[j]
			def := fmt.Sprintf("%s %s", l.Kind, formatDef(l))
			loc := shortPath(l.File, l.Line)
			ref := shortPath(g.Winner.File, g.Winner.Line)

			if l.Value == g.Winner.Value {
				r.Safe = append(r.Safe, Entry{
					Definition: def,
					Location:   loc,
					Reference:  ref,
				})
			} else {
				r.Review = append(r.Review, Entry{
					Definition:  def,
					Location:    loc,
					Reference:   ref,
					ActiveValue: g.Winner.Value,
				})
			}
		}
	}

	findings := dangling.Find(files)
	for _, f := range findings {
		def := fmt.Sprintf("%s %s", f.Kind, formatDanglingDef(f))
		r.Dangling = append(r.Dangling, Entry{
			Definition: def,
			Location:   shortPath(f.File, f.Line),
			Reference:  f.Value,
		})
	}

	lostFindings := lost.Find(files)
	orphanMap := map[string][]lost.Finding{}
	var orphanOrder []string
	for _, f := range lostFindings {
		if _, seen := orphanMap[f.File]; !seen {
			orphanOrder = append(orphanOrder, f.File)
		}
		orphanMap[f.File] = append(orphanMap[f.File], f)
	}
	for _, path := range orphanOrder {
		r.Orphans = append(r.Orphans, OrphanFile{
			Path:    tildePath(path),
			Summary: summarizeFindings(orphanMap[path]),
		})
	}

	return r
}

func formatDef(o *dedup.Occurrence) string {
	if o.Kind == inventory.KindFunction {
		return o.Name
	}
	if o.Value == "" {
		return o.Name
	}
	return o.Name + "=" + o.Value
}

func formatDanglingDef(f dangling.Finding) string {
	if f.Kind == inventory.KindSource {
		return f.Value
	}
	if f.Value == "" {
		return f.Name
	}
	return f.Name + "=" + f.Value
}

func shortPath(file string, line int) string {
	return fmt.Sprintf("%s:%d", tildePath(file), line)
}

func summarizeFindings(findings []lost.Finding) string {
	counts := map[inventory.Kind]int{}
	for _, f := range findings {
		counts[f.Kind]++
	}
	parts := make([]string, 0, len(counts))
	order := []inventory.Kind{
		inventory.KindExport, inventory.KindAssign,
		inventory.KindAlias, inventory.KindFunction, inventory.KindSource,
	}
	for _, k := range order {
		n := counts[k]
		if n == 0 {
			continue
		}
		label := string(k) + "s"
		if n == 1 {
			label = string(k)
		}
		parts = append(parts, fmt.Sprintf("%d %s", n, label))
	}
	return strings.Join(parts, ", ")
}
