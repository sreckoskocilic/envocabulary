package report

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/sreckoskocilic/envocabulary/internal/inventory"
)

func testFiles() []inventory.File {
	return []inventory.File{
		{Path: "/home/u/.zprofile", Role: inventory.RoleCanonicalZsh, Items: []inventory.Item{
			{Kind: inventory.KindExport, Name: "EDITOR", Line: 3, Value: "nvim"},
			{Kind: inventory.KindExport, Name: "GOPATH", Line: 5, Value: "$HOME/go"},
			{Kind: inventory.KindExport, Name: "JAVA_HOME", Line: 8, Value: "/usr/local/java11"},
		}},
		{Path: "/home/u/.zshrc", Role: inventory.RoleCanonicalZsh, Items: []inventory.Item{
			{Kind: inventory.KindExport, Name: "EDITOR", Line: 2, Value: "nvim"},
			{Kind: inventory.KindExport, Name: "JAVA_HOME", Line: 10, Value: "/opt/openjdk"},
			{Kind: inventory.KindAlias, Name: "ll", Line: 15, Value: "ls -lA"},
		}},
		{Path: "/home/u/.zshrc.old", Role: inventory.RoleOrphan, Items: []inventory.Item{
			{Kind: inventory.KindExport, Name: "OLD_VAR", Line: 1, Value: "stale"},
			{Kind: inventory.KindAlias, Name: "gp", Line: 3, Value: "git push"},
		}},
	}
}

func TestBuildSafeEntries(t *testing.T) {
	r := Build(testFiles())
	if len(r.Safe) != 1 {
		t.Fatalf("expected 1 safe entry, got %d: %+v", len(r.Safe), r.Safe)
	}
	if !strings.Contains(r.Safe[0].Definition, "EDITOR") {
		t.Errorf("expected EDITOR in safe, got %q", r.Safe[0].Definition)
	}
}

func TestBuildReviewEntries(t *testing.T) {
	r := Build(testFiles())
	if len(r.Review) != 1 {
		t.Fatalf("expected 1 review entry, got %d: %+v", len(r.Review), r.Review)
	}
	if !strings.Contains(r.Review[0].Definition, "JAVA_HOME") {
		t.Errorf("expected JAVA_HOME in review, got %q", r.Review[0].Definition)
	}
	if r.Review[0].ActiveValue != "/opt/openjdk" {
		t.Errorf("expected active value /opt/openjdk, got %q", r.Review[0].ActiveValue)
	}
}

func TestBuildOrphans(t *testing.T) {
	r := Build(testFiles())
	if len(r.Orphans) != 1 {
		t.Fatalf("expected 1 orphan file, got %d: %+v", len(r.Orphans), r.Orphans)
	}
	if !strings.Contains(r.Orphans[0].Summary, "1 export") {
		t.Errorf("expected '1 export' in summary, got %q", r.Orphans[0].Summary)
	}
	if !strings.Contains(r.Orphans[0].Summary, "1 alias") {
		t.Errorf("expected '1 alias' in summary, got %q", r.Orphans[0].Summary)
	}
}

func TestBuildNoDuplicates(t *testing.T) {
	files := []inventory.File{
		{Path: "/home/u/.zshrc", Role: inventory.RoleCanonicalZsh, Items: []inventory.Item{
			{Kind: inventory.KindExport, Name: "UNIQUE", Line: 1, Value: "val"},
		}},
	}
	r := Build(files)
	if len(r.Safe) != 0 || len(r.Review) != 0 {
		t.Errorf("expected no safe/review entries for unique vars, got safe=%d review=%d", len(r.Safe), len(r.Review))
	}
}

func TestBuildFunctionDef(t *testing.T) {
	files := []inventory.File{
		{Path: "/home/u/.zprofile", Role: inventory.RoleCanonicalZsh, Items: []inventory.Item{
			{Kind: inventory.KindFunction, Name: "mkcd", Line: 5},
		}},
		{Path: "/home/u/.zshrc", Role: inventory.RoleCanonicalZsh, Items: []inventory.Item{
			{Kind: inventory.KindFunction, Name: "mkcd", Line: 10},
		}},
	}
	r := Build(files)
	if len(r.Safe) != 1 {
		t.Fatalf("expected 1 safe entry for function, got %d", len(r.Safe))
	}
	if r.Safe[0].Definition != "function mkcd" {
		t.Errorf("expected 'function mkcd', got %q", r.Safe[0].Definition)
	}
}

func TestWriteText(t *testing.T) {
	r := Build(testFiles())
	var buf bytes.Buffer
	WriteText(&buf, r)
	out := buf.String()

	for _, want := range []string{"SAFE TO DELETE", "REVIEW", "DANGLING", "ORPHANED FILES"} {
		if !strings.Contains(out, want) {
			t.Errorf("text output missing section %q", want)
		}
	}
}

