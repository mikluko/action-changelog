package git_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mikluko/action-changelog/internal/git"
)

const (
	oldChangelog = "# Changelog\n\n## [0.1.0] - 2026-01-01\n\n### Added\n\n- The first thing.\n"
	newChangelog = "# Changelog\n\n## [0.2.0] - 2026-02-01\n\n### Added\n\n- The second thing.\n\n" +
		"## [0.1.0] - 2026-01-01\n\n### Added\n\n- The first thing.\n"
)

// tagged is a repository carrying one lightweight version tag, one annotated
// version tag, and one tag that names no version at all.
func tagged(t *testing.T) string {
	t.Helper()
	dir := repo(t)
	write(t, dir, "CHANGELOG.md", oldChangelog)
	run(t, dir, "add", "CHANGELOG.md")
	run(t, dir, "commit", "-m", "the first release")
	run(t, dir, "tag", "v0.1.0")
	write(t, dir, "CHANGELOG.md", newChangelog)
	run(t, dir, "add", "CHANGELOG.md")
	run(t, dir, "commit", "-m", "the second release")
	run(t, dir, "tag", "-a", "v0.2.0", "-m", "v0.2.0")
	run(t, dir, "tag", "not-a-version")
	return dir
}

func TestTagsCoverAnnotatedAndLightweight(t *testing.T) {
	r, err := git.Open(tagged(t))
	if err != nil {
		t.Fatal(err)
	}
	tags, err := r.Tags()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, tag := range tags {
		got[tag.Name] = true
	}
	for _, want := range []string{"v0.1.0", "v0.2.0", "not-a-version"} {
		if !got[want] {
			t.Errorf("tag %s is missing from %v", want, got)
		}
	}
}

// A repository tagging no pre-release reads the same under both settings, which
// is what makes the default free: nothing is skipped that anybody meant to be
// the baseline.
func TestReferenceIgnoresTagsThatNameNoVersion(t *testing.T) {
	dir := tagged(t)
	for _, admit := range []git.Eligible{git.Final, git.All} {
		t.Run(admit.String(), func(t *testing.T) {
			if got := reference(t, dir, admit); got != "v0.2.0" {
				t.Errorf("the reference is %q, want v0.2.0", got)
			}
		})
	}

	_, all := open(t, dir)
	if v := git.Versions(all); len(v) != 2 || v[1].Name != "v0.1.0" {
		t.Errorf("versions are %v, want v0.2.0 then v0.1.0", v)
	}
}

// The reference is the newest tag this checkout can reach and not the newest
// the repository holds, which is what lets a maintained support line compare
// against its own last release while the trunk moves on above it.
func TestReferenceIsReachableFromHead(t *testing.T) {
	dir := repo(t)
	write(t, dir, "CHANGELOG.md", oldChangelog)
	run(t, dir, "add", "CHANGELOG.md")
	run(t, dir, "commit", "-m", "the first release")
	run(t, dir, "tag", "v1.0.0")

	// A support line branching off the first release, and a trunk that carries
	// on past it. Neither reaches the other's later commits.
	run(t, dir, "checkout", "-b", "support/1.x")
	write(t, dir, "CHANGELOG.md", oldChangelog+"\n### Fixed\n\n- A thing on the support line.\n")
	run(t, dir, "commit", "-am", "a fix on the support line")

	run(t, dir, "checkout", "main")
	write(t, dir, "CHANGELOG.md", newChangelog)
	run(t, dir, "commit", "-am", "the second release")
	run(t, dir, "tag", "v2.0.0")

	if got := reference(t, dir, git.Final); got != "v2.0.0" {
		t.Errorf("the trunk's reference is %q, want v2.0.0", got)
	}

	run(t, dir, "checkout", "support/1.x")
	if got := reference(t, dir, git.Final); got != "v1.0.0" {
		t.Errorf("the support line's reference is %q, want v1.0.0; v2.0.0 is not reachable from it", got)
	}
}

