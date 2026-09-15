package cleaner

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestClean(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			"single-line user header kept",
			"# aliases\nalias ll='ls -la'\n",
			"# aliases\nalias ll='ls -la'\n",
		},
		{
			"decorated multi-line header kept",
			"# ---------\n# env vars\n# ---------\nexport FOO=1\n",
			"# ---------\n# env vars\n# ---------\nexport FOO=1\n",
		},
		{
			"multi-line prose block stripped",
			"# If you come from bash you might have to change your $PATH.\n# export PATH=$HOME/bin:/usr/local/bin:$PATH\nexport FOO=1\n",
			"export FOO=1\n",
		},
		{
			"commented-out code stripped even as single line",
			"# export FOO=bar\nexport REAL=1\n",
			"export REAL=1\n",
		},
		{
			"shebang preserved",
			"#!/usr/bin/env zsh\nexport FOO=1\n",
			"#!/usr/bin/env zsh\nexport FOO=1\n",
		},
		{
			"oh-my-zsh template style block stripped",
			strings.Join([]string{
				`# Set name of the theme to load --- if set to "random", it will`,
				`# load a theme from ~/.oh-my-zsh/themes/`,
				`# Optionally, if you set this to "random", you can set a list`,
				`# ZSH_THEME="robbyrussell"`,
				`ZSH_THEME="agnoster"`,
				``,
			}, "\n"),
			`ZSH_THEME="agnoster"` + "\n",
		},
		{
			"label kept when its commented-out code is stripped",
			"# Aliases\n# alias ll='ls -l'\nalias la='ls -a'\n",
			"# Aliases\nalias la='ls -a'\n",
		},
		{
			"boilerplate prose still stripped with its commented-out code",
			"# Uncomment the following line to use case-sensitive completion.\n# CASE_SENSITIVE=\"true\"\nexport FOO=1\n",
			"export FOO=1\n",
		},
		{
			"real code is always kept",
			"export FOO=1\nalias ll='ls'\n",
			"export FOO=1\nalias ll='ls'\n",
		},
		{
			"long single-line comment kept (safe default)",
			"# this comment is not a header but is a single line standalone\nexport FOO=1\n",
			"# this comment is not a header but is a single line standalone\nexport FOO=1\n",
		},
		{
			"single-line decoration stripped",
			"# ===\nexport FOO=1\n",
			"export FOO=1\n",
		},
		{
			"two-line decoration stripped",
			"# ===\n# ===\nexport FOO=1\n",
			"export FOO=1\n",
		},
		{
			"single-line dashes stripped",
			"# ---\nexport FOO=1\n",
			"export FOO=1\n",
		},
		{
			"commented plugins array stripped",
			"# plugins=(git docker rails)\nplugins=(git)\n",
			"plugins=(git)\n",
		},
		{
			"blank lines preserved between entries",
			"export A=1\n\nexport B=2\n",
			"export A=1\n\nexport B=2\n",
		},
		{
			"empty input returns empty string",
			"",
			"",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, _, err := Clean(strings.NewReader(tc.in))
			if err != nil {
				t.Fatalf("Clean: %v", err)
			}
			if got != tc.want {
				t.Errorf("got:\n%q\nwant:\n%q", got, tc.want)
			}
		})
	}
}

