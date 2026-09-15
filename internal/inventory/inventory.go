package inventory

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

type Kind string

const (
	KindExport   Kind = "export"
	KindAssign   Kind = "assign"
	KindAlias    Kind = "alias"
	KindFunction Kind = "function"
	KindSource   Kind = "source"
)

type Item struct {
	Kind  Kind
	Name  string
	Line  int
	Value string
}

type Role string

const (
	RoleCanonicalZsh  Role = "canonical-zsh"
	RoleCanonicalBash Role = "canonical-bash"
	RoleOrphan        Role = "orphan"
)

type File struct {
	Path  string
	Role  Role
	Items []Item
	Err   error
}

var (
	canonicalZshNames  = []string{".zshenv", ".zprofile", ".zshrc", ".zlogin", ".zlogout"}
	canonicalBashNames = []string{".bashrc", ".bash_profile", ".profile"}
)

var orphanPrefixes = []string{
	".zshenv", ".zprofile", ".zshrc", ".zlogin", ".zlogout",
	".bashrc", ".bash_profile", ".profile",
}

var Discover = discover

var userHomeDir = os.UserHomeDir

func discover() ([]File, error) {
	home, err := userHomeDir()
	if err != nil {
		return nil, fmt.Errorf("home directory: %w", err)
	}

	files := make([]File, 0, len(canonicalZshNames)+len(canonicalBashNames))
	seen := map[string]bool{}

	for _, n := range canonicalZshNames {
		p := filepath.Join(home, n)
		if _, err := os.Stat(p); err == nil {
			files = append(files, parseFile(p, RoleCanonicalZsh))
			seen[p] = true
		}
	}
	for _, n := range canonicalBashNames {
		p := filepath.Join(home, n)
		if _, err := os.Stat(p); err == nil {
			files = append(files, parseFile(p, RoleCanonicalBash))
			seen[p] = true
		}
	}

	for _, p := range scanOrphans(home, seen) {
		files = append(files, parseFile(p, RoleOrphan))
	}
	return files, nil
}

func scanOrphans(dir string, seen map[string]bool) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !hasOrphanPrefix(name) {
			continue
		}
		if isCanonical(name) {
			continue
		}
		p := filepath.Join(dir, name)
		if seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	slices.Sort(out)
	return out
}

func hasOrphanPrefix(name string) bool {
	for _, p := range orphanPrefixes {
		if name == p || strings.HasPrefix(name, p+".") || strings.HasPrefix(name, p+"_") || strings.HasPrefix(name, p+"-") {
			return true
		}
	}
	return false
}

func isCanonical(name string) bool {
	if slices.Contains(canonicalZshNames, name) {
		return true
	}
	return slices.Contains(canonicalBashNames, name)
}

func parseFile(path string, role Role) File {
	f, err := os.Open(path)
	if err != nil {
		return File{Path: path, Role: role, Err: err}
	}
	defer f.Close()
	items, err := ParseReader(f)
	return File{Path: path, Role: role, Items: items, Err: err}
}

var (
	exportRe    = regexp.MustCompile(`^\s*export\s+([A-Za-z_][A-Za-z0-9_]*)=(.*)$`)
	assignRe    = regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)=(.*)$`)
	aliasRe     = regexp.MustCompile(`^\s*alias\s+(?:-[a-zA-Z]+\s+)*([A-Za-z_][A-Za-z0-9_.-]*)=(.*)$`)
	funcKwRe    = regexp.MustCompile(`^\s*function\s+([A-Za-z_][A-Za-z0-9_.-]*)`)
	funcParenRe = regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_.-]*)\s*\(\s*\)`)
	sourceRe    = regexp.MustCompile(`^\s*(?:source|\.)\s+("[^"]*"|'[^']*'|[^\s;&|]+)`)
)

var reservedFuncNames = map[string]bool{
	"if": true, "elif": true, "then": true, "else": true, "fi": true,
	"for": true, "while": true, "until": true, "do": true, "done": true,
	"case": true, "esac": true, "select": true, "time": true, "return": true,
	"export": true, "local": true, "typeset": true, "declare": true, "alias": true,
	"unalias": true, "unset": true, "readonly": true, "source": true,
}

