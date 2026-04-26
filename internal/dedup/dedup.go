package dedup

import (
	"sort"
	"strconv"
	"strings"

	"github.com/sreckoskocilic/envocabulary/internal/inventory"
)

type Occurrence struct {
	File string
	Kind inventory.Kind
	Name string
	Line int
}

type Group struct {
	Kind   inventory.Kind
	Name   string
	Winner Occurrence
	Losers []Occurrence
}

// Kinds eligible for dedup. Sources are excluded — re-sourcing the same file
// from multiple places is usually intentional, not drift.
var dedupKinds = map[inventory.Kind]bool{
	inventory.KindExport:   true,
	inventory.KindAssign:   true,
	inventory.KindAlias:    true,
	inventory.KindFunction: true,
}

// Colon-accumulated variables: multiple export lines extend them rather than
// overwrite. Flagging them as duplicates lies (see CLAUDE.md "deferred-list-var").
func isDeferredListVar(name string) bool {
	switch name {
	case "PATH", "MANPATH", "FPATH", "INFOPATH", "CDPATH":
		return true
	}
	return strings.HasPrefix(name, "DYLD_")
}

// Find groups duplicate items across files. The input slice's order is treated
// as the execution order — the last occurrence in that order is the winner.
func Find(files []inventory.File) []Group {
	type entry struct {
		occ  Occurrence
		rank int
	}
	var entries []entry
	rank := 0
	for _, f := range files {
		for _, it := range f.Items {
			if !dedupKinds[it.Kind] {
				continue
			}
			if (it.Kind == inventory.KindExport || it.Kind == inventory.KindAssign) && isDeferredListVar(it.Name) {
				continue
			}
			entries = append(entries, entry{
				occ:  Occurrence{File: f.Path, Kind: it.Kind, Name: it.Name, Line: it.Line},
				rank: rank,
			})
			rank++
		}
	}

	buckets := map[string][]entry{}
	for _, e := range entries {
		key := string(e.occ.Kind) + "\x00" + e.occ.Name
		buckets[key] = append(buckets[key], e)
	}

	groups := make([]Group, 0, len(buckets))
	for _, b := range buckets {
		if len(b) < 2 {
			continue
		}
		sort.Slice(b, func(i, j int) bool { return b[i].rank < b[j].rank })
		winner := b[len(b)-1].occ
		losers := make([]Occurrence, 0, len(b)-1)
		for _, e := range b[:len(b)-1] {
			losers = append(losers, e.occ)
		}
		groups = append(groups, Group{
			Kind:   winner.Kind,
			Name:   winner.Name,
			Winner: winner,
			Losers: losers,
		})
	}

	sort.Slice(groups, func(i, j int) bool {
		if groups[i].Kind != groups[j].Kind {
			return groups[i].Kind < groups[j].Kind
		}
		return groups[i].Name < groups[j].Name
	})
	return groups
}

// LoserSet returns a map keyed by "file\x00line" for quick lookup when
// annotating catalog output.
func LoserSet(groups []Group) map[string]Occurrence {
	out := map[string]Occurrence{}
	for _, g := range groups {
		for _, l := range g.Losers {
			out[Key(l.File, l.Line)] = g.Winner
		}
	}
	return out
}

func Key(file string, line int) string {
	return file + "\x00" + strconv.Itoa(line)
}
