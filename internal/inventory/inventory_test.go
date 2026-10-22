package inventory

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseReader(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []Item
	}{
		{
			"export with value",
			`export FOO=bar`,
			[]Item{{Kind: KindExport, Name: "FOO", Line: 1, Value: "bar", TopLevel: true}},
		},
		{
			"export without value is not a definition",
			`export FOO`,
			nil,
		},
		{
			"export with empty value",
			`export FOO=`,
			[]Item{{Kind: KindExport, Name: "FOO", Line: 1, TopLevel: true}},
		},
		{
			"source target stops at semicolon",
			`source ~/x.zsh; echo hi`,
			[]Item{{Kind: KindSource, Name: "~/x.zsh", Line: 1, TopLevel: true}},
		},
		{
			"source target stops at &&",
			`. /u/x.sh&&echo hi`,
			[]Item{{Kind: KindSource, Name: "/u/x.sh", Line: 1, TopLevel: true}},
		},
		{
			"export with double-quoted value",
			`export FOO="hello world"`,
			[]Item{{Kind: KindExport, Name: "FOO", Line: 1, Value: "hello world", TopLevel: true}},
		},
		{
			"export with single-quoted value",
			`export FOO='/usr/local/bin'`,
			[]Item{{Kind: KindExport, Name: "FOO", Line: 1, Value: "/usr/local/bin", TopLevel: true}},
		},
		{
			"export with trailing inline comment stripped",
			`export FOO=/path # trailing`,
			[]Item{{Kind: KindExport, Name: "FOO", Line: 1, Value: "/path", TopLevel: true}},
		},
		{
			"bare assignment",
			`ZSH_THEME="robbyrussell"`,
			[]Item{{Kind: KindAssign, Name: "ZSH_THEME", Line: 1, Value: "robbyrussell", TopLevel: true}},
		},
		{
			"export with backslash-escaped space in path",
			`export TOOLS=/opt/my\ app/bin`,
			[]Item{{Kind: KindExport, Name: "TOOLS", Line: 1, Value: "/opt/my app/bin", TopLevel: true}},
		},
		{
			"export unquoted value stops before trailing comment",
			`export A=1 # trailing`,
			[]Item{{Kind: KindExport, Name: "A", Line: 1, Value: "1", TopLevel: true}},
		},
		{
			"alias simple",
			`alias ll='ls -la'`,
			[]Item{{Kind: KindAlias, Name: "ll", Line: 1, Value: "ls -la", TopLevel: true}},
		},
		{
			"alias with flag",
			`alias -g G='| grep'`,
			[]Item{{Kind: KindAlias, Name: "G", Line: 1, Value: "| grep", TopLevel: true}},
		},
		{
			"function keyword form",
			`function cdp { cd "$@"; }`,
			[]Item{{Kind: KindFunction, Name: "cdp", Line: 1, TopLevel: true}},
		},
		{
			"function paren form",
			`mkcd() { mkdir -p "$1" && cd "$1"; }`,
			[]Item{{Kind: KindFunction, Name: "mkcd", Line: 1, TopLevel: true}},
		},
		{
			"source keyword",
			`source /Users/me/.extras`,
			[]Item{{Kind: KindSource, Name: "/Users/me/.extras", Line: 1, TopLevel: true}},
		},
		{
			"dot source",
			`. /Users/me/.extras`,
			[]Item{{Kind: KindSource, Name: "/Users/me/.extras", Line: 1, TopLevel: true}},
		},
		{
			"source double-quoted path with spaces",
			`source "/path with spaces/file"`,
			[]Item{{Kind: KindSource, Name: "/path with spaces/file", Line: 1, TopLevel: true}},
		},
		{
			"source single-quoted path with spaces",
			`source '/path with spaces/file'`,
			[]Item{{Kind: KindSource, Name: "/path with spaces/file", Line: 1, TopLevel: true}},
		},
		{
			"comment ignored",
			`# export FOO=bar`,
			nil,
		},
		{
			"blank line ignored",
			"\n\n",
			nil,
		},
		{
			"control-flow keyword not a function",
			`if [[ -f foo ]]; then`,
			nil,
		},
		{
			"function keyword with reserved name rejected",
			`function if { something; }`,
			nil,
		},
		{
			"local assignment rejected as assign",
			`local x=1`,
			nil,
		},
		{
			"line numbers across mixed content",
			strings.Join([]string{
				`# comment`,
				`export A=1`,
				``,
				`alias ll='ls'`,
				`mkcd() { :; }`,
			}, "\n"),
			[]Item{
				{Kind: KindExport, Name: "A", Line: 2, Value: "1", TopLevel: true},
				{Kind: KindAlias, Name: "ll", Line: 4, Value: "ls", TopLevel: true},
				{Kind: KindFunction, Name: "mkcd", Line: 5, TopLevel: true},
			},
		},
		{
			"indented entries still captured",
			"    export FOO=bar",
			[]Item{{Kind: KindExport, Name: "FOO", Line: 1, Value: "bar"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseReader(strings.NewReader(tc.in))
			if err != nil {
				t.Fatalf("ParseReader: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %+v\nwant %+v", got, tc.want)
			}
		})
	}
}

