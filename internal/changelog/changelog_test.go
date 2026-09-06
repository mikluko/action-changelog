package changelog

import (
	"errors"
	"strings"
	"testing"

	"github.com/mikluko/action-changelog/internal/semver"
)

func TestParseHeadingForms(t *testing.T) {
	for _, tc := range []struct {
		name       string
		in         string
		version    string
		date       string
		unreleased bool
		rule       semver.Rule
	}{
		{name: "bracketed with date", in: "[1.2.3] - 2026-09-04", version: "v1.2.3", date: "2026-09-04"},
		{name: "bracketed no date", in: "[1.2.3]", version: "v1.2.3"},
		{name: "bare with date", in: "1.2.3 - 2026-09-04", version: "v1.2.3", date: "2026-09-04"},
		{name: "unreleased", in: "[Unreleased]", unreleased: true},
		{name: "unreleased bare", in: "Unreleased", unreleased: true},
		{name: "prerelease", in: "[1.2.3-rc.1] - 2026-09-04", version: "v1.2.3-rc.1", date: "2026-09-04"},

		// Build metadata is valid Semantic Versioning, so the heading names a
		// version. Whether this repository wants one is oci-incompatible-version's
		// to say, and not this parser's.
		{name: "build metadata", in: "[1.2.3+build.1] - 2026-09-04", version: "v1.2.3+build.1", date: "2026-09-04"},

		// The leading "v" belongs to a tag namespace, not to the specification,
		// and a heading is not a tag.
		{name: "v prefix", in: "[v1.2.3] - 2026-09-04", rule: semver.RuleVPrefix},

		{name: "not a version", in: "[Yanked] - 2026-09-04", rule: semver.RuleCore},
		{name: "not a number", in: "[1.2.x] - 2026-09-04", rule: semver.RuleNumeric},
		{name: "partial version", in: "[1.2] - 2026-09-04", rule: semver.RuleCore},
		{name: "leading zero", in: "[01.2.3] - 2026-09-04", rule: semver.RuleLeadingZero},
		{name: "empty prerelease", in: "[1.2.3-] - 2026-09-04", rule: semver.RuleEmptyIdent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := Entry{Raw: tc.in}
			parseHeading(&e)
			if e.Version != tc.version || e.Date != tc.date || e.Unreleased != tc.unreleased {
				t.Errorf("parseHeading(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tc.in, e.Version, e.Date, e.Unreleased, tc.version, tc.date, tc.unreleased)
			}
			if tc.version != "" && e.Semver.Tag() != tc.version {
				t.Errorf("parseHeading(%q).Semver = %q, want %q", tc.in, e.Semver.Tag(), tc.version)
			}
			err := e.VersionErr
			if tc.rule == "" {
				if err != nil {
					t.Errorf("parseHeading(%q) = %v, want no error", tc.in, err)
				}
				return
			}
			var se *semver.SyntaxError
			if !errors.As(err, &se) {
				t.Fatalf("parseHeading(%q) = %v, want a *semver.SyntaxError", tc.in, err)
			}
			if se.Rule != tc.rule {
				t.Errorf("parseHeading(%q) broke %s, want %s", tc.in, se.Rule, tc.rule)
			}
		})
	}
}

// A version-like line inside a fenced block is content. This is the case that
// makes a Markdown parser worth the dependency: line matching reads it as a
// heading and splits the entry in two.
func TestParseIgnoresFencedHeadings(t *testing.T) {
	src := []byte(strings.Join([]string{
		"# Changelog",
		"",
		"## [1.1.0] - 2026-09-04",
		"",
		"### Added",
		"",
		"- a worked example of the format:",
		"",
		"  ```markdown",
		"  ## [9.9.9] - 1999-01-01",
		"  ```",
		"",
		"## [1.0.0] - 2026-08-01",
		"",
		"### Added",
		"",
		"- the first release",
		"",
		"[1.1.0]: https://example.test/compare/v1.0.0...v1.1.0",
		"[1.0.0]: https://example.test/releases/v1.0.0",
	}, "\n"))

	c := Parse(src)
	got := make([]string, 0, len(c.Entries))
	for _, e := range c.Entries {
		got = append(got, e.Version)
	}
	if want := []string{"v1.1.0", "v1.0.0"}; !equal(got, want) {
		t.Fatalf("versions = %v, want %v", got, want)
	}
	if c.Title != "Changelog" {
		t.Errorf("title = %q, want %q", c.Title, "Changelog")
	}
}

