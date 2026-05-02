package dedup

import (
	"cmp"
	"slices"
	"strconv"

	"github.com/sreckoskocilic/envocabulary/internal/inventory"
	"github.com/sreckoskocilic/envocabulary/internal/model"
)

type Occurrence struct {
	File  string
	Kind  inventory.Kind
	Name  string
	Line  int
	Value string
}

type Group struct {
	Kind   inventory.Kind
	Name   string
	Winner Occurrence
	Losers []Occurrence
}

var dedupKinds = map[inventory.Kind]bool{
	inventory.KindExport:   true,
	inventory.KindAssign:   true,
	inventory.KindAlias:    true,
	inventory.KindFunction: true,
}

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
			if (it.Kind == inventory.KindExport || it.Kind == inventory.KindAssign) && model.IsDeferredListVar(it.Name) {
				continue
			}
			entries = append(entries, entry{
				occ:  Occurrence{File: f.Path, Kind: it.Kind, Name: it.Name, Line: it.Line, Value: it.Value},
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
		slices.SortFunc(b, func(a, b entry) int { return cmp.Compare(a.rank, b.rank) })
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

	slices.SortFunc(groups, func(a, b Group) int {
		if c := cmp.Compare(a.Kind, b.Kind); c != 0 {
			return c
		}
		return cmp.Compare(a.Name, b.Name)
	})
	return groups
}

func LoserSet(groups []Group) map[string]Occurrence {
	out := map[string]Occurrence{}
	for i := range groups {
		for j := range groups[i].Losers {
			out[Key(groups[i].Losers[j].File, groups[i].Losers[j].Line)] = groups[i].Winner
		}
	}
	return out
}

func Key(file string, line int) string {
	return file + "\x00" + strconv.Itoa(line)
}
