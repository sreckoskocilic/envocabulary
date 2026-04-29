package catalog

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sreckoskocilic/envocabulary/internal/dedup"
	"github.com/sreckoskocilic/envocabulary/internal/inventory"
)

type Options struct {
	IncludeOrphans bool
	IncludeBash    bool
	LineNumbers    bool
	Dedup          bool
}

var zshLoginOrder = map[string]int{
	".zshenv":   0,
	".zprofile": 1,
	".zshrc":    2,
	".zlogin":   3,
	".zlogout":  4,
}

func Write(w io.Writer, opts Options) error {
	files := inventory.Discover()
	keep := filterFiles(files, opts)
	sort.SliceStable(keep, func(i, j int) bool {
		return roleOrder(keep[i]) < roleOrder(keep[j])
	})

	var losers map[string]dedup.Occurrence
	if opts.Dedup {
		losers = dedup.LoserSet(dedup.Find(keep))
	}

	for i, f := range keep {
		if i > 0 {
			fmt.Fprintln(w)
		}
		if err := writeFile(w, f, opts, losers); err != nil {
			fmt.Fprintf(w, "# error reading %s: %v\n", f.Path, err)
		}
	}
	return nil
}

func filterFiles(files []inventory.File, opts Options) []inventory.File {
	var out []inventory.File
	for _, f := range files {
		switch f.Role {
		case inventory.RoleCanonicalZsh:
			out = append(out, f)
		case inventory.RoleCanonicalBash:
			if opts.IncludeBash {
				out = append(out, f)
			}
		case inventory.RoleOrphan:
			if opts.IncludeOrphans && isZshOrphan(f.Path, opts.IncludeBash) {
				out = append(out, f)
			}
		}
	}
	return out
}

func isZshOrphan(path string, includeBash bool) bool {
	name := filepath.Base(path)
	if strings.Contains(name, "zsh") || strings.HasPrefix(name, ".zsh") || strings.HasPrefix(name, ".zprofile") || strings.HasPrefix(name, ".zlog") {
		return true
	}
	if includeBash {
		return true
	}
	return false
}

func roleOrder(f inventory.File) int {
	base := filepath.Base(f.Path)
	switch f.Role {
	case inventory.RoleCanonicalZsh:
		return zshLoginOrder[base]
	case inventory.RoleCanonicalBash:
		return 100
	case inventory.RoleOrphan:
		return 200
	}
	return 999
}

func writeFile(w io.Writer, f inventory.File, opts Options, losers map[string]dedup.Occurrence) error {
	fh, err := os.Open(f.Path)
	if err != nil {
		return err
	}
	defer fh.Close()

	suffix := ""
	switch f.Role {
	case inventory.RoleOrphan:
		suffix = "  (orphan)"
	case inventory.RoleCanonicalBash:
		suffix = "  (bash)"
	case inventory.RoleCanonicalZsh:
	}
	bar := strings.Repeat("=", 68)
	fmt.Fprintf(w, "# %s\n", bar)
	fmt.Fprintf(w, "# %s%s\n", f.Path, suffix)
	fmt.Fprintf(w, "# %s\n", bar)

	sc := bufio.NewScanner(fh)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		text := sc.Text()
		if opts.Dedup && losers != nil {
			if winner, ok := losers[dedup.Key(f.Path, lineNo)]; ok {
				text = fmt.Sprintf("# [overridden by %s:%d] %s", winner.File, winner.Line, text)
			}
		}
		if opts.LineNumbers {
			fmt.Fprintf(w, "%5d  %s\n", lineNo, text)
		} else {
			fmt.Fprintln(w, text)
		}
	}
	return sc.Err()
}
