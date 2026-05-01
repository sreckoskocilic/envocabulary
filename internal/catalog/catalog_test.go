package catalog

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sreckoskocilic/envocabulary/internal/dedup"
	"github.com/sreckoskocilic/envocabulary/internal/inventory"
)

func TestRoleOrder(t *testing.T) {
	cases := []struct {
		path string
		role inventory.Role
		want int
	}{
		{"/u/.zshenv", inventory.RoleCanonicalZsh, 0},
		{"/u/.zprofile", inventory.RoleCanonicalZsh, 1},
		{"/u/.zshrc", inventory.RoleCanonicalZsh, 2},
		{"/u/.zlogin", inventory.RoleCanonicalZsh, 3},
		{"/u/.zlogout", inventory.RoleCanonicalZsh, 4},
		{"/u/.bashrc", inventory.RoleCanonicalBash, 100},
		{"/u/.zshrc.backup", inventory.RoleOrphan, 200},
		{"/u/something", inventory.Role("garbage"), 999},
	}
	for _, tc := range cases {
		got := roleOrder(inventory.File{Path: tc.path, Role: tc.role})
		if got != tc.want {
			t.Errorf("%s/%s: got %d, want %d", tc.path, tc.role, got, tc.want)
		}
	}
}

func TestIsZshOrphan(t *testing.T) {
	cases := []struct {
		name        string
		path        string
		includeBash bool
		want        bool
	}{
		{"zsh in name", "/u/.zshrc.backup", false, true},
		{".zsh prefix", "/u/.zshrc.old", false, true},
		{".zprofile prefix", "/u/.zprofile_old", false, true},
		{".zlog prefix", "/u/.zlogin.bak", false, true},
		{"bashrc without --bash", "/u/.bashrc.backup", false, false},
		{"bashrc with --bash", "/u/.bashrc.backup", true, true},
		{"random file", "/u/.foo.bak", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isZshOrphan(tc.path, tc.includeBash); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFilterFiles(t *testing.T) {
	files := []inventory.File{
		{Path: "/u/.zshrc", Role: inventory.RoleCanonicalZsh},
		{Path: "/u/.bashrc", Role: inventory.RoleCanonicalBash},
		{Path: "/u/.zshrc.backup", Role: inventory.RoleOrphan},
		{Path: "/u/.bashrc.old", Role: inventory.RoleOrphan},
	}

	t.Run("default zsh-only", func(t *testing.T) {
		got := filterFiles(files, Options{})
		if len(got) != 1 || got[0].Path != "/u/.zshrc" {
			t.Errorf("expected only canonical zsh, got %+v", got)
		}
	})

	t.Run("with --bash", func(t *testing.T) {
		got := filterFiles(files, Options{IncludeBash: true})
		if len(got) != 2 {
			t.Errorf("expected zsh + bash canonical, got %d files", len(got))
		}
	})

	t.Run("with --orphans only includes zsh-flavored orphans", func(t *testing.T) {
		got := filterFiles(files, Options{IncludeOrphans: true})
		paths := make([]string, len(got))
		for i, f := range got {
			paths[i] = f.Path
		}
		hasBashOrphan := false
		for _, p := range paths {
			if p == "/u/.bashrc.old" {
				hasBashOrphan = true
			}
		}
		if hasBashOrphan {
			t.Errorf("bash orphan should be excluded without --bash; got %+v", paths)
		}
	})

	t.Run("with --orphans --bash includes everything", func(t *testing.T) {
		got := filterFiles(files, Options{IncludeOrphans: true, IncludeBash: true})
		if len(got) != 4 {
			t.Errorf("expected all 4 files, got %d", len(got))
		}
	})
}

func TestWrite_BasicEmission(t *testing.T) {
	dir := t.TempDir()
	zshrc := filepath.Join(dir, ".zshrc")
	if err := os.WriteFile(zshrc, []byte("export FOO=1\nexport BAR=2\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", dir)

	var buf bytes.Buffer
	if err := Write(&buf, Options{}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, ".zshrc") {
		t.Errorf("expected file path in banner; got:\n%s", out)
	}
	if !strings.Contains(out, "export FOO=1") || !strings.Contains(out, "export BAR=2") {
		t.Errorf("expected file contents emitted; got:\n%s", out)
	}
}

func TestWrite_LineNumbers(t *testing.T) {
	dir := t.TempDir()
	zshrc := filepath.Join(dir, ".zshrc")
	if err := os.WriteFile(zshrc, []byte("a\nb\nc\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", dir)

	var buf bytes.Buffer
	if err := Write(&buf, Options{LineNumbers: true}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "    1  a") {
		t.Errorf("expected line-numbered output; got:\n%s", out)
	}
}

func TestWrite_DedupAnnotation(t *testing.T) {
	dir := t.TempDir()
	zprof := filepath.Join(dir, ".zprofile")
	zshrc := filepath.Join(dir, ".zshrc")
	if err := os.WriteFile(zprof, []byte("export FOO=first\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(zshrc, []byte("export FOO=second\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", dir)

	var buf bytes.Buffer
	if err := Write(&buf, Options{Dedup: true}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "# [overridden by") {
		t.Errorf("expected override annotation; got:\n%s", out)
	}
}

func TestWriteFile_MissingFile(t *testing.T) {
	f := inventory.File{Path: "/nonexistent/.zshrc", Role: inventory.RoleCanonicalZsh}
	var buf bytes.Buffer
	err := writeFile(&buf, f, Options{}, nil)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output for missing file; got:\n%s", buf.String())
	}
}

func TestWriteFile_DedupAnnotationOnCorrectLine(t *testing.T) {
	dir := t.TempDir()
	rc := filepath.Join(dir, ".zshrc")
	if err := os.WriteFile(rc, []byte("export FOO=old\nexport BAR=ok\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	losers := map[string]dedup.Occurrence{
		dedup.Key(rc, 1): {File: filepath.Join(dir, ".zprofile"), Line: 3},
	}

	var buf bytes.Buffer
	if err := writeFile(&buf, inventory.File{Path: rc, Role: inventory.RoleCanonicalZsh}, Options{Dedup: true}, losers); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "# [overridden by") {
		t.Errorf("expected override annotation on line 1; got:\n%s", out)
	}
	if strings.Count(out, "# [overridden by") != 1 {
		t.Errorf("expected exactly one override annotation; got:\n%s", out)
	}
	if !strings.Contains(out, "export BAR=ok") {
		t.Errorf("non-overridden line should appear unchanged; got:\n%s", out)
	}
}

func TestWrite_BashGated(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".bashrc"), []byte("alias ll='ls'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", dir)

	var defaultBuf bytes.Buffer
	_ = Write(&defaultBuf, Options{})
	if strings.Contains(defaultBuf.String(), ".bashrc") {
		t.Errorf("default mode should not include bash files; got:\n%s", defaultBuf.String())
	}

	var bashBuf bytes.Buffer
	_ = Write(&bashBuf, Options{IncludeBash: true})
	if !strings.Contains(bashBuf.String(), ".bashrc") {
		t.Errorf("with --bash, expected .bashrc in output; got:\n%s", bashBuf.String())
	}
}

func TestWrite_DiscoverError(t *testing.T) {
	orig := inventory.Discover
	t.Cleanup(func() { inventory.Discover = orig })
	inventory.Discover = func() ([]inventory.File, error) {
		return nil, errors.New("mock discover error")
	}

	var buf bytes.Buffer
	err := Write(&buf, Options{})
	if err == nil {
		t.Fatal("expected error from Write")
	}
	if !strings.Contains(err.Error(), "mock discover error") {
		t.Errorf("expected mock error; got %v", err)
	}
}
