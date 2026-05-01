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

func Discover() ([]File, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("home directory: %w", err)
	}
	zdot := os.Getenv("ZDOTDIR")
	if zdot == "" {
		zdot = home
	}

	files := make([]File, 0, len(canonicalZshNames)+len(canonicalBashNames))
	seen := map[string]bool{}

	for _, n := range canonicalZshNames {
		p := filepath.Join(zdot, n)
		if _, err := os.Stat(p); err == nil {
			files = append(files, parseFile(p, RoleCanonicalZsh))
			seen[p] = true
		}
	}
	for _, n := range canonicalBashNames {
		p := filepath.Join(home, n)
		if seen[p] {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			files = append(files, parseFile(p, RoleCanonicalBash))
			seen[p] = true
		}
	}

	for _, p := range scanOrphans(home, seen) {
		files = append(files, parseFile(p, RoleOrphan))
	}
	if zdot != home {
		for _, p := range scanOrphans(zdot, seen) {
			files = append(files, parseFile(p, RoleOrphan))
		}
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
	for _, n := range canonicalZshNames {
		if name == n {
			return true
		}
	}
	for _, n := range canonicalBashNames {
		if name == n {
			return true
		}
	}
	return false
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
	exportRe    = regexp.MustCompile(`^\s*export\s+([A-Za-z_][A-Za-z0-9_]*)(?:=(.*))?$`)
	assignRe    = regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)=(.*)$`)
	aliasRe     = regexp.MustCompile(`^\s*alias\s+(?:-[a-zA-Z]+\s+)*([A-Za-z_][A-Za-z0-9_.-]*)=`)
	funcKwRe    = regexp.MustCompile(`^\s*function\s+([A-Za-z_][A-Za-z0-9_.-]*)`)
	funcParenRe = regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_.-]*)\s*\(\s*\)`)
	sourceRe    = regexp.MustCompile(`^\s*(?:source|\.)\s+(\S+)`)
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
	if c := raw[0]; c == '"' || c == '\'' {
		if end := strings.IndexByte(raw[1:], c); end >= 0 {
			return raw[1 : 1+end]
		}
		return raw[1:]
	}
	if i := strings.IndexAny(raw, " \t"); i >= 0 {
		return raw[:i]
	}
	return raw
}

func ParseReader(r io.Reader) ([]Item, error) {
	var items []Item
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := sc.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if m := exportRe.FindStringSubmatch(line); m != nil {
			items = append(items, Item{Kind: KindExport, Name: m[1], Line: lineNo, Value: extractValue(m[2])})
			continue
		}
		if m := aliasRe.FindStringSubmatch(line); m != nil {
			items = append(items, Item{Kind: KindAlias, Name: m[1], Line: lineNo})
			continue
		}
		if m := funcKwRe.FindStringSubmatch(line); m != nil {
			items = append(items, Item{Kind: KindFunction, Name: m[1], Line: lineNo})
			continue
		}
		if m := funcParenRe.FindStringSubmatch(line); len(m) > 1 && !reservedFuncNames[m[1]] {
			items = append(items, Item{Kind: KindFunction, Name: m[1], Line: lineNo})
			continue
		}
		if m := sourceRe.FindStringSubmatch(line); m != nil {
			items = append(items, Item{Kind: KindSource, Name: m[1], Line: lineNo})
			continue
		}
		if m := assignRe.FindStringSubmatch(line); len(m) > 1 && !reservedFuncNames[m[1]] {
			items = append(items, Item{Kind: KindAssign, Name: m[1], Line: lineNo, Value: extractValue(m[2])})
			continue
		}
	}
	return items, sc.Err()
}
