package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sreckoskocilic/envocabulary/internal/dangling"
	"github.com/sreckoskocilic/envocabulary/internal/dedup"
	"github.com/sreckoskocilic/envocabulary/internal/inventory"
	"github.com/sreckoskocilic/envocabulary/internal/lost"
	"github.com/sreckoskocilic/envocabulary/internal/model"
)

func TestTruncate(t *testing.T) {
	cases := []struct {
		s    string
		n    int
		want string
	}{
		{"short", 10, "short"},
		{"exactlyten", 10, "exactlyten"},
		{"longer than ten chars", 10, "longer tha..."},
		{"", 5, ""},
		{"héllo wörld café", 10, "héllo wörl..."},
	}
	for _, tc := range cases {
		if got := truncate(tc.s, tc.n); got != tc.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tc.s, tc.n, got, tc.want)
		}
	}
}

func TestDedupFileRank(t *testing.T) {
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
		{".zshrc", inventory.RoleCanonicalZsh, 2},
		{"/u/.bashrc", inventory.RoleCanonicalBash, 100},
		{"/u/.zshrc.bak", inventory.RoleOrphan, 200},
		{"/u/x", inventory.Role("nonsense"), 999},
	}
	for _, tc := range cases {
		got := dedupFileRank(inventory.File{Path: tc.path, Role: tc.role})
		if got != tc.want {
			t.Errorf("%s/%s: got %d, want %d", tc.path, tc.role, got, tc.want)
		}
	}
}

func TestGroupItems(t *testing.T) {
	items := []inventory.Item{
		{Kind: inventory.KindExport, Name: "A"},
		{Kind: inventory.KindExport, Name: "B"},
		{Kind: inventory.KindAlias, Name: "ll"},
	}
	got := groupItems(items)
	if len(got[inventory.KindExport]) != 2 {
		t.Errorf("exports: got %d, want 2", len(got[inventory.KindExport]))
	}
	if len(got[inventory.KindAlias]) != 1 {
		t.Errorf("aliases: got %d, want 1", len(got[inventory.KindAlias]))
	}
	if len(got[inventory.KindFunction]) != 0 {
		t.Errorf("functions: got %d, want 0", len(got[inventory.KindFunction]))
	}
}

func TestPrintGroup(t *testing.T) {
	var buf bytes.Buffer
	printGroup(&buf, "exports", nil)
	if buf.Len() != 0 {
		t.Errorf("empty group should print nothing; got %q", buf.String())
	}

	buf.Reset()
	printGroup(&buf, "exports", []inventory.Item{
		{Name: "FOO"}, {Name: "BAR"},
	})
	out := buf.String()
	if !strings.Contains(out, "exports") || !strings.Contains(out, "2") || !strings.Contains(out, "FOO") || !strings.Contains(out, "BAR") {
		t.Errorf("expected label/count/names; got %q", out)
	}
}

func TestDie(t *testing.T) {
	var buf bytes.Buffer
	code := die(&buf, io.ErrUnexpectedEOF)
	if code != 1 {
		t.Errorf("die should return 1, got %d", code)
	}
	if !strings.Contains(buf.String(), "error:") {
		t.Errorf("expected 'error:' prefix; got %q", buf.String())
	}
}

func TestEmitDedupText(t *testing.T) {
	groups := []dedup.Group{
		{
			Kind:   inventory.KindExport,
			Name:   "JAVA_HOME",
			Winner: dedup.Occurrence{File: "/u/.zprofile", Line: 46, Kind: inventory.KindExport, Name: "JAVA_HOME"},
			Losers: []dedup.Occurrence{
				{File: "/u/.zprofile", Line: 37, Kind: inventory.KindExport, Name: "JAVA_HOME"},
			},
		},
		{
			Kind:   inventory.KindAlias,
			Name:   "ll",
			Winner: dedup.Occurrence{File: "/u/.zshrc", Line: 5, Kind: inventory.KindAlias, Name: "ll"},
			Losers: []dedup.Occurrence{
				{File: "/u/.bashrc", Line: 12, Kind: inventory.KindAlias, Name: "ll"},
			},
		},
	}
	var buf bytes.Buffer
	emitDedupText(&buf, groups)
	out := buf.String()
	if !strings.Contains(out, "## export") || !strings.Contains(out, "## alias") {
		t.Errorf("expected kind headers; got:\n%s", out)
	}
	if !strings.Contains(out, "winner  /u/.zprofile:46") {
		t.Errorf("expected winner line; got:\n%s", out)
	}
	if !strings.Contains(out, "loser   /u/.zprofile:37") {
		t.Errorf("expected loser line; got:\n%s", out)
	}
}

