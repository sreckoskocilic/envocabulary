package pathentry

import (
	"testing"

	"github.com/sreckoskocilic/envocabulary/internal/inventory"
)

func TestOverrideFromConfig(t *testing.T) {
	dead := false
	results := []VarBreakdown{
		{
			Name: "PATH",
			Entries: []Entry{
				{Dir: "/opt/dead", File: "/etc/zprofile", Line: 1, Exists: &dead},
				{Dir: "/gone/bin", File: "/etc/zprofile", Line: 1, Exists: &dead},
			},
		},
	}
	files := []inventory.File{
		{
			Path: "/u/.zshrc",
			Items: []inventory.Item{
				{Kind: inventory.KindExport, Name: "PATH", Line: 10, Value: "/opt/dead:$PATH"},
			},
		},
	}
	OverrideFromConfig(results, files)
	if results[0].Entries[0].File != "/u/.zshrc" || results[0].Entries[0].Line != 10 {
		t.Errorf("/opt/dead: got %s:%d, want /u/.zshrc:10", results[0].Entries[0].File, results[0].Entries[0].Line)
	}
	if results[0].Entries[1].File != "/etc/zprofile" {
		t.Errorf("/gone/bin: should keep original when no config match; got %s", results[0].Entries[1].File)
	}
}

func TestOverrideFromConfig_SkipsNonExportAssign(t *testing.T) {
	dead := false
	results := []VarBreakdown{
		{
			Name: "PATH",
			Entries: []Entry{
				{Dir: "/opt/dead", File: "/etc/zprofile", Line: 1, Exists: &dead},
			},
		},
	}
	files := []inventory.File{
		{
			Path: "/u/.zshrc",
			Items: []inventory.Item{
				{Kind: inventory.KindAlias, Name: "mypath", Line: 5, Value: "/opt/dead"},
				{Kind: inventory.KindSource, Name: "/opt/dead/init.sh", Line: 7},
			},
		},
	}
	OverrideFromConfig(results, files)
	if results[0].Entries[0].File != "/etc/zprofile" {
		t.Errorf("should not match alias/source; got %s", results[0].Entries[0].File)
	}
}

func TestOverrideFromConfig_NoSubstringFalsePositive(t *testing.T) {
	dead := false
	results := []VarBreakdown{
		{
			Name: "PATH",
			Entries: []Entry{
				{Dir: "/opt/homebrew", File: "/etc/zprofile", Line: 1, Exists: &dead},
			},
		},
	}
	files := []inventory.File{
		{
			Path: "/u/.zshrc",
			Items: []inventory.Item{
				{Kind: inventory.KindExport, Name: "PATH", Line: 10, Value: "/opt/homebrew/bin:$PATH"},
			},
		},
	}
	OverrideFromConfig(results, files)
	if results[0].Entries[0].File != "/etc/zprofile" {
		t.Errorf("/opt/homebrew should not match /opt/homebrew/bin; got %s:%d", results[0].Entries[0].File, results[0].Entries[0].Line)
	}
}

func TestOverrideFromConfig_IgnoresCrossVarMatch(t *testing.T) {
	dead := false
	results := []VarBreakdown{
		{
			Name: "PATH",
			Entries: []Entry{
				{Dir: "/usr/local/go", File: "/etc/zprofile", Line: 1, Exists: &dead},
			},
		},
	}
	files := []inventory.File{
		{
			Path: "/u/.zshrc",
			Items: []inventory.Item{
				{Kind: inventory.KindExport, Name: "GOPATH", Line: 5, Value: "/usr/local/go"},
			},
		},
	}
	OverrideFromConfig(results, files)
	if results[0].Entries[0].File != "/etc/zprofile" {
		t.Errorf("should not match GOPATH export for PATH entry; got %s:%d", results[0].Entries[0].File, results[0].Entries[0].Line)
	}
}

