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

// The finding names the rule the heading broke rather than restating the
// grammar. A single message covering every rejection told the author of
// "1.3.0+build.1" that a version states all three of major, minor and patch,
// which it does.
func TestLintUnreadableVersionNamesTheRule(t *testing.T) {
	for _, tc := range []struct{ heading, want string }{
		{"[9.9]", "three components"},
		{"[01.2.3]", "leading zero"},
		{"[v1.2.3]", `leading "v"`},
		{"[1.2.x]", "patch is not a number"},
		{"[1.2.3-alpha_1]", "outside [0-9A-Za-z-]"},
		{"[1.2.3-]", "identifier 1 is empty"},
	} {
		t.Run(tc.heading, func(t *testing.T) {
			got := lint(t, nil,
				"# Changelog", "",
				"## [Unreleased]", "",
				"## "+tc.heading, "", "### Added", "", "- a thing", "",
				"## [9.8.0] - 2026-01-01", "", "### Fixed", "", "- a bug",
			)
			if len(got) != 1 || got[0].Check != CheckUnreadableVersion {
				t.Fatalf("findings %v, want exactly the %s one", got, CheckUnreadableVersion)
			}
			if !strings.Contains(got[0].Msg, tc.want) {
				t.Errorf("finding %q does not say %q", got[0].Msg, tc.want)
			}
		})
	}
}

// Build metadata is valid Semantic Versioning, so the heading is readable and
// this check is silent on it. Whether the version is one this repository wants
// is a separate question, asked under a name of its own.
func TestLintBuildMetadataIsAReadableVersion(t *testing.T) {
	got := lint(t, nil,
		"# Changelog", "",
		"## [Unreleased]", "",
		"## [1.2.3+build.1] - 2026-01-01", "", "### Added", "", "- a thing",
	)
	for _, f := range got {
		if f.Check == CheckUnreadableVersion {
			t.Errorf("finding %v: build metadata is a version the specification accepts", f)
		}
	}
	if len(got) != 0 {
		t.Errorf("findings %v, want none", got)
	}
}

// lintGit runs a document against a repository state, which is what the checks
// reading tags need.
func lintGit(t *testing.T, g *Git, lines ...string) []Finding {
	t.Helper()
	return Parse([]byte(strings.Join(lines, "\n"))).Lint(Options{Git: g})
}

// A released entry's date is a claim about the day its release was cut, and
// nothing else in the register checks it: date-format reads the shape, and
// date-order and date-future read the document alone.
func TestLintDateMismatch(t *testing.T) {
	doc := []string{
		"# Changelog", "",
		"## [1.1.0] - 2026-02-02", "", "### Added", "", "- a thing", "",
		"## [1.0.0] - 2026-01-01", "", "### Added", "", "- the first thing",
	}

	for _, tc := range []struct {
		name  string
		git   *Git
		want  []string
		lines []int
	}{
		{
			name: "every tagged entry agrees",
			git: &Git{
				Tags:    []string{"v1.1.0", "v1.0.0"},
				TagDays: map[string]string{"v1.1.0": "2026-02-02", "v1.0.0": "2026-01-01"},
			},
		},
		{
			name: "the newest entry disagrees",
			git: &Git{
				Tags:    []string{"v1.1.0", "v1.0.0"},
				TagDays: map[string]string{"v1.1.0": "2026-02-03", "v1.0.0": "2026-01-01"},
			},
			want:  []string{CheckDateMismatch},
			lines: []int{3},
		},
		{
			// An entry below the newest is history, and a date that disagrees
			// there is history somebody rewrote.
			name: "an older entry disagrees",
			git: &Git{
				Tags:    []string{"v1.1.0", "v1.0.0"},
				TagDays: map[string]string{"v1.1.0": "2026-02-02", "v1.0.0": "2026-01-09"},
			},
			want:  []string{CheckDateMismatch},
			lines: []int{9},
		},
		{
			name: "both disagree",
			git: &Git{
				Tags:    []string{"v1.1.0", "v1.0.0"},
				TagDays: map[string]string{"v1.1.0": "2026-02-03", "v1.0.0": "2026-01-09"},
			},
			want:  []string{CheckDateMismatch, CheckDateMismatch},
			lines: []int{3, 9},
		},
		{
			// No tag names 1.1.0, so its date is a release pending rather than
			// a date that disagrees. That is B1713's subject, not this check's.
			name: "the newest entry is untagged",
			git: &Git{
				Tags:    []string{"v1.0.0"},
				TagDays: map[string]string{"v1.0.0": "2026-01-01"},
			},
		},
		{
			// A tag whose day could not be read is nothing to compare against.
			name: "the tag's day is unknown",
			git: &Git{
				Tags:    []string{"v1.1.0", "v1.0.0"},
				TagDays: map[string]string{"v1.0.0": "2026-01-01"},
			},
		},
		{
			// A repository that could not be read is no-git-tags' to report.
			name:  "the history could not be read",
			git:   &Git{Err: errUnreadable},
			want:  []string{CheckNoGitTags},
			lines: []int{0},
		},
		{
			name: "no repository was offered",
			git:  nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := lintGit(t, tc.git, doc...)
			if len(got) != len(tc.want) {
				t.Fatalf("findings %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i].Check != tc.want[i] {
					t.Errorf("finding %d is %s, want %s", i, got[i].Check, tc.want[i])
				}
				if got[i].Line != tc.lines[i] {
					t.Errorf("finding %d is at line %d, want %d", i, got[i].Line, tc.lines[i])
				}
			}
		})
	}
}

