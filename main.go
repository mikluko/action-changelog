// Command action-changelog reads a Keep a Changelog document and reports where
// it departs from the format.
//
// It runs as a GitHub Action and as a local command; under GitHub Actions each
// finding is additionally emitted as a workflow annotation, so a failing check
// lands on the offending line of the diff rather than only in the log.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/mikluko/action-changelog/internal/changelog"
	"github.com/mikluko/action-changelog/internal/git"
)

func main() {
	var (
		path       = flag.String("changelog", "CHANGELOG.md", "path to the changelog")
		validate   = flag.Bool("validate", false, "report where the changelog departs from the format")
		sections   = flag.String("sections", "", "comma-separated level-3 headings to accept; the Keep a Changelog six when empty")
		asError    = flag.String("error", "", "comma-separated checks to raise as errors")
		asWarning  = flag.String("warn", "", "comma-separated checks to raise as warnings")
		asOff      = flag.String("off", "", "comma-separated checks to switch off")
		failOn     = flag.String("fail-on", "error", "exit non-zero on error, warning, or never")
		listChecks = flag.Bool("list-checks", false, "print the checks and their default severities")
	)
	flag.Parse()

	if *listChecks {
		listRegister(os.Stdout)
		return
	}
	if !*validate {
		fmt.Fprintln(os.Stderr, "nothing to do: pass -validate")
		flag.Usage()
		os.Exit(2)
	}

	severities, err := severities(*asError, *asWarning, *asOff)
	if err != nil {
		fail(err)
	}
	threshold, err := threshold(*failOn)
	if err != nil {
		fail(err)
	}

	opts := changelog.Options{
		Sections:   split(*sections),
		Severities: severities,
		Git:        state(*path),
	}
	red, err := run(*path, opts, threshold, os.Stderr, os.Stdout)
	if err != nil {
		fail(err)
	}
	if red {
		os.Exit(1)
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "action-changelog: %v\n", err)
	os.Exit(1)
}

// run validates the document and reports whether anything it found is bad
// enough to turn the build red.
//
// Findings go to log, and under GitHub Actions each is repeated on annotations
// as a workflow command carrying the check's name as its title.
func run(path string, opts changelog.Options, threshold changelog.Severity, log, annotations io.Writer) (bool, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	annotate := os.Getenv("GITHUB_ACTIONS") == "true"

	var red bool
	for _, f := range changelog.Parse(src).Lint(opts) {
		fmt.Fprintf(log, "%s:%d: %s: %s (%s)\n", path, f.Line, f.Severity, f.Msg, f.Check)
		if annotate {
			fmt.Fprintf(annotations, "::%s file=%s,line=%d,title=%s::%s\n", f.Severity, path, f.Line, f.Check, f.Msg)
		}
		if threshold != changelog.Off && f.Severity >= threshold {
			red = true
		}
	}
	return red, nil
}

// state reads what the tag-dependent checks compare against: the newest version
// tag of the repository holding the changelog, and the changelog as it stood
// there.
//
// Every way that reading can fail is a populated Err rather than a returned
// error, because failing to read the history is itself one of the checks and is
// reported by name at the severity the caller configured.
func state(path string) *changelog.Git {
	repo, err := git.Open(filepath.Dir(path))
	if err != nil {
		return &changelog.Git{Err: err}
	}
	tags, err := repo.Tags()
	if err != nil {
		return &changelog.Git{Err: err}
	}
	newest, ok := git.Newest(tags)
	if !ok {
		// A shallow clone carries the history it was given rather than the one
		// that exists, so the absence of tags says nothing about the repository
		// and everything about the checkout. A complete history holding no tag
		// is a repository before its first release, which is not a defect.
		shallow, err := repo.Shallow()
		if err != nil {
			return &changelog.Git{Err: err}
		}
		if shallow {
			return &changelog.Git{Err: errors.New("the checkout is shallow and carries no tags")}
		}
		return &changelog.Git{}
	}

	out := &changelog.Git{NewestTag: newest.Name}
	rel, err := repoRelative(repo.Root(), path)
	if err != nil {
		return out
	}
	// A tag cut before the file existed, or before it moved here, carries no
	// such blob. There is nothing to compare and nothing to report.
	if src, err := repo.FileAt(newest.Name, rel); err == nil {
		out.TaggedChangelog = src
	}
	return out
}

// repoRelative rewrites a path on this machine as the slash-separated path a
// git tree stores.
func repoRelative(root, path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

// severities resolves the register's defaults against the three override flags.
//
// They are applied in the order error, warn, off, so a check named in two of
// them takes the later spelling rather than an order the caller cannot see.
func severities(asError, asWarning, asOff string) (changelog.Severities, error) {
	out := changelog.DefaultSeverities()
	for _, o := range []struct {
		names string
		sev   changelog.Severity
	}{
		{asError, changelog.Error},
		{asWarning, changelog.Warning},
		{asOff, changelog.Off},
	} {
		for _, name := range split(o.names) {
			if err := out.Set(name, o.sev); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

// threshold reads -fail-on, whose "never" is changelog.Off: no finding is ever
// at or above it, so the command exits 0 and a workflow reads the outputs and
// branches on them instead.
func threshold(s string) (changelog.Severity, error) {
	if strings.TrimSpace(s) == "never" {
		return changelog.Off, nil
	}
	sev, err := changelog.ParseSeverity(s)
	if err != nil || sev == changelog.Off {
		return changelog.Off, fmt.Errorf("-fail-on %q is not one of error, warning, never", s)
	}
	return sev, nil
}

// listRegister prints every check and the severity it carries unless a flag
// says otherwise.
func listRegister(w io.Writer) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "CHECK\tDEFAULT\tDESCRIPTION")
	for _, c := range changelog.Checks {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", c.Name, c.Default, c.Description)
	}
	tw.Flush()
}

// split reads a comma-separated flag value, dropping empty fields so a trailing
// comma is not a section named "".
func split(s string) []string {
	var out []string
	for _, f := range strings.Split(s, ",") {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}
