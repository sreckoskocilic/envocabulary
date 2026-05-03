package pathentry

import (
	"os"
	"strings"

	"github.com/sreckoskocilic/envocabulary/internal/model"
)

type Entry struct {
	Dir    string   `json:"dir"`
	File   string   `json:"file,omitempty"`
	Line   int      `json:"line,omitempty"`
	Chain  []string `json:"chain,omitempty"`
	Exists *bool    `json:"exists,omitempty"`
}

var statDir = os.Stat

func CheckExists(entries []Entry) {
	for i := range entries {
		_, err := statDir(entries[i].Dir)
		b := err == nil
		entries[i].Exists = &b
	}
}

type VarBreakdown struct {
	Name    string  `json:"name"`
	Entries []Entry `json:"entries"`
}

func Attribute(varName, currentValue string, trace []model.TraceEntry) VarBreakdown {
	dirs := splitPath(currentValue)
	if len(dirs) == 0 {
		return VarBreakdown{Name: varName}
	}

	var writers []model.TraceEntry
	for _, e := range trace {
		if e.Name == varName {
			writers = append(writers, e)
		}
	}

	provenance := make(map[string]model.TraceEntry)
	var prev map[string]bool
	for _, w := range writers {
		val := extractValue(w.Raw, varName)
		cur := toSet(splitPath(val))
		for d := range cur {
			if !prev[d] {
				provenance[d] = w
			}
		}
		prev = cur
	}

	entries := make([]Entry, 0, len(dirs))
	for _, d := range dirs {
		e := Entry{Dir: d}
		if w, ok := provenance[d]; ok {
			e.File = w.File
			e.Line = w.Line
			if len(w.Chain) > 0 {
				e.Chain = make([]string, len(w.Chain))
				copy(e.Chain, w.Chain)
			}
		}
		entries = append(entries, e)
	}

	return VarBreakdown{Name: varName, Entries: entries}
}

func extractValue(raw, name string) string {
	target := name + "="
	idx := strings.Index(raw, target)
	if idx < 0 {
		return ""
	}
	val := raw[idx+len(target):]
	if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
		val = val[1 : len(val)-1]
	}
	return val
}

func splitPath(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ":")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func toSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}
