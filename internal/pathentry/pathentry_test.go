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
		{"substring reject", "export CLASSPATH=/opt/java/lib", "PATH", ""},
		{"substring reject GOPATH", "GOPATH=/go/path", "PATH", ""},
		{"quoted with trailing comment", `export PATH="/usr/bin:/usr/local/bin" # setup`, "PATH", "/usr/bin:/usr/local/bin"},
		{"single quoted with trailing", `export PATH='/usr/bin' # comment`, "PATH", "/usr/bin"},
		{"unclosed quote", `export PATH="/usr/bin`, "PATH", "/usr/bin"},
		{"word boundary after space", `export CLASSPATH=/java PATH=/usr/bin`, "PATH", "/usr/bin"},
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
	r := Attribute("PATH", "/usr/bin:/usr/local/bin", "", trace)
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
	r := Attribute("PATH", "/usr/bin:/usr/local/bin:/opt/homebrew/bin", "", trace)
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
	r := Attribute("PATH", "/opt/homebrew/bin:/usr/bin:/usr/local/bin", "", trace)
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
	r := Attribute("PATH", "/c:/a", "", trace)
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
	r := Attribute("PATH", "/usr/bin:/usr/local/bin", "", nil)
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
	r := Attribute("PATH", "/usr/bin:/opt/new:/inherited", "", trace)
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
	r := Attribute("PATH", "/usr/bin", "", trace)
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
	r := Attribute("PATH", "/usr/bin", "", trace)

	r.Entries[0].Chain[0] = "mutated"
	if chain[0] != "/u/.zshrc" {
		t.Errorf("mutating output chain must not affect caller input")
	}

	chain2 := []string{"/u/.zshrc"}
	trace2 := []model.TraceEntry{
		{Name: "PATH", File: "/u/helpers.sh", Line: 5, Raw: "export PATH=/usr/bin", Chain: chain2},
	}
	r2 := Attribute("PATH", "/usr/bin", "", trace2)
	chain2[0] = "mutated"
	if r2.Entries[0].Chain[0] != "/u/.zshrc" {
		t.Errorf("mutating input chain after call must not affect output")
	}
}

func TestAttribute_OtherVarsFiltered(t *testing.T) {
	trace := []model.TraceEntry{
		{Name: "FOO", File: "/u/.zshrc", Line: 1, Raw: "export FOO=bar"},
		{Name: "PATH", File: "/u/.zshrc", Line: 3, Raw: "export PATH=/usr/bin"},
	}
	r := Attribute("PATH", "/usr/bin", "", trace)
	if len(r.Entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(r.Entries))
	}
	if r.Entries[0].Line != 3 {
		t.Errorf("got line %d, want 3", r.Entries[0].Line)
	}
}

func TestAttribute_InitialValueIsNotAttributed(t *testing.T) {
	trace := []model.TraceEntry{
		{File: "/etc/zprofile", Line: 1, Name: "PATH", Raw: `PATH="/usr/bin:/bin:/opt/new"`},
	}
	r := Attribute("PATH", "/usr/bin:/bin:/opt/new", "/usr/bin:/bin", trace)
	byDir := map[string]Entry{}
	for _, e := range r.Entries {
		byDir[e.Dir] = e
	}
	for _, d := range []string{"/usr/bin", "/bin"} {
		if byDir[d].File != "" {
			t.Errorf("%s predates the login chain; it must not be credited to %s:%d",
				d, byDir[d].File, byDir[d].Line)
		}
	}
	if byDir["/opt/new"].File != "/etc/zprofile" {
		t.Errorf("/opt/new is genuinely new, expected /etc/zprofile, got %q", byDir["/opt/new"].File)
	}
}

func TestAttribute_EmptyCurrentValue(t *testing.T) {
	r := Attribute("PATH", "", "", nil)
	if len(r.Entries) != 0 {
		t.Errorf("got %d entries, want 0", len(r.Entries))
	}
}

