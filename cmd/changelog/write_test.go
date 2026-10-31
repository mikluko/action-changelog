package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

const unreleased = "# Changelog\n\n## [Unreleased]\n\n### Fixed\n\n- f\n\n## [1.2.3] - 2020-01-01\n\n- x\n"

func fixed(t *testing.T) func() time.Time {
	t.Helper()
	day, err := time.Parse("2006-01-02", "2020-02-02")
	if err != nil {
		t.Fatal(err)
	}
	return func() time.Time { return day }
}

func TestAppendCmdWritesTheFile(t *testing.T) {
	path := write(t, unreleased)
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := appendCmd([]string{"-changelog", path, "-section", "Added", "-note", "a"}, os.Stderr); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "## [Unreleased]\n\n### Added\n\n- a\n\n### Fixed\n") {
		t.Errorf("not appended:\n%s", got)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
		t.Errorf("mode is %v, want the file's own 0600", info.Mode().Perm())
	}
}

func TestReleaseCmdComputesTheVersion(t *testing.T) {
	path := write(t, unreleased)
	var out bytes.Buffer
	if err := releaseCmd([]string{"-changelog", path}, &out, os.Stderr, fixed(t)); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "## [Unreleased]\n\n## [1.2.4] - 2020-02-02\n\n### Fixed\n\n- f\n") {
		t.Errorf("not released:\n%s", got)
	}
	for _, want := range []string{"version=1.2.4\n", "prerelease=false\n", "### Fixed\n\n- f\n"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("outputs carry no %q:\n%s", want, out.String())
		}
	}
}

func TestReleaseCmdGivenVersionAndOpen(t *testing.T) {
	path := write(t, unreleased)
	var out bytes.Buffer
	if err := releaseCmd([]string{"-changelog", path, "-version", "v2.0.0-rc.1", "-open"}, &out, os.Stderr, fixed(t)); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "## [Unreleased]\n\n## [2.0.0-rc.1]\n\n") {
		t.Errorf("not released as an open entry:\n%s", got)
	}
	if !strings.Contains(out.String(), "prerelease=true\n") {
		t.Errorf("outputs: %s", out.String())
	}
}

func TestReleaseCmdRefusesOpenWithDate(t *testing.T) {
	path := write(t, unreleased)
	if err := releaseCmd([]string{"-changelog", path, "-open", "-date", "2020-02-02"}, os.Stdout, os.Stderr, fixed(t)); err == nil {
		t.Error("no error")
	}
	if got, _ := os.ReadFile(path); string(got) != unreleased {
		t.Errorf("the file changed:\n%s", got)
	}
}

func TestReleaseCmdLeavesTheFileOnError(t *testing.T) {
	path := write(t, unreleased)
	if err := releaseCmd([]string{"-changelog", path, "-version", "1.0.0"}, os.Stdout, os.Stderr, fixed(t)); err == nil {
		t.Error("no error for a version below the newest")
	}
	if got, _ := os.ReadFile(path); string(got) != unreleased {
		t.Errorf("the file changed:\n%s", got)
	}
}
