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
	"strconv"
	"strings"
	"text/tabwriter"

	"golang.org/x/mod/semver"

	"github.com/mikluko/action-changelog/internal/changelog"
	"github.com/mikluko/action-changelog/internal/git"
	"github.com/mikluko/action-changelog/internal/output"
)

func main() {
	var (
		path       = flag.String("changelog", "CHANGELOG.md", "path to the changelog")
		sections   = flag.String("sections", "", "comma-separated level-3 headings to accept; the Keep a Changelog six when empty")
		asError    = flag.String("error", "", "comma-separated checks to raise as errors")
		asWarning  = flag.String("warn", "", "comma-separated checks to raise as warnings")
		asOff      = flag.String("off", "", "comma-separated checks to switch off")
		failOn     = flag.String("fail-on", "error", "exit non-zero on error, warning, or never")
		refTags    = flag.String("reference-tags", "final", "which tags may be the reference tag: final or all")
		listChecks = flag.Bool("list-checks", false, "print the checks and their default severities")
	)
	flag.Parse()

	if *listChecks {
		listRegister(os.Stdout)
		return
	}
	severities, err := severities(*asError, *asWarning, *asOff)
	if err != nil {
		fail(err)
	}
	threshold, err := threshold(*failOn)
	if err != nil {
		fail(err)
	}
	admit, err := git.ParseEligible(*refTags)
	if err != nil {
		fail(err)
	}

	repo := state(*path, admit)
	opts := changelog.Options{
		Sections:   split(*sections),
		Severities: severities,
		Git:        repo.Check,
	}
	doc, findings, err := run(*path, opts, os.Stderr, os.Stdout)
	if err != nil {
		fail(err)
	}
	if err := emit(outputs(doc, repo.Tags, repo.Reference, findings), os.Stdout); err != nil {
		fail(err)
	}
	if red(findings, threshold) {
		os.Exit(1)
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "action-changelog: %v\n", err)
	os.Exit(1)
}

// run validates the document, reporting every finding to log and, under GitHub
// Actions, repeating each on annotations as a workflow command carrying the
// check's name as its title.
//
// It returns the parsed document beside the findings because the outputs are
// read off both.
func run(path string, opts changelog.Options, log, annotations io.Writer) (*changelog.Changelog, []changelog.Finding, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	doc := changelog.Parse(src)
	findings := doc.Lint(opts)

	annotate := os.Getenv("GITHUB_ACTIONS") == "true"
	for _, f := range findings {
		fmt.Fprintf(log, "%s:%d: %s: %s (%s)\n", path, f.Line, f.Severity, f.Msg, f.Check)
		if annotate {
			fmt.Fprintf(annotations, "::%s file=%s,line=%d,title=%s::%s\n", f.Severity, path, f.Line, f.Check, f.Msg)
		}
	}
	return doc, findings, nil
}

// red reports whether anything found is bad enough to turn the build red.
//
// A threshold of Off is -fail-on never: no finding is at or above it.
func red(findings []changelog.Finding, threshold changelog.Severity) bool {
	if threshold == changelog.Off {
		return false
	}
	for _, f := range findings {
		if f.Severity >= threshold {
			return true
		}
	}
	return false
}

// valid reports whether the document conforms, which is a finding at Error and
// nothing weaker.
//
// It is deliberately independent of -fail-on: a workflow passes never precisely
// so it can read this verdict and branch on it, and a verdict derived from the
// threshold would answer true every time it did.
func valid(findings []changelog.Finding) bool {
	for _, f := range findings {
		if f.Severity >= changelog.Error {
			return false
		}
	}
	return true
}

// repoState is what one run reads from the repository: the state the
// tag-dependent checks compare against, the reference tag they and the outputs
// report, and the tags already-tagged is answered from.
//
// The reference is resolved once and carried here because four consumers ask
// for it and resolving it walks the commit graph.
type repoState struct {
	Check     *changelog.Git
	Tags      []git.Tag
	Reference string
}