func TestAttribute_NonPathVar(t *testing.T) {
	trace := []model.TraceEntry{
		{Name: "MANPATH", File: "/u/.zshrc", Line: 7, Raw: "export MANPATH=/usr/share/man"},
	}
	r := Attribute("MANPATH", "/usr/share/man", "", trace)
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
	r := Attribute("PATH", "/a:/a", "", trace)
	if len(r.Entries) != 2 {
		t.Fatalf("got %d entries, want 2 (preserves duplicates)", len(r.Entries))
	}
	for _, e := range r.Entries {
		if e.File != "/u/.zshrc" {
			t.Errorf("got file %q, want /u/.zshrc", e.File)
		}
	}
}

func TestAttribute_NestedTraceMarkerIgnored(t *testing.T) {
	trace := []model.TraceEntry{
		{Name: "PATH", File: "/u/.zshrc", Line: 63, Raw: "export PATH=/u/go/bin:/usr/bin"},
		{Name: "PATH", File: "/u/nvm.sh", Line: 904, Raw: "PATH=+/u/nvm.sh:904> nvm_change_path /u/go/bin:/usr/bin"},
		{Name: "PATH", File: "/u/nvm.sh", Line: 904, Raw: "PATH=/u/node/bin:/u/go/bin:/usr/bin"},
	}
	r := Attribute("PATH", "/u/node/bin:/u/go/bin:/usr/bin", "/usr/bin", trace)
	if len(r.Entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(r.Entries))
	}
	if r.Entries[0].File != "/u/nvm.sh" || r.Entries[0].Line != 904 {
		t.Errorf("/u/node/bin: got %s:%d, want /u/nvm.sh:904", r.Entries[0].File, r.Entries[0].Line)
	}
	if r.Entries[1].File != "/u/.zshrc" || r.Entries[1].Line != 63 {
		t.Errorf("/u/go/bin: got %s:%d, want /u/.zshrc:63", r.Entries[1].File, r.Entries[1].Line)
	}
	if r.Entries[2].File != "" {
		t.Errorf("/usr/bin predates the chain, got %s:%d", r.Entries[2].File, r.Entries[2].Line)
	}
}

func TestAttribute_RepeatedPrependGetsOwnWriter(t *testing.T) {
	trace := []model.TraceEntry{
		{Name: "PATH", File: "/u/.zshrc", Line: 22, Raw: "export PATH=/u/.local/bin:/usr/bin"},
		{Name: "PATH", File: "/u/.zshrc", Line: 25, Raw: "export PATH=/u/.cargo/bin:/u/.local/bin:/usr/bin"},
		{Name: "PATH", File: "/u/.zshrc", Line: 70, Raw: "export PATH=/u/.local/bin:/u/.cargo/bin:/u/.local/bin:/usr/bin"},
	}
	r := Attribute("PATH", "/u/.local/bin:/u/.cargo/bin:/u/.local/bin:/usr/bin", "/usr/bin", trace)
	if len(r.Entries) != 4 {
		t.Fatalf("got %d entries, want 4", len(r.Entries))
	}
	if r.Entries[0].Line != 70 {
		t.Errorf("leading /u/.local/bin was re-added at line 70, got line %d", r.Entries[0].Line)
	}
	if r.Entries[2].Line != 22 {
		t.Errorf("trailing /u/.local/bin came from line 22, got line %d", r.Entries[2].Line)
	}
}

func TestAlignGreedyFallback(t *testing.T) {
	a := []string{"/a", "/b", "/c"}
	b := []string{"/x", "/b", "/c", "/b"}
	match := alignGreedy(a, b, []int{-1, -1, -1, -1})
	want := []int{-1, 1, 2, -1}
	for i := range want {
		if match[i] != want[i] {
			t.Errorf("match[%d] = %d, want %d", i, match[i], want[i])
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