// The link reference definitions at the foot belong to the document, not to
// the last entry's release notes.
func TestParseBodyExcludesLinkDefinitions(t *testing.T) {
	src := []byte(strings.Join([]string{
		"# Changelog",
		"",
		"## [1.0.0] - 2026-08-01",
		"",
		"### Added",
		"",
		"- the first release",
		"",
		"[1.0.0]: https://example.test/releases/v1.0.0",
	}, "\n"))

	c := Parse(src)
	e, ok := c.Latest()
	if !ok {
		t.Fatal("no released entry")
	}
	if strings.Contains(e.Body, "https://example.test") {
		t.Errorf("body carries the link definitions:\n%s", e.Body)
	}
	if !strings.Contains(e.Body, "the first release") {
		t.Errorf("body lost its content:\n%s", e.Body)
	}
	if want := "### Added"; !strings.HasPrefix(e.Body, want) {
		t.Errorf("body = %q, want it to start with %q", e.Body, want)
	}
}

func TestParseSectionsAndLatest(t *testing.T) {
	src := []byte(strings.Join([]string{
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
		"### Fixed",
		"",
		"- a bug",
		"",
		"## [1.0.0] - 2026-08-01",
		"",
		"### Added",
		"",
		"- the first release",
	}, "\n"))

	c := Parse(src)
	if n := len(c.Entries); n != 3 {
		t.Fatalf("entries = %d, want 3", n)
	}
	if !c.Entries[0].Unreleased {
		t.Error("first entry is not Unreleased")
	}
	// Latest skips Unreleased: the ceremony releases a named version.
	e, ok := c.Latest()
	if !ok || e.Version != "v1.1.0" {
		t.Fatalf("Latest() = (%q, %v), want v1.1.0", e.Version, ok)
	}
	if n := len(e.Sections); n != 2 {
		t.Fatalf("sections = %d, want 2", n)
	}
	if e.Sections[0].Name != "Added" || e.Sections[1].Name != "Fixed" {
		t.Errorf("sections = %q, %q", e.Sections[0].Name, e.Sections[1].Name)
	}
	if got, ok := c.Find("1.0.0"); !ok || got.Version != "v1.0.0" {
		t.Errorf("Find(1.0.0) = (%q, %v)", got.Version, ok)
	}
	if n := len(c.Released()); n != 2 {
		t.Errorf("Released() = %d, want 2", n)
	}
}

// The definitions sit at the foot of the document rather than under the entry
// they name, and they stay out of every entry's body.
func TestParseMatchesLinkReferenceDefinitionsToEntries(t *testing.T) {
	src := []byte(strings.Join([]string{
		"# Changelog", "",
		"## [1.1.0] - 2026-09-04", "", "### Added", "", "- a thing", "",
		"## [1.0.0] - 2026-08-01", "", "### Fixed", "", "- a bug", "",
		"[1.1.0]: https://example.test/compare/v1.0.0...v1.1.0",
	}, "\n"))

	entries := Parse(src).Released()
	if n := len(entries); n != 2 {
		t.Fatalf("entries = %d, want 2", n)
	}
	if !entries[0].LinkRef {
		t.Error("1.1.0 has no LinkRef, want one")
	}
	if entries[1].LinkRef {
		t.Error("1.0.0 has a LinkRef, want none")
	}
	if strings.Contains(entries[1].Body, "example.test") {
		t.Errorf("1.0.0 body carries the definition: %q", entries[1].Body)
	}
}

// The newest entry is the first one that is not Unreleased, whether or not its
// heading names a version anything can read. Reading past an unreadable heading
// makes the entry below it the newest, which is a release the document does not
// name being handed to whatever consumes Latest.
func TestLatestStopsAtAnUnreadableHeading(t *testing.T) {
	src := []byte(strings.Join([]string{
		"# Changelog", "",
		"## [Unreleased]", "",
		"## [9.9]", "", "### Added", "", "- a thing", "",
		"## [9.8.0] - 2026-01-01", "", "### Fixed", "", "- a bug",
	}, "\n"))

	if e, ok := Parse(src).Latest(); ok {
		t.Errorf("Latest() = (%q, true), want no entry", e.Version)
	}
	// The entry is still an entry, and the immutability checks still compare it.
	if n := len(Parse(src).Released()); n != 1 {
		t.Errorf("Released() = %d, want 1", n)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
