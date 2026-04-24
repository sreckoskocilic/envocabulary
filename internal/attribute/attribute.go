package attribute

import (
	"fmt"
	"sort"

	"envocabulary/internal/buckets"
	"envocabulary/internal/model"
)

var deferredListVars = map[string]bool{
	"PATH":                       true,
	"MANPATH":                    true,
	"INFOPATH":                   true,
	"FPATH":                      true,
	"CDPATH":                     true,
	"DYLD_LIBRARY_PATH":          true,
	"DYLD_FALLBACK_LIBRARY_PATH": true,
	"DYLD_FRAMEWORK_PATH":        true,
}

var direnvVars = map[string]bool{
	"DIRENV_DIR":     true,
	"DIRENV_FILE":    true,
	"DIRENV_DIFF":    true,
	"DIRENV_WATCHES": true,
}

func Attribute(current map[string]string, trace map[string]model.TraceEntry) []model.EnWord {
	out := make([]model.EnWord, 0, len(current))
	for name, value := range current {
		w := model.EnWord{Name: name, Value: value}

		switch {
		case deferredListVars[name]:
			w.Origin = model.OriginDeferred
			w.Source = "multi-source; `envocabulary path` (TODO)"

		case direnvVars[name]:
			w.Origin = model.OriginDirenv

		default:
			if t, ok := trace[name]; ok {
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

	sort.Slice(out, func(i, j int) bool {
		if out[i].Origin != out[j].Origin {
			return originRank(out[i].Origin) < originRank(out[j].Origin)
		}
		return out[i].Name < out[j].Name
	})
	return out
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
