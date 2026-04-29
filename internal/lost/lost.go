package lost

import (
	"github.com/sreckoskocilic/envocabulary/internal/inventory"
	"github.com/sreckoskocilic/envocabulary/internal/model"
)

type Finding struct {
	File string
	Kind inventory.Kind
	Name string
	Line int
}

func Find(files []inventory.File) []Finding {
	canonical := map[string]bool{}
	for _, f := range files {
		if f.Role == inventory.RoleOrphan {
			continue
		}
		for _, it := range f.Items {
			canonical[key(it.Kind, it.Name)] = true
		}
	}

	var out []Finding
	for _, f := range files {
		if f.Role != inventory.RoleOrphan {
			continue
		}
		seen := map[string]bool{}
		for _, it := range f.Items {
			if (it.Kind == inventory.KindExport || it.Kind == inventory.KindAssign) && model.IsDeferredListVar(it.Name) {
				continue
			}
			k := key(it.Kind, it.Name)
			if canonical[k] || seen[k] {
				continue
			}
			seen[k] = true
			out = append(out, Finding{
				File: f.Path,
				Kind: it.Kind,
				Name: it.Name,
				Line: it.Line,
			})
		}
	}
	return out
}

func key(kind inventory.Kind, name string) string {
	return string(kind) + "\x00" + name
}
