package git_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
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