func TestOverrideFromConfig_LastWriterWins(t *testing.T) {
	dead := false
	results := []VarBreakdown{
		{
			Name: "PATH",
			Entries: []Entry{
				{Dir: "/opt/bin", File: "/etc/zprofile", Line: 1, Exists: &dead},
			},
		},
	}
	files := []inventory.File{
		{
			Path: "/u/.zshenv",
			Items: []inventory.Item{
				{Kind: inventory.KindExport, Name: "PATH", Line: 3, Value: "/opt/bin:$PATH"},
			},
		},
		{
			Path: "/u/.zshrc",
			Items: []inventory.Item{
				{Kind: inventory.KindExport, Name: "PATH", Line: 7, Value: "/opt/bin:/usr/bin"},
			},
		},
	}
	OverrideFromConfig(results, files)
	if results[0].Entries[0].File != "/u/.zshrc" || results[0].Entries[0].Line != 7 {
		t.Errorf("should attribute to last writer; got %s:%d, want /u/.zshrc:7", results[0].Entries[0].File, results[0].Entries[0].Line)
	}
}

func TestValueContainsDir(t *testing.T) {
	cases := []struct {
		value, dir string
		want       bool
	}{
		{"/opt/dead:$PATH", "/opt/dead", true},
		{"/opt/homebrew/bin:$PATH", "/opt/homebrew", false},
		{"/usr/bin:/usr/local/bin", "/usr/bin", true},
		{"/usr/local/bin", "/usr", false},
		{"/opt/dead", "/opt/dead", true},
		{"", "/opt/dead", false},
		{"/opt/dead", "", false},
	}
	for _, tc := range cases {
		if got := valueContainsDir(tc.value, tc.dir); got != tc.want {
			t.Errorf("valueContainsDir(%q, %q) = %v, want %v", tc.value, tc.dir, got, tc.want)
		}
	}
}

func TestFindPathsDRef_ExactMatch(t *testing.T) {
	entry := Entry{Dir: "/opt/dead", File: "/etc/zprofile", Line: 1}
	refs := []pathsDEntry{{File: "/etc/paths.d/foo", Line: 2, Dir: "/opt/dead"}}
	findPathsDRef(&entry, "/opt/dead", refs)
	if entry.File != "/etc/paths.d/foo" || entry.Line != 2 {
		t.Errorf("got %s:%d, want /etc/paths.d/foo:2", entry.File, entry.Line)
	}
}

func TestFindPathsDRef_PrefixMatch(t *testing.T) {
	entry := Entry{Dir: "/Applications/VMware", File: "/etc/zprofile", Line: 1}
	refs := []pathsDEntry{{File: "/etc/paths.d/vmware", Line: 1, Dir: "/Applications/VMware Fusion.app/Contents/Public"}}
	findPathsDRef(&entry, "/Applications/VMware", refs)
	if entry.File != "/etc/paths.d/vmware" || entry.Line != 1 {
		t.Errorf("got %s:%d, want /etc/paths.d/vmware:1", entry.File, entry.Line)
	}
}

func TestFindPathsDRef_NoMatch(t *testing.T) {
	entry := Entry{Dir: "/nope", File: "/etc/zprofile", Line: 1}
	refs := []pathsDEntry{{File: "/etc/paths.d/foo", Line: 1, Dir: "/opt/other"}}
	findPathsDRef(&entry, "/nope", refs)
	if entry.File != "/etc/zprofile" {
		t.Errorf("should keep original; got %s", entry.File)
	}
}

func TestOverrideFromConfig_FallsBackToPathsD(t *testing.T) {
	dead := false
	results := []VarBreakdown{
		{
			Name: "PATH",
			Entries: []Entry{
				{Dir: "/opt/system", File: "/etc/zprofile", Line: 1, Exists: &dead},
			},
		},
	}
	orig := scanPathsD
	t.Cleanup(func() { scanPathsD = orig })
	scanPathsD = func() []pathsDEntry {
		return []pathsDEntry{{File: "/etc/paths.d/sys", Line: 1, Dir: "/opt/system"}}
	}
	OverrideFromConfig(results, nil)
	if results[0].Entries[0].File != "/etc/paths.d/sys" {
		t.Errorf("expected paths.d fallback; got %s", results[0].Entries[0].File)
	}
}
