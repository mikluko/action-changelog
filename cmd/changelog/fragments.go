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

	"sarak.sh/mikluko/changelog/v2/pkg/changelog"
	"sarak.sh/mikluko/changelog/v2/pkg/git"
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
			if err := report(p, []changelog.Finding{f}, log, annotations); err != nil {
				return nil, err
			}
			all = append(all, f)
		}
	}
	for _, p := range paths {
		src, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		findings := changelog.LintFragment(src, opts)
		if err := report(p, findings, log, annotations); err != nil {
			return nil, err
		}
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

// forecast reports what releasing the changelog at path today, as
// ReleasePending does with the fragments in dir folded in, would raise that
// current, the findings about it as it stands, does not. Each is a
// fragments-release finding at the heading of the entry being released: the
// open entry where there is one, and otherwise [Unreleased]. An empty dir,
// nothing pending, and fragments that do not parse, which fragment-form
// reports, forecast nothing.
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
	doc := changelog.Parse(src)
	line, pending := 1, len(frags) > 0
	for _, e := range doc.Entries {
		if e.Unreleased {
			line, pending = e.Line, pending || e.Body != ""
			break
		}
	}
	if open, ok := doc.Latest(); ok && open.Date == "" {
		line, pending = open.Line, true
	}
	if !pending {
		return nil, nil
	}

	finding := func(format string, args ...any) changelog.Finding {
		return changelog.Finding{Check: changelog.CheckFragmentsRelease, Severity: sev, Line: line, Msg: fmt.Sprintf(format, args...)}
	}
	var out []changelog.Finding
	released, v, err := changelog.ReleasePending(src, frags, today)
	if err != nil {
		out = append(out, finding("releasing what is pending, with the fragments folded in, fails: %v", err))
	} else {
		type key struct{ check, msg string }
		seen := make(map[key]bool, len(current))
		for _, f := range current {
			seen[key{f.Check, f.Msg}] = true
		}
		for _, f := range changelog.Parse(released).Lint(withoutNextTagDay(opts)) {
			if !seen[key{f.Check, f.Msg}] {
				out = append(out, finding("releasing %s with the fragments folded in raises %s: %s", v, f.Check, f.Msg))
			}
		}
	}
	return out, report(path, out, log, annotations)
}

// withoutNextTagDay is opts with the day a tag cut on HEAD would carry left
// out, on a copy: the caller's options and their Git are not touched. A release
// is cut on a commit not yet made, so that day is not the day of the one the
// forecast folds, and pending-date-mismatch is silent in it.
func withoutNextTagDay(opts changelog.Options) changelog.Options {
	if opts.Git == nil {
		return opts
	}
	g := *opts.Git
	g.NextTagDay = ""
	opts.Git = &g
	return opts
}

// missingFragment reports a missing-fragment finding against the changelog at
// path where the pull request carries no notes: none of the fragments in dir is
// missing from the merge base of HEAD and base, and the changelog carries no
// note it lacked there. An empty base or dir reports nothing, and so does a
// check switched off.
func missingFragment(path, dir, base string, opts changelog.Options, log, annotations io.Writer) ([]changelog.Finding, error) {
	sev := severity(opts, changelog.CheckMissingFragment)
	if base == "" || dir == "" || sev == changelog.Off {
		return nil, nil
	}
	paths, _, err := fragmentFiles(dir)
	if err != nil {
		return nil, err
	}
	msg, err := carriesNoNotes(path, dir, base, paths)
	if err != nil {
		msg = fmt.Sprintf("cannot tell whether this pull request carries notes: %v; a checkout with fetch-depth: 0 carries %s", err, base)
	}
	if msg == "" {
		return nil, nil
	}
	out := []changelog.Finding{{Check: changelog.CheckMissingFragment, Severity: sev, Line: 1, Msg: msg}}
	return out, report(path, out, log, annotations)
}

// carriesNoNotes returns why the pull request carries no notes, or "" where a
// fragment at paths or a note in the changelog at path is new since the merge
// base with base.
func carriesNoNotes(path, dir, base string, paths []string) (string, error) {
	repo, err := git.Open(filepath.Dir(path))
	if err != nil {
		return "", err
	}
	snap, err := repo.MergeBase(base)
	if err != nil {
		return "", err
	}
	relDir, err := repoRelative(repo.Root(), dir)
	if err != nil {
		return "", err
	}
	before, err := snap.Files(relDir)
	if err != nil {
		return "", err
	}
	for _, p := range paths {
		if !slices.Contains(before, filepath.Base(p)) {
			return "", nil
		}
	}

	relPath, err := repoRelative(repo.Root(), path)
	if err != nil {
		return "", err
	}
	was, err := snap.File(relPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	now, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	known := make(map[string]bool)
	for _, e := range changelog.Parse(was).Entries {
		for _, n := range e.Notes() {
			known[n] = true
		}
	}
	for _, e := range changelog.Parse(now).Entries {
		for _, n := range e.Notes() {
			if !known[n] {
				return "", nil
			}
		}
	}
	return fmt.Sprintf("this pull request carries no notes; add a *.md fragment to %s carrying them", dir), nil
}
