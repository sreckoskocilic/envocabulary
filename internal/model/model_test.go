package model

import "testing"

func TestIsDeferredListVar(t *testing.T) {
	cases := map[string]bool{
		"PATH":                       true,
		"MANPATH":                    true,
		"INFOPATH":                   true,
		"FPATH":                      true,
		"CDPATH":                     true,
		"DYLD_LIBRARY_PATH":          true,
		"DYLD_FALLBACK_LIBRARY_PATH": true,
		"DYLD_FRAMEWORK_PATH":        true,
		"DYLD_INSERT_LIBRARIES":      true,
		"EDITOR":                     false,
		"HOME":                       false,
		"":                           false,
		"path":                       false, // case-sensitive
	}
	for name, want := range cases {
		if got := IsDeferredListVar(name); got != want {
			t.Errorf("IsDeferredListVar(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestIsDirenvVar(t *testing.T) {
	cases := map[string]bool{
		"DIRENV_DIR":     true,
		"DIRENV_FILE":    true,
		"DIRENV_DIFF":    true,
		"DIRENV_WATCHES": true,
		"DIRENV_OTHER":   false, // not in the explicit list
		"DIRENV":         false,
		"":               false,
	}
	for name, want := range cases {
		if got := IsDirenvVar(name); got != want {
			t.Errorf("IsDirenvVar(%q) = %v, want %v", name, got, want)
		}
	}
}
