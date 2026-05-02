package attribute

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/sreckoskocilic/envocabulary/internal/buckets"
	"github.com/sreckoskocilic/envocabulary/internal/model"
)

func Attribute(current map[string]string, trace []model.TraceEntry) []model.EnWord {
	last := lastWriters(trace)

	out := make([]model.EnWord, 0, len(current))
	for name, value := range current {
		w := model.EnWord{Name: name, Value: value}

		switch {
		case model.IsDeferredListVar(name):
			w.Origin = model.OriginDeferred
			w.Source = "multi-source; envocabulary path (TODO)"

		case model.IsDirenvVar(name):
			w.Origin = model.OriginDirenv

		default:
			if t, ok := last[name]; ok {
				w.Origin = model.OriginShellFile
				w.Source = fmt.Sprintf("%s:%d", t.File, t.Line)
			} else {
				origin, note := buckets.Classify(name, value)
				w.Origin = origin
				w.Notes = note
			}
		}

		out = append(out, w)
	}

	slices.SortFunc(out, func(a, b model.EnWord) int {
		if c := cmp.Compare(originRank(a.Origin), originRank(b.Origin)); c != 0 {
			return c
		}
		return cmp.Compare(a.Name, b.Name)
	})
	return out
}

func lastWriters(trace []model.TraceEntry) map[string]model.TraceEntry {
	m := make(map[string]model.TraceEntry, len(trace))
	for _, e := range trace {
		m[e.Name] = e
	}
	return m
}

func originRank(o model.Origin) int {
	switch o {
	case model.OriginShellFile:
		return 0
	case model.OriginDirenv:
		return 1
	case model.OriginLaunchd:
		return 2
	case model.OriginSystem:
		return 3
	case model.OriginTerminal:
		return 4
	case model.OriginSSH:
		return 5
	case model.OriginDeferred:
		return 6
	case model.OriginUnknown:
		return 7
	default:
		return 99
	}
}