func TestEmitInventoryText(t *testing.T) {
	files := []inventory.File{
		{
			Path: "/u/.zshrc",
			Role: inventory.RoleCanonicalZsh,
			Items: []inventory.Item{
				{Kind: inventory.KindExport, Name: "PATH"},
				{Kind: inventory.KindAlias, Name: "ll"},
			},
		},
		{
			Path: "/u/.zshrc.bak",
			Role: inventory.RoleOrphan,
			Items: []inventory.Item{
				{Kind: inventory.KindExport, Name: "OLD"},
			},
		},
	}
	var buf bytes.Buffer
	emitInventoryText(&buf, files)
	out := buf.String()
	if !strings.Contains(out, "## /u/.zshrc\n") {
		t.Errorf("expected zshrc header; got:\n%s", out)
	}
	if !strings.Contains(out, "(orphan)") {
		t.Errorf("expected (orphan) suffix; got:\n%s", out)
	}
	if !strings.Contains(out, "PATH") || !strings.Contains(out, "ll") || !strings.Contains(out, "OLD") {
		t.Errorf("expected item names; got:\n%s", out)
	}
}

func TestEmitInventoryText_PerFileError(t *testing.T) {
	files := []inventory.File{
		{Path: "/u/.zshrc", Role: inventory.RoleCanonicalZsh, Err: io.EOF},
	}
	var buf bytes.Buffer
	emitInventoryText(&buf, files)
	if !strings.Contains(buf.String(), "error:") {
		t.Errorf("expected per-file error line; got:\n%s", buf.String())
	}
}

func TestEmitScanText(t *testing.T) {
	words := []model.EnWord{
		{Name: "JAVA_HOME", Origin: model.OriginShellFile, Source: "/u/.zprofile:46", Value: "/opt/java"},
		{Name: "EDITOR", Origin: model.OriginShellFile, Source: "/u/.zshrc:12", Value: "vim"},
		{Name: "PATH", Origin: model.OriginDeferred, Notes: "multi-source"},
	}
	var buf bytes.Buffer
	emitScanText(&buf, words, false)
	out := buf.String()
	if !strings.Contains(out, "## shell-file") || !strings.Contains(out, "## deferred-list-var") {
		t.Errorf("expected origin headers; got:\n%s", out)
	}
	if !strings.Contains(out, "/u/.zprofile:46") || !strings.Contains(out, "/u/.zshrc:12") {
		t.Errorf("expected source columns; got:\n%s", out)
	}
	if strings.Contains(out, "/opt/java") {
		t.Errorf("values should be hidden without --values; got:\n%s", out)
	}

	buf.Reset()
	emitScanText(&buf, words, true)
	if !strings.Contains(buf.String(), "/opt/java") {
		t.Errorf("values should appear with showValues=true; got:\n%s", buf.String())
	}
}

func TestEmitScanJSON(t *testing.T) {
	words := []model.EnWord{
		{Name: "FOO", Origin: model.OriginShellFile, Source: "/x:1", Value: "bar"},
	}
	var stdout, stderr bytes.Buffer

	code := emitScanJSON(&stdout, &stderr, words, false)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	var got []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, stdout.String())
	}
	if got[0]["name"] != "FOO" {
		t.Errorf("expected name=FOO; got %v", got[0])
	}
	if _, ok := got[0]["value"]; ok {
		t.Errorf("value should be omitted without showValues; got %v", got[0])
	}

	stdout.Reset()
	stderr.Reset()
	_ = emitScanJSON(&stdout, &stderr, words, true)
	_ = json.Unmarshal(stdout.Bytes(), &got)
	if got[0]["value"] != "bar" {
		t.Errorf("value should appear with showValues; got %v", got[0])
	}
}

