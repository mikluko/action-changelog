package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"sarak.sh/mikluko/changelog/pkg/changelog"
)

// fragmentDir writes files, keyed by name, into a directory beside a fresh
// changelog, and returns the changelog's path and the directory's.
func fragmentDir(t *testing.T, body string, files map[string]string) (path, dir string) {
	t.Helper()
	path = write(t, body)
	dir = filepath.Join(filepath.Dir(path), ".changelog")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return path, dir
}

func TestReleaseCmdFoldsAndDeletesTheFragments(t *testing.T) {
	path, dir := fragmentDir(t, unreleased, map[string]string{
		"b-parser.md": "### Fixed\n\n- from b\n",
		"a-input.md":  "### Added\n\n- from a\n",
		".gitkeep":    "",
	})
	var out bytes.Buffer
	if err := releaseCmd([]string{"-changelog", path, "-fragments", dir}, &out, os.Stderr, fixed(t)); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	want := "## [Unreleased]\n\n## [1.3.0] - 2020-02-02\n\n### Added\n\n- from a\n\n### Fixed\n\n- f\n- from b\n"
	if !strings.Contains(string(got), want) {
		t.Errorf("not folded and released as 1.3.0:\n%s", got)
	}
	left, _ := os.ReadDir(dir)
	if len(left) != 1 || left[0].Name() != ".gitkeep" {
		t.Errorf("left in the directory: %v", left)
	}
	if !strings.Contains(out.String(), "- from b\n") {
		t.Errorf("notes output carries no folded note:\n%s", out.String())
	}
}

func TestReleaseCmdReleasesFragmentsAlone(t *testing.T) {
	path, dir := fragmentDir(t, "# Changelog\n\n## [Unreleased]\n\n## [1.2.3] - 2020-01-01\n\n- x\n",
		map[string]string{"fix.md": "### Fixed\n\n- f\n"})
	if err := releaseCmd([]string{"-changelog", path, "-fragments", dir}, &bytes.Buffer{}, os.Stderr, fixed(t)); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); !strings.Contains(string(got), "## [1.2.4] - 2020-02-02\n\n### Fixed\n\n- f\n") {
		t.Errorf("not released:\n%s", got)
	}
}

// One file that cannot fold refuses the release before anything is written,
// so the fragments that could fold are not deleted with nothing to show.
func TestReleaseCmdRefusesABadFragmentAndChangesNothing(t *testing.T) {
	for name, files := range map[string]map[string]string{
		"malformed": {"good.md": "### Added\n\n- a\n", "bad.md": "- under no section\n"},
		"stray":     {"good.md": "### Added\n\n- a\n", "note.txt": "### Added\n\n- a\n"},
	} {
		t.Run(name, func(t *testing.T) {
			path, dir := fragmentDir(t, unreleased, files)
			if err := releaseCmd([]string{"-changelog", path, "-fragments", dir}, os.Stdout, os.Stderr, fixed(t)); err == nil {
				t.Error("no error")
			}
			if got, _ := os.ReadFile(path); string(got) != unreleased {
				t.Errorf("the changelog changed:\n%s", got)
			}
			if left, _ := os.ReadDir(dir); len(left) != len(files) {
				t.Errorf("fragments deleted: %v", left)
			}
		})
	}
}

func TestReleaseCmdWithoutAFragmentsDirectory(t *testing.T) {
	path := write(t, unreleased)
	missing := filepath.Join(filepath.Dir(path), ".changelog")
	if err := releaseCmd([]string{"-changelog", path, "-fragments", missing}, &bytes.Buffer{}, os.Stderr, fixed(t)); err != nil {
		t.Fatal(err)
	}
}

func TestLintFragmentsReportsEachAgainstItsFile(t *testing.T) {
	_, dir := fragmentDir(t, unreleased, map[string]string{
		"good.md":  "### Fixed\n\n- f\n",
		"form.md":  "### Fixed\n\n- f\n\nprose\n",
		"vocab.md": "### Fixes\n\n- f\n",
		"note.txt": "### Fixed\n\n- f\n",
	})
	var log bytes.Buffer
	findings, err := lintFragments(dir, changelog.Options{}, &log, &log)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range findings {
		got = append(got, f.Check)
	}
	want := []string{changelog.CheckFragmentForm, changelog.CheckFragmentForm, changelog.CheckUnknownSection}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v; log:\n%s", got, want, log.String())
	}
	for _, line := range []string{
		filepath.Join(dir, "note.txt") + ":1: error:",
		filepath.Join(dir, "form.md") + ":5: error:",
		filepath.Join(dir, "vocab.md") + ":1: error:",
	} {
		if !strings.Contains(log.String(), line) {
			t.Errorf("log carries no %q:\n%s", line, log.String())
		}
	}
	if !red(findings, changelog.Error) {
		t.Error("a malformed fragment does not turn the build red")
	}
}