func extractValue(raw string) string {
	raw = strings.TrimLeft(raw, " \t")
	if raw == "" {
		return ""
	}
	if raw[0] == '(' {
		return arrayValue(raw)
	}
	var b strings.Builder
	var quote byte
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if quote != 0 {
			i += quotedByte(&b, &quote, raw, i)
			continue
		}
		switch {
		case c == '\'' || c == '"':
			quote = c
		case c == '\\' && i+1 < len(raw):
			b.WriteByte(raw[i+1])
			i++
		case c == ' ' || c == '\t':
			return b.String()
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// Returns how many extra bytes past raw[i] were consumed.
func quotedByte(b *strings.Builder, quote *byte, raw string, i int) int {
	c := raw[i]
	if c == *quote {
		*quote = 0
		return 0
	}
	if *quote == '"' && c == '\\' && i+1 < len(raw) && (raw[i+1] == '"' || raw[i+1] == '\\') {
		b.WriteByte(raw[i+1])
		return 1
	}
	b.WriteByte(c)
	return 0
}

func arrayValue(raw string) string {
	var quote byte
	for i := 1; i < len(raw); i++ {
		c := raw[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"':
			quote = c
		case c == ')':
			return "(" + strings.Join(strings.Fields(raw[1:i]), " ") + ")"
		}
	}
	return "(" + strings.Join(strings.Fields(raw[1:]), " ")
}

func stripQuotes(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

var ZshLoginRank = map[string]int{
	".zshenv": 0, ".zprofile": 1, ".zshrc": 2, ".zlogin": 3, ".zlogout": 4,
}

var BashLoginRank = map[string]int{
	".profile": 0, ".bash_profile": 1, ".bashrc": 2,
}

func FileRank(f File) int {
	base := filepath.Base(f.Path)
	switch f.Role {
	case RoleCanonicalZsh:
		return ZshLoginRank[base]
	case RoleCanonicalBash:
		return 100 + BashLoginRank[base]
	case RoleOrphan:
		return 200
	}
	return 999
}

func FilterFiles(files []File, bash, orphans bool) []File {
	keep := make([]File, 0, len(files))
	for _, f := range files {
		switch f.Role {
		case RoleCanonicalZsh:
			keep = append(keep, f)
		case RoleCanonicalBash:
			if bash {
				keep = append(keep, f)
			}
		case RoleOrphan:
			if orphans && IsShellOrphan(f.Path, bash) {
				keep = append(keep, f)
			}
		}
	}
	return keep
}

func IsShellOrphan(path string, includeBash bool) bool {
	name := filepath.Base(path)
	if strings.Contains(name, "zsh") || strings.HasPrefix(name, ".zprofile") || strings.HasPrefix(name, ".zlog") {
		return true
	}
	if includeBash {
		return strings.Contains(name, "bash") || strings.HasPrefix(name, ".profile")
	}
	return false
}

func ParseReader(r io.Reader) ([]Item, error) {
	var items []Item
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := sc.Text()
		if lineNo == 1 && strings.HasPrefix(line, "\ufeff") {
			continue
		}
		if it, ok := parseLine(line, lineNo); ok {
			items = append(items, it)
		}
	}
	if err := sc.Err(); err != nil {
		return items, fmt.Errorf("line %d: %w", lineNo+1, err)
	}
	return items, nil
}

func parseLine(line string, lineNo int) (Item, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return Item{}, false
	}
	if m := exportRe.FindStringSubmatch(line); m != nil {
		return Item{Kind: KindExport, Name: m[1], Line: lineNo, Value: extractValue(m[2])}, true
	}
	if m := aliasRe.FindStringSubmatch(line); m != nil {
		return Item{Kind: KindAlias, Name: m[1], Line: lineNo, Value: extractValue(m[2])}, true
	}
	if m := funcKwRe.FindStringSubmatch(line); m != nil {
		return Item{Kind: KindFunction, Name: m[1], Line: lineNo}, !reservedFuncNames[m[1]]
	}
	if m := funcParenRe.FindStringSubmatch(line); len(m) > 1 && !reservedFuncNames[m[1]] {
		return Item{Kind: KindFunction, Name: m[1], Line: lineNo}, true
	}
	if m := sourceRe.FindStringSubmatch(line); m != nil {
		return Item{Kind: KindSource, Name: stripQuotes(m[1]), Line: lineNo}, true
	}
	if m := assignRe.FindStringSubmatch(line); len(m) > 1 && !reservedFuncNames[m[1]] {
		return Item{Kind: KindAssign, Name: m[1], Line: lineNo, Value: extractValue(m[2])}, true
	}
	return Item{}, false
}