func TestRun_Help(t *testing.T) {
	cases := []string{"-h", "--help", "help"}
	for _, arg := range cases {
		t.Run(arg, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run([]string{arg}, &stdout, &stderr)
			if code != 0 {
				t.Errorf("expected 0, got %d", code)
			}
			if !strings.Contains(stdout.String(), "envocabulary") {
				t.Errorf("expected usage banner; got:\n%s", stdout.String())
			}
		})
	}
}

func TestRun_Version(t *testing.T) {
	cases := []string{"-V", "--version", "version"}
	for _, arg := range cases {
		t.Run(arg, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run([]string{arg}, &stdout, &stderr)
			if code != 0 {
				t.Errorf("expected 0, got %d", code)
			}
			if !strings.Contains(stdout.String(), "envocabulary ") {
				t.Errorf("expected version line; got:\n%s", stdout.String())
			}
		})
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"flarp"}, &stdout, &stderr)
	if code != 2 {
		t.Errorf("expected 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Errorf("expected unknown-command error; got:\n%s", stderr.String())
	}
}

func TestUsageAndHelpFunctionsAllOutput(t *testing.T) {
	helpers := []func(io.Writer){usage, helpScan, helpExplain, helpInventory, helpCatalog, helpDedup, helpDangling, helpLost, helpClean, helpReport}
	for i, h := range helpers {
		var buf bytes.Buffer
		h(&buf)
		if buf.Len() == 0 {
			t.Errorf("helper %d produced empty output", i)
		}
	}
}

func setupFakeShellHome(t *testing.T, files map[string]string) {
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
}

func TestRunInventory_BasicCanonicalZsh(t *testing.T) {
	setupFakeShellHome(t, map[string]string{
		".zshrc": "export FOO=1\nalias ll='ls'\n",
	})
	var stdout, stderr bytes.Buffer
	code := runInventory(nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), ".zshrc") || !strings.Contains(stdout.String(), "FOO") {
		t.Errorf("expected file + items; got:\n%s", stdout.String())
	}
}

func TestRunInventory_NoFiles(t *testing.T) {
	setupFakeShellHome(t, nil)
	var stdout, stderr bytes.Buffer
	code := runInventory(nil, &stdout, &stderr)
	if code != 0 {
		t.Errorf("expected 0 even with no files, got %d", code)
	}
	if !strings.Contains(stderr.String(), "no shell config files found") {
		t.Errorf("expected empty-state notice; got stderr=%q", stderr.String())
	}
}

func TestRunCatalog_Basic(t *testing.T) {
	setupFakeShellHome(t, map[string]string{
		".zshrc": "export FOO=1\n",
	})
	var stdout, stderr bytes.Buffer
	code := runCatalog(nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "export FOO=1") {
		t.Errorf("expected file content; got:\n%s", stdout.String())
	}
}

