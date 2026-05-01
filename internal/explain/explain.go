package explain

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/sreckoskocilic/envocabulary/internal/buckets"
	"github.com/sreckoskocilic/envocabulary/internal/model"
)

type Result struct {
	Name    string             `json:"name"`
	Present bool               `json:"present"`
	Value   string             `json:"value,omitempty"`
	Origin  model.Origin       `json:"origin"`
	Primary string             `json:"primary,omitempty"`
	Writers []model.TraceEntry `json:"writers,omitempty"`
	Notes   string             `json:"notes,omitempty"`
}

func Explain(name string, current map[string]string, trace []model.TraceEntry) Result {
	r := Result{Name: name}
	value, present := current[name]
	r.Value = value
	r.Present = present

	for _, e := range trace {
		if e.Name == name {
			r.Writers = append(r.Writers, e)
		}
	}

	switch {
	case model.IsDeferredListVar(name):
		r.Origin = model.OriginDeferred
		r.Notes = "multi-source; envocabulary path (TODO)"

	case model.IsDirenvVar(name):
		r.Origin = model.OriginDirenv

	default:
		if len(r.Writers) > 0 {
			last := r.Writers[len(r.Writers)-1]
			r.Origin = model.OriginShellFile
			r.Primary = fmt.Sprintf("%s:%d", last.File, last.Line)
		} else if present {
			origin, note := buckets.Classify(name, value)
			r.Origin = origin
			r.Notes = note
		} else {
			r.Origin = model.OriginUnknown
			r.Notes = "not in current environment"
		}
	}

	return r
}

func EmitText(w io.Writer, r Result, showValues bool) {
	fmt.Fprintln(w, r.Name)

	if !r.Present {
		fmt.Fprintln(w, "  not in current environment")
		if len(r.Writers) > 0 {
			fmt.Fprintf(w, "  seen %d writer(s) in trace — was it unset after assignment?\n", len(r.Writers))
		} else {
			return
		}
	}

	fmt.Fprintf(w, "  origin   %s\n", r.Origin)
	if r.Primary != "" {
		fmt.Fprintf(w, "  primary  %s\n", r.Primary)
	}

	if len(r.Writers) > 1 {
		fmt.Fprintln(w, "  writers")
		for i, e := range r.Writers {
			marker := ""
			if i == len(r.Writers)-1 {
				marker = "  (winner)"
			}
			if showValues {
				fmt.Fprintf(w, "    %s:%d  %s%s\n", e.File, e.Line, e.Raw, marker)
			} else {
				fmt.Fprintf(w, "    %s:%d%s\n", e.File, e.Line, marker)
			}
		}
	} else if len(r.Writers) == 1 && showValues {
		fmt.Fprintf(w, "  raw      %s\n", r.Writers[0].Raw)
	}

	if r.Notes != "" {
		fmt.Fprintf(w, "  notes    %s\n", r.Notes)
	}

	if showValues && r.Present {
		fmt.Fprintf(w, "  value    %s\n", r.Value)
	} else if r.Present {
		fmt.Fprintln(w, "  value    [hidden, use --values]")
	}
}

func EmitJSON(w io.Writer, r Result, showValues bool) error {
	if !showValues {
		r.Value = ""
		writers := make([]model.TraceEntry, len(r.Writers))
		copy(writers, r.Writers)
		for i := range writers {
			writers[i].Raw = ""
		}
		r.Writers = writers
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}
