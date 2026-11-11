package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"sarak.sh/mikluko/changelog/pkg/changelog"
	"sarak.sh/mikluko/changelog/pkg/git"
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

// forecast reports what releasing the changelog at path today, with the
// fragments in dir folded in, would raise that current, the findings about it
// as it stands, does not. Each is a fragments-release finding at the
// [Unreleased] heading, where the release lands. An empty dir, nothing to
// release, and fragments that do not parse, which fragment-form reports,
// forecast nothing.
func forecast(path, dir string, opts changelog.Options, current []changelog.Finding, today string, log, annotations io.Writer) ([]changelog.Finding, error) {
	sev := severity(opts, changelog.CheckFragmentsRelease)
	if dir == "" || sev == changelog.Off {
		return nil, nil
	}
	_, frags, err := readFragments(dir)
	if err != nil {
		return nil, nil
	}
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	line, notes := 1, ""
	for _, e := range changelog.Parse(src).Entries {
		if e.Unreleased {
			line, notes = e.Line, e.Body
			break
		}
	}
	if len(frags) == 0 && notes == "" {
		return nil, nil
	}

	finding := func(format string, args ...any) changelog.Finding {
		return changelog.Finding{Check: changelog.CheckFragmentsRelease, Severity: sev, Line: line, Msg: fmt.Sprintf(format, args...)}
	}
	var out []changelog.Finding
	released, v, err := changelog.ReleaseFragments(src, frags, nil, today)
	if err != nil {
		out = append(out, finding("releasing [Unreleased] with the fragments folded in fails: %v", err))
	} else {
		type key struct{ check, msg string }
		seen := make(map[key]bool, len(current))
		for _, f := range current {
			seen[key{f.Check, f.Msg}] = true
		}
		for _, f := range changelog.Parse(released).Lint(opts) {
			if !seen[key{f.Check, f.Msg}] {
				out = append(out, finding("releasing the fragments as %s raises %s: %s", v, f.Check, f.Msg))
			}
		}
	}
	report(path, out, log, annotations)
	return out, nil
}

// missingFragment reports a missing-fragment finding against the changelog at
// path where the fragments in dir include none that the merge base of HEAD and
// base lacks. An empty base or dir reports nothing, and so does a check
// switched off.
func missingFragment(path, dir, base string, opts changelog.Options, log, annotations io.Writer) ([]changelog.Finding, error) {
	sev := severity(opts, changelog.CheckMissingFragment)
	if base == "" || dir == "" || sev == changelog.Off {
		return nil, nil
	}
	paths, _, err := fragmentFiles(dir)
	if err != nil {
		return nil, err
	}
	msg, err := addsNoFragment(path, dir, base, paths)
	if err != nil {
		msg = fmt.Sprintf("cannot tell whether this pull request adds a fragment: %v; a checkout with fetch-depth: 0 carries %s", err, base)
	}
	if msg == "" {
		return nil, nil
	}
	out := []changelog.Finding{{Check: changelog.CheckMissingFragment, Severity: sev, Line: 1, Msg: msg}}
	report(path, out, log, annotations)
	return out, nil
}

// addsNoFragment returns why the fragments at paths add nothing to the merge
// base with base, or "" where one of them is new.
func addsNoFragment(path, dir, base string, paths []string) (string, error) {
	repo, err := git.Open(filepath.Dir(path))
	if err != nil {
		return "", err
	}
	rel, err := repoRelative(repo.Root(), dir)
	if err != nil {
		return "", err
	}
	before, err := repo.FilesAtMergeBase(base, rel)
	if err != nil {
		return "", err
	}
	for _, p := range paths {
		if !slices.Contains(before, filepath.Base(p)) {
			return "", nil
		}
	}
	return fmt.Sprintf("this pull request adds no fragment to %s; add a *.md file there carrying its notes", dir), nil
}
