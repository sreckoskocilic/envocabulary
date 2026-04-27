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
