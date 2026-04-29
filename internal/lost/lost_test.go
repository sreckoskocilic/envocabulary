package lost

import (
	"testing"

	"github.com/sreckoskocilic/envocabulary/internal/inventory"
)

func TestFind(t *testing.T) {
	files := []inventory.File{
		{Path: "/z/.zshrc", Role: inventory.RoleCanonicalZsh, Items: []inventory.Item{
			{Kind: inventory.KindExport, Name: "EDITOR", Line: 1},
			{Kind: inventory.KindAlias, Name: "ll", Line: 2},
		}},
		{Path: "/z/.zshrc.backup", Role: inventory.RoleOrphan, Items: []inventory.Item{
			{Kind: inventory.KindExport, Name: "EDITOR", Line: 1},
			{Kind: inventory.KindExport, Name: "JAVA_HOME", Line: 5},
			{Kind: inventory.KindAlias, Name: "gs", Line: 7},
			{Kind: inventory.KindFunction, Name: "myfunc", Line: 10},
		}},
	}

	findings := Find(files)

	if len(findings) != 3 {
		t.Fatalf("expected 3 findings, got %d: %+v", len(findings), findings)
	}

	for _, f := range findings {
		if f.Name == "EDITOR" {
			t.Error("EDITOR should not be in findings (present in canonical)")
		}
	}

	names := map[string]bool{}
	for _, f := range findings {
		names[f.Name] = true
	}
	for _, want := range []string{"JAVA_HOME", "gs", "myfunc"} {
		if !names[want] {
			t.Errorf("expected %s in findings", want)
		}
	}
}

func TestFindExcludesDeferredListVars(t *testing.T) {
	files := []inventory.File{
		{Path: "/z/.zshrc", Role: inventory.RoleCanonicalZsh},
		{Path: "/z/.zshrc.old", Role: inventory.RoleOrphan, Items: []inventory.Item{
			{Kind: inventory.KindExport, Name: "PATH", Line: 1},
			{Kind: inventory.KindExport, Name: "MANPATH", Line: 2},
			{Kind: inventory.KindAssign, Name: "FPATH", Line: 3},
			{Kind: inventory.KindExport, Name: "DYLD_LIBRARY_PATH", Line: 4},
		}},
	}
	if findings := Find(files); len(findings) != 0 {
		t.Errorf("expected deferred-list-vars excluded, got %+v", findings)
	}
}

func TestFindDedupsWithinOrphan(t *testing.T) {
	files := []inventory.File{
		{Path: "/z/.zshrc", Role: inventory.RoleCanonicalZsh},
		{Path: "/z/.zshrc.bak", Role: inventory.RoleOrphan, Items: []inventory.Item{
			{Kind: inventory.KindExport, Name: "FOO", Line: 1},
			{Kind: inventory.KindExport, Name: "FOO", Line: 10},
		}},
	}

	findings := Find(files)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding (dedup within orphan), got %d: %+v", len(findings), findings)
	}
	if findings[0].Line != 1 {
		t.Errorf("expected first occurrence (line 1), got line %d", findings[0].Line)
	}
}

func TestFindNoOrphans(t *testing.T) {
	files := []inventory.File{
		{Path: "/z/.zshrc", Role: inventory.RoleCanonicalZsh, Items: []inventory.Item{
			{Kind: inventory.KindExport, Name: "FOO", Line: 1},
		}},
	}
	if findings := Find(files); len(findings) != 0 {
		t.Errorf("expected no findings without orphans, got %+v", findings)
	}
}

func TestFindMultipleOrphans(t *testing.T) {
	files := []inventory.File{
		{Path: "/z/.zshrc", Role: inventory.RoleCanonicalZsh, Items: []inventory.Item{
			{Kind: inventory.KindExport, Name: "EDITOR", Line: 1},
		}},
		{Path: "/z/.zshrc.old", Role: inventory.RoleOrphan, Items: []inventory.Item{
			{Kind: inventory.KindExport, Name: "A", Line: 1},
		}},
		{Path: "/z/.zshrc.bak", Role: inventory.RoleOrphan, Items: []inventory.Item{
			{Kind: inventory.KindExport, Name: "B", Line: 1},
			{Kind: inventory.KindExport, Name: "A", Line: 2},
		}},
	}

	findings := Find(files)
	if len(findings) != 3 {
		t.Fatalf("expected 3 findings (A from each orphan + B), got %d: %+v", len(findings), findings)
	}
}

func TestFindSameNameDifferentKind(t *testing.T) {
	files := []inventory.File{
		{Path: "/z/.zshrc", Role: inventory.RoleCanonicalZsh, Items: []inventory.Item{
			{Kind: inventory.KindExport, Name: "FOO", Line: 1},
		}},
		{Path: "/z/.zshrc.old", Role: inventory.RoleOrphan, Items: []inventory.Item{
			{Kind: inventory.KindExport, Name: "FOO", Line: 1},
			{Kind: inventory.KindAlias, Name: "FOO", Line: 2},
		}},
	}

	findings := Find(files)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding (alias FOO only), got %d: %+v", len(findings), findings)
	}
	if findings[0].Kind != inventory.KindAlias {
		t.Errorf("expected alias kind, got %s", findings[0].Kind)
	}
}

func TestFindIncludesBashCanonical(t *testing.T) {
	files := []inventory.File{
		{Path: "/h/.bashrc", Role: inventory.RoleCanonicalBash, Items: []inventory.Item{
			{Kind: inventory.KindExport, Name: "FROM_BASH", Line: 1},
		}},
		{Path: "/h/.zshrc.old", Role: inventory.RoleOrphan, Items: []inventory.Item{
			{Kind: inventory.KindExport, Name: "FROM_BASH", Line: 3},
			{Kind: inventory.KindExport, Name: "ONLY_ORPHAN", Line: 5},
		}},
	}

	findings := Find(files)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(findings), findings)
	}
	if findings[0].Name != "ONLY_ORPHAN" {
		t.Errorf("expected ONLY_ORPHAN, got %s", findings[0].Name)
	}
}

func TestFindSourceItems(t *testing.T) {
	files := []inventory.File{
		{Path: "/z/.zshrc", Role: inventory.RoleCanonicalZsh, Items: []inventory.Item{
			{Kind: inventory.KindSource, Name: "~/.aliases", Line: 1},
		}},
		{Path: "/z/.zshrc.old", Role: inventory.RoleOrphan, Items: []inventory.Item{
			{Kind: inventory.KindSource, Name: "~/.aliases", Line: 1},
			{Kind: inventory.KindSource, Name: "~/.oldstuff", Line: 2},
		}},
	}

	findings := Find(files)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(findings), findings)
	}
	if findings[0].Name != "~/.oldstuff" {
		t.Errorf("expected ~/.oldstuff, got %s", findings[0].Name)
	}
}
