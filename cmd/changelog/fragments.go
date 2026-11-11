package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"sarak.sh/mikluko/changelog/pkg/changelog"
)

// fragmentFiles returns the fragments in dir in name order, and the entries
// beside them that are not fragments: subdirectories and files not named *.md.
// A name starting with a dot is neither, so a .gitkeep can hold the directory
// open. An empty dir, or one that does not exist, holds no fragments.
func fragmentFiles(dir string) (frags, strays []string, err error) {
	if dir == "" {
		return nil, nil, nil
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	for _, e := range entries {
		name := e.Name()
		path := filepath.Join(dir, name)
		switch {
		case strings.HasPrefix(name, "."):
		case e.IsDir() || !strings.HasSuffix(name, ".md"):
			strays = append(strays, path)
		default:
			frags = append(frags, path)
		}
	}
	return frags, strays, nil
}

// readFragments parses every fragment in dir, in name order, refusing the
// whole directory where any entry in it is not a fragment.
func readFragments(dir string) (paths []string, frags []changelog.Fragment, err error) {
	paths, strays, err := fragmentFiles(dir)
	if err != nil {
		return nil, nil, err
	}
	if len(strays) > 0 {
		return nil, nil, fmt.Errorf("%s is not a fragment: a fragment is a file named *.md", strays[0])
	}
	for _, p := range paths {
		src, err := os.ReadFile(p)
		if err != nil {
			return nil, nil, err
		}
		frag, err := changelog.ParseFragment(src)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", p, err)
		}
		frags = append(frags, frag)
	}
	return paths, frags, nil
}

// lintFragments reports every finding about the fragments in dir, each against
// its own file, as run does for the changelog.
func lintFragments(dir string, opts changelog.Options, log, annotations io.Writer) ([]changelog.Finding, error) {
	paths, strays, err := fragmentFiles(dir)
	if err != nil {
		return nil, err
	}
	var all []changelog.Finding
	if sev := severity(opts, changelog.CheckFragmentForm); sev != changelog.Off {
		for _, p := range strays {
			f := changelog.Finding{Check: changelog.CheckFragmentForm, Severity: sev, Line: 1, Msg: "not a fragment: a fragment is a file named *.md"}
			report(p, []changelog.Finding{f}, log, annotations)
			all = append(all, f)
		}
	}
	for _, p := range paths {
		src, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		findings := changelog.LintFragment(src, opts)
		report(p, findings, log, annotations)
		all = append(all, findings...)
	}
	return all, nil
}

// severity is the severity opts puts check at.
func severity(opts changelog.Options, check string) changelog.Severity {
	if opts.Severities == nil {
		c, _ := changelog.Lookup(check)
		return c.Default
	}
	return opts.Severities[check]
}
