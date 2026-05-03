package pathentry

import (
	"errors"
	"os"
	"testing"

	"github.com/sreckoskocilic/envocabulary/internal/model"
)

func TestExtractValue(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		varN string
		want string
	}{
		{"export", "export PATH=/usr/bin:/usr/local/bin", "PATH", "/usr/bin:/usr/local/bin"},
		{"bare", "PATH=/usr/bin", "PATH", "/usr/bin"},
		{"typeset", "typeset -gx PATH=/usr/bin", "PATH", "/usr/bin"},
		{"quoted double", `export PATH="/usr/bin"`, "PATH", "/usr/bin"},
		{"quoted single", `export PATH='/usr/bin'`, "PATH", "/usr/bin"},
		{"empty value", "export PATH=", "PATH", ""},
		{"not found", "export FOO=bar", "PATH", ""},
		{"value with equals", "export PATH=/a=b:/c", "PATH", "/a=b:/c"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractValue(tc.raw, tc.varN)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSplitPath(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", nil},
		{"single", "/usr/bin", []string{"/usr/bin"}},
		{"multiple", "/usr/bin:/usr/local/bin", []string{"/usr/bin", "/usr/local/bin"}},
		{"skip empty middle", "/usr/bin::/usr/local/bin", []string{"/usr/bin", "/usr/local/bin"}},
		{"skip leading trailing", ":/usr/bin:", []string{"/usr/bin"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := splitPath(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("len: got %d, want %d (%v)", len(got), len(tc.want), got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("[%d]: got %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestAttribute_SingleWriter(t *testing.T) {
	trace := []model.TraceEntry{
		{Name: "PATH", File: "/u/.zshrc", Line: 3, Raw: "export PATH=/usr/bin:/usr/local/bin"},
	}
	r := Attribute("PATH", "/usr/bin:/usr/local/bin", trace)
	if r.Name != "PATH" {
		t.Errorf("name = %q", r.Name)
	}
	if len(r.Entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(r.Entries))
	}
	for _, e := range r.Entries {
		if e.File != "/u/.zshrc" || e.Line != 3 {
			t.Errorf("entry %+v: want /u/.zshrc:3", e)
		}
	}
}

func TestAttribute_AppendPattern(t *testing.T) {
	trace := []model.TraceEntry{
		{Name: "PATH", File: "/u/.zprofile", Line: 3, Raw: "export PATH=/usr/bin:/usr/local/bin"},
		{Name: "PATH", File: "/u/.zshrc", Line: 10, Raw: "export PATH=/usr/bin:/usr/local/bin:/opt/homebrew/bin"},
	}
	r := Attribute("PATH", "/usr/bin:/usr/local/bin:/opt/homebrew/bin", trace)
	if len(r.Entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(r.Entries))
	}
	if r.Entries[0].File != "/u/.zprofile" {
		t.Errorf("/usr/bin: got %s, want /u/.zprofile", r.Entries[0].File)
	}
	if r.Entries[1].File != "/u/.zprofile" {
		t.Errorf("/usr/local/bin: got %s, want /u/.zprofile", r.Entries[1].File)
	}
	if r.Entries[2].File != "/u/.zshrc" || r.Entries[2].Line != 10 {
		t.Errorf("/opt/homebrew/bin: got %s:%d, want /u/.zshrc:10", r.Entries[2].File, r.Entries[2].Line)
	}
}

func TestAttribute_PrependPattern(t *testing.T) {
	trace := []model.TraceEntry{
		{Name: "PATH", File: "/u/.zprofile", Line: 3, Raw: "export PATH=/usr/bin:/usr/local/bin"},
		{Name: "PATH", File: "/u/.zshrc", Line: 10, Raw: "export PATH=/opt/homebrew/bin:/usr/bin:/usr/local/bin"},
	}
	r := Attribute("PATH", "/opt/homebrew/bin:/usr/bin:/usr/local/bin", trace)
	if len(r.Entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(r.Entries))
	}
	if r.Entries[0].File != "/u/.zshrc" {
		t.Errorf("/opt/homebrew/bin: got %s, want /u/.zshrc", r.Entries[0].File)
	}
	if r.Entries[1].File != "/u/.zprofile" {
		t.Errorf("/usr/bin: got %s, want /u/.zprofile", r.Entries[1].File)
	}
}

func TestAttribute_ReintroducedEntry(t *testing.T) {
	trace := []model.TraceEntry{
		{Name: "PATH", File: "/u/.zprofile", Line: 1, Raw: "PATH=/a:/b"},
		{Name: "PATH", File: "/u/.zshrc", Line: 1, Raw: "PATH=/c"},
		{Name: "PATH", File: "/u/.zshrc", Line: 5, Raw: "PATH=/c:/a"},
	}
	r := Attribute("PATH", "/c:/a", trace)
	if len(r.Entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(r.Entries))
	}
	if r.Entries[0].File != "/u/.zshrc" || r.Entries[0].Line != 1 {
		t.Errorf("/c: got %s:%d, want /u/.zshrc:1", r.Entries[0].File, r.Entries[0].Line)
	}
	if r.Entries[1].File != "/u/.zshrc" || r.Entries[1].Line != 5 {
		t.Errorf("/a: got %s:%d, want /u/.zshrc:5", r.Entries[1].File, r.Entries[1].Line)
	}
}

func TestAttribute_InheritedEntries(t *testing.T) {
	r := Attribute("PATH", "/usr/bin:/usr/local/bin", nil)
	if len(r.Entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(r.Entries))
	}
	for _, e := range r.Entries {
		if e.File != "" {
			t.Errorf("expected empty File for inherited, got %q", e.File)
		}
	}
}

func TestAttribute_MixedInheritedAndTraced(t *testing.T) {
	trace := []model.TraceEntry{
		{Name: "PATH", File: "/u/.zshrc", Line: 5, Raw: "export PATH=/usr/bin:/opt/new"},
	}
	r := Attribute("PATH", "/usr/bin:/opt/new:/inherited", trace)
	if len(r.Entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(r.Entries))
	}
	if r.Entries[0].File != "/u/.zshrc" {
		t.Errorf("/usr/bin: got file %q, want /u/.zshrc", r.Entries[0].File)
	}
	if r.Entries[2].File != "" {
		t.Errorf("/inherited: got file %q, want empty (inherited)", r.Entries[2].File)
	}
}

func TestAttribute_WithChain(t *testing.T) {
	trace := []model.TraceEntry{
		{Name: "PATH", File: "/u/helpers.sh", Line: 5, Raw: "export PATH=/usr/bin", Chain: []string{"/u/.zshrc"}},
	}
	r := Attribute("PATH", "/usr/bin", trace)
	if len(r.Entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(r.Entries))
	}
	if len(r.Entries[0].Chain) != 1 || r.Entries[0].Chain[0] != "/u/.zshrc" {
		t.Errorf("chain: got %v, want [/u/.zshrc]", r.Entries[0].Chain)
	}
}

