package pathentry

import (
	"os"
	"strings"

	"github.com/sreckoskocilic/envocabulary/internal/inventory"
)

func OverrideFromConfig(results []VarBreakdown, files []inventory.File) {
	pathsFiles := scanPathsD()
	for i := range results {
		for j := range results[i].Entries {
			e := &results[i].Entries[j] //nolint:gosec // index-based, no aliasing
			if findConfigRef(e, results[i].Name, e.Dir, files) {
				continue
			}
			findPathsDRef(e, e.Dir, pathsFiles)
		}
	}
}

func findConfigRef(entry *Entry, varName, dir string, files []inventory.File) bool {
	found := false
	for _, f := range files {
		for _, item := range f.Items {
			if (item.Kind == inventory.KindExport || item.Kind == inventory.KindAssign) &&
				item.Name == varName &&
				valueContainsDir(item.Value, dir) {
				entry.File = f.Path
				entry.Line = item.Line
				found = true
			}
		}
	}
	return found
}

func valueContainsDir(value, dir string) bool {
	for _, seg := range strings.Split(value, ":") {
		if seg == dir {
			return true
		}
	}
	return false
}

type pathsDEntry struct {
	File string
	Line int
	Dir  string
}

var scanPathsD = scanPathsDFiles

func scanPathsDFiles() []pathsDEntry {
	var entries []pathsDEntry
	readLines := func(path string) {
		data, err := os.ReadFile(path)
		if err != nil {
			return
		}
		for i, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				entries = append(entries, pathsDEntry{File: path, Line: i + 1, Dir: line})
			}
		}
	}
	readLines("/etc/paths")
	dirEntries, err := os.ReadDir("/etc/paths.d")
	if err != nil {
		return entries
	}
	for _, de := range dirEntries {
		if !de.IsDir() {
			readLines("/etc/paths.d/" + de.Name())
		}
	}
	return entries
}

func findPathsDRef(entry *Entry, dir string, pathsFiles []pathsDEntry) {
	for _, p := range pathsFiles {
		if p.Dir == dir || strings.HasPrefix(p.Dir, dir+" ") {
			entry.File = p.File
			entry.Line = p.Line
			return
		}
	}
}
