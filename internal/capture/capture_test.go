package capture

import (
	"reflect"
	"testing"

	"github.com/sreckoskocilic/envocabulary/internal/model"
)

func TestParseNullSeparated(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want map[string]string
	}{
		{"empty", []byte{}, map[string]string{}},
		{"single", []byte("FOO=bar\x00"), map[string]string{"FOO": "bar"}},
		{"multiple", []byte("A=1\x00B=2\x00"), map[string]string{"A": "1", "B": "2"}},
		{"entry without equals is skipped", []byte("A=1\x00JUNK\x00B=2\x00"), map[string]string{"A": "1", "B": "2"}},
		{"empty value", []byte("A=\x00"), map[string]string{"A": ""}},
		{"value contains equals sign", []byte("A=b=c\x00"), map[string]string{"A": "b=c"}},
		{"newline in value", []byte("A=line1\nline2\x00B=2\x00"), map[string]string{"A": "line1\nline2", "B": "2"}},
		{"no trailing null", []byte("A=1"), map[string]string{"A": "1"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseNullSeparated(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("len: got %d want %d (got=%v)", len(got), len(tc.want), got)
			}
			for k, v := range tc.want {
				if got[k] != v {
					t.Errorf("key %q: got %q want %q", k, got[k], v)
				}
			}
		})
	}
}

func TestParseTrace(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []model.TraceEntry
	}{
		{
			"export assignment",
			"+/u/foo/.zshrc:3> export FOO=bar\n",
			[]model.TraceEntry{{File: "/u/foo/.zshrc", Line: 3, Name: "FOO", Raw: "export FOO=bar"}},
		},
		{
			"bare assignment",
			"+/u/foo/.zshrc:5> FOO=bar\n",
			[]model.TraceEntry{{File: "/u/foo/.zshrc", Line: 5, Name: "FOO", Raw: "FOO=bar"}},
		},
		{
			"typeset with combined flags",
			"+/u/foo/.zshrc:8> typeset -gx FOO=bar\n",
			[]model.TraceEntry{{File: "/u/foo/.zshrc", Line: 8, Name: "FOO", Raw: "typeset -gx FOO=bar"}},
		},
		{
			"typeset with separate flags",
			"+/u/foo/.zshrc:9> typeset -g -x FOO=bar\n",
			[]model.TraceEntry{{File: "/u/foo/.zshrc", Line: 9, Name: "FOO", Raw: "typeset -g -x FOO=bar"}},
		},
		{
			"declare with no flags",
			"+/u/foo/.zshrc:10> declare FOO=bar\n",
			[]model.TraceEntry{{File: "/u/foo/.zshrc", Line: 10, Name: "FOO", Raw: "declare FOO=bar"}},
		},
		{
			"local with flag",
			"+/u/foo/.zshrc:11> local -r FOO=bar\n",
			[]model.TraceEntry{{File: "/u/foo/.zshrc", Line: 11, Name: "FOO", Raw: "local -r FOO=bar"}},
		},
		{
			"all writers preserved in order",
			"+/u/foo/.zprofile:1> FOO=first\n+/u/foo/.zshrc:20> FOO=second\n",
			[]model.TraceEntry{
				{File: "/u/foo/.zprofile", Line: 1, Name: "FOO", Raw: "FOO=first"},
				{File: "/u/foo/.zshrc", Line: 20, Name: "FOO", Raw: "FOO=second"},
			},
		},
		{
			"nested context double plus",
			"++/u/foo/.zshrc:15> FOO=bar\n",
			[]model.TraceEntry{{File: "/u/foo/.zshrc", Line: 15, Name: "FOO", Raw: "FOO=bar"}},
		},
		{
			"deeply nested context",
			"++++/u/foo/helpers.zsh:4> FOO=bar\n",
			[]model.TraceEntry{{File: "/u/foo/helpers.zsh", Line: 4, Name: "FOO", Raw: "FOO=bar"}},
		},
		{
			"non-assignment line ignored",
			"+/u/foo/.zshrc:30> echo hello\n",
			nil,
		},
		{
			"value contains equals signs",
			"+/u/foo/.zshrc:31> FOO=a=b=c\n",
			[]model.TraceEntry{{File: "/u/foo/.zshrc", Line: 31, Name: "FOO", Raw: "FOO=a=b=c"}},
		},
		{
			"non-trace lines in stream are ignored",
			"noise line\n+/u/foo/.zshrc:40> FOO=bar\nmore noise\n",
			[]model.TraceEntry{{File: "/u/foo/.zshrc", Line: 40, Name: "FOO", Raw: "FOO=bar"}},
		},
		{
			"multiple vars in one line captures only first (known limitation)",
			"+/u/foo/.zshrc:50> export A=1 B=2\n",
			[]model.TraceEntry{{File: "/u/foo/.zshrc", Line: 50, Name: "A", Raw: "export A=1 B=2"}},
		},
		{
			"empty input",
			"",
			nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseTrace(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %+v\nwant %+v", got, tc.want)
			}
		})
	}
}
