// Package dangling reports config items that point to filesystem targets which
// no longer exist: `source` lines whose target file is missing, and exports /
// assignments whose path-like literal value points nowhere.
//
// Scope is deliberately narrow (see CLAUDE.md):
//   - Only `source` and path-like exports/assignments are checked.
//   - Path-like means the literal value starts with `/` or `~`. Values that
//     contain `$` (variable expansion) or `:` (PATH-like accumulators) are
//     skipped — we can't reliably evaluate them statically.
//   - Colon-accumulating vars (PATH, MANPATH, FPATH, INFOPATH, CDPATH, DYLD_*)
//     are excluded the same way `dedup` excludes them: last-writer reasoning
//     does not apply.
//   - Aliases are out of scope for v1 (would need PATH resolution and alias
//     target parsing).
package dangling

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/sreckoskocilic/envocabulary/internal/inventory"
)

type Reason string

const (
	ReasonSourceMissing Reason = "source target missing"
	ReasonPathMissing   Reason = "path does not exist"
)

type Finding struct {
	File   string
	Line   int
	Kind   inventory.Kind
	Name   string
	Value  string
	Reason Reason
}

// Find walks every parsed item across the given files and returns one Finding
// per dangling reference. Order matches the input file order, then line order.
func Find(files []inventory.File) []Finding {
	var out []Finding
	for _, f := range files {
		for _, it := range f.Items {
			if finding, ok := check(f.Path, it); ok {
				out = append(out, finding)
			}
		}
	}
	return out
}

func check(filePath string, it inventory.Item) (Finding, bool) {
	switch it.Kind {
	case inventory.KindSource:
		return checkSource(filePath, it)
	case inventory.KindExport, inventory.KindAssign:
		return checkPathValue(filePath, it)
	case inventory.KindAlias, inventory.KindFunction:
		// Out of v1 scope (see package doc).
		return Finding{}, false
	}
	return Finding{}, false
}

func checkSource(filePath string, it inventory.Item) (Finding, bool) {
	// Skip targets we can't resolve statically: $VAR expansions, command
	// substitutions, anything but a plain absolute or ~-rooted path.
	if strings.ContainsAny(it.Name, "$`") {
		return Finding{}, false
	}
	if !(strings.HasPrefix(it.Name, "/") || strings.HasPrefix(it.Name, "~")) {
		return Finding{}, false
	}
	target := expand(it.Name)
	if target == "" || exists(target) {
		return Finding{}, false
	}
	return Finding{
		File:   filePath,
		Line:   it.Line,
		Kind:   it.Kind,
		Name:   it.Name,
		Value:  it.Name,
		Reason: ReasonSourceMissing,
	}, true
}

func checkPathValue(filePath string, it inventory.Item) (Finding, bool) {
	if isDeferredListVar(it.Name) || !looksLikeLiteralPath(it.Value) {
		return Finding{}, false
	}
	target := expand(it.Value)
	if target == "" || exists(target) {
		return Finding{}, false
	}
	return Finding{
		File:   filePath,
		Line:   it.Line,
		Kind:   it.Kind,
		Name:   it.Name,
		Value:  it.Value,
		Reason: ReasonPathMissing,
	}, true
}

// looksLikeLiteralPath returns true for values we can statically check:
// they start with `/` or `~`, contain no `$` (variable expansion), and no `:`
// (PATH-like). Anything else is too ambiguous for v1.
func looksLikeLiteralPath(v string) bool {
	if v == "" {
		return false
	}
	if !(strings.HasPrefix(v, "/") || strings.HasPrefix(v, "~")) {
		return false
	}
	if strings.ContainsAny(v, "$:") {
		return false
	}
	return true
}

func expand(p string) string {
	if p == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		return home
	}
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		return filepath.Join(home, p[2:])
	}
	return p
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// Duplicated from internal/dedup deliberately — both packages share the same
// domain rule (colon-accumulated vars don't follow last-writer semantics) but
// a shared package is overkill for two callers. Keep in sync if extended.
func isDeferredListVar(name string) bool {
	switch name {
	case "PATH", "MANPATH", "FPATH", "INFOPATH", "CDPATH":
		return true
	}
	return strings.HasPrefix(name, "DYLD_")
}
