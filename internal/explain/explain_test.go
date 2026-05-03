package explain

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sreckoskocilic/envocabulary/internal/model"
)

func TestExplain_DeferredListVar(t *testing.T) {
	r := Explain("PATH", map[string]string{"PATH": "/usr/bin"}, nil)
	if r.Origin != model.OriginDeferred {
		t.Errorf("origin = %q, want %q", r.Origin, model.OriginDeferred)
	}
	if r.Notes == "" {
		t.Errorf("expected notes for deferred-list-var")
	}
}

func TestExplain_DirenvVar(t *testing.T) {
	r := Explain("DIRENV_DIR", map[string]string{"DIRENV_DIR": "/tmp"}, nil)
	if r.Origin != model.OriginDirenv {
		t.Errorf("origin = %q, want %q", r.Origin, model.OriginDirenv)
	}
}

func TestExplain_ShellFileWithSingleWriter(t *testing.T) {
	current := map[string]string{"EDITOR": "vim"}
	trace := []model.TraceEntry{
		{Name: "EDITOR", File: "/u/.zshrc", Line: 12, Raw: "export EDITOR=vim"},
	}
	r := Explain("EDITOR", current, trace)
	if r.Origin != model.OriginShellFile {
		t.Errorf("origin = %q, want %q", r.Origin, model.OriginShellFile)
	}
	if r.Primary != "/u/.zshrc:12" {
		t.Errorf("primary = %q, want %q", r.Primary, "/u/.zshrc:12")
	}
	if len(r.Writers) != 1 {
		t.Errorf("writers = %d, want 1", len(r.Writers))
	}
}

func TestExplain_ShellFileMultipleWritersLastWins(t *testing.T) {
	current := map[string]string{"FOO": "second"}
	trace := []model.TraceEntry{
		{Name: "FOO", File: "/u/.zprofile", Line: 5},
		{Name: "FOO", File: "/u/.zshrc", Line: 20},
	}
	r := Explain("FOO", current, trace)
	if r.Primary != "/u/.zshrc:20" {
		t.Errorf("primary = %q, want %q (last writer)", r.Primary, "/u/.zshrc:20")
	}
	if len(r.Writers) != 2 {
		t.Errorf("writers = %d, want 2", len(r.Writers))
	}
}

func TestExplain_FallsBackToClassifier(t *testing.T) {
	r := Explain("USER", map[string]string{"USER": "alice"}, nil)
	if r.Origin != model.OriginSystem {
		t.Errorf("origin = %q, want %q", r.Origin, model.OriginSystem)
	}
}

func TestExplain_NotInEnvButHasTrace(t *testing.T) {
	trace := []model.TraceEntry{{Name: "TEMP", File: "/x", Line: 1}}
	r := Explain("TEMP", map[string]string{}, trace)
	if r.Present {
		t.Errorf("expected Present=false")
	}
	if len(r.Writers) != 1 {
		t.Errorf("expected 1 writer despite absence")
	}
}

func TestExplain_NotPresentNoTrace(t *testing.T) {
	r := Explain("ABSENT_VAR", map[string]string{}, nil)
	if r.Present {
		t.Errorf("expected Present=false")
	}
	if r.Origin != model.OriginUnknown {
		t.Errorf("origin = %q, want %q", r.Origin, model.OriginUnknown)
	}
}

func TestEmitText_WinnerMarkerOnLastWriter(t *testing.T) {
	r := Result{
		Name:    "FOO",
		Present: true,
		Origin:  model.OriginShellFile,
		Primary: "/u/.zshrc:20",
		Writers: []model.TraceEntry{
			{File: "/u/.zprofile", Line: 5},
			{File: "/u/.zshrc", Line: 20},
		},
	}
	var buf bytes.Buffer
	EmitText(&buf, r, false, false)
	out := buf.String()
	if !strings.Contains(out, "/u/.zshrc:20  (winner)") {
		t.Errorf("expected winner marker on last writer; got:\n%s", out)
	}
	if !strings.Contains(out, "[hidden, use --values]") {
		t.Errorf("expected hidden value notice; got:\n%s", out)
	}
}