func TestExtractValue(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"unquoted simple", "bar", "bar"},
		{"unquoted trailing comment", "/path # comment", "/path"},
		{"double-quoted", `"hello world"`, "hello world"},
		{"single-quoted", `'/usr/local'`, "/usr/local"},
		{"double-quoted with internal single", `"it's here"`, "it's here"},
		{"single-quoted with internal double", `'say "hi"'`, `say "hi"`},
		{"unclosed double quote", `"no end`, "no end"},
		{"unclosed single quote", `'no end`, "no end"},
		{"empty double-quoted", `""`, ""},
		{"empty single-quoted", `''`, ""},
		{"leading whitespace stripped", "  bar", "bar"},
		{"value contains equals", "b=c", "b=c"},
		{"tab-separated trailing", "/path\t# comment", "/path"},
		{"escaped quote inside double quotes", `"say \"hi\"" # c`, `say "hi"`},
		{"quoted then unquoted segment", `"abc"def ghi`, "abcdef"},
		{"unquoted then quoted segment", `abc"d e"f`, "abcd ef"},
		{"array", `(git docker  kubectl) # c`, "(git docker kubectl)"},
		{"array with quoted element containing paren", `("a)b" c)`, `("a)b" c)`},
		{"array unclosed on this line", `(git`, "(git"},
		{"empty array", `()`, "()"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractValue(tc.in); got != tc.want {
				t.Errorf("extractValue(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestFileRank(t *testing.T) {
	cases := []struct {
		path string
		role Role
		want int
	}{
		{"/u/.zshenv", RoleCanonicalZsh, 0},
		{"/u/.zprofile", RoleCanonicalZsh, 1},
		{"/u/.zshrc", RoleCanonicalZsh, 2},
		{"/u/.zlogin", RoleCanonicalZsh, 3},
		{"/u/.zlogout", RoleCanonicalZsh, 4},
		{".zshrc", RoleCanonicalZsh, 2},
		{"/u/.profile", RoleCanonicalBash, 100},
		{"/u/.bash_profile", RoleCanonicalBash, 101},
		{"/u/.bashrc", RoleCanonicalBash, 102},
		{"/u/.zshrc.bak", RoleOrphan, 200},
		{"/u/x", Role("nonsense"), 999},
	}
	for _, tc := range cases {
		got := FileRank(File{Path: tc.path, Role: tc.role})
		if got != tc.want {
			t.Errorf("%s/%s: got %d, want %d", tc.path, tc.role, got, tc.want)
		}
	}
}

func TestIsShellOrphan(t *testing.T) {
	cases := []struct {
		name        string
		path        string
		includeBash bool
		want        bool
	}{
		{"zsh in name", "/u/.zshrc.backup", false, true},
		{".zprofile prefix", "/u/.zprofile_old", false, true},
		{".zlog prefix", "/u/.zlogin.bak", false, true},
		{"bashrc without --bash", "/u/.bashrc.backup", false, false},
		{"bashrc with --bash", "/u/.bashrc.backup", true, true},
		{".profile with --bash", "/u/.profile.bak", true, true},
		{".profile without --bash", "/u/.profile.bak", false, false},
		{"random file", "/u/.foo.bak", false, false},
		{"random file with --bash", "/u/.foo.bak", true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsShellOrphan(tc.path, tc.includeBash); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFilterFiles(t *testing.T) {
	files := []File{
		{Path: "/u/.zshrc", Role: RoleCanonicalZsh},
		{Path: "/u/.bashrc", Role: RoleCanonicalBash},
		{Path: "/u/.zshrc.backup", Role: RoleOrphan},
		{Path: "/u/.bashrc.old", Role: RoleOrphan},
		{Path: "/u/.foo.bak", Role: RoleOrphan},
	}

	t.Run("zsh only", func(t *testing.T) {
		got := FilterFiles(files, false, false)
		if len(got) != 1 || got[0].Path != "/u/.zshrc" {
			t.Errorf("expected only canonical zsh, got %+v", got)
		}
	})

	t.Run("with bash", func(t *testing.T) {
		got := FilterFiles(files, true, false)
		if len(got) != 2 {
			t.Errorf("expected zsh + bash canonical, got %d", len(got))
		}
	})

	t.Run("orphans filters by shell family", func(t *testing.T) {
		got := FilterFiles(files, false, true)
		for _, f := range got {
			if f.Path == "/u/.bashrc.old" || f.Path == "/u/.foo.bak" {
				t.Errorf("non-zsh orphan %s should be excluded; got %+v", f.Path, got)
			}
		}
	})

	t.Run("bash orphans included with --bash", func(t *testing.T) {
		got := FilterFiles(files, true, true)
		hasBash := false
		for _, f := range got {
			if f.Path == "/u/.bashrc.old" {
				hasBash = true
			}
			if f.Path == "/u/.foo.bak" {
				t.Errorf("non-shell orphan should still be excluded; got %+v", got)
			}
		}
		if !hasBash {
			t.Errorf("expected bash orphan with --bash --orphans; got %+v", got)
		}
	})
}

func TestHasOrphanPrefix(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"canonical match", ".zshrc", true},
		{"suffix with dot", ".zshrc.backup", true},
		{"suffix with underscore", ".zshrc_old", true},
		{"suffix with dash", ".zshrc-2023", true},
		{"unrelated name", ".gitconfig", false},
		{"prefix-only collision", ".zshrcsomething", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasOrphanPrefix(tc.in); got != tc.want {
				t.Errorf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestParseReader_BOMLineIsNotADefinition(t *testing.T) {
	items, err := ParseReader(strings.NewReader("\ufeffexport BOM_VAR=1\nexport SECOND=2\n"))
	if err != nil {
		t.Fatalf("ParseReader: %v", err)
	}
	if len(items) != 1 || items[0].Name != "SECOND" {
		t.Fatalf("zsh and bash reject a BOM-prefixed line 1 (command not found); got %+v", items)
	}
}

func TestParseReader_TooLongLineReportsLineNumber(t *testing.T) {
	in := "export A=1\nexport B=" + strings.Repeat("x", 1024*1024+1) + "\nexport C=3\n"
	items, err := ParseReader(strings.NewReader(in))
	if err == nil || !strings.Contains(err.Error(), "line 2:") {
		t.Fatalf("want error naming line 2, got %v", err)
	}
	if len(items) != 1 || items[0].Name != "A" {
		t.Errorf("items before the long line must survive; got %+v", items)
	}
}

func TestParseReader_BOMOnlyStrippedFromFirstLine(t *testing.T) {
	items, err := ParseReader(strings.NewReader("export A=1\n\ufeffexport B=2\n"))
	if err != nil {
		t.Fatalf("ParseReader: %v", err)
	}
	if len(items) != 1 || items[0].Name != "A" {
		t.Errorf("a BOM mid-file is not a BOM; got %+v", items)
	}
}

func TestParseReader_TopLevel(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want map[int]bool
	}{
		{"plain lines", "export A=1\nB=2", map[int]bool{1: true, 2: true}},
		{"indented line is not proven", "  export A=1", map[int]bool{1: false}},
		{"function body", "f() {\nexport A=1\n}\nexport B=2", map[int]bool{1: true, 2: false, 4: true}},
		{"function keyword body", "function f {\nA=1\n}", map[int]bool{1: true, 2: false}},
		{"if and else branches", "if x; then\nA=1\nelse\nA=2\nfi\nB=3", map[int]bool{2: false, 4: false, 6: true}},
		{"case arms", "case $x in\na) A=1 ;;\nb)\nB=2\n;;\nesac\nC=3", map[int]bool{4: false, 7: true}},
		{"loop body", "for d in a b; do\nA=1\ndone\nB=2", map[int]bool{2: false, 4: true}},
		{"multi-line array", "ARR=(\nA=1\n)\nB=2", map[int]bool{1: true, 2: false, 4: true}},
		{"heredoc body", "cat <<'EOF'\nA=1\nEOF\nB=2", map[int]bool{2: false, 4: true}},
		{"line continuation", "export A=1 \\\nB=2\nC=3", map[int]bool{1: true, 2: false, 3: true}},
		{"multi-line string", "export A=\"x\nB=2\"\nC=3", map[int]bool{1: true, 3: true}},
		{"expansions and comments do not nest", "A=${B:-x} # it's {\nC={a,b}\nD=$#\nE=1", map[int]bool{1: true, 2: true, 3: true, 4: true}},
		{"keyword as argument is not a block", "echo if for\nA=1", map[int]bool{2: true}},
		{"unclosed block makes the whole file unproven", "A=1\nif x; then\nB=2", map[int]bool{1: false, 3: false}},
		{"stray closer makes the whole file unproven", "A=1\nfi\nB=2", map[int]bool{1: false, 3: false}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			items, err := ParseReader(strings.NewReader(tc.in))
			if err != nil {
				t.Fatal(err)
			}
			got := map[int]bool{}
			for _, it := range items {
				got[it.Line] = it.TopLevel
			}
			for line, want := range tc.want {
				g, ok := got[line]
				if !ok {
					t.Errorf("line %d: no item parsed; items %+v", line, items)
					continue
				}
				if g != want {
					t.Errorf("line %d: TopLevel = %v, want %v", line, g, want)
				}
			}
		})
	}
}
