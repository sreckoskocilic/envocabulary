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
		"+/u/.zshrc:20> export EDITOR=nvim", // overwrites
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

func TestEnvWithPS4_InjectsPS4(t *testing.T) {
	t.Setenv("PS4", "OLD")
	t.Setenv("FOO_TEST_VAR", "marker")

	got := envWithPS4()

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

func TestEnvWithPS4_AddsPS4WhenAbsent(t *testing.T) {
	// Note: even after t.Setenv("PS4", "") the var still exists with empty value;
	// to truly remove we'd need t.Setenv with empty + special handling. Instead,
	// just verify PS4 always ends up exactly the expected one.
	got := envWithPS4()
	hasPS4 := false
	for _, kv := range got {
		if kv == "PS4=+%x:%i> " {
			hasPS4 = true
		}
	}
	if !hasPS4 {
		t.Errorf("expected injected PS4; got: %v", got)
	}
}

// Smoke-test: ZshTracer either runs zsh successfully or returns an error.
// We don't assert the content because user's zsh config varies; this just
// covers the ZshTracer.RawTrace code path.
func TestZshTracer_Smoke(t *testing.T) {
	out, err := ZshTracer{}.RawTrace()
	// One of these two states is fine; both exercise the code path.
	if err != nil {
		t.Logf("zsh tracer error (acceptable, e.g. no zsh installed): %v", err)
		return
	}
	t.Logf("zsh tracer produced %d bytes of trace output", len(out))
}

func TestZshTracer_ErrorWhenZshUnavailable(t *testing.T) {
	// Force exec.Command("zsh") to fail by emptying PATH so the binary cannot be located.
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

// TracedStartup is a 1-line wrapper for TracedStartupWith(ZshTracer{}). The smoke test
// exercises it end-to-end; we don't separately assert content for the same reason
// TestZshTracer_Smoke doesn't.
func TestTracedStartup_Smoke(t *testing.T) {
	_, _ = TracedStartup()
}

// Ensure model.TraceEntry round-trips correctly through TracedStartupWith.
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
