package capture

import (
	"errors"
	"strings"
	"testing"

	"github.com/sreckoskocilic/envocabulary/internal/model"
)

type fakeTracer struct {
	output string
	err    error
}

func (f fakeTracer) RawTrace() (string, error) {
	return f.output, f.err
}

func TestTracedStartupWith_ParsesOutput(t *testing.T) {
	raw := strings.Join([]string{
		"+/u/.zprofile:5> export EDITOR=vim",
		"+/u/.zshrc:12> FOO=bar",
		"+/u/.zshrc:20> export EDITOR=nvim",
		"some non-trace noise",
		"",
	}, "\n")

	got, err := TracedStartupWith(fakeTracer{output: raw})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %d: %+v", len(got), got)
	}
	wantNames := []string{"EDITOR", "FOO", "EDITOR"}
	for i, e := range got {
		if e.Name != wantNames[i] {
			t.Errorf("entry %d: name=%q, want %q", i, e.Name, wantNames[i])
		}
	}
	if got[2].File != "/u/.zshrc" || got[2].Line != 20 {
		t.Errorf("last EDITOR writer: got %s:%d, want /u/.zshrc:20", got[2].File, got[2].Line)
	}
}

func TestTracedStartupWith_PropagatesError(t *testing.T) {
	want := errors.New("tracer boom")
	_, err := TracedStartupWith(fakeTracer{err: want})
	if !errors.Is(err, want) {
		t.Errorf("expected wrapped tracer error; got %v", err)
	}
}

func TestTracedStartupWith_EmptyOutputIsValid(t *testing.T) {
	got, err := TracedStartupWith(fakeTracer{output: ""})
	if err != nil {
		t.Fatalf("empty output should not error; got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no entries, got %+v", got)
	}
}

func TestTracedStartupWith_AllNoiseProducesNoEntries(t *testing.T) {
	raw := "random log line\nanother line\nno trace prefix here\n"
	got, err := TracedStartupWith(fakeTracer{output: raw})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected zero entries from non-trace input; got %+v", got)
	}
}

func TestEnvWithPS4_InjectsPS4ZshFormat(t *testing.T) {
	t.Setenv("PS4", "OLD")
	t.Setenv("FOO_TEST_VAR", "marker")

	got := envWithPS4("+%x:%i> ")

	var ps4Count, fooCount int
	for _, kv := range got {
		switch {
		case strings.HasPrefix(kv, "PS4="):
			ps4Count++
			if kv != "PS4=+%x:%i> " {
				t.Errorf("PS4 = %q, want %q", kv, "PS4=+%x:%i> ")
			}
		case strings.HasPrefix(kv, "FOO_TEST_VAR="):
			fooCount++
		}
	}
	if ps4Count != 1 {
		t.Errorf("expected exactly 1 PS4 entry, got %d", ps4Count)
	}
	if fooCount != 1 {
		t.Errorf("expected FOO_TEST_VAR to be preserved exactly once; got %d", fooCount)
	}
}

func TestEnvWithPS4_InjectsBashFormat(t *testing.T) {
	got := envWithPS4("+${BASH_SOURCE}:${LINENO}> ")
	hasBashPS4 := false
	for _, kv := range got {
		if kv == "PS4=+${BASH_SOURCE}:${LINENO}> " {
			hasBashPS4 = true
		}
	}
	if !hasBashPS4 {
		t.Errorf("expected bash PS4 to be injected; got: %v", got)
	}
}

