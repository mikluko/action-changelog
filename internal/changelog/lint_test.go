package changelog

import (
	"strings"
	"testing"
	"time"
)

func lint(t *testing.T, sections []string, lines ...string) []Finding {
	t.Helper()
	return Parse([]byte(strings.Join(lines, "\n"))).Lint(Options{Sections: sections})
}

func TestLintAcceptsAWellFormedDocument(t *testing.T) {
	got := lint(t, nil,
		"# Changelog",
		"",
		"## [Unreleased]",
		"",
		"## [1.1.0] - 2026-09-04",
		"",
		"### Added",
		"",
		"- a thing",
		"",
		"## [1.0.0] - 2026-08-01",
		"",
		"### Fixed",
		"",
		"- a bug",
		"",
		"[1.1.0]: https://example.test/compare/v1.0.0...v1.1.0",
		"[1.0.0]: https://example.test/releases/tag/v1.0.0",
	)
	if len(got) != 0 {
		t.Fatalf("findings on a well formed document: %v", got)
	}
}

func TestLintFindings(t *testing.T) {
	for _, tc := range []struct {
		name  string
		lines []string
		check string
		want  string
	}{
		{
			"heading is not a version",
			[]string{"# Changelog", "", "## Release Two", "", "- a thing"},
			CheckUnreadableVersion,
			"neither [Unreleased] nor a version",
		},
		{
			"heading states two of the three version numbers",
			[]string{"# Changelog", "", "## [1.1] - 2026-09-04", "", "- a thing"},
			CheckUnreadableVersion,
			"neither [Unreleased] nor a version",
		},
		{
			"a version below the newest without a date",
			[]string{
				"# Changelog", "",
				"## [1.1.0] - 2026-09-04", "", "- a thing", "",
				"## [1.0.0]", "", "- another",
			},
			CheckHeadingForm,
			"carries no date",
		},
		{
			"date is not YYYY-MM-DD",
			[]string{"# Changelog", "", "## [1.0.0] - 4 Sep 2026", "", "- a thing"},
			CheckDateFormat,
			"is not YYYY-MM-DD",
		},
		{
			"entries out of order",
			[]string{
				"# Changelog", "",
				"## [1.0.0] - 2026-08-01", "", "- a thing", "",
				"## [1.1.0] - 2026-09-04", "", "- another",
			},
			CheckVersionOrder,
			"does not come before",
		},
		{
			"same version twice",
			[]string{
				"# Changelog", "",
				"## [1.0.0] - 2026-08-01", "", "- a thing", "",
				"## [1.0.0] - 2026-07-01", "", "- another",
			},
			CheckVersionOrder,
			"does not come before",
		},
		{
			"version with no entries",
			[]string{"# Changelog", "", "## [1.0.0] - 2026-08-01", "", "## [0.9.0] - 2026-07-01", "", "- a thing"},
			CheckEmptyEntry,
			"has no entries under it",
		},
		{
			"section outside the vocabulary",
			[]string{"# Changelog", "", "## [1.0.0] - 2026-08-01", "", "### Improved", "", "- a thing"},
			CheckUnknownSection,
			`section "Improved" is not one of`,
		},
		{
			"date increases going down the file",
			[]string{
				"# Changelog", "",
				"## [1.1.0] - 2025-08-01", "", "- a thing", "",
				"## [1.0.0] - 2025-09-04", "", "- another",
			},
			CheckDateOrder,
			"later than 2025-08-01 above it",
		},
		{
			"one entry carries a link reference and another does not",
			[]string{
				"# Changelog", "",
				"## [1.1.0] - 2025-09-04", "", "- a thing", "",
				"## [1.0.0] - 2025-08-01", "", "- another", "",
				"[1.1.0]: https://example.test/compare/v1.0.0...v1.1.0",
			},
			CheckPartialLinkRef,
			"has no link reference definition",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := lint(t, nil, tc.lines...)
			if len(got) == 0 {
				t.Fatalf("no findings, want one matching %q", tc.want)
			}
			for _, f := range got {
				if !strings.Contains(f.Msg, tc.want) {
					continue
				}
				if f.Check != tc.check {
					t.Errorf("finding %v raised by %q, want %q", f, f.Check, tc.check)
				}
				if f.Severity != Error {
					t.Errorf("finding %v at %s, want error by default", f, f.Severity)
				}
				return
			}
			t.Errorf("findings %v, want one matching %q", got, tc.want)
		})
	}
}

// An Unreleased section is held to the vocabulary like any other, but is not
// required to carry a version, a date, or any entries.
func TestLintUnreleasedNeedsNoContent(t *testing.T) {
	got := lint(t, nil, "# Changelog", "", "## [Unreleased]")
	if len(got) != 0 {
		t.Fatalf("findings on an empty Unreleased: %v", got)
	}
}

// A document carrying no link reference definitions at all is written without
// them rather than half finished, so partial-link-refs says nothing about it.
func TestLintLinkRefsAreAllOrNothing(t *testing.T) {
	got := lint(t, nil,
		"# Changelog", "",
		"## [1.1.0] - 2025-09-04", "", "- a thing", "",
		"## [1.0.0] - 2025-08-01", "", "- another",
	)
	if len(got) != 0 {
		t.Fatalf("findings on a document with no link references: %v", got)
	}
}

// date-future is read against the clock, so the boundary is what a test can
// pin: today passes and the next day does not.
func TestLintDateFutureIsRelativeToToday(t *testing.T) {
	defer func(orig func() time.Time) { now = orig }(now)
	now = func() time.Time { return time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC) }

	if got := lint(t, nil, "# Changelog", "", "## [1.0.0] - 2026-09-04", "", "- a thing"); len(got) != 0 {
		t.Errorf("findings on an entry dated today: %v", got)
	}

	got := lint(t, nil, "# Changelog", "", "## [1.0.0] - 2026-09-05", "", "- a thing")
	if len(got) != 1 {
		t.Fatalf("findings on an entry dated tomorrow: %v, want one", got)
	}
	if got[0].Check != CheckDateFuture || got[0].Severity != Error {
		t.Errorf("finding %v, want %s at error", got[0], CheckDateFuture)
	}
}