// A release candidate above the newest final is the case the default exists
// for: the candidate is a tag nobody wants as a baseline, and a repository that
// does want it says so.
func TestReferenceSkipsPrereleasesUnlessAsked(t *testing.T) {
	dir := repo(t)
	write(t, dir, "CHANGELOG.md", oldChangelog)
	run(t, dir, "add", "CHANGELOG.md")
	run(t, dir, "commit", "-m", "2.1.0")
	run(t, dir, "tag", "v2.1.0")
	write(t, dir, "CHANGELOG.md", newChangelog)
	run(t, dir, "commit", "-am", "2.2.0-rc.1")
	run(t, dir, "tag", "-a", "v2.2.0-rc.1", "-m", "v2.2.0-rc.1")

	for _, tc := range []struct {
		admit git.Eligible
		want  string
	}{
		{git.Final, "v2.1.0"},
		{git.All, "v2.2.0-rc.1"},
	} {
		t.Run(tc.admit.String(), func(t *testing.T) {
			if got := reference(t, dir, tc.admit); got != tc.want {
				t.Errorf("the reference is %q, want %q", got, tc.want)
			}
		})
	}
}

// A repository following the GitHub Actions convention carries a moving major
// tag beside the release it names, and both read as the same version. The
// reference has to be the one that will still name this commit tomorrow.
func TestReferencePrefersTheTagThatSpellsTheVersionMostFully(t *testing.T) {
	for _, order := range [][]string{
		{"v1.0.0", "v1.0", "v1"},
		{"v1", "v1.0", "v1.0.0"},
	} {
		t.Run(strings.Join(order, ","), func(t *testing.T) {
			dir := repo(t)
			write(t, dir, "CHANGELOG.md", oldChangelog)
			run(t, dir, "add", "CHANGELOG.md")
			run(t, dir, "commit", "-m", "1.0.0")
			// Written in both orders, because the tags name one commit and
			// nothing but this rule decides between them.
			for _, tag := range order {
				run(t, dir, "tag", tag)
			}

			if got := reference(t, dir, git.Final); got != "v1.0.0" {
				t.Errorf("the reference is %q, want %q", got, "v1.0.0")
			}
		})
	}
}

func TestReferenceIsAbsentWhereNothingReachableIsTagged(t *testing.T) {
	dir := repo(t)
	write(t, dir, "CHANGELOG.md", oldChangelog)
	run(t, dir, "add", "CHANGELOG.md")
	run(t, dir, "commit", "-m", "the first commit")

	r, all := open(t, dir)
	if _, ok, err := r.Reference(all, git.Final); err != nil || ok {
		t.Errorf("a repository with no tags reports ok=%v, err=%v; want false and no error", ok, err)
	}
}

func TestParseEligibleRefusesAnythingElse(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want git.Eligible
	}{
		{"final", git.Final},
		{"all", git.All},
		{" all ", git.All},
	} {
		got, err := git.ParseEligible(tc.in)
		if err != nil || got != tc.want {
			t.Errorf("%q parsed to %v, err=%v", tc.in, got, err)
		}
	}
	for _, in := range []string{"", "none", "Final", "release"} {
		if _, err := git.ParseEligible(in); err == nil {
			t.Errorf("-reference-tags accepted %q", in)
		}
	}
}

// reference resolves the reference tag of the repository at dir, failing the
// test where none is found: every caller here has tagged one.
func reference(t *testing.T, dir string, admit git.Eligible) string {
	t.Helper()
	r, all := open(t, dir)
	ref, ok, err := r.Reference(all, admit)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("no reference tag under %s among %v", admit, all)
	}
	return ref.Name
}

func open(t *testing.T, dir string) (*git.Repo, []git.Tag) {
	t.Helper()
	r, err := git.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	out, err := r.Tags()
	if err != nil {
		t.Fatal(err)
	}
	return r, out
}

func TestFileAtReadsTheTreeTheTagPointsAt(t *testing.T) {
	r, err := git.Open(tagged(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ tag, want string }{
		{"v0.1.0", oldChangelog},
		{"v0.2.0", newChangelog},
	} {
		got, err := r.FileAt(tc.tag, "CHANGELOG.md")
		if err != nil {
			t.Fatalf("%s: %v", tc.tag, err)
		}
		if string(got) != tc.want {
			t.Errorf("%s: got %q, want %q", tc.tag, got, tc.want)
		}
	}
}

func TestFileAtReportsWhatIsNotThere(t *testing.T) {
	r, err := git.Open(tagged(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, tag, path string }{
		{"no such file", "v0.1.0", "MISSING.md"},
		{"no such tag", "v9.9.9", "CHANGELOG.md"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := r.FileAt(tc.tag, tc.path); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("error is %v, want one wrapping os.ErrNotExist", err)
			}
		})
	}
}

