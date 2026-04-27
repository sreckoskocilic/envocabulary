package dangling

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/sreckoskocilic/envocabulary/internal/inventory"
)

func TestLooksLikeLiteralPath(t *testing.T) {
	cases := map[string]bool{
		"":                false,
		"/usr/local/bin":  true,
		"~/Projects":      true,
		"~":               true,
		"vim":             false,
		"$HOME/foo":       false, // contains $
		"/a:/b":           false, // contains :
		"~/foo:~/bar":     false, // contains :
		"/path/with/$VAR": false, // contains $
	}
	for in, want := range cases {
		if got := looksLikeLiteralPath(in); got != want {
			t.Errorf("looksLikeLiteralPath(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestExpand_Tilde(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if got := expand("~"); got != home {
		t.Errorf("expand(~) = %q, want %q", got, home)
	}
	want := filepath.Join(home, "foo", "bar")
	if got := expand("~/foo/bar"); got != want {
		t.Errorf("expand(~/foo/bar) = %q, want %q", got, want)
	}
	if got := expand("/abs/path"); got != "/abs/path" {
		t.Errorf("expand(/abs/path) = %q, want %q", got, "/abs/path")
	}
}

func TestIsDeferredListVar(t *testing.T) {
	cases := map[string]bool{
		"PATH":                true,
		"MANPATH":             true,
		"FPATH":               true,
		"INFOPATH":            true,
		"CDPATH":              true,
		"DYLD_LIBRARY_PATH":   true,
		"DYLD_FALLBACK_PATHS": true,
		"JAVA_HOME":           false,
		"FOO":                 false,
	}
	for in, want := range cases {
		if got := isDeferredListVar(in); got != want {
			t.Errorf("isDeferredListVar(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestFind_SourceSkipsVarExpansion(t *testing.T) {
	files := []inventory.File{{
		Path: "/fake/.zshrc",
		Items: []inventory.Item{
			{Kind: inventory.KindSource, Name: "$ZSH/oh-my-zsh.sh", Line: 1},
			{Kind: inventory.KindSource, Name: "relative/path.sh", Line: 2},
			{Kind: inventory.KindSource, Name: "$(pwd)/x", Line: 3},
		},
	}}
	if got := Find(files); len(got) != 0 {
		t.Errorf("expected no findings for non-literal source targets; got %+v", got)
	}
}

func TestFind_SourceMissing(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "real.sh")
	if err := os.WriteFile(existing, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "ghost.sh")

	files := []inventory.File{{
		Path: filepath.Join(dir, ".zshrc"),
		Items: []inventory.Item{
			{Kind: inventory.KindSource, Name: existing, Line: 1},
			{Kind: inventory.KindSource, Name: missing, Line: 2},
		},
	}}

	got := Find(files)
	want := []Finding{{
		File:   filepath.Join(dir, ".zshrc"),
		Line:   2,
		Kind:   inventory.KindSource,
		Name:   missing,
		Value:  missing,
		Reason: ReasonSourceMissing,
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

func TestFind_SourceWithTildeExpanded(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	missing := "~/never-existed.sh"

	files := []inventory.File{{
		Path: filepath.Join(home, ".zshrc"),
		Items: []inventory.Item{
			{Kind: inventory.KindSource, Name: missing, Line: 5},
		},
	}}
	got := Find(files)
	if len(got) != 1 || got[0].Reason != ReasonSourceMissing || got[0].Name != missing {
		t.Errorf("expected one source-missing finding for %q; got %+v", missing, got)
	}
}

func TestFind_PathLikeExportMissing(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "sdk")
	if err := os.Mkdir(existing, 0o755); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "gone")

	files := []inventory.File{{
		Path: filepath.Join(dir, ".zshrc"),
		Items: []inventory.Item{
			{Kind: inventory.KindExport, Name: "JAVA_HOME", Line: 1, Value: existing},
			{Kind: inventory.KindExport, Name: "ANDROID_HOME", Line: 2, Value: missing},
			{Kind: inventory.KindAssign, Name: "GOROOT", Line: 3, Value: missing},
		},
	}}

	got := Find(files)
	want := []Finding{
		{File: filepath.Join(dir, ".zshrc"), Line: 2, Kind: inventory.KindExport, Name: "ANDROID_HOME", Value: missing, Reason: ReasonPathMissing},
		{File: filepath.Join(dir, ".zshrc"), Line: 3, Kind: inventory.KindAssign, Name: "GOROOT", Value: missing, Reason: ReasonPathMissing},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

func TestFind_SkipsNonPathValues(t *testing.T) {
	files := []inventory.File{{
		Path: "/fake/.zshrc",
		Items: []inventory.Item{
			{Kind: inventory.KindExport, Name: "EDITOR", Line: 1, Value: "vim"},
			{Kind: inventory.KindExport, Name: "EXPANDED", Line: 2, Value: "$HOME/foo"},
			{Kind: inventory.KindExport, Name: "MULTI", Line: 3, Value: "/a:/b"},
			{Kind: inventory.KindExport, Name: "EMPTY", Line: 4, Value: ""},
		},
	}}
	if got := Find(files); len(got) != 0 {
		t.Errorf("expected no findings for non-path values; got %+v", got)
	}
}

func TestFind_SkipsDeferredListVars(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "gone")

	files := []inventory.File{{
		Path: "/fake/.zshrc",
		Items: []inventory.Item{
			{Kind: inventory.KindExport, Name: "PATH", Line: 1, Value: missing},
			{Kind: inventory.KindExport, Name: "DYLD_LIBRARY_PATH", Line: 2, Value: missing},
		},
	}}
	if got := Find(files); len(got) != 0 {
		t.Errorf("PATH-like vars should be skipped; got %+v", got)
	}
}

func TestFind_SkipsAliasesAndFunctions(t *testing.T) {
	files := []inventory.File{{
		Path: "/fake/.zshrc",
		Items: []inventory.Item{
			{Kind: inventory.KindAlias, Name: "ll", Line: 1, Value: "ls -la"},
			{Kind: inventory.KindFunction, Name: "mkcd", Line: 2},
		},
	}}
	if got := Find(files); len(got) != 0 {
		t.Errorf("aliases/functions are out of v1 scope; got %+v", got)
	}
}

func TestFind_OrderPreserved(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "gone")

	files := []inventory.File{
		{
			Path: "/a/.zshenv",
			Items: []inventory.Item{
				{Kind: inventory.KindSource, Name: missing, Line: 10},
			},
		},
		{
			Path: "/b/.zshrc",
			Items: []inventory.Item{
				{Kind: inventory.KindExport, Name: "FOO", Line: 1, Value: missing},
				{Kind: inventory.KindSource, Name: missing, Line: 2},
			},
		},
	}
	got := Find(files)
	wantPaths := []string{"/a/.zshenv", "/b/.zshrc", "/b/.zshrc"}
	wantLines := []int{10, 1, 2}
	if len(got) != 3 {
		t.Fatalf("expected 3 findings; got %d (%+v)", len(got), got)
	}
	for i, f := range got {
		if f.File != wantPaths[i] || f.Line != wantLines[i] {
			t.Errorf("finding %d: got %s:%d, want %s:%d", i, f.File, f.Line, wantPaths[i], wantLines[i])
		}
	}
}

func TestFind_EmptyInput(t *testing.T) {
	if got := Find(nil); got != nil {
		t.Errorf("expected nil for nil input; got %+v", got)
	}
	if got := Find([]inventory.File{}); got != nil {
		t.Errorf("expected nil for empty input; got %+v", got)
	}
}
