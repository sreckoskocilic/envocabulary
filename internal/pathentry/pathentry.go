package pathentry

import (
	"os"
	"regexp"
	"strings"

	"github.com/sreckoskocilic/envocabulary/internal/model"
)

var nestedTraceMarkerRe = regexp.MustCompile(`\+\S*:\d+>`)

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

type slot struct {
	dir    string
	writer *model.TraceEntry
}

func Attribute(varName, currentValue, initialValue string, trace []model.TraceEntry) VarBreakdown {
	dirs := splitPath(currentValue)
	if len(dirs) == 0 {
		return VarBreakdown{Name: varName}
	}

	slots := make([]slot, 0, len(dirs))
	for _, d := range splitPath(initialValue) {
		slots = append(slots, slot{dir: d})
	}

	for i := range trace {
		if trace[i].Name != varName {
			continue
		}
		val := extractValue(trace[i].Raw, varName)
		if val == "" || nestedTraceMarkerRe.MatchString(val) {
			continue
		}
		slots = carry(slots, splitPath(val), &trace[i])
	}

	entries := make([]Entry, 0, len(dirs))
	for _, s := range carry(slots, dirs, nil) {
		e := Entry{Dir: s.dir}
		if s.writer != nil {
			e.File = s.writer.File
			e.Line = s.writer.Line
			if len(s.writer.Chain) > 0 {
				e.Chain = make([]string, len(s.writer.Chain))
				copy(e.Chain, s.writer.Chain)
			}
		}
		entries = append(entries, e)
	}

	return VarBreakdown{Name: varName, Entries: entries}
}

func carry(prev []slot, cur []string, w *model.TraceEntry) []slot {
	prevDirs := make([]string, len(prev))
	for i := range prev {
		prevDirs[i] = prev[i].dir
	}
	match := align(prevDirs, cur)

	out := make([]slot, len(cur))
	for j, d := range cur {
		out[j] = slot{dir: d, writer: w}
		if m := match[j]; m >= 0 {
			out[j].writer = prev[m].writer
		}
	}
	return out
}

const alignBudget = 1 << 18

func align(a, b []string) []int {
	match := make([]int, len(b))
	for i := range match {
		match[i] = -1
	}
	if len(a) == 0 || len(b) == 0 {
		return match
	}
	if len(a)*len(b) > alignBudget {
		return alignGreedy(a, b, match)
	}
	return lcsBacktrack(lcsTable(a, b), a, b, match)
}

func lcsTable(a, b []string) []int32 {
	stride := len(b) + 1
	dp := make([]int32, (len(a)+1)*stride)
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			switch {
			case a[i] == b[j]:
				dp[i*stride+j] = dp[(i+1)*stride+j+1] + 1
			case dp[(i+1)*stride+j] >= dp[i*stride+j+1]:
				dp[i*stride+j] = dp[(i+1)*stride+j]
			default:
				dp[i*stride+j] = dp[i*stride+j+1]
			}
		}
	}
	return dp
}

func lcsBacktrack(dp []int32, a, b []string, match []int) []int {
	stride := len(b) + 1
	for i, j := 0, 0; i < len(a) && j < len(b); {
		switch {
		case a[i] == b[j]:
			match[j] = i
			i++
			j++
		case dp[(i+1)*stride+j] >= dp[i*stride+j+1]:
			i++
		default:
			j++
		}
	}
	return match
}

func alignGreedy(a, b []string, match []int) []int {
	positions := make(map[string][]int, len(a))
	for i, s := range a {
		positions[s] = append(positions[s], i)
	}
	last := -1
	for j, s := range b {
		for len(positions[s]) > 0 && positions[s][0] <= last {
			positions[s] = positions[s][1:]
		}
		if len(positions[s]) == 0 {
			continue
		}
		match[j] = positions[s][0]
		last = positions[s][0]
		positions[s] = positions[s][1:]
	}
	return match
}

func extractValue(raw, name string) string {
	target := name + "="
	search := raw
	offset := 0
	for {
		idx := strings.Index(search, target)
		if idx < 0 {
			return ""
		}
		abs := offset + idx
		if abs > 0 && isWordChar(raw[abs-1]) {
			search = search[idx+len(target):]
			offset = abs + len(target)
			continue
		}
		val := raw[abs+len(target):]
		if val != "" && (val[0] == '"' || val[0] == '\'') {
			if end := strings.IndexByte(val[1:], val[0]); end >= 0 {
				val = val[1 : 1+end]
			} else {
				val = val[1:]
			}
		}
		return val
	}
}

func isWordChar(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_'
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