func TestEmitText_ShowValues(t *testing.T) {
	r := Result{
		Name:    "FOO",
		Present: true,
		Value:   "bar",
		Origin:  model.OriginShellFile,
		Writers: []model.TraceEntry{{File: "/u/.zshrc", Line: 1, Raw: "export FOO=bar"}},
	}
	var buf bytes.Buffer
	EmitText(&buf, r, true, false)
	out := buf.String()
	if !strings.Contains(out, "value    bar") {
		t.Errorf("expected value to appear; got:\n%s", out)
	}
	if !strings.Contains(out, "raw      export FOO=bar") {
		t.Errorf("expected raw line for single writer with showValues; got:\n%s", out)
	}
}

func TestEmitText_NotPresentButHasWriters(t *testing.T) {
	r := Result{
		Name:    "GHOST",
		Present: false,
		Origin:  model.OriginShellFile,
		Writers: []model.TraceEntry{
			{File: "/u/.zshrc", Line: 5, Raw: "export GHOST=here"},
			{File: "/u/.zshrc", Line: 99, Raw: "unset GHOST"},
		},
	}
	var buf bytes.Buffer
	EmitText(&buf, r, false, false)
	out := buf.String()
	if !strings.Contains(out, "not in current environment") {
		t.Errorf("expected not-in-env line; got:\n%s", out)
	}
	if !strings.Contains(out, "seen 2 writer(s)") {
		t.Errorf("expected writer-count notice for not-present-with-writers case; got:\n%s", out)
	}
}

func TestEmitText_Notes(t *testing.T) {
	r := Result{
		Name:    "PATH",
		Present: true,
		Origin:  model.OriginDeferred,
		Notes:   "multi-source; envocabulary path (TODO)",
	}
	var buf bytes.Buffer
	EmitText(&buf, r, false, false)
	if !strings.Contains(buf.String(), "multi-source") {
		t.Errorf("expected notes in output; got:\n%s", buf.String())
	}
}

func TestEmitText_NotPresent(t *testing.T) {
	var buf bytes.Buffer
	EmitText(&buf, Result{Name: "GONE"}, false, false)
	if !strings.Contains(buf.String(), "not in current environment") {
		t.Errorf("expected not-in-env notice; got:\n%s", buf.String())
	}
}

func TestEmitText_MultipleWritersShowValues(t *testing.T) {
	r := Result{
		Name:    "FOO",
		Present: true,
		Value:   "second",
		Origin:  model.OriginShellFile,
		Primary: "/u/.zshrc:20",
		Writers: []model.TraceEntry{
			{File: "/u/.zprofile", Line: 5, Raw: "export FOO=first"},
			{File: "/u/.zshrc", Line: 20, Raw: "export FOO=second"},
		},
	}
	var buf bytes.Buffer
	EmitText(&buf, r, true, false)
	out := buf.String()
	if !strings.Contains(out, "export FOO=first") {
		t.Errorf("expected raw for first writer; got:\n%s", out)
	}
	if !strings.Contains(out, "export FOO=second  (winner)") {
		t.Errorf("expected raw+winner marker for last writer; got:\n%s", out)
	}
	if !strings.Contains(out, "value    second") {
		t.Errorf("expected value line; got:\n%s", out)
	}
}

func TestEmitJSON_ValuesHidden(t *testing.T) {
	r := Result{
		Name:    "X",
		Value:   "secret",
		Writers: []model.TraceEntry{{File: "/x", Line: 1, Raw: "export X=secret"}},
	}
	var buf bytes.Buffer
	if err := EmitJSON(&buf, r, false); err != nil {
		t.Fatal(err)
	}
	var got Result
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Value != "" {
		t.Errorf("expected Value to be cleared without --values, got %q", got.Value)
	}
	if got.Writers[0].Raw != "" {
		t.Errorf("expected Raw to be cleared without --values, got %q", got.Writers[0].Raw)
	}
}

