package capture

import "testing"

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

type traceWant struct {
	file string
	line int
}

func TestParseTrace(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want map[string]traceWant
	}{
		{
			"export assignment",
			"+/u/foo/.zshrc:3> export FOO=bar\n",
			map[string]traceWant{"FOO": {"/u/foo/.zshrc", 3}},
		},
		{
			"bare assignment",
			"+/u/foo/.zshrc:5> FOO=bar\n",
			map[string]traceWant{"FOO": {"/u/foo/.zshrc", 5}},
		},
		{
			"typeset with combined flags",
			"+/u/foo/.zshrc:8> typeset -gx FOO=bar\n",
			map[string]traceWant{"FOO": {"/u/foo/.zshrc", 8}},
		},
		{
			"typeset with separate flags",
			"+/u/foo/.zshrc:9> typeset -g -x FOO=bar\n",
			map[string]traceWant{"FOO": {"/u/foo/.zshrc", 9}},
		},
		{
			"declare with no flags",
			"+/u/foo/.zshrc:10> declare FOO=bar\n",
			map[string]traceWant{"FOO": {"/u/foo/.zshrc", 10}},
		},
		{
			"local with flag",
			"+/u/foo/.zshrc:11> local -r FOO=bar\n",
			map[string]traceWant{"FOO": {"/u/foo/.zshrc", 11}},
		},
		{
			"last writer wins",
			"+/u/foo/.zprofile:1> FOO=first\n+/u/foo/.zshrc:20> FOO=second\n",
			map[string]traceWant{"FOO": {"/u/foo/.zshrc", 20}},
		},
		{
			"nested context double plus",
			"++/u/foo/.zshrc:15> FOO=bar\n",
			map[string]traceWant{"FOO": {"/u/foo/.zshrc", 15}},
		},
		{
			"deeply nested context",
			"++++/u/foo/helpers.zsh:4> FOO=bar\n",
			map[string]traceWant{"FOO": {"/u/foo/helpers.zsh", 4}},
		},
		{
			"non-assignment line ignored",
			"+/u/foo/.zshrc:30> echo hello\n",
			map[string]traceWant{},
		},
		{
			"value contains equals signs",
			"+/u/foo/.zshrc:31> FOO=a=b=c\n",
			map[string]traceWant{"FOO": {"/u/foo/.zshrc", 31}},
		},
		{
			"non-trace lines in stream are ignored",
			"noise line\n+/u/foo/.zshrc:40> FOO=bar\nmore noise\n",
			map[string]traceWant{"FOO": {"/u/foo/.zshrc", 40}},
		},
		{
			"multiple vars in one line captures only first (known limitation)",
			"+/u/foo/.zshrc:50> export A=1 B=2\n",
			map[string]traceWant{"A": {"/u/foo/.zshrc", 50}},
		},
		{
			"empty input",
			"",
			map[string]traceWant{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseTrace(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("len: got %d want %d (got=%+v)", len(got), len(tc.want), got)
			}
			for k, v := range tc.want {
				entry, ok := got[k]
				if !ok {
					t.Errorf("missing key %q", k)
					continue
				}
				if entry.File != v.file || entry.Line != v.line {
					t.Errorf("key %q: got %s:%d want %s:%d", k, entry.File, entry.Line, v.file, v.line)
				}
			}
		})
	}
}