// The message names both dates and the tag, because the reader has to decide
// which of the two is wrong and cannot do that from either alone.
func TestLintDateMismatchNamesBothDates(t *testing.T) {
	got := lintGit(t, &Git{
		Tags:    []string{"v1.0.0"},
		TagDays: map[string]string{"v1.0.0": "2026-01-09"},
	},
		"# Changelog", "",
		"## [1.0.0] - 2026-01-01", "", "### Added", "", "- a thing",
	)
	if len(got) != 1 {
		t.Fatalf("findings %v, want exactly one", got)
	}
	for _, want := range []string{"1.0.0", "2026-01-01", "v1.0.0", "2026-01-09"} {
		if !strings.Contains(got[0].Msg, want) {
			t.Errorf("finding %q does not name %q", got[0].Msg, want)
		}
	}
}

// A tag matches the version it names rather than the text it is written in, so
// a repository tagging 1.0.0 is read the same as one tagging v1.0.0.
func TestLintDateMismatchMatchesTheVersionNotTheSpelling(t *testing.T) {
	for _, tag := range []string{"v1.0.0", "1.0.0", "V1.0.0"} {
		t.Run(tag, func(t *testing.T) {
			got := lintGit(t, &Git{
				Tags:    []string{tag},
				TagDays: map[string]string{tag: "2026-01-09"},
			},
				"# Changelog", "",
				"## [1.0.0] - 2026-01-01", "", "### Added", "", "- a thing",
			)
			if len(got) != 1 || got[0].Check != CheckDateMismatch {
				t.Fatalf("findings %v, want the %s one", got, CheckDateMismatch)
			}
		})
	}
}

var errUnreadable = errTest("the tag history cannot be read")

type errTest string

func (e errTest) Error() string { return string(e) }

// A repository following the GitHub Actions convention keeps a moving major tag
// beside the full one it points at, and both name the same version. The release
// was cut at the fuller spelling; the shorter moves to the next release, so its
// date says when it last moved and comparing an entry against it reports every
// shipped release as misdated the moment the pointer advances.
func TestLintDateMismatchPrefersTheFullestTagSpelling(t *testing.T) {
	doc := []string{
		"# Changelog", "",
		"## [1.0.0] - 2026-01-01", "", "### Added", "- a thing",
	}
	// v1 has moved on since; v1.0.0 is where the release was cut.
	g := &Git{
		Tags:    []string{"v1", "v1.0.0"},
		TagDays: map[string]string{"v1": "2026-06-06", "v1.0.0": "2026-01-01"},
	}
	if got := lintGit(t, g, doc...); len(got) != 0 {
		t.Errorf("findings %v, want none: the release was cut at v1.0.0", got)
	}

	// Order must not decide it either.
	g.Tags = []string{"v1.0.0", "v1"}
	if got := lintGit(t, g, doc...); len(got) != 0 {
		t.Errorf("findings %v with the tags the other way round, want none", got)
	}

	// And the fuller spelling is what is judged, so a real mismatch there still
	// fires and names that tag rather than the pointer.
	g.TagDays["v1.0.0"] = "2026-01-09"
	got := lintGit(t, g, doc...)
	if len(got) != 1 || got[0].Check != CheckDateMismatch {
		t.Fatalf("findings %v, want the %s one", got, CheckDateMismatch)
	}
	if !strings.Contains(got[0].Msg, "v1.0.0") || strings.Contains(got[0].Msg, "tag v1 ") {
		t.Errorf("finding %q names the wrong tag", got[0].Msg)
	}
}