// state reads what the tag-dependent checks compare against: the reference tag
// of the repository holding the changelog, and the changelog as it stood there.
//
// Every way that reading can fail is a populated Err rather than a returned
// error, because failing to read the history is itself one of the checks and is
// reported by name at the severity the caller configured.
func state(path string, admit git.Eligible) repoState {
	repo, err := git.Open(filepath.Dir(path))
	if err != nil {
		return repoState{Check: &changelog.Git{Err: err}}
	}
	tags, err := repo.Tags()
	if err != nil {
		return repoState{Check: &changelog.Git{Err: err}}
	}
	reference, ok, err := repo.Reference(tags, admit)
	if err != nil {
		return repoState{Check: &changelog.Git{Err: err}}
	}
	if !ok {
		// A shallow clone carries the history it was given rather than the one
		// that exists, so a reference the checkout cannot reach says nothing
		// about the repository and everything about the checkout. A complete
		// history reaching no tag is a repository before its first release, or
		// a line that has never been released, and neither is a defect.
		shallow, err := repo.Shallow()
		if err != nil {
			return repoState{Check: &changelog.Git{Err: err}}
		}
		if shallow {
			return repoState{Check: &changelog.Git{Err: errors.New("the checkout is shallow and reaches no version tag")}}
		}
		return repoState{Check: &changelog.Git{}, Tags: tags}
	}

	out := repoState{Check: &changelog.Git{ReferenceTag: reference.Name}, Tags: tags, Reference: reference.Name}
	rel, err := repoRelative(repo.Root(), path)
	if err != nil {
		return out
	}
	// A tag cut before the file existed, or before it moved here, carries no
	// such blob. There is nothing to compare and nothing to report.
	if src, err := repo.FileAt(reference.Name, rel); err == nil {
		out.Check.TaggedChangelog = src
	}
	return out
}

// outputs is what a consuming workflow reads: the verdict, the version the
// newest entry names, that entry's body, whether a tag naming the version
// exists, and the reference tag as the repository spells it.
//
// Every value is something the run read. None is a spelling this command chose:
// how a repository writes its tags belongs to whatever cuts them, so a consumer
// wanting a ref uses latest-tag and a consumer wanting a version uses version.
//
// A document naming no version still answers, with version and notes empty,
// which is what keeps the verdict independent of whether anything is
// releasable.
func outputs(doc *changelog.Changelog, tags []git.Tag, reference string, findings []changelog.Finding) []output.Output {
	var version, notes, want string
	var prerelease bool
	if latest, ok := doc.Latest(); ok {
		version, notes, want = strings.TrimPrefix(latest.Version, "v"), latest.Body, semver.Canonical(latest.Version)
		prerelease = semver.Prerelease(latest.Version) != ""
	}

	return []output.Output{
		{Name: "valid", Value: strconv.FormatBool(valid(findings))},
		{Name: "version", Value: version},
		{Name: "notes", Value: notes},
		{Name: "already-tagged", Value: strconv.FormatBool(tagged(tags, want))},
		{Name: "latest-tag", Value: reference},
		// A fact about the newest entry, where the prerelease-entry check is a
		// judgement about the whole document. A workflow gating on what it is
		// about to release wants the fact: the check also fires on entries long
		// since released, which never stop being pre-releases.
		{Name: "prerelease", Value: strconv.FormatBool(prerelease)},
	}
}

// tagged reports whether the repository carries a tag naming version, which is
// a canonical semver string, or empty for a document naming no version.
//
// Tags are compared by the version they name, so a repository tagging 1.2.3
// answers for a heading reading 1.2.3 as one tagging v1.2.3 would.
func tagged(tags []git.Tag, version string) bool {
	if version == "" {
		return false
	}
	for _, t := range git.Versions(tags) {
		if t.Version() == version {
			return true
		}
	}
	return false
}

// emit prints the outputs and, where the runner named a file to collect them,
// appends them to it as well.
func emit(outs []output.Output, stdout io.Writer) error {
	if err := output.Write(stdout, outs); err != nil {
		return err
	}
	path := os.Getenv("GITHUB_OUTPUT")
	if path == "" {
		return nil
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if err := output.Write(f, outs); err != nil {
		f.Close()
		return err
	}
	return f.Close()
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