func TestRunCatalog_LineNumbers(t *testing.T) {
	setupFakeShellHome(t, map[string]string{
		".zshrc": "a\nb\n",
	})
	var stdout, stderr bytes.Buffer
	code := runCatalog([]string{"-n"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("got %d", code)
	}
	if !strings.Contains(stdout.String(), "    1  a") {
		t.Errorf("expected numbered output; got:\n%s", stdout.String())
	}
}

func TestRunCatalog_DedupAnnotation(t *testing.T) {
	setupFakeShellHome(t, map[string]string{
		".zprofile": "export FOO=first\n",
		".zshrc":    "export FOO=second\n",
	})
	var stdout, stderr bytes.Buffer
	code := runCatalog([]string{"--dedup"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("got %d", code)
	}
	if !strings.Contains(stdout.String(), "# [overridden by") {
		t.Errorf("expected dedup annotation; got:\n%s", stdout.String())
	}
}

func TestRunDedup_NoDuplicates(t *testing.T) {
	setupFakeShellHome(t, map[string]string{
		".zshrc": "export A=1\nexport B=2\n",
	})
	var stdout, stderr bytes.Buffer
	code := runDedup(nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("got %d", code)
	}
	if !strings.Contains(stdout.String(), "no duplicates found") {
		t.Errorf("expected empty-state notice; got:\n%s", stdout.String())
	}
}

func TestRunDedup_BashAndOrphansFlags(t *testing.T) {
	setupFakeShellHome(t, map[string]string{
		".zshrc":        "export FOO=zsh\n",
		".bashrc":       "export FOO=bash\n",
		".zshrc.backup": "export FOO=backup\n",
	})
	var stdout, stderr bytes.Buffer
	code := runDedup([]string{"--bash", "--orphans"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("got %d", code)
	}
	if !strings.Contains(stdout.String(), "FOO") {
		t.Errorf("expected FOO duplicates across zsh/bash/orphan; got:\n%s", stdout.String())
	}
}

func TestRunDedup_FindsDuplicates(t *testing.T) {
	setupFakeShellHome(t, map[string]string{
		".zprofile": "export FOO=first\n",
		".zshrc":    "export FOO=second\n",
	})
	var stdout, stderr bytes.Buffer
	code := runDedup(nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("got %d", code)
	}
	if !strings.Contains(stdout.String(), "FOO") {
		t.Errorf("expected duplicate FOO in output; got:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "winner") || !strings.Contains(stdout.String(), "loser") {
		t.Errorf("expected winner/loser markers; got:\n%s", stdout.String())
	}
}

func TestRunClean_DryRunDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	content := "# export REMOVED=1\nexport KEPT=1\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runClean([]string{path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("got %d", code)
	}
	if !strings.Contains(stdout.String(), "- ") {
		t.Errorf("expected diff line for stripped item; got:\n%s", stdout.String())
	}
	if strings.Contains(stdout.String(), "export KEPT=1") {
		t.Errorf("default mode should not emit cleaned content; got:\n%s", stdout.String())
	}
}

func TestRunClean_FullMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	content := "# export REMOVED=1\nexport KEPT=1\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runClean([]string{"--full", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("got %d", code)
	}
	if !strings.Contains(stdout.String(), "export KEPT=1") {
		t.Errorf("full mode should emit cleaned content; got:\n%s", stdout.String())
	}
	if strings.Contains(stdout.String(), "REMOVED") {
		t.Errorf("stripped lines should not appear in full output; got:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "stripped (--full mode)") {
		t.Errorf("expected stats line on stderr; got:\n%s", stderr.String())
	}
}

func TestRunClean_NoFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runClean(nil, &stdout, &stderr)
	if code != 2 {
		t.Errorf("expected 2 for missing FILE arg; got %d", code)
	}
}

func TestRunClean_FileNotFound(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runClean([]string{"/nonexistent/path/to/nothing"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected 1 for nonexistent file; got %d", code)
	}
	if !strings.Contains(stderr.String(), "error:") {
		t.Errorf("expected error on stderr; got:\n%s", stderr.String())
	}
}

func TestRunExplain_NoName(t *testing.T) {
	var stdout, stderr bytes.Buffer
	t.Setenv("HOME", t.TempDir())
	code := runExplain(nil, &stdout, &stderr)
	if code != 2 {
		t.Errorf("expected 2 for missing NAME; got %d", code)
	}
}

func TestRun_DispatchToInventory(t *testing.T) {
	setupFakeShellHome(t, map[string]string{".zshrc": "export X=1\n"})
	var stdout, stderr bytes.Buffer
	code := run([]string{"inventory"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("expected 0; got %d", code)
	}
	if !strings.Contains(stdout.String(), ".zshrc") {
		t.Errorf("expected inventory output; got:\n%s", stdout.String())
	}
}

func TestRun_DispatchToDedup(t *testing.T) {
	setupFakeShellHome(t, map[string]string{".zshrc": "export X=1\n"})
	var stdout, stderr bytes.Buffer
	code := run([]string{"dedup"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("expected 0; got %d", code)
	}
	if !strings.Contains(stdout.String(), "no duplicates") {
		t.Errorf("expected dedup empty-state output; got:\n%s", stdout.String())
	}
}

func TestRun_DispatchToClean(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte("export X=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"clean", path}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("expected 0; got %d", code)
	}
}

func TestRun_LeadingDashNonHelpFallsThroughToScanArgs(t *testing.T) {
	t.Setenv("PATH", "")
	var stdout, stderr bytes.Buffer
	code := run([]string{"--bogus-flag"}, &stdout, &stderr)
	if code == 0 {
		t.Errorf("expected non-zero exit from leading-dash dispatch; got %d", code)
	}
}

func TestRunInventory_FlagParseError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runInventory([]string{"--bogus"}, &stdout, &stderr)
	if code != 2 {
		t.Errorf("expected 2 for unknown flag; got %d", code)
	}
}

func TestRunCatalog_FlagParseError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runCatalog([]string{"--bogus"}, &stdout, &stderr)
	if code != 2 {
		t.Errorf("expected 2 for unknown flag; got %d", code)
	}
}

func TestRunDedup_FlagParseError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runDedup([]string{"--bogus"}, &stdout, &stderr)
	if code != 2 {
		t.Errorf("expected 2 for unknown flag; got %d", code)
	}
}

