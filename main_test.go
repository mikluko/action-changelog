package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mikluko/action-changelog/internal/changelog"
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
	red, err := run(path, changelog.Options{Severities: sev}, changelog.Error, &log, &log)
	if err != nil {
		t.Fatal(err)
	}
	if !red {
		t.Fatalf("date-format did not turn the build red; log: %s", log.String())
	}

	sev, err = severities("", "", changelog.CheckDateFormat)
	if err != nil {
		t.Fatal(err)
	}
	log.Reset()
	red, err = run(path, changelog.Options{Severities: sev}, changelog.Error, &log, &log)
	if err != nil {
		t.Fatal(err)
	}
	if red {
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
	red, err := run(path, changelog.Options{}, th, &log, &log)
	if err != nil {
		t.Fatal(err)
	}
	if red {
		t.Error("-fail-on never turned the build red")
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
			red, err := run(path, changelog.Options{Severities: sev}, th, &log, &log)
			if err != nil {
				t.Fatal(err)
			}
			if red != tc.red {
				t.Errorf("red is %v, want %v; log: %s", red, tc.red, log.String())
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