func TestTagsAreEmptyWhereTheRepositoryHasNone(t *testing.T) {
	dir := repo(t)
	write(t, dir, "CHANGELOG.md", oldChangelog)
	run(t, dir, "add", "CHANGELOG.md")
	run(t, dir, "commit", "-m", "the first commit")

	r, err := git.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	tags, err := r.Tags()
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 0 {
		t.Errorf("tags are %v, want none", tags)
	}
}

func TestOpenSearchesUpwards(t *testing.T) {
	dir := tagged(t)
	sub := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	r, err := git.Open(sub)
	if err != nil {
		t.Fatal(err)
	}
	tags, err := r.Tags()
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) == 0 {
		t.Error("no tags found from a subdirectory of the repository")
	}
}

func TestShallowSeparatesADepthFromAnEmptyHistory(t *testing.T) {
	origin := tagged(t)

	deep, err := git.Open(origin)
	if err != nil {
		t.Fatal(err)
	}
	if shallow, err := deep.Shallow(); err != nil || shallow {
		t.Errorf("the origin reads shallow=%v, err=%v; want false", shallow, err)
	}

	// A depth-1 clone is what a checkout that fetched no tags leaves behind:
	// the tags exist at the origin and this copy carries none of them.
	dir := t.TempDir()
	clone := filepath.Join(dir, "clone")
	run(t, dir, "clone", "--depth", "1", "--no-tags", "file://"+filepath.ToSlash(origin), clone)

	r, err := git.Open(clone)
	if err != nil {
		t.Fatal(err)
	}
	shallow, err := r.Shallow()
	if err != nil {
		t.Fatal(err)
	}
	if !shallow {
		t.Error("a depth-1 clone does not read as shallow")
	}
	tags, err := r.Tags()
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 0 {
		t.Errorf("the clone carries tags %v, want none", tags)
	}
}

func TestOpenReportsWhereThereIsNoRepository(t *testing.T) {
	if _, err := git.Open(t.TempDir()); err == nil {
		t.Error("opening a directory under no repository succeeded")
	}
}

// A linked worktree's .git is a file naming a directory under the parent
// repository. Where that directory is out of reach, opening the worktree used
// to succeed and read as a repository holding no tags at all, which is the
// evidence every tag-dependent check then passed on.
func TestOpenReportsARepositoryItCannotRead(t *testing.T) {
	origin := tagged(t)
	tree := filepath.Join(t.TempDir(), "linked")
	run(t, origin, "worktree", "add", "--detach", tree)

	if _, err := git.Open(tree); err != nil {
		t.Fatalf("opening an intact linked worktree failed: %v", err)
	}

	if err := os.RemoveAll(filepath.Join(origin, ".git", "worktrees")); err != nil {
		t.Fatal(err)
	}
	if _, err := git.Open(tree); err == nil {
		t.Error("opening a worktree whose gitdir does not resolve succeeded")
	}
}

// A submodule's .git is the same shape and breaks the same way, so the answer
// does not turn on the file having been written by git worktree add.
func TestOpenReportsAGitFileNamingNothing(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, ".git", "gitdir: ./nowhere\n")
	if _, err := git.Open(dir); err == nil {
		t.Error("opening a directory whose .git names a missing directory succeeded")
	}
}

// repo initialises an empty repository whose commits do not depend on the
// machine's git configuration.
func repo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
	dir := t.TempDir()
	run(t, dir, "init", "-b", "main")
	run(t, dir, "config", "user.name", "Fixture")
	run(t, dir, "config", "user.email", "fixture@example.invalid")
	return dir
}

