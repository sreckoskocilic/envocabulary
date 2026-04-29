package cleaner

import (
	"bufio"
	"io"
	"regexp"
	"strings"
)

type Stats struct {
	Kept     int
	Stripped int
}

type Decision struct {
	LineNum int
	Content string
	Kept    bool
}

var (
	commentInnerRe = regexp.MustCompile(`^\s*#\s?(.*)$`)
	decorationRe   = regexp.MustCompile(`^[-=#*~_+/\\]+$`)

	commentedExportRe    = regexp.MustCompile(`^export\s+[A-Za-z_]`)
	commentedAliasRe     = regexp.MustCompile(`^alias\s+`)
	commentedFuncKwRe    = regexp.MustCompile(`^function\s+[A-Za-z_]`)
	commentedFuncParenRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*\s*\(\s*\)`)
	commentedSourceRe    = regexp.MustCompile(`^(?:source|\.)\s+\S`)
	commentedAssignRe    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*\s*=`)
	commentedPluginsRe   = regexp.MustCompile(`^plugins\s*=\s*\(`)
)

type lineInfo struct {
	raw       string
	isComment bool
	isShebang bool
	inner     string
}

func Process(r io.Reader) ([]Decision, Stats, error) {
	var lines []string
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if err := sc.Err(); err != nil {
		return nil, Stats{}, err
	}

	info := make([]lineInfo, len(lines))
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		info[i].raw = ln
		if strings.HasPrefix(t, "#!") && i == 0 {
			info[i].isShebang = true
			continue
		}
		if strings.HasPrefix(t, "#") {
			info[i].isComment = true
			if m := commentInnerRe.FindStringSubmatch(ln); m != nil {
				info[i].inner = m[1]
			}
		}
	}

	keepMask := make([]bool, len(lines))
	for i := 0; i < len(lines); {
		if !info[i].isComment {
			keepMask[i] = true
			i++
			continue
		}
		j := i
		for j < len(lines) && info[j].isComment {
			j++
		}
		keep := shouldKeepBlock(info[i:j])
		for k := i; k < j; k++ {
			keepMask[k] = keep
		}
		i = j
	}

	var stats Stats
	decisions := make([]Decision, len(lines))
	for i, ln := range lines {
		decisions[i] = Decision{LineNum: i + 1, Content: ln, Kept: keepMask[i]}
		if keepMask[i] {
			stats.Kept++
		} else {
			stats.Stripped++
		}
	}
	return decisions, stats, nil
}

func Clean(r io.Reader) (string, Stats, error) {
	decisions, stats, err := Process(r)
	if err != nil {
		return "", stats, err
	}
	var out []string
	for _, d := range decisions {
		if d.Kept {
			out = append(out, d.Content)
		}
	}
	if len(out) == 0 {
		return "", stats, nil
	}
	return strings.Join(out, "\n") + "\n", stats, nil
}

func shouldKeepBlock(block []lineInfo) bool {
	if len(block) == 1 {
		return !isCommentedCode(block[0].inner)
	}
	sawLabel := false
	for _, li := range block {
		s := strings.TrimSpace(li.inner)
		if s == "" {
			continue
		}
		if isCommentedCode(s) {
			return false
		}
		if isDecoration(s) {
			continue
		}
		if looksLikeLabel(s) {
			sawLabel = true
			continue
		}
		return false
	}
	return sawLabel
}

func isDecoration(s string) bool {
	return decorationRe.MatchString(s)
}

func looksLikeLabel(s string) bool {
	if len(s) > 50 {
		return false
	}
	if strings.HasSuffix(s, ".") {
		return false
	}
	return len(strings.Fields(s)) <= 5
}

func isCommentedCode(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	return commentedExportRe.MatchString(s) ||
		commentedAliasRe.MatchString(s) ||
		commentedFuncKwRe.MatchString(s) ||
		commentedFuncParenRe.MatchString(s) ||
		commentedSourceRe.MatchString(s) ||
		commentedPluginsRe.MatchString(s) ||
		commentedAssignRe.MatchString(s)
}