func TestEnvWithPS4_StripsPreexistingPS4(t *testing.T) {
	t.Setenv("PS4", "STALE")
	got := envWithPS4("NEW")
	count := 0
	for _, kv := range got {
		if strings.HasPrefix(kv, "PS4=") {
			count++
			if kv != "PS4=NEW" {
				t.Errorf("expected fresh PS4=NEW, got %q", kv)
			}
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 PS4, got %d", count)
	}
}

func TestZshTracer_Smoke(t *testing.T) {
	out, err := ZshTracer{}.RawTrace()
	if err != nil {
		t.Logf("zsh tracer error (acceptable, e.g. no zsh installed): %v", err)
		return
	}
	t.Logf("zsh tracer produced %d bytes of trace output", len(out))
}

func TestZshTracer_ErrorWhenZshUnavailable(t *testing.T) {
	t.Setenv("PATH", "")
	_, err := ZshTracer{}.RawTrace()
	if err == nil {
		t.Errorf("expected error when zsh cannot be found on $PATH")
	}
}

func TestCurrentEnv_Smoke(t *testing.T) {
	got, err := CurrentEnv()
	if err != nil {
		t.Logf("env -0 unavailable (acceptable in restricted environments): %v", err)
		return
	}
	if len(got) == 0 {
		t.Errorf("expected at least one env var")
	}
}

func TestCurrentEnv_ErrorWhenEnvBinaryMissing(t *testing.T) {
	t.Setenv("PATH", "")
	_, err := CurrentEnv()
	if err == nil {
		t.Errorf("expected error when `env` binary cannot be located")
	}
}

func TestTracedStartup_Smoke(t *testing.T) {
	_, _ = TracedStartup()
}

func TestBashTracer_Smoke(t *testing.T) {
	out, err := BashTracer{}.RawTrace()
	if err != nil {
		t.Logf("bash tracer error (acceptable, e.g. no bash installed): %v", err)
		return
	}
	t.Logf("bash tracer produced %d bytes of trace output", len(out))
}

func TestBashTracer_ErrorWhenBashUnavailable(t *testing.T) {
	t.Setenv("PATH", "")
	_, err := BashTracer{}.RawTrace()
	if err == nil {
		t.Errorf("expected error when bash cannot be found on $PATH")
	}
}

func TestDetectShell(t *testing.T) {
	cases := map[string]string{
		"/bin/zsh":               "zsh",
		"/usr/local/bin/zsh":     "zsh",
		"/bin/bash":              "bash",
		"/opt/homebrew/bin/bash": "bash",
		"":                       "zsh",
		"/usr/bin/fish":          "zsh", // unsupported defaults to zsh
		"/bin/sh":                "zsh", // sh defaults to zsh too
	}
	for shellPath, want := range cases {
		t.Run(shellPath, func(t *testing.T) {
			t.Setenv("SHELL", shellPath)
			if got := DetectShell(); got != want {
				t.Errorf("DetectShell with SHELL=%q = %q, want %q", shellPath, got, want)
			}
		})
	}
}

func TestTracerForShell_ExplicitNames(t *testing.T) {
	z, err := TracerForShell("zsh")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := z.(ZshTracer); !ok {
		t.Errorf("zsh: got %T, want ZshTracer", z)
	}

	b, err := TracerForShell("bash")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := b.(BashTracer); !ok {
		t.Errorf("bash: got %T, want BashTracer", b)
	}
}

func TestTracerForShell_AutoDetect(t *testing.T) {
	t.Setenv("SHELL", "/bin/bash")
	got, err := TracerForShell("")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.(BashTracer); !ok {
		t.Errorf("auto-detect with bash $SHELL: got %T, want BashTracer", got)
	}

	t.Setenv("SHELL", "/bin/zsh")
	got, err = TracerForShell("")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.(ZshTracer); !ok {
		t.Errorf("auto-detect with zsh $SHELL: got %T, want ZshTracer", got)
	}
}

func TestTracerForShell_UnknownErrors(t *testing.T) {
	if _, err := TracerForShell("fish"); err == nil {
		t.Errorf("expected error for unsupported shell")
	}
	if _, err := TracerForShell("powershell"); err == nil {
		t.Errorf("expected error for unsupported shell")
	}
}

func TestTracedStartupWith_TypeIntegrity(t *testing.T) {
	raw := "+/x:1> export FOO=bar"
	got, err := TracedStartupWith(fakeTracer{output: raw})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d entries, want 1", len(got))
	}
	want := model.TraceEntry{File: "/x", Line: 1, Name: "FOO", Raw: "export FOO=bar"}
	if got[0] != want {
		t.Errorf("got %+v, want %+v", got[0], want)
	}
}
