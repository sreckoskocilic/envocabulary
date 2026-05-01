package catalog

import (
	"bufio"
	"cmp"
	"fmt"
	"io"
	"os"
	"slices"
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

func Write(w io.Writer, opts Options) error {
	files, err := inventory.Discover()
	if err != nil {
		return err
	}
	keep := inventory.FilterFiles(files, opts.IncludeBash, opts.IncludeOrphans)
	slices.SortStableFunc(keep, func(a, b inventory.File) int {
		return cmp.Compare(inventory.FileRank(a), inventory.FileRank(b))
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
