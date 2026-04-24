package cleaner

import (
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
			"commented plugins array stripped",
			"# plugins=(git docker rails)\nplugins=(git)\n",
			"plugins=(git)\n",
		},
		{
			"blank lines preserved between entries",
			"export A=1\n\nexport B=2\n",
			"export A=1\n\nexport B=2\n",
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

func TestIsCommentedCode(t *testing.T) {
	cases := map[string]bool{
		`export FOO=bar`:                    true,
		`alias ll='ls -la'`:                 true,
		`function mkcd { :; }`:              true,
		`mkcd() { :; }`:                     true,
		`source /tmp/foo`:                   true,
		`. /tmp/foo`:                        true,
		`plugins=(git)`:                     true,
		`ZSH_THEME="robbyrussell"`:          true,
		`If you come from bash you might...`: false,
		`aliases`:                           false,
		``:                                  false,
	}
	for in, want := range cases {
		if got := isCommentedCode(in); got != want {
			t.Errorf("isCommentedCode(%q) = %v, want %v", in, got, want)
		}
	}
}