func TestWriteHTML(t *testing.T) {
	r := Build(testFiles())
	var buf bytes.Buffer
	if err := WriteHTML(&buf, r); err != nil {
		t.Fatalf("WriteHTML error: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "<!DOCTYPE html>") {
		t.Error("missing DOCTYPE")
	}
	for _, want := range []string{"safe to delete", "review", "dangling", "orphaned files"} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML output missing section %q", want)
		}
	}
}

func TestBuildFilesScanned(t *testing.T) {
	r := Build(testFiles())
	if r.FilesScanned != 3 {
		t.Errorf("expected 3 files scanned, got %d", r.FilesScanned)
	}
}

func TestBuildDanglingSource(t *testing.T) {
	files := []inventory.File{
		{Path: "/home/u/.zshrc", Role: inventory.RoleCanonicalZsh, Items: []inventory.Item{
			{Kind: inventory.KindSource, Name: "/nonexistent/file.zsh", Line: 5, Value: "/nonexistent/file.zsh"},
		}},
	}
	r := Build(files)
	if len(r.Dangling) != 1 {
		t.Fatalf("expected 1 dangling entry, got %d", len(r.Dangling))
	}
	if !strings.Contains(r.Dangling[0].Definition, "source") {
		t.Errorf("expected 'source' in definition, got %q", r.Dangling[0].Definition)
	}
}

func TestBuildDanglingSourceDefinition(t *testing.T) {
	files := []inventory.File{
		{Path: "/home/u/.zshrc", Role: inventory.RoleCanonicalZsh, Items: []inventory.Item{
			{Kind: inventory.KindSource, Name: "/nonexistent/file.zsh", Line: 5, Value: "/nonexistent/file.zsh"},
		}},
	}
	r := Build(files)
	if len(r.Dangling) != 1 {
		t.Fatalf("expected 1 dangling, got %d", len(r.Dangling))
	}
	if !strings.Contains(r.Dangling[0].Definition, "/nonexistent/file.zsh") {
		t.Errorf("source definition should contain path; got %q", r.Dangling[0].Definition)
	}
}

func TestBuildDanglingExportNoValue(t *testing.T) {
	files := []inventory.File{
		{Path: "/home/u/.zprofile", Role: inventory.RoleCanonicalZsh, Items: []inventory.Item{
			{Kind: inventory.KindExport, Name: "EMPTY", Line: 1},
		}},
		{Path: "/home/u/.zshrc", Role: inventory.RoleCanonicalZsh, Items: []inventory.Item{
			{Kind: inventory.KindExport, Name: "EMPTY", Line: 2},
		}},
	}
	r := Build(files)
	if len(r.Safe) != 1 {
		t.Fatalf("expected 1 safe entry, got %d", len(r.Safe))
	}
	if !strings.Contains(r.Safe[0].Definition, "EMPTY") {
		t.Errorf("expected 'EMPTY' in definition, got %q", r.Safe[0].Definition)
	}
	if strings.Contains(r.Safe[0].Definition, "=") {
		t.Errorf("expected no '=' for empty value, got %q", r.Safe[0].Definition)
	}
}

func TestWriteTextEmptySections(t *testing.T) {
	r := Report{}
	var buf bytes.Buffer
	WriteText(&buf, r)
	out := buf.String()
	if !strings.Contains(out, "(none)") {
		t.Error("expected (none) for empty sections")
	}
}

func TestWriteHTMLEmptySections(t *testing.T) {
	r := Report{}
	var buf bytes.Buffer
	if err := WriteHTML(&buf, r); err != nil {
		t.Fatalf("WriteHTML error: %v", err)
	}
	if !strings.Contains(buf.String(), "none") {
		t.Error("expected 'none' for empty sections in HTML")
	}
}

func TestTildePath(t *testing.T) {
	orig := userHomeDir
	t.Cleanup(func() { userHomeDir = orig })
	userHomeDir = func() (string, error) { return "/home/u", nil }

	cases := []struct {
		in, want string
	}{
		{"/home/u", "~"},
		{"/home/u/foo", "~/foo"},
		{"/other/path", "/other/path"},
	}
	for _, tc := range cases {
		if got := tildePath(tc.in); got != tc.want {
			t.Errorf("tildePath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestTildePath_HomeDirError(t *testing.T) {
	orig := userHomeDir
	t.Cleanup(func() { userHomeDir = orig })
	userHomeDir = func() (string, error) { return "", errors.New("no home") }

	if got := tildePath("/some/path"); got != "/some/path" {
		t.Errorf("expected fallback to raw path; got %q", got)
	}
}
