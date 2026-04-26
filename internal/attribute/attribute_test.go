package attribute

import (
	"testing"

	"github.com/sreckoskocilic/envocabulary/internal/model"
)

func TestAttribute_DeferredListVar(t *testing.T) {
	current := map[string]string{"PATH": "/usr/bin"}
	got := Attribute(current, nil)
	if len(got) != 1 {
		t.Fatalf("got %d entries, want 1", len(got))
	}
	if got[0].Origin != model.OriginDeferred {
		t.Errorf("PATH origin = %q, want %q", got[0].Origin, model.OriginDeferred)
	}
	if got[0].Source == "" {
		t.Errorf("expected non-empty Source for deferred var")
	}
}

func TestAttribute_DirenvVar(t *testing.T) {
	current := map[string]string{"DIRENV_DIR": "/tmp/proj"}
	got := Attribute(current, nil)
	if got[0].Origin != model.OriginDirenv {
		t.Errorf("DIRENV_DIR origin = %q, want %q", got[0].Origin, model.OriginDirenv)
	}
}

func TestAttribute_ShellFileWins(t *testing.T) {
	current := map[string]string{"EDITOR": "vim"}
	trace := []model.TraceEntry{
		{Name: "EDITOR", File: "/u/.zshrc", Line: 12},
	}
	got := Attribute(current, trace)
	if got[0].Origin != model.OriginShellFile {
		t.Errorf("origin = %q, want %q", got[0].Origin, model.OriginShellFile)
	}
	if got[0].Source != "/u/.zshrc:12" {
		t.Errorf("source = %q, want %q", got[0].Source, "/u/.zshrc:12")
	}
}

func TestAttribute_ShellFileTakesLastWriter(t *testing.T) {
	current := map[string]string{"FOO": "bar"}
	trace := []model.TraceEntry{
		{Name: "FOO", File: "/u/.zprofile", Line: 5},
		{Name: "FOO", File: "/u/.zshrc", Line: 20},
	}
	got := Attribute(current, trace)
	if got[0].Source != "/u/.zshrc:20" {
		t.Errorf("expected last writer to win; got source = %q", got[0].Source)
	}
}

func TestAttribute_FallsBackToClassifier(t *testing.T) {
	current := map[string]string{"USER": "alice"}
	got := Attribute(current, nil)
	if got[0].Origin != model.OriginSystem {
		t.Errorf("USER origin = %q, want %q", got[0].Origin, model.OriginSystem)
	}
}

func TestAttribute_SortedByOriginThenName(t *testing.T) {
	current := map[string]string{
		"USER":    "alice",
		"PATH":    "/usr/bin",
		"FOO":     "bar",
		"BAR":     "baz",
		"SSH_TTY": "/dev/pty",
	}
	trace := []model.TraceEntry{
		{Name: "FOO", File: "/u/.zshrc", Line: 1},
		{Name: "BAR", File: "/u/.zshrc", Line: 2},
	}
	got := Attribute(current, trace)
	// Expect: shell-file group (BAR, FOO sorted), then system (USER), ssh (SSH_TTY), then deferred (PATH)
	wantOrder := []string{"BAR", "FOO", "USER", "SSH_TTY", "PATH"}
	if len(got) != len(wantOrder) {
		t.Fatalf("got %d entries, want %d", len(got), len(wantOrder))
	}
	for i, w := range got {
		if w.Name != wantOrder[i] {
			t.Errorf("position %d: got %q, want %q", i, w.Name, wantOrder[i])
		}
	}
}

func TestOriginRank(t *testing.T) {
	cases := []struct {
		o    model.Origin
		want int
	}{
		{model.OriginShellFile, 0},
		{model.OriginDirenv, 1},
		{model.OriginLaunchd, 2},
		{model.OriginSystem, 3},
		{model.OriginTerminal, 4},
		{model.OriginSSH, 5},
		{model.OriginDeferred, 6},
		{model.OriginUnknown, 7},
		{model.Origin("nonsense"), 99},
	}
	for _, tc := range cases {
		if got := originRank(tc.o); got != tc.want {
			t.Errorf("originRank(%q) = %d, want %d", tc.o, got, tc.want)
		}
	}
}

func TestLastWriters(t *testing.T) {
	trace := []model.TraceEntry{
		{Name: "A", File: "/x", Line: 1},
		{Name: "A", File: "/y", Line: 2}, // overwrites
		{Name: "B", File: "/z", Line: 3},
	}
	m := lastWriters(trace)
	if m["A"].File != "/y" || m["A"].Line != 2 {
		t.Errorf("A: expected last writer /y:2, got %+v", m["A"])
	}
	if m["B"].File != "/z" {
		t.Errorf("B: got %+v", m["B"])
	}
}