func TestAttribute_ChainNotShared(t *testing.T) {
	chain := []string{"/u/.zshrc"}
	trace := []model.TraceEntry{
		{Name: "PATH", File: "/u/helpers.sh", Line: 5, Raw: "export PATH=/usr/bin", Chain: chain},
	}
	r := Attribute("PATH", "/usr/bin", trace)
	r.Entries[0].Chain[0] = "mutated"
	if chain[0] != "/u/.zshrc" {
		t.Errorf("Attribute must not share chain slices with caller")
	}
}

func TestAttribute_OtherVarsFiltered(t *testing.T) {
	trace := []model.TraceEntry{
		{Name: "FOO", File: "/u/.zshrc", Line: 1, Raw: "export FOO=bar"},
		{Name: "PATH", File: "/u/.zshrc", Line: 3, Raw: "export PATH=/usr/bin"},
	}
	r := Attribute("PATH", "/usr/bin", trace)
	if len(r.Entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(r.Entries))
	}
	if r.Entries[0].Line != 3 {
		t.Errorf("got line %d, want 3", r.Entries[0].Line)
	}
}

func TestAttribute_EmptyCurrentValue(t *testing.T) {
	r := Attribute("PATH", "", nil)
	if len(r.Entries) != 0 {
		t.Errorf("got %d entries, want 0", len(r.Entries))
	}
}

func TestAttribute_NonPathVar(t *testing.T) {
	trace := []model.TraceEntry{
		{Name: "MANPATH", File: "/u/.zshrc", Line: 7, Raw: "export MANPATH=/usr/share/man"},
	}
	r := Attribute("MANPATH", "/usr/share/man", trace)
	if len(r.Entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(r.Entries))
	}
	if r.Entries[0].Dir != "/usr/share/man" || r.Entries[0].File != "/u/.zshrc" {
		t.Errorf("got %+v", r.Entries[0])
	}
}

func TestAttribute_DuplicateEntriesInCurrentValue(t *testing.T) {
	trace := []model.TraceEntry{
		{Name: "PATH", File: "/u/.zshrc", Line: 3, Raw: "export PATH=/a:/a"},
	}
	r := Attribute("PATH", "/a:/a", trace)
	if len(r.Entries) != 2 {
		t.Fatalf("got %d entries, want 2 (preserves duplicates)", len(r.Entries))
	}
	for _, e := range r.Entries {
		if e.File != "/u/.zshrc" {
			t.Errorf("got file %q, want /u/.zshrc", e.File)
		}
	}
}

func TestCheckExists(t *testing.T) {
	existing := map[string]bool{"/usr/bin": true, "/opt/dead": false}
	orig := statDir
	statDir = func(name string) (os.FileInfo, error) {
		if existing[name] {
			return nil, nil
		}
		return nil, errors.New("not found")
	}
	t.Cleanup(func() { statDir = orig })

	entries := []Entry{
		{Dir: "/usr/bin"},
		{Dir: "/opt/dead"},
	}
	CheckExists(entries)

	if entries[0].Exists == nil || !*entries[0].Exists {
		t.Errorf("/usr/bin: want exists=true, got %v", entries[0].Exists)
	}
	if entries[1].Exists == nil || *entries[1].Exists {
		t.Errorf("/opt/dead: want exists=false, got %v", entries[1].Exists)
	}
}

func TestCheckExists_Empty(t *testing.T) {
	CheckExists(nil)
}
