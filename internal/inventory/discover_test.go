package inventory

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func setupHome(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", dir)
	t.Setenv("ZDOTDIR", dir)
	return dir
}

func paths(files []File) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, filepath.Base(f.Path))
	}
	slices.Sort(out)
	return out
}

func TestDiscover_EmptyHome(t *testing.T) {
	setupHome(t, nil)
	got, _ := Discover()
	if len(got) != 0 {
		t.Errorf("expected 0 files in empty dir, got %d: %v", len(got), paths(got))
	}
}

func TestDiscover_FindsCanonicalZsh(t *testing.T) {
	setupHome(t, map[string]string{
		".zshrc":        "export X=1\n",
		".zshenv":       "export Y=2\n",
		".bashrc":       "export Z=3\n",
		".bash_profile": "export W=4\n",
	})
	got, _ := Discover()
	wantNames := map[string]Role{
		".zshrc":        RoleCanonicalZsh,
		".zshenv":       RoleCanonicalZsh,
		".bashrc":       RoleCanonicalBash,
		".bash_profile": RoleCanonicalBash,
	}
	for _, f := range got {
		base := filepath.Base(f.Path)
		if wantRole, ok := wantNames[base]; ok {
			if f.Role != wantRole {
				t.Errorf("%s: role=%q, want %q", base, f.Role, wantRole)
			}
		}
	}
	found := paths(got)
	for n := range wantNames {
		if !slices.Contains(found, n) {
			t.Errorf("missing canonical file %s; got %v", n, found)
		}
	}
}

func TestDiscover_FindsOrphans(t *testing.T) {
	setupHome(t, map[string]string{
		".zshrc":          "export X=1\n",
		".zshrc.backup":   "export OLD1=1\n",
		".zshrc.old":      "export OLD2=1\n",
		".zshrc_2023":     "export OLD3=1\n",
		".bashrc.bak":     "export B1=1\n",
		".gitconfig":      "[user]\n",
		".zshrcsomething": "noise\n",
	})
	got, _ := Discover()
	found := paths(got)
	for _, name := range []string{".zshrc.backup", ".zshrc.old", ".zshrc_2023", ".bashrc.bak"} {
		if !slices.Contains(found, name) {
			t.Errorf("expected orphan %s in discovery; got %v", name, found)
		}
	}
	if slices.Contains(found, ".gitconfig") {
		t.Errorf("non-shell file .gitconfig should not be discovered; got %v", found)
	}
	if slices.Contains(found, ".zshrcsomething") {
		t.Errorf("prefix-collision .zshrcsomething should not be discovered; got %v", found)
	}
}

func TestDiscover_ZDOTDIRSeparateFromHome(t *testing.T) {
	home := t.TempDir()
	zdot := t.TempDir()
	if err := os.WriteFile(filepath.Join(zdot, ".zshrc"), []byte("export X=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".bashrc"), []byte("export Y=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("ZDOTDIR", zdot)

	got, _ := Discover()
	found := paths(got)
	if !slices.Contains(found, ".zshrc") {
		t.Errorf("expected .zshrc from ZDOTDIR; got %v", found)
	}
	if !slices.Contains(found, ".bashrc") {
		t.Errorf("expected .bashrc from HOME; got %v", found)
	}
}

func TestDiscover_OrphansFromBothHomeAndZDOTDIR(t *testing.T) {
	home := t.TempDir()
	zdot := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".bashrc.old"), []byte("# old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(zdot, ".zshrc.backup"), []byte("# bak\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("ZDOTDIR", zdot)

	got, _ := Discover()
	found := paths(got)
	if !slices.Contains(found, ".bashrc.old") || !slices.Contains(found, ".zshrc.backup") {
		t.Errorf("expected orphans from both dirs; got %v", found)
	}
}

func TestDiscover_DeduplicatesWhenZDOTDIREqualsHome(t *testing.T) {
	dir := setupHome(t, map[string]string{".zshrc": "export X=1\n"})
	t.Setenv("HOME", dir)
	t.Setenv("ZDOTDIR", dir)

	got, _ := Discover()
	count := 0
	for _, f := range got {
		if filepath.Base(f.Path) == ".zshrc" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected .zshrc to appear exactly once; got %d", count)
	}
}

func TestParseFile_Success(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".zshrc")
	if err := os.WriteFile(p, []byte("export FOO=1\nalias ll='ls'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := parseFile(p, RoleCanonicalZsh)
	if got.Err != nil {
		t.Errorf("unexpected error: %v", got.Err)
	}
	if len(got.Items) != 2 {
		t.Errorf("expected 2 items; got %d", len(got.Items))
	}
}

func TestParseFile_OpenError(t *testing.T) {
	got := parseFile("/nonexistent/path", RoleCanonicalZsh)
	if got.Err == nil {
		t.Errorf("expected open error for missing file")
	}
}

func TestScanOrphans_DirectoryNotReadable(t *testing.T) {
	got := scanOrphans("/this/path/does/not/exist", map[string]bool{})
	if got != nil {
		t.Errorf("expected nil for unreadable directory; got %v", got)
	}
}

func TestScanOrphans_SkipsSubdirectories(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, ".zshrc.d")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	got := scanOrphans(dir, map[string]bool{})
	for _, p := range got {
		if strings.HasSuffix(p, ".zshrc.d") {
			t.Errorf("subdirectory should be skipped; got %v", got)
		}
	}
}

func TestScanOrphans_HonorsSeen(t *testing.T) {
	dir := t.TempDir()
	bak := filepath.Join(dir, ".zshrc.backup")
	if err := os.WriteFile(bak, []byte("# old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{bak: true}
	got := scanOrphans(dir, seen)
	for _, p := range got {
		if p == bak {
			t.Errorf("seen path should be skipped; got %v", got)
		}
	}
}

func TestIsCanonical(t *testing.T) {
	cases := map[string]bool{
		".zshrc":          true,
		".zshenv":         true,
		".zprofile":       true,
		".zlogin":         true,
		".zlogout":        true,
		".bashrc":         true,
		".bash_profile":   true,
		".profile":        true,
		".zshrc.backup":   false,
		".gitconfig":      false,
		".zshrcsomething": false,
		"":                false,
	}
	for in, want := range cases {
		if got := isCanonical(in); got != want {
			t.Errorf("isCanonical(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestDiscover_HomeDirError(t *testing.T) {
	orig := userHomeDir
	t.Cleanup(func() { userHomeDir = orig })
	userHomeDir = func() (string, error) {
		return "", errors.New("no home")
	}

	_, err := discover()
	if err == nil {
		t.Fatal("expected error from discover")
	}
	if !strings.Contains(err.Error(), "home directory") {
		t.Errorf("expected wrapped error; got %v", err)
	}
}
