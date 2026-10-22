package inventory

import (
	"regexp"
	"strings"
)

type frame byte

const (
	frameParen frame = iota
	frameBrace
	frameExpansion
	frameIf
	frameCase
	frameLoop
)

var heredocRe = regexp.MustCompile(`^<<(-?)\s*(?:'([^']+)'|"([^"]+)"|\\?([A-Za-z_][A-Za-z0-9_]*))`)

type heredoc struct {
	delim     string
	stripTabs bool
}

type nesting struct {
	stack        []frame
	quote        byte
	continuation bool
	heredocs     []heredoc
	broken       bool
}

func (n *nesting) topLevel(line string) bool {
	top := len(n.stack) == 0 && n.quote == 0 && !n.continuation && len(n.heredocs) == 0 &&
		!strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t")
	n.scan(line)
	return top
}

func (n *nesting) balanced() bool {
	return !n.broken && len(n.stack) == 0 && n.quote == 0 && !n.continuation && len(n.heredocs) == 0
}

func (n *nesting) scan(line string) {
	if len(n.heredocs) > 0 {
		n.heredocLine(line)
		return
	}
	n.continuation = false
	cmdPos := true
	for i := 0; i < len(line); i++ {
		if n.quote != 0 {
			i = n.quoted(line, i)
			continue
		}
		var stop bool
		i, cmdPos, stop = n.token(line, i, cmdPos)
		if stop {
			return
		}
	}
}

func (n *nesting) heredocLine(line string) {
	h := n.heredocs[0]
	if h.stripTabs {
		line = strings.TrimLeft(line, "\t")
	}
	if line == h.delim {
		n.heredocs = n.heredocs[1:]
	}
}

func (n *nesting) quoted(line string, i int) int {
	switch c := line[i]; {
	case c == '\\' && n.quote != '\'':
		return i + 1
	case c == n.quote:
		n.quote = 0
	}
	return i
}

func (n *nesting) token(line string, i int, cmdPos bool) (next int, nextCmdPos, comment bool) {
	c := line[i]
	switch {
	case strings.IndexByte("\\'\"`#", c) >= 0:
		return n.quoteOrComment(line, i, cmdPos)
	case strings.IndexByte("(){}$", c) >= 0:
		next, nextCmdPos = n.bracket(line, i)
		return next, nextCmdPos, false
	case c == '<':
		return n.redirect(line, i), false, false
	case c == ';' || c == '&' || c == '|':
		return i, true, false
	case isBlank(c):
		return i, cmdPos, false
	}
	next, nextCmdPos = n.word(line, i, cmdPos)
	return next, nextCmdPos, false
}

func (n *nesting) quoteOrComment(line string, i int, cmdPos bool) (next int, nextCmdPos, comment bool) {
	switch c := line[i]; c {
	case '\\':
		if i == len(line)-1 {
			n.continuation = true
		}
		return i + 1, false, false
	case '#':
		if i == 0 || isBlank(line[i-1]) || strings.IndexByte(";&|(", line[i-1]) >= 0 {
			return i, cmdPos, true
		}
		return i, false, false
	default:
		n.quote = c
		return i, false, false
	}
}

func (n *nesting) bracket(line string, i int) (next int, nextCmdPos bool) {
	switch line[i] {
	case '(':
		n.stack = append(n.stack, frameParen)
		return i, true
	case ')':
		n.closeParen()
		return i, true
	case '$':
		if i+1 < len(line) && line[i+1] == '{' {
			n.stack = append(n.stack, frameExpansion)
			return i + 1, false
		}
		return i, false
	case '{':
		if i+1 == len(line) || isBlank(line[i+1]) {
			n.stack = append(n.stack, frameBrace)
			return i, true
		}
		n.stack = append(n.stack, frameExpansion)
		return i, false
	default:
		n.closeBrace()
		return i, false
	}
}

func (n *nesting) redirect(line string, i int) int {
	switch {
	case strings.HasPrefix(line[i:], "<<<"):
		return i + 2
	case strings.HasPrefix(line[i:], "<<"):
		return i + n.openHeredoc(line[i:]) - 1
	}
	return i
}

func (n *nesting) word(line string, i int, cmdPos bool) (next int, nextCmdPos bool) {
	end := i
	for end < len(line) && isWordByte(line[end]) {
		end++
	}
	if end == i {
		return i, false
	}
	if cmdPos {
		cmdPos = n.keyword(line[i:end])
	}
	return end - 1, cmdPos
}

func (n *nesting) keyword(w string) bool {
	switch w {
	case "if":
		n.stack = append(n.stack, frameIf)
	case "case":
		n.stack = append(n.stack, frameCase)
		return false
	case "for", "foreach", "while", "until", "select", "repeat":
		n.stack = append(n.stack, frameLoop)
	case "end":
		if len(n.stack) > 0 && n.stack[len(n.stack)-1] == frameLoop {
			n.stack = n.stack[:len(n.stack)-1]
		}
		return false
	case "fi":
		n.pop(frameIf)
		return false
	case "esac":
		n.pop(frameCase)
		return false
	case "done":
		n.pop(frameLoop)
		return false
	case "then", "else", "elif", "do", "!", "time", "function":
	default:
		return false
	}
	return true
}

func (n *nesting) pop(f frame) {
	if len(n.stack) == 0 || n.stack[len(n.stack)-1] != f {
		n.broken = true
		return
	}
	n.stack = n.stack[:len(n.stack)-1]
}

func (n *nesting) closeParen() {
	if len(n.stack) > 0 && n.stack[len(n.stack)-1] == frameCase {
		return
	}
	n.pop(frameParen)
}

func (n *nesting) closeBrace() {
	if len(n.stack) > 0 && n.stack[len(n.stack)-1] == frameExpansion {
		n.stack = n.stack[:len(n.stack)-1]
		return
	}
	n.pop(frameBrace)
}

func (n *nesting) openHeredoc(s string) int {
	m := heredocRe.FindStringSubmatch(s)
	if m == nil {
		n.broken = true
		return 2
	}
	n.heredocs = append(n.heredocs, heredoc{delim: m[2] + m[3] + m[4], stripTabs: m[1] == "-"})
	return len(m[0])
}

func isBlank(c byte) bool { return c == ' ' || c == '\t' }

func isWordByte(c byte) bool {
	return c == '_' || c == '!' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}