// prerelease-entry encodes a policy rather than catching a defect, so it is the
// one check that says nothing until a caller turns it on.
func TestLintPrereleaseEntryIsOffByDefault(t *testing.T) {
	lines := []string{"# Changelog", "", "## [1.0.0-rc.1] - 2025-09-04", "", "- a thing"}

	if got := lint(t, nil, lines...); len(got) != 0 {
		t.Fatalf("findings on a pre-release heading by default: %v", got)
	}

	severities := DefaultSeverities()
	if err := severities.Set(CheckPrereleaseEntry, Error); err != nil {
		t.Fatal(err)
	}
	got := Parse([]byte(strings.Join(lines, "\n"))).Lint(Options{Severities: severities})
	if len(got) != 1 {
		t.Fatalf("findings once enabled: %v, want one", got)
	}
	if got[0].Check != CheckPrereleaseEntry || !strings.Contains(got[0].Msg, "is a pre-release") {
		t.Errorf("finding %v, want %s", got[0], CheckPrereleaseEntry)
	}
}

// The vocabulary is configurable so a repository already using "Breaking" as a
// heading passes without that spelling becoming the default for everyone.
func TestLintVocabularyIsConfigurable(t *testing.T) {
	lines := []string{"# Changelog", "", "## [1.0.0] - 2026-08-01", "", "### Breaking", "", "- a thing"}

	if got := lint(t, nil, lines...); len(got) == 0 {
		t.Error("Breaking accepted by default, want a finding")
	}
	extended := append(append([]string{}, DefaultSections...), "Breaking")
	if got := lint(t, extended, lines...); len(got) != 0 {
		t.Errorf("Breaking rejected after being allowed: %v", got)
	}
}

// A release opened on a branch of its own names its version before its date is
// known, so the newest entry may carry a version alone. undated-entry answers
// for that entry where heading-form answers for every other, and switching it
// off is the whole of what such a branch's invocation carries.
//
// No repository is offered here, so the entry is read as open rather than as a
// release nobody dated; which of the two an offered repository decides is
// TestGitChecks.
//
// With the check off the open entry passes everything: no date to format, none
// to order against the entry below it, none to be in the future, and the link
// reference definition it already carries in released form.
func TestLintUndatedEntryOpensTheNewestEntry(t *testing.T) {
	lines := []string{
		"# Changelog", "",
		"## [Unreleased]", "",
		"## [1.1.0]", "", "### Added", "", "- a thing", "",
		"## [1.0.0] - 2026-08-01", "", "### Fixed", "", "- a bug", "",
		"[1.1.0]: https://example.test/compare/v1.0.0...v1.1.0",
		"[1.0.0]: https://example.test/releases/tag/v1.0.0",
	}

	got := lint(t, nil, lines...)
	if len(got) != 1 {
		t.Fatalf("findings %v by default, want exactly the undated-entry one", got)
	}
	if got[0].Check != CheckUndatedEntry || got[0].Severity != Error {
		t.Errorf("finding %v raised by %q at %s, want %s at error",
			got[0], got[0].Check, got[0].Severity, CheckUndatedEntry)
	}
	if !strings.Contains(got[0].Msg, "1.1.0") {
		t.Errorf("finding %v does not name the entry it is about", got[0])
	}

	severities := DefaultSeverities()
	if err := severities.Set(CheckUndatedEntry, Off); err != nil {
		t.Fatal(err)
	}
	if got := Parse([]byte(strings.Join(lines, "\n"))).Lint(Options{Severities: severities}); len(got) != 0 {
		t.Fatalf("findings on an open entry with undated-entry off: %v", got)
	}
}

// A heading naming a version that cannot be read is reported by a check of its
// own rather than by heading-form, which is what makes the two configurable
// apart: heading-form asks about the shape of a heading, a question a
// repository may reasonably differ on, and this one stands between a heading
// nothing can read and the entry below it being taken for the newest.
//
// [Unreleased] names no version either and is no finding at all, which is the
// one heading this check has to tell apart from an unreadable one.
func TestLintUnreadableVersionIsSeparateFromHeadingForm(t *testing.T) {
	lines := []string{
		"# Changelog", "",
		"## [Unreleased]", "",
		"## [9.9]", "", "### Added", "", "- a thing", "",
		"## [9.8.0] - 2026-01-01", "", "### Fixed", "", "- a bug",
	}

	got := lint(t, nil, lines...)
	if len(got) != 1 {
		t.Fatalf("findings %v, want exactly the %s one", got, CheckUnreadableVersion)
	}
	if got[0].Check != CheckUnreadableVersion || got[0].Severity != Error {
		t.Errorf("finding %v raised by %q at %s, want %s at error",
			got[0], got[0].Check, got[0].Severity, CheckUnreadableVersion)
	}
	if !strings.Contains(got[0].Msg, "9.9") {
		t.Errorf("finding %v does not name the heading it is about", got[0])
	}

	severities := DefaultSeverities()
	if err := severities.Set(CheckHeadingForm, Off); err != nil {
		t.Fatal(err)
	}
	off := Parse([]byte(strings.Join(lines, "\n"))).Lint(Options{Severities: severities})
	if len(off) != 1 || off[0].Check != CheckUnreadableVersion {
		t.Errorf("findings %v with heading-form off, want the %s one", off, CheckUnreadableVersion)
	}
}
