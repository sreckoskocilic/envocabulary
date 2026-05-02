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
			[]Item{{Kind: KindExport, Name: "FOO", Line: 1, Value: "bar"}},
		},
		{
			"export without value",
			`export FOO`,
			[]Item{{Kind: KindExport, Name: "FOO", Line: 1}},
		},
		{
			"export with double-quoted value",
			`export FOO="hello world"`,
			[]Item{{Kind: KindExport, Name: "FOO", Line: 1, Value: "hello world"}},
		},
		{
			"export with single-quoted value",
			`export FOO='/usr/local/bin'`,
			[]Item{{Kind: KindExport, Name: "FOO", Line: 1, Value: "/usr/local/bin"}},
		},
		{
			"export with trailing inline comment stripped",
			`export FOO=/path # trailing`,
			[]Item{{Kind: KindExport, Name: "FOO", Line: 1, Value: "/path"}},
		},
		{
			"bare assignment",
			`ZSH_THEME="robbyrussell"`,
			[]Item{{Kind: KindAssign, Name: "ZSH_THEME", Line: 1, Value: "robbyrussell"}},
		},
		{
			"alias simple",
			`alias ll='ls -la'`,
			[]Item{{Kind: KindAlias, Name: "ll", Line: 1}},
		},
		{
			"alias with flag",
			`alias -g G='| grep'`,
			[]Item{{Kind: KindAlias, Name: "G", Line: 1}},
		},
		{
			"function keyword form",
			`function cdp { cd "$@"; }`,
			[]Item{{Kind: KindFunction, Name: "cdp", Line: 1}},
		},
		{
			"function paren form",
			`mkcd() { mkdir -p "$1" && cd "$1"; }`,
			[]Item{{Kind: KindFunction, Name: "mkcd", Line: 1}},
		},
		{
			"source keyword",
			`source /Users/me/.extras`,
			[]Item{{Kind: KindSource, Name: "/Users/me/.extras", Line: 1}},
		},
		{
			"dot source",
			`. /Users/me/.extras`,
			[]Item{{Kind: KindSource, Name: "/Users/me/.extras", Line: 1}},
		},
		{
			"source double-quoted path with spaces",
			`source "/path with spaces/file"`,
			[]Item{{Kind: KindSource, Name: "/path with spaces/file", Line: 1}},
		},
		{
			"source single-quoted path with spaces",
			`source '/path with spaces/file'`,
			[]Item{{Kind: KindSource, Name: "/path with spaces/file", Line: 1}},
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
				{Kind: KindExport, Name: "A", Line: 2, Value: "1"},
				{Kind: KindAlias, Name: "ll", Line: 4},
				{Kind: KindFunction, Name: "mkcd", Line: 5},
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
		{"/u/.bashrc", RoleCanonicalBash, 100},
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
