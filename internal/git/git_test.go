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

func TestNewestIgnoresTagsThatNameNoVersion(t *testing.T) {
	r, err := git.Open(tagged(t))
	if err != nil {
		t.Fatal(err)
	}
	tags, err := r.Tags()
	if err != nil {
		t.Fatal(err)
	}
	newest, ok := git.Newest(tags)
	if !ok {
		t.Fatal("no newest tag")
	}
	if newest.Name != "v0.2.0" {
		t.Errorf("newest is %q, want v0.2.0", newest.Name)
	}
	if v := git.Versions(tags); len(v) != 2 || v[1].Name != "v0.1.0" {
		t.Errorf("versions are %v, want v0.2.0 then v0.1.0", v)
	}
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
