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