func TestClean_PreservesNoTrailingNewline(t *testing.T) {
	got, _, err := Clean(strings.NewReader("export FOO=1"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasSuffix(got, "\n") {
		t.Errorf("expected no trailing newline; got %q", got)
	}
	if got != "export FOO=1" {
		t.Errorf("got %q, want %q", got, "export FOO=1")
	}
}

func TestLooksLikeLabel(t *testing.T) {
	cases := map[string]bool{
		"aliases":                 true,
		"env vars":                true,
		"five word label here ok": true, // 5 fields
		"this section configures development environment":            true,  // 6 fields, <=8
		"section header for the development environment config area": true,  // 8 fields
		"this has nine words which is too many for a label":          false, // 9 fields, >8
		"sentence ending in period.":                                 false,
		strings.Repeat("x", 81):                                      false, // too long
		"":                                                           true,  // 0 fields, treated as label
	}
	for in, want := range cases {
		if got := looksLikeLabel(in); got != want {
			t.Errorf("looksLikeLabel(%q) = %v, want %v", in, got, want)
		}
	}
}

type errReader struct{ err error }

func (e *errReader) Read([]byte) (int, error) { return 0, e.err }

func TestProcess_ReaderError(t *testing.T) {
	r := &errReader{err: io.ErrUnexpectedEOF}
	if _, _, err := Process(r); err == nil {
		t.Errorf("expected error from failing reader")
	}
}

func TestClean_RejectsOversizedInput(t *testing.T) {
	r := io.MultiReader(strings.NewReader("export A=1\n"), &zeroReader{n: maxCleanBytes})
	if _, _, err := Clean(r); !errors.Is(err, errTooLarge) {
		t.Errorf("want errTooLarge, got %v", err)
	}
}

type zeroReader struct{ n int }

func (z *zeroReader) Read(p []byte) (int, error) {
	if z.n <= 0 {
		return 0, io.EOF
	}
	if len(p) > z.n {
		p = p[:z.n]
	}
	for i := range p {
		p[i] = '#'
	}
	z.n -= len(p)
	return len(p), nil
}

func TestClean_PropagatesProcessError(t *testing.T) {
	r := &errReader{err: io.ErrUnexpectedEOF}
	if _, _, err := Clean(r); err == nil {
		t.Errorf("expected Clean to propagate Process error")
	}
}

func TestBlockKeep_MultiLineWithCommentedCode(t *testing.T) {
	block := []lineInfo{
		{isComment: true, inner: ""},
		{isComment: true, inner: "export FOO=bar"},
	}
	for i, k := range blockKeep(block) {
		if k {
			t.Errorf("line %d: block of only commented-out code should be stripped", i)
		}
	}
}

func TestBlockKeep_MultiLineProseNoLabel(t *testing.T) {
	block := []lineInfo{
		{isComment: true, inner: "If you come from bash you might have to change things."},
		{isComment: true, inner: "This is another long explanation that is definitely prose."},
	}
	for i, k := range blockKeep(block) {
		if k {
			t.Errorf("line %d: multi-line prose without a label should be stripped", i)
		}
	}
}

func TestBlockKeep_LabelSurvivesAdjacentCommentedCode(t *testing.T) {
	block := []lineInfo{
		{isComment: true, inner: "Aliases"},
		{isComment: true, inner: "alias ll='ls -l'"},
		{isComment: true, inner: "these are grouped by tool"},
	}
	got := blockKeep(block)
	want := []bool{true, false, true}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d: keep=%v, want %v", i, got[i], want[i])
		}
	}
}

func TestIsCommentedCode(t *testing.T) {
	cases := map[string]bool{
		`export FOO=bar`:                     true,
		`alias ll='ls -la'`:                  true,
		`function mkcd { :; }`:               true,
		`mkcd() { :; }`:                      true,
		`source /tmp/foo`:                    true,
		`. /tmp/foo`:                         true,
		`plugins=(git)`:                      true,
		`ZSH_THEME="robbyrussell"`:           true,
		`If you come from bash you might...`: false,
		`aliases`:                            false,
		``:                                   false,

		`max = 10 retries before giving up`:         false,
		`HISTSIZE = how many lines zsh keeps`:       false,
		`NOTE = this is prose, not an assignment`:   false,
		`export these for the build tools`:          false,
		`source of truth for my aliases lives here`: false,

		`alias for the k8s cluster`:                                  false,
		`alias names are short on purpose, see README`:               false,
		`function to reload the shell config`:                        false,
		`function keys are mapped in .inputrc`:                       false,
		`main() is invoked at the end of this file`:                  false,
		`source https://github.com/ohmyzsh/ohmyzsh/wiki for details`: false,
		`source https://github.com/ohmyzsh/ohmyzsh/wiki`:             false,
		`source www.example.com has the docs`:                        false,
		`source ~/.secrets if you need the work credentials`:         false,
		`alias -g G='| grep'`:                                        true,
		`function mkcd() {`:                                          true,
		`function mkcd`:                                              true,
		`mkcd()`:                                                     true,
		`source ~/x.zsh # disabled`:                                  true,
		`source ~/x.zsh; rehash`:                                     true,
		`. "$HOME/.cargo/env"`:                                       true,
		`source helpers.zsh`:                                         true,
	}
	for in, want := range cases {
		if got := isCommentedCode(in); got != want {
			t.Errorf("isCommentedCode(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestProcess_KeepsProseWithSpacedEquals(t *testing.T) {
	in := strings.Join([]string{
		`# max = 10 retries before giving up`,
		`export RETRIES=10`,
		`# HISTSIZE = how many lines zsh keeps in memory`,
		`export HISTSIZE=5000`,
		`# source of truth for my aliases lives here`,
		`source ~/.aliases`,
	}, "\n")
	decisions, stats, err := Process(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if stats.Stripped != 0 {
		t.Fatalf("prose comments must never be stripped; stripped %d", stats.Stripped)
	}
	for _, d := range decisions {
		if !d.Kept {
			t.Errorf("line %d dropped: %q", d.LineNum, d.Content)
		}
	}
}
