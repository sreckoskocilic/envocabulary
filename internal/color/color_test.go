package color

import (
	"bytes"
	"testing"
)

func TestParseMode(t *testing.T) {
	cases := map[string]Mode{
		"":       Auto,
		"auto":   Auto,
		"always": Always,
		"yes":    Always,
		"never":  Never,
		"off":    Never,
	}
	for in, want := range cases {
		got, err := ParseMode(in)
		if err != nil {
			t.Errorf("ParseMode(%q) error: %v", in, err)
		}
		if got != want {
			t.Errorf("ParseMode(%q) = %v, want %v", in, got, want)
		}
	}
	if _, err := ParseMode("rainbow"); err == nil {
		t.Errorf("ParseMode(rainbow) should error")
	}
}

func TestEnabledNeverAlways(t *testing.T) {
	var buf bytes.Buffer
	if Never.Enabled(&buf) {
		t.Errorf("Never should never be enabled")
	}
	if !Always.Enabled(&buf) {
		t.Errorf("Always should always be enabled")
	}
}

func TestEnabledAutoNonTerminal(t *testing.T) {
	var buf bytes.Buffer
	if Auto.Enabled(&buf) {
		t.Errorf("Auto on non-terminal writer should be disabled")
	}
}

func TestEnabledAutoNoColorEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	if Auto.Enabled(&buf) {
		t.Errorf("Auto with NO_COLOR set should be disabled")
	}
}

func TestWrap(t *testing.T) {
	if got := Wrap("x", LightRed, false); got != "x" {
		t.Errorf("Wrap(off) = %q, want %q", got, "x")
	}
	if got := Wrap("x", LightRed, true); got != "\x1b[91mx\x1b[0m" {
		t.Errorf("Wrap(on) = %q, want %q", got, "\x1b[91mx\x1b[0m")
	}
}