func TestEmitJSON_DoesNotMutateCaller(t *testing.T) {
	r := Result{
		Name:    "X",
		Value:   "secret",
		Writers: []model.TraceEntry{{File: "/x", Line: 1, Raw: "export X=secret"}},
	}
	var buf bytes.Buffer
	if err := EmitJSON(&buf, r, false); err != nil {
		t.Fatal(err)
	}
	if r.Writers[0].Raw != "export X=secret" {
		t.Errorf("EmitJSON mutated caller's Writers; Raw = %q", r.Writers[0].Raw)
	}
}

func TestEmitJSON_ValuesShown(t *testing.T) {
	r := Result{Name: "X", Value: "secret"}
	var buf bytes.Buffer
	if err := EmitJSON(&buf, r, true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "secret") {
		t.Errorf("expected value to appear with --values; got:\n%s", buf.String())
	}
}

func TestEmitText_ChainShown(t *testing.T) {
	r := Result{
		Name:    "FOO",
		Present: true,
		Origin:  model.OriginShellFile,
		Primary: "/u/helpers.sh:3",
		Writers: []model.TraceEntry{
			{File: "/u/helpers.sh", Line: 3, Chain: []string{"/u/.zshrc"}},
		},
	}
	var buf bytes.Buffer
	EmitText(&buf, r, false, true)
	out := buf.String()
	if !strings.Contains(out, "chain    /u/.zshrc → /u/helpers.sh") {
		t.Errorf("expected chain line; got:\n%s", out)
	}
}

func TestEmitText_ChainHiddenWithoutFlag(t *testing.T) {
	r := Result{
		Name:    "FOO",
		Present: true,
		Origin:  model.OriginShellFile,
		Primary: "/u/helpers.sh:3",
		Writers: []model.TraceEntry{
			{File: "/u/helpers.sh", Line: 3, Chain: []string{"/u/.zshrc"}},
		},
	}
	var buf bytes.Buffer
	EmitText(&buf, r, false, false)
	if strings.Contains(buf.String(), "chain") {
		t.Errorf("expected no chain without flag; got:\n%s", buf.String())
	}
}

func TestEmitText_ChainOmittedForTopLevel(t *testing.T) {
	r := Result{
		Name:    "FOO",
		Present: true,
		Origin:  model.OriginShellFile,
		Primary: "/u/.zshrc:5",
		Writers: []model.TraceEntry{
			{File: "/u/.zshrc", Line: 5},
		},
	}
	var buf bytes.Buffer
	EmitText(&buf, r, false, true)
	if strings.Contains(buf.String(), "chain") {
		t.Errorf("expected no chain for top-level file; got:\n%s", buf.String())
	}
}

func TestEmitText_MultiWriterChain(t *testing.T) {
	r := Result{
		Name:    "FOO",
		Present: true,
		Origin:  model.OriginShellFile,
		Primary: "/u/helpers.sh:3",
		Writers: []model.TraceEntry{
			{File: "/u/.zprofile", Line: 10},
			{File: "/u/helpers.sh", Line: 3, Chain: []string{"/u/.zshrc"}},
		},
	}
	var buf bytes.Buffer
	EmitText(&buf, r, false, true)
	out := buf.String()
	if !strings.Contains(out, "chain    /u/.zshrc → /u/helpers.sh") {
		t.Errorf("expected primary chain; got:\n%s", out)
	}
	if !strings.Contains(out, "(via /u/.zshrc)") {
		t.Errorf("expected per-writer chain annotation; got:\n%s", out)
	}
}

func TestEmitJSON_ChainIncluded(t *testing.T) {
	r := Result{
		Name:    "FOO",
		Present: true,
		Writers: []model.TraceEntry{
			{File: "/u/helpers.sh", Line: 3, Chain: []string{"/u/.zshrc"}},
		},
	}
	var buf bytes.Buffer
	if err := EmitJSON(&buf, r, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"chain"`) {
		t.Errorf("expected chain in JSON; got:\n%s", buf.String())
	}
}
