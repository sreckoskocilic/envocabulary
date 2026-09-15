package cleaner

import (
	"bufio"
	"errors"
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

	commentedExportRe    = regexp.MustCompile(`^export\s+[A-Za-z_][A-Za-z0-9_]*=`)
	commentedAliasRe     = regexp.MustCompile(`^alias\s+(?:-[A-Za-z]+\s+)*[^\s=]+=`)
	commentedFuncKwRe    = regexp.MustCompile(`^function\s+[A-Za-z_][A-Za-z0-9_.-]*\s*(?:\(\s*\))?\s*(?:\{|$)`)
	commentedFuncParenRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*\s*\(\s*\)\s*(?:\{|$)`)
	commentedSourceRe    = regexp.MustCompile(`^(?:source|\.)\s+(\S+)\s*(?:[;#].*)?$`)
	commentedAssignRe    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)
	commentedPluginsRe   = regexp.MustCompile(`^plugins\s*=\s*\(`)
)

type lineInfo struct {
	raw       string
	isComment bool
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
		copy(keepMask[i:j], blockKeep(info[i:j]))
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

const maxCleanBytes = 16 * 1024 * 1024

var errTooLarge = errors.New("file larger than 16 MB")

func Clean(r io.Reader) (string, Stats, error) {
	raw, err := io.ReadAll(io.LimitReader(r, maxCleanBytes+1))
	if err != nil {
		return "", Stats{}, err
	}
	if len(raw) > maxCleanBytes {
		return "", Stats{}, errTooLarge
	}
	if len(raw) == 0 {
		return "", Stats{}, nil
	}
	trailingNewline := raw[len(raw)-1] == '\n'

	decisions, stats, err := Process(strings.NewReader(string(raw)))
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
	joined := strings.Join(out, "\n")
	if trailingNewline {
		joined += "\n"
	}
	return joined, stats, nil
}

func blockKeep(block []lineInfo) []bool {
	keep := make([]bool, len(block))
	var prose []int
	for i, li := range block {
		if !isCommentedCode(li.inner) {
			prose = append(prose, i)
		}
	}
	if len(prose) == 0 {
		return keep
	}
	var k bool
	if len(block) == 1 {
		k = !isDecoration(strings.TrimSpace(block[0].inner))
	} else {
		k = keepProseBlock(block, prose)
	}
	for _, i := range prose {
		keep[i] = k
	}
	return keep
}

func keepProseBlock(block []lineInfo, idx []int) bool {
	sawLabel := false
	for _, i := range idx {
		s := strings.TrimSpace(block[i].inner)
		if s == "" || isDecoration(s) {
			continue
		}
		if !looksLikeLabel(s) {
			return false
		}
		sawLabel = true
	}
	return sawLabel
}

func isDecoration(s string) bool {
	return decorationRe.MatchString(s)
}

func looksLikeLabel(s string) bool {
	if len(s) > 80 {
		return false
	}
	if strings.HasSuffix(s, ".") {
		return false
	}
	return len(strings.Fields(s)) <= 8
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
		isCommentedSource(s) ||
		commentedPluginsRe.MatchString(s) ||
		commentedAssignRe.MatchString(s)
}

func isCommentedSource(s string) bool {
	m := commentedSourceRe.FindStringSubmatch(s)
	if m == nil {
		return false
	}
	target := strings.Trim(m[1], `"'`)
	if target == "" || strings.Contains(target, "://") {
		return false
	}
	return strings.ContainsAny(target[:1], "~$/.") || strings.ContainsAny(target, "/.")
}
