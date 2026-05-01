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

func TestWriteFile_OrphanSuffix(t *testing.T) {
	dir := t.TempDir()
	rc := filepath.Join(dir, ".zshrc.old")
	if err := os.WriteFile(rc, []byte("export X=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := writeFile(&buf, inventory.File{Path: rc, Role: inventory.RoleOrphan}, Options{}, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "(orphan)") {
		t.Errorf("expected orphan suffix in banner; got:\n%s", buf.String())
	}
}

func TestWriteFile_BashSuffix(t *testing.T) {
	dir := t.TempDir()
	rc := filepath.Join(dir, ".bashrc")
	if err := os.WriteFile(rc, []byte("export X=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := writeFile(&buf, inventory.File{Path: rc, Role: inventory.RoleCanonicalBash}, Options{}, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "(bash)") {
		t.Errorf("expected bash suffix in banner; got:\n%s", buf.String())
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