func TestLintFragmentsHonoursOff(t *testing.T) {
	_, dir := fragmentDir(t, unreleased, map[string]string{"note.txt": "x\n", "form.md": "x\n"})
	sev, err := severities("", "", changelog.CheckFragmentForm)
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	findings, err := lintFragments(dir, changelog.Options{Severities: sev}, &log, &log)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 || log.Len() != 0 {
		t.Errorf("findings %v, log %q", findings, log.String())
	}
}

func TestForecast(t *testing.T) {
	const (
		clean = "# Changelog\n\n## [Unreleased]\n\n## [1.2.3] - 2020-01-01\n\n- x\n"
		open  = "# Changelog\n\n## [Unreleased]\n\n## [1.3.0]\n\n- y\n\n## [1.2.3] - 2020-01-01\n\n- x\n"
	)
	for _, tc := range []struct {
		name, changelog string
		files           map[string]string
		want, line      int
	}{
		{"releasable", clean, map[string]string{"a.md": "### Added\n\n- a\n"}, 0, 0},
		{"nothing to release", clean, nil, 0, 0},
		{"a section the release would carry unknown", clean, map[string]string{"a.md": "### Fixes\n\n- a\n"}, 1, 3},
		{"fragments folded into the open entry", open, map[string]string{"a.md": "### Added\n\n- a\n"}, 0, 0},
		{"an open entry is pending without fragments", open, nil, 0, 0},
		{"an open entry that would close dated before the entry below", "# Changelog\n\n## [Unreleased]\n\n## [1.3.0]\n\n- y\n\n## [1.2.3] - 2020-03-01\n\n- x\n", nil, 1, 5},
		{"a defect the changelog already raises is not raised again", "# Changelog\n\n## [Unreleased]\n\n### Fixes\n\n- a\n\n## [1.2.3] - 2020-01-01\n\n- x\n", nil, 0, 0},
		{"malformed fragments are fragment-form's", clean, map[string]string{"a.md": "- a\n"}, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path, dir := fragmentDir(t, tc.changelog, tc.files)
			src, _ := os.ReadFile(path)
			current := changelog.Parse(src).Lint(changelog.Options{})
			var log bytes.Buffer
			got, err := forecast(path, dir, changelog.Options{}, current, "2020-02-02", &log, &log)
			if err != nil {
				t.Fatal(err)
			}
			if none, err := forecast(path, "", changelog.Options{}, current, "2020-02-02", &log, &log); err != nil || len(none) != 0 {
				t.Errorf("no fragments directory forecasts %v, %v", none, err)
			}
			if len(got) != tc.want {
				t.Errorf("%d findings, want %d; log:\n%s", len(got), tc.want, log.String())
			}
			for _, f := range got {
				if f.Check != changelog.CheckFragmentsRelease || f.Line != tc.line {
					t.Errorf("finding %+v, want fragments-release at line %d", f, tc.line)
				}
			}
		})
	}
}

