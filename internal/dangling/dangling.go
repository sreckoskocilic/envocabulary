package dangling

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/sreckoskocilic/envocabulary/internal/inventory"
	"github.com/sreckoskocilic/envocabulary/internal/model"
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
		return Finding{}, false
	}
	return Finding{}, false
}

func checkSource(filePath string, it inventory.Item) (Finding, bool) {
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
	if model.IsDeferredListVar(it.Name) || !looksLikeLiteralPath(it.Value) {
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
