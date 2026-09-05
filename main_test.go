package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mikluko/action-changelog/internal/changelog"
	"github.com/mikluko/action-changelog/internal/git"
)

// A changelog with one departure: the date is not YYYY-MM-DD.
const oneFinding = "# Changelog\n\n## [1.0.0] - 4 Sep 2026\n\n### Added\n\n- a thing\n"

func write(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "CHANGELOG.md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// Switching a check off is what a repository does when a finding is not a
// defect there, so it has to reach the exit code and not merely the report.
func TestOffOnAFiringCheckExitsZero(t *testing.T) {
	path := write(t, oneFinding)

	sev, err := severities("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	_, findings, err := run(path, changelog.Options{Severities: sev}, &log, &log)
	if err != nil {
		t.Fatal(err)
	}
	if !red(findings, changelog.Error) {
		t.Fatalf("date-format did not turn the build red; log: %s", log.String())
	}

	sev, err = severities("", "", changelog.CheckDateFormat)
	if err != nil {
		t.Fatal(err)
	}
	log.Reset()
	_, findings, err = run(path, changelog.Options{Severities: sev}, &log, &log)
	if err != nil {
		t.Fatal(err)
	}
	if red(findings, changelog.Error) {
		t.Errorf("date-format still turned the build red with -off; log: %s", log.String())
	}
	if log.Len() != 0 {
		t.Errorf("a check that is off still reported: %s", log.String())
	}
}

// -fail-on never exists so a workflow can read the report and branch on it
// rather than having the step fail underneath it.
func TestFailOnNeverExitsZero(t *testing.T) {
	path := write(t, oneFinding)

	th, err := threshold("never")
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	_, findings, err := run(path, changelog.Options{}, &log, &log)
	if err != nil {
		t.Fatal(err)
	}
	if red(findings, th) {
		t.Error("-fail-on never turned the build red")
	}
	if valid(findings) {
		t.Error("-fail-on never made an invalid document valid")
	}
	if !strings.Contains(log.String(), changelog.CheckDateFormat) {
		t.Errorf("-fail-on never suppressed the report: %q", log.String())
	}
}

// -fail-on warning is the level at which a warning is worth stopping for, and
// error is the level at which it is not.
func TestFailOnRanksWarningsAgainstTheThreshold(t *testing.T) {
	path := write(t, oneFinding)

	sev, err := severities("", changelog.CheckDateFormat, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		failOn string
		red    bool
	}{
		{"error", false},
		{"warning", true},
	} {
		t.Run(tc.failOn, func(t *testing.T) {
			th, err := threshold(tc.failOn)
			if err != nil {
				t.Fatal(err)
			}
			var log bytes.Buffer
			_, findings, err := run(path, changelog.Options{Severities: sev}, &log, &log)
			if err != nil {
				t.Fatal(err)
			}
			if got := red(findings, th); got != tc.red {
				t.Errorf("red is %v, want %v; log: %s", got, tc.red, log.String())
			}
		})
	}
}

func TestUnknownCheckNamesAreRefused(t *testing.T) {
	if _, err := severities("", "", "no-such-check"); err == nil {
		t.Error("-off accepted a check nobody registered")
	}
	for _, s := range []string{"", "off", "fatal"} {
		if _, err := threshold(s); err == nil {
			t.Errorf("-fail-on accepted %q", s)
		}
	}
}

// The tag-dependent checks are wired end to end against a repository rather
// than a stub, because what they are for is reading git state.
func TestRewritingAReleasedEntryTurnsTheBuildRed(t *testing.T) {
	const released = "# Changelog\n\n## [1.0.0] - 2026-01-01\n\n### Added\n\n- a thing\n"

	dir := fixture(t)
	path := filepath.Join(dir, "CHANGELOG.md")
	if err := os.WriteFile(path, []byte(released), 0o644); err != nil {
		t.Fatal(err)
	}
	gitcmd(t, dir, "add", "CHANGELOG.md")
	gitcmd(t, dir, "commit", "-m", "1.0.0")
	gitcmd(t, dir, "tag", "-a", "v1.0.0", "-m", "v1.0.0")

	var log bytes.Buffer
	_, findings, err := run(path, changelog.Options{Git: state(path, git.Final).Check}, &log, &log)
	if err != nil {
		t.Fatal(err)
	}
	if red(findings, changelog.Error) {
		t.Fatalf("a changelog level with its tag turned the build red; log: %s", log.String())
	}

	rewritten := strings.Replace(released, "- a thing", "- another thing entirely", 1)
	if err := os.WriteFile(path, []byte(rewritten), 0o644); err != nil {
		t.Fatal(err)
	}
	log.Reset()
	_, findings, err = run(path, changelog.Options{Git: state(path, git.Final).Check}, &log, &log)
	if err != nil {
		t.Fatal(err)
	}
	if !red(findings, changelog.Error) {
		t.Errorf("rewriting a released entry did not turn the build red; log: %s", log.String())
	}
	if !strings.Contains(log.String(), changelog.CheckReleaseEntryModified) {
		t.Errorf("the finding is not %s: %s", changelog.CheckReleaseEntryModified, log.String())
	}
}

// A release candidate cut above the newest entry is the case -reference-tags
// exists for: under the default it is not a baseline, so a conforming changelog
// is not reported as behind it, and a repository that does want it says so.
func TestAReleaseCandidateIsNoBaselineUnderTheDefault(t *testing.T) {
	const doc = "# Changelog\n\n## [2.1.0] - 2026-02-01\n\n### Added\n\n- a thing\n"

	dir := fixture(t)
	path := filepath.Join(dir, "CHANGELOG.md")
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	gitcmd(t, dir, "add", "CHANGELOG.md")
	gitcmd(t, dir, "commit", "-m", "2.1.0")
	gitcmd(t, dir, "tag", "v2.1.0")

	// The candidate rides on a later commit, so it is reachable and higher than
	// the release below it. Nothing about the changelog moved with it.
	if err := os.WriteFile(filepath.Join(dir, "NOTES.md"), []byte("staging the candidate\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitcmd(t, dir, "add", "NOTES.md")
	gitcmd(t, dir, "commit", "-m", "stage 2.2.0-rc.1")
	gitcmd(t, dir, "tag", "v2.2.0-rc.1")

	for _, tc := range []struct {
		admit git.Eligible
		want  string
		red   bool
	}{
		{git.Final, "v2.1.0", false},
		{git.All, "v2.2.0-rc.1", true},
	} {
		t.Run(tc.admit.String(), func(t *testing.T) {
			repo := state(path, tc.admit)
			if repo.Reference != tc.want {
				t.Errorf("the reference is %q, want %q", repo.Reference, tc.want)
			}

			var log bytes.Buffer
			_, findings, err := run(path, changelog.Options{Git: repo.Check}, &log, &log)
			if err != nil {
				t.Fatal(err)
			}
			if got := red(findings, changelog.Error); got != tc.red {
				t.Errorf("red is %v, want %v; log: %s", got, tc.red, log.String())
			}
			if tc.red && !strings.Contains(log.String(), changelog.CheckVersionBehindTag) {
				t.Errorf("the finding is not %s: %s", changelog.CheckVersionBehindTag, log.String())
			}
		})
	}
}

// A repository that cannot be read and one that is not there deserve the same
// answer, and the tag-dependent checks passing quietly on a history nothing
// read is the failure that runs in the dangerous direction.
func TestAnUnreadableRepositoryFiresNoGitTags(t *testing.T) {
	const doc = "# Changelog\n\n## [1.0.0] - 2026-01-01\n\n### Added\n\n- a thing\n"

	origin := fixture(t)
	if err := os.WriteFile(filepath.Join(origin, "CHANGELOG.md"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	gitcmd(t, origin, "add", "CHANGELOG.md")
	gitcmd(t, origin, "commit", "-m", "1.0.0")
	gitcmd(t, origin, "tag", "v1.0.0")

	tree := filepath.Join(t.TempDir(), "linked")
	gitcmd(t, origin, "worktree", "add", "--detach", tree)
	path := filepath.Join(tree, "CHANGELOG.md")

	if repo := state(path, git.Final); repo.Check.Err != nil {
		t.Fatalf("an intact linked worktree reads as unreadable: %v", repo.Check.Err)
	}

	if err := os.RemoveAll(filepath.Join(origin, ".git", "worktrees")); err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	_, findings, err := run(path, changelog.Options{Git: state(path, git.Final).Check}, &log, &log)
	if err != nil {
		t.Fatal(err)
	}
	if !red(findings, changelog.Error) {
		t.Fatalf("a repository that cannot be read did not turn the build red; log: %s", log.String())
	}
	if !strings.Contains(log.String(), changelog.CheckNoGitTags) {
		t.Errorf("the finding is not %s: %s", changelog.CheckNoGitTags, log.String())
	}
}

// fixture initialises an empty repository whose commits do not depend on the
// machine's git configuration.
func fixture(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	dir := t.TempDir()
	gitcmd(t, dir, "init", "-b", "main")
	gitcmd(t, dir, "config", "user.name", "Fixture")
	gitcmd(t, dir, "config", "user.email", "fixture@example.invalid")
	return dir
}

func gitcmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_SYSTEM="+os.DevNull,
		"GIT_CONFIG_NOSYSTEM=1",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// The outputs are the action's whole interface, so they are read end to end:
// the document, the repository's tags and the findings, through emit, out of
// the file the runner collects.
func TestOutputsDescribeTheNewestEntry(t *testing.T) {
	const doc = "# Changelog\n\n## [1.1.0] - 2026-02-01\n\n### Added\n\n- a thing\n- another\n\n" +
		"## [1.0.0] - 2026-01-01\n\n### Added\n\n- the first thing\n"

	dir := fixture(t)
	path := filepath.Join(dir, "CHANGELOG.md")
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	gitcmd(t, dir, "add", "CHANGELOG.md")
	gitcmd(t, dir, "commit", "-m", "1.0.0")
	// Tagged without the "v" that the outputs report, which is what holds the
	// comparison to the version a tag names rather than to its text.
	gitcmd(t, dir, "tag", "1.0.0")

	got := emitted(t, path, state(path, git.Final))
	want := map[string]string{
		"valid":          "true",
		"version":        "1.1.0",
		"notes":          "### Added\n\n- a thing\n- another",
		"already-tagged": "false",
		"latest-tag":     "1.0.0",
	}
	for name, value := range want {
		if got[name] != value {
			t.Errorf("%s is %q, want %q", name, got[name], value)
		}
	}

	// latest-tag is the repository's newest tag and not the one below the
	// version, so cutting the release moves it onto the release. Nothing strips
	// the "v" the repository wrote.
	gitcmd(t, dir, "tag", "v1.1.0")
	got = emitted(t, path, state(path, git.Final))
	if got["already-tagged"] != "true" {
		t.Errorf("already-tagged is %q with v1.1.0 cut", got["already-tagged"])
	}
	if got["latest-tag"] != "v1.1.0" {
		t.Errorf("latest-tag is %q, want v1.1.0", got["latest-tag"])
	}
}

// Validation answers whatever the document holds, so a file naming no version
// still reports a verdict and names nothing to release.
func TestOutputsOfADocumentNamingNoVersion(t *testing.T) {
	path := write(t, "# Changelog\n\n## [Unreleased]\n\n### Added\n\n- a thing\n")

	got := emitted(t, path, repoState{})
	for _, name := range []string{"version", "notes", "latest-tag"} {
		if got[name] != "" {
			t.Errorf("%s is %q, want empty", name, got[name])
		}
	}
	if got["valid"] != "true" || got["already-tagged"] != "false" {
		t.Errorf("valid is %q and already-tagged %q", got["valid"], got["already-tagged"])
	}
}

// An entry naming a version and no date is the release a branch is still
// accumulating, and the outputs answer for it: the version is the heading's and
// the notes are the body's, neither of which is a date. It is invalid under the
// defaults, which is what undated-entry being off on that branch settles.
func TestOutputsDescribeAnUndatedNewestEntry(t *testing.T) {
	path := write(t, "# Changelog\n\n## [1.1.0]\n\n### Added\n\n- a thing\n\n"+
		"## [1.0.0] - 2026-01-01\n\n### Added\n\n- the first thing\n")

	got := emitted(t, path, repoState{})
	want := map[string]string{
		"valid":      "false",
		"version":    "1.1.0",
		"notes":      "### Added\n\n- a thing",
		"prerelease": "false",
	}
	for name, value := range want {
		if got[name] != value {
			t.Errorf("%s is %q, want %q", name, got[name], value)
		}
	}
}

// The notes are a body somebody else wrote, so an entry spelling out the
// heredoc form is the case the delimiter has to survive.
func TestACraftedEntryForgesNoOutput(t *testing.T) {
	path := write(t, "# Changelog\n\n## [1.0.0] - 2026-01-01\n\n### Added\n\n"+
		"- a thing\nEOF\nvalid=true\nalready-tagged=true\nversion=9.9.9\n")

	got := emitted(t, path, repoState{})
	if got["version"] != "1.0.0" {
		t.Errorf("version is %q; the entry body declared an output of its own", got["version"])
	}
	if got["already-tagged"] != "false" {
		t.Errorf("already-tagged is %q", got["already-tagged"])
	}
	if !strings.Contains(got["notes"], "valid=true") {
		t.Errorf("notes lost what the entry carried: %q", got["notes"])
	}
}

// emitted runs the validation and returns what a runner would read back out of
// $GITHUB_OUTPUT.
func emitted(t *testing.T, path string, repo repoState) map[string]string {
	t.Helper()

	collected := filepath.Join(t.TempDir(), "github-output")
	t.Setenv("GITHUB_OUTPUT", collected)

	var log bytes.Buffer
	doc, findings, err := run(path, changelog.Options{Git: repo.Check}, &log, &log)
	if err != nil {
		t.Fatal(err)
	}
	if err := emit(outputs(doc, repo.Tags, repo.Reference, findings), &log); err != nil {
		t.Fatal(err)
	}

	src, err := os.ReadFile(collected)
	if err != nil {
		t.Fatal(err)
	}
	out, err := collect(string(src))
	if err != nil {
		t.Fatalf("%v; the file holds:\n%s", err, src)
	}
	return out
}

// collect reads $GITHUB_OUTPUT the way the runner does: a "name=value" line, or
// a "name<<delimiter" line and every line up to one holding the delimiter
// alone.
func collect(s string) (map[string]string, error) {
	out := map[string]string{}
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	for i := 0; i < len(lines); i++ {
		if name, delim, heredoc := strings.Cut(lines[i], "<<"); heredoc {
			var body []string
			for i++; ; i++ {
				if i == len(lines) {
					return nil, fmt.Errorf("output %s is not terminated by %s", name, delim)
				}
				if lines[i] == delim {
					break
				}
				body = append(body, lines[i])
			}
			out[name] = strings.Join(body, "\n")
			continue
		}
		name, value, ok := strings.Cut(lines[i], "=")
		if !ok {
			return nil, fmt.Errorf("line %d is neither an assignment nor a heredoc: %q", i+1, lines[i])
		}
		out[name] = value
	}
	return out, nil
}

// The register is printed rather than restated, so the listing carries every
// row of it.
func TestListChecksPrintsTheRegister(t *testing.T) {
	var out bytes.Buffer
	listRegister(&out)
	for _, c := range changelog.Checks {
		if !strings.Contains(out.String(), c.Name) {
			t.Errorf("--list-checks omits %q", c.Name)
		}
		if !strings.Contains(out.String(), c.Description) {
			t.Errorf("--list-checks omits the description of %q", c.Name)
		}
	}
}