// branchWithFragments is a repository whose main carries old.md in
// .changelog/, checked out on a branch off it that adds the files given.
func branchWithFragments(t *testing.T, added map[string]string) (path, dir string) {
	t.Helper()
	root := fixture(t)
	path = filepath.Join(root, "CHANGELOG.md")
	dir = filepath.Join(root, ".changelog")
	if err := os.WriteFile(path, []byte(unreleased), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "old.md"), []byte("### Added\n\n- old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitcmd(t, root, "add", ".")
	gitcmd(t, root, "commit", "-m", "main")
	gitcmd(t, root, "switch", "-c", "feature")
	for name, src := range added {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return path, dir
}

func TestMissingFragment(t *testing.T) {
	for _, tc := range []struct {
		name  string
		added map[string]string
		base  string
		sev   changelog.Severities
		noDir bool
		want  int
	}{
		{"a fragment added", map[string]string{"new.md": "### Fixed\n\n- f\n"}, "main", nil, false, 0},
		{"none added", nil, "main", nil, false, 1},
		{"only a stray added", map[string]string{"new.txt": "x\n"}, "main", nil, false, 1},
		{"outside a pull request", nil, "", nil, false, 0},
		{"switched off", nil, "main", changelog.Severities{changelog.CheckMissingFragment: changelog.Off}, false, 0},
		{"a base that names nothing", map[string]string{"new.md": "### Fixed\n\n- f\n"}, "nowhere", nil, false, 1},
		{"no fragments directory", nil, "main", nil, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path, dir := branchWithFragments(t, tc.added)
			if tc.noDir {
				dir = ""
			}
			var log bytes.Buffer
			got, err := missingFragment(path, dir, tc.base, changelog.Options{Severities: tc.sev}, &log, &log)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != tc.want {
				t.Errorf("%d findings, want %d; log:\n%s", len(got), tc.want, log.String())
			}
		})
	}
}

// A pull request carries notes in a fragment or in the changelog itself, which
// is what a release branch assembling its entry by hand or by fold does.
func TestMissingFragmentCountsNotesInTheChangelog(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(t *testing.T, path, dir string)
		want   int
	}{
		{"a note written into the changelog", func(t *testing.T, path, _ string) {
			edit(t, path, "- f\n", "- f\n- a note\n")
		}, 0},
		{"the fragment folded into the changelog", func(t *testing.T, path, dir string) {
			if err := foldCmd([]string{"-changelog", path, "-fragments", dir}, os.Stderr); err != nil {
				t.Fatal(err)
			}
		}, 0},
		{"a fragment deleted and nothing written", func(t *testing.T, _, dir string) {
			if err := os.Remove(filepath.Join(dir, "old.md")); err != nil {
				t.Fatal(err)
			}
		}, 1},
		{"a note reworded reads as a new one", func(t *testing.T, path, _ string) {
			edit(t, path, "- x\n", "- y\n")
		}, 0},
		{"a note removed", func(t *testing.T, path, _ string) {
			edit(t, path, "\n- f\n", "\n")
		}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path, dir := branchWithFragments(t, nil)
			tc.change(t, path, dir)
			var log bytes.Buffer
			got, err := missingFragment(path, dir, "main", changelog.Options{}, &log, &log)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != tc.want {
				t.Errorf("%d findings, want %d; log:\n%s", len(got), tc.want, log.String())
			}
		})
	}
}

func edit(t *testing.T, path, old, new string) {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), old) {
		t.Fatalf("%s carries no %q", path, old)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(src), old, new, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFoldCmd(t *testing.T) {
	for _, tc := range []struct{ name, changelog, want string }{
		{"into the open entry", "# Changelog\n\n## [Unreleased]\n\n## [1.3.0]\n\n### Fixed\n\n- f\n\n## [1.2.3] - 2020-01-01\n\n- x\n",
			"## [Unreleased]\n\n## [1.3.0]\n\n### Added\n\n- a\n\n### Fixed\n\n- f\n"},
		{"into [Unreleased] where no entry is open", unreleased,
			"## [Unreleased]\n\n### Added\n\n- a\n\n### Fixed\n\n- f\n\n## [1.2.3]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path, dir := fragmentDir(t, tc.changelog, map[string]string{"a.md": "### Added\n\n- a\n"})
			if err := foldCmd([]string{"-changelog", path, "-fragments", dir}, os.Stderr); err != nil {
				t.Fatal(err)
			}
			if got, _ := os.ReadFile(path); !strings.Contains(string(got), tc.want) {
				t.Errorf("not folded:\n%s", got)
			}
			if left, _ := os.ReadDir(dir); len(left) != 0 {
				t.Errorf("left in the directory: %v", left)
			}
		})
	}
}

func TestFoldCmdWithNothingToFold(t *testing.T) {
	path := write(t, unreleased)
	if err := foldCmd([]string{"-changelog", path, "-fragments", filepath.Join(filepath.Dir(path), ".changelog")}, os.Stderr); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != unreleased {
		t.Errorf("the file changed:\n%s", got)
	}
}