func run(t *testing.T, dir string, args ...string) {
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

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The GitHub Actions convention keeps a moving major tag beside the full one it
// points at. Both name the same version, and the fuller spelling wins the tie,
// so the reference tag is the immutable v1.0.0 rather than the v1 that moves to
// the next release under whoever checked it out.
//
// It is pinned because it rests on a tag namespace being laxer than Semantic
// Versioning, which the specification's own grammar has no shorthand for: read
// strictly, "v1" names no version at all and drops out of this list.
func TestVersionsReadsTheMovingMajorTag(t *testing.T) {
	got := git.Versions([]git.Tag{
		{Name: "v1.0.0"},
		{Name: "v1"},
		{Name: "v0.9.0"},
		{Name: "not-a-version"},
		{Name: "v1.1"},
	})
	want := []string{"v1.1", "v1.0.0", "v1", "v0.9.0"}
	if len(got) != len(want) {
		t.Fatalf("versions are %v, want %v", names(got), want)
	}
	for i := range want {
		if got[i].Name != want[i] {
			t.Errorf("versions are %v, want %v", names(got), want)
			break
		}
	}
	for _, tc := range []struct{ name, version string }{
		{"v1", "v1.0.0"},
		{"v1.1", "v1.1.0"},
		{"v1.0.0", "v1.0.0"},
	} {
		if got := (git.Tag{Name: tc.name}).Version(); got != tc.version {
			t.Errorf("Tag(%q).Version() = %q, want %q", tc.name, got, tc.version)
		}
	}
	if v := (git.Tag{Name: "not-a-version"}).Version(); v != "" {
		t.Errorf("a tag naming no version reads as %q, want empty", v)
	}
}

func names(tags []git.Tag) []string {
	out := make([]string, len(tags))
	for i, t := range tags {
		out[i] = t.Name
	}
	return out
}

// runAt runs git with the author and committer clocks pinned to when, which is
// what lets a fixture cut a tag at a known instant in a known zone.
func runAt(t *testing.T, dir, when string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_SYSTEM="+os.DevNull,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_DATE="+when,
		"GIT_COMMITTER_DATE="+when,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func day(t *testing.T, r *git.Repo, tags []git.Tag, name string) string {
	t.Helper()
	for _, tag := range tags {
		if tag.Name == name {
			d, err := r.TagDay(tag)
			if err != nil {
				t.Fatalf("TagDay(%s): %v", name, err)
			}
			return d
		}
	}
	t.Fatalf("no tag %s in %v", name, tags)
	return ""
}

// The two tag kinds carry their date in different places and a repository uses
// one or the other, so reading only one is wrong on half the repositories in
// reach. An annotated tag has a tagger date of its own; a lightweight tag is a
// bare reference whose only date is on the commit it names.
func TestTagDayReadsBothTagKinds(t *testing.T) {
	dir := repo(t)
	write(t, dir, "f", "one")
	run(t, dir, "add", "f")
	runAt(t, dir, "2026-01-01T12:00:00+00:00", "commit", "-m", "one")
	run(t, dir, "tag", "v0.1.0")

	write(t, dir, "f", "two")
	run(t, dir, "add", "f")
	runAt(t, dir, "2026-02-01T12:00:00+00:00", "commit", "-m", "two")
	// Cut a month after the commit it points at, which is what tells the two
	// dates apart: reading the commit here would answer 2026-02-01.
	runAt(t, dir, "2026-03-01T12:00:00+00:00", "tag", "-a", "v0.2.0", "-m", "v0.2.0")

	r, tags := open(t, dir)
	if got := day(t, r, tags, "v0.1.0"); got != "2026-01-01" {
		t.Errorf("lightweight v0.1.0 was cut on %q, want 2026-01-01", got)
	}
	if got := day(t, r, tags, "v0.2.0"); got != "2026-03-01" {
		t.Errorf("annotated v0.2.0 was cut on %q, want 2026-03-01", got)
	}
}

// The day is the tag's own, not UTC. A changelog date is a bare calendar day
// meaning the day the human cut the release, and normalising these two to UTC
// moves each onto the wrong side of a midnight.
func TestTagDayIsInTheTagsOwnZone(t *testing.T) {
	dir := repo(t)
	write(t, dir, "f", "late")
	run(t, dir, "add", "f")
	// 2026-03-01 23:30 -07:00 is 2026-03-02 06:30 UTC.
	runAt(t, dir, "2026-03-01T23:30:00-07:00", "commit", "-m", "late")
	run(t, dir, "tag", "v1.0.0")

	write(t, dir, "f", "early")
	run(t, dir, "add", "f")
	// 2026-03-02 00:30 +09:00 is 2026-03-01 15:30 UTC.
	runAt(t, dir, "2026-03-02T00:30:00+09:00", "commit", "-m", "early")
	runAt(t, dir, "2026-03-02T00:30:00+09:00", "tag", "-a", "v1.1.0", "-m", "v1.1.0")

	r, tags := open(t, dir)
	if got := day(t, r, tags, "v1.0.0"); got != "2026-03-01" {
		t.Errorf("v1.0.0 was cut on %q, want 2026-03-01; UTC would say 2026-03-02", got)
	}
	if got := day(t, r, tags, "v1.1.0"); got != "2026-03-02" {
		t.Errorf("v1.1.0 was cut on %q, want 2026-03-02; UTC would say 2026-03-01", got)
	}
}