func TestRunClean_FlagParseError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runClean([]string{"--bogus"}, &stdout, &stderr)
	if code != 2 {
		t.Errorf("expected 2 for unknown flag; got %d", code)
	}
}

func TestRun_DispatchToCatalog(t *testing.T) {
	setupFakeShellHome(t, map[string]string{".zshrc": "export X=1\n"})
	var stdout, stderr bytes.Buffer
	code := run([]string{"catalog"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("expected 0; got %d", code)
	}
	if !strings.Contains(stdout.String(), "export X=1") {
		t.Errorf("expected catalog output; got:\n%s", stdout.String())
	}
}

func TestRunDangling_NoneFound(t *testing.T) {
	dir := t.TempDir()
	realPath := filepath.Join(dir, "real.sh")
	if err := os.WriteFile(realPath, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	setupFakeShellHome(t, map[string]string{
		".zshrc": "source " + realPath + "\nexport EDITOR=vim\n",
	})
	var stdout, stderr bytes.Buffer
	code := runDangling(nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("got exit %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "no dangling references found") {
		t.Errorf("expected empty-state notice; got:\n%s", stdout.String())
	}
}

func TestRunDangling_FindsDangling(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "ghost.sh")
	setupFakeShellHome(t, map[string]string{
		".zshrc": "source " + missing + "\nexport JAVA_HOME=" + missing + "\n",
	})
	var stdout, stderr bytes.Buffer
	code := runDangling(nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("got exit %d, want 1 (dangling found)", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "source target missing") {
		t.Errorf("expected source-missing reason; got:\n%s", out)
	}
	if !strings.Contains(out, "path does not exist") {
		t.Errorf("expected path-missing reason; got:\n%s", out)
	}
	if !strings.Contains(out, "JAVA_HOME") {
		t.Errorf("expected JAVA_HOME in output; got:\n%s", out)
	}
}

func TestRunDangling_BashAndOrphansFlags(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "gone")
	setupFakeShellHome(t, map[string]string{
		".bashrc":       "export FROM_BASH=" + missing + "\n",
		".zshrc.backup": "export FROM_ORPHAN=" + missing + "\n",
	})
	var stdout, stderr bytes.Buffer
	code := runDangling([]string{"--bash", "--orphans"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("got exit %d, want 1", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "FROM_BASH") || !strings.Contains(out, "FROM_ORPHAN") {
		t.Errorf("expected both bash and orphan findings; got:\n%s", out)
	}
}

func TestRunDangling_FlagParseError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runDangling([]string{"--bogus"}, &stdout, &stderr)
	if code != 2 {
		t.Errorf("expected 2 for unknown flag; got %d", code)
	}
}

func TestEmitDanglingText(t *testing.T) {
	findings := []dangling.Finding{
		{File: "/u/.zshrc", Line: 3, Kind: inventory.KindSource, Name: "~/gone.sh", Value: "~/gone.sh", Reason: dangling.ReasonSourceMissing},
		{File: "/u/.zshrc", Line: 7, Kind: inventory.KindExport, Name: "JAVA_HOME", Value: "/opt/gone", Reason: dangling.ReasonPathMissing},
		{File: "/u/.zprofile", Line: 1, Kind: inventory.KindExport, Name: "FOO", Value: "/no/such", Reason: dangling.ReasonPathMissing},
	}
	var buf bytes.Buffer
	emitDanglingText(&buf, findings)
	out := buf.String()
	if !strings.Contains(out, "## /u/.zshrc") || !strings.Contains(out, "## /u/.zprofile") {
		t.Errorf("expected file headers; got:\n%s", out)
	}
	if !strings.Contains(out, "source target missing") || !strings.Contains(out, "path does not exist") {
		t.Errorf("expected reason strings; got:\n%s", out)
	}
	if !strings.Contains(out, "JAVA_HOME") || !strings.Contains(out, "~/gone.sh") {
		t.Errorf("expected item details; got:\n%s", out)
	}
}

func TestRun_DispatchToDangling(t *testing.T) {
	setupFakeShellHome(t, map[string]string{".zshrc": "export X=1\n"})
	var stdout, stderr bytes.Buffer
	code := run([]string{"dangling"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("expected 0; got %d", code)
	}
	if !strings.Contains(stdout.String(), "no dangling references") {
		t.Errorf("expected dangling empty-state output; got:\n%s", stdout.String())
	}
}

func TestRunLost_NoOrphans(t *testing.T) {
	setupFakeShellHome(t, map[string]string{
		".zshrc": "export FOO=1\n",
	})
	var stdout, stderr bytes.Buffer
	code := runLost(nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "no lost items found") {
		t.Errorf("expected empty-state notice; got:\n%s", stdout.String())
	}
}

func TestRunLost_FindsLostItems(t *testing.T) {
	setupFakeShellHome(t, map[string]string{
		".zshrc":        "export EDITOR=vim\nalias ll='ls -la'\n",
		".zshrc.backup": "export EDITOR=vim\nexport JAVA_HOME=/opt/java\nalias gs='git status'\n",
	})
	var stdout, stderr bytes.Buffer
	code := runLost(nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "JAVA_HOME") {
		t.Errorf("expected JAVA_HOME in lost output; got:\n%s", out)
	}
	if !strings.Contains(out, "gs") {
		t.Errorf("expected gs alias in lost output; got:\n%s", out)
	}
	if strings.Contains(out, "EDITOR") {
		t.Errorf("EDITOR should not be lost (present in canonical); got:\n%s", out)
	}
}

func TestRunLost_BashFlag(t *testing.T) {
	setupFakeShellHome(t, map[string]string{
		".bashrc":       "export FROM_BASH=1\n",
		".zshrc.backup": "export FROM_BASH=1\nexport ONLY_ORPHAN=1\n",
	})
	var stdout, stderr bytes.Buffer
	code := runLost([]string{"--bash"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d", code)
	}
	out := stdout.String()
	if strings.Contains(out, "FROM_BASH") {
		t.Errorf("FROM_BASH should not be lost with --bash (present in .bashrc); got:\n%s", out)
	}
	if !strings.Contains(out, "ONLY_ORPHAN") {
		t.Errorf("expected ONLY_ORPHAN in lost output; got:\n%s", out)
	}
}

func TestRunLost_FlagParseError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runLost([]string{"--bogus"}, &stdout, &stderr)
	if code != 2 {
		t.Errorf("expected 2 for unknown flag; got %d", code)
	}
}

func TestRun_DispatchToLost(t *testing.T) {
	setupFakeShellHome(t, map[string]string{".zshrc": "export X=1\n"})
	var stdout, stderr bytes.Buffer
	code := run([]string{"lost"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("expected 0; got %d", code)
	}
	if !strings.Contains(stdout.String(), "no lost items") {
		t.Errorf("expected lost empty-state output; got:\n%s", stdout.String())
	}
}

func TestEmitLostText(t *testing.T) {
	findings := []lost.Finding{
		{File: "/u/.zshrc.backup", Kind: inventory.KindExport, Name: "JAVA_HOME", Line: 5},
		{File: "/u/.zshrc.backup", Kind: inventory.KindAlias, Name: "gs", Line: 7},
		{File: "/u/.zshrc.old", Kind: inventory.KindFunction, Name: "myfunc", Line: 10},
	}
	var buf bytes.Buffer
	emitLostText(&buf, findings)
	out := buf.String()
	if !strings.Contains(out, "## /u/.zshrc.backup") || !strings.Contains(out, "## /u/.zshrc.old") {
		t.Errorf("expected file headers; got:\n%s", out)
	}
	if !strings.Contains(out, "JAVA_HOME") || !strings.Contains(out, "gs") || !strings.Contains(out, "myfunc") {
		t.Errorf("expected item names; got:\n%s", out)
	}
	if !strings.Contains(out, ":5") || !strings.Contains(out, ":7") || !strings.Contains(out, ":10") {
		t.Errorf("expected line numbers; got:\n%s", out)
	}
}

func TestRunReport_TextDefault(t *testing.T) {
	setupFakeShellHome(t, map[string]string{
		".zprofile": "export EDITOR=nvim\n",
		".zshrc":    "export EDITOR=nvim\n",
	})
	var stdout, stderr bytes.Buffer
	code := runReport(nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d", code)
	}
	out := stdout.String()
	if !strings.Contains(out, "SAFE TO DELETE") {
		t.Errorf("expected SAFE TO DELETE section; got:\n%s", out)
	}
	if !strings.Contains(out, "EDITOR") {
		t.Errorf("expected EDITOR in output; got:\n%s", out)
	}
}

func TestRunReport_HTML(t *testing.T) {
	setupFakeShellHome(t, map[string]string{
		".zshrc": "export FOO=1\n",
	})
	dir := t.TempDir()
	t.Chdir(dir)

	var stdout, stderr bytes.Buffer
	code := runReport([]string{"--html"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d", code)
	}
	filename := strings.TrimSpace(stdout.String())
	if !strings.HasSuffix(filename, ".html") {
		t.Errorf("expected .html filename; got %q", filename)
	}
	data, err := os.ReadFile(filepath.Join(dir, filename))
	if err != nil {
		t.Fatalf("expected HTML file to exist: %v", err)
	}
	if !strings.Contains(string(data), "<!DOCTYPE html>") {
		t.Error("expected valid HTML content")
	}
}

func TestRunReport_BashFlag(t *testing.T) {
	setupFakeShellHome(t, map[string]string{
		".zshrc":  "export FOO=zsh\n",
		".bashrc": "export FOO=bash\n",
	})
	var stdout, stderr bytes.Buffer
	code := runReport([]string{"--bash"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "FOO") {
		t.Errorf("expected FOO in output with --bash; got:\n%s", stdout.String())
	}
}

func TestRunReport_FlagParseError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runReport([]string{"--bogus"}, &stdout, &stderr)
	if code != 2 {
		t.Errorf("expected 2 for unknown flag; got %d", code)
	}
}

func TestRun_DispatchToReport(t *testing.T) {
	setupFakeShellHome(t, map[string]string{".zshrc": "export X=1\n"})
	var stdout, stderr bytes.Buffer
	code := run([]string{"report"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("expected 0; got %d", code)
	}
	if !strings.Contains(stdout.String(), "SAFE TO DELETE") {
		t.Errorf("expected report output; got:\n%s", stdout.String())
	}
}

func stubDiscoverError(t *testing.T) {
	t.Helper()
	orig := inventory.Discover
	t.Cleanup(func() { inventory.Discover = orig })
	inventory.Discover = func() ([]inventory.File, error) {
		return nil, errors.New("mock discover error")
	}
}

func TestRunInventory_DiscoverError(t *testing.T) {
	stubDiscoverError(t)
	var stdout, stderr bytes.Buffer
	code := runInventory(nil, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "mock discover error") {
		t.Errorf("expected error on stderr; got %q", stderr.String())
	}
}

func TestRunDedup_DiscoverError(t *testing.T) {
	stubDiscoverError(t)
	var stdout, stderr bytes.Buffer
	code := runDedup(nil, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "mock discover error") {
		t.Errorf("expected error on stderr; got %q", stderr.String())
	}
}

func TestRunDangling_DiscoverError(t *testing.T) {
	stubDiscoverError(t)
	var stdout, stderr bytes.Buffer
	code := runDangling(nil, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "mock discover error") {
		t.Errorf("expected error on stderr; got %q", stderr.String())
	}
}

func TestRunLost_DiscoverError(t *testing.T) {
	stubDiscoverError(t)
	var stdout, stderr bytes.Buffer
	code := runLost(nil, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "mock discover error") {
		t.Errorf("expected error on stderr; got %q", stderr.String())
	}
}

func TestRunReport_DiscoverError(t *testing.T) {
	stubDiscoverError(t)
	var stdout, stderr bytes.Buffer
	code := runReport(nil, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "mock discover error") {
		t.Errorf("expected error on stderr; got %q", stderr.String())
	}
}
