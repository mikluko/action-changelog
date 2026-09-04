package changelog

import (
	"strings"
	"testing"
)

func lint(t *testing.T, sections []string, lines ...string) []Finding {
	t.Helper()
	return Parse([]byte(strings.Join(lines, "\n"))).Lint(sections)
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
	)
	if len(got) != 0 {
		t.Fatalf("findings on a well formed document: %v", got)
	}
}

func TestLintFindings(t *testing.T) {
	for _, tc := range []struct {
		name  string
		lines []string
		want  string
	}{
		{
			"heading is not a version",
			[]string{"# Changelog", "", "## Release Two", "", "- a thing"},
			"neither [Unreleased] nor a version",
		},
		{
			"version without a date",
			[]string{"# Changelog", "", "## [1.0.0]", "", "- a thing"},
			"carries no date",
		},
		{
			"date is not YYYY-MM-DD",
			[]string{"# Changelog", "", "## [1.0.0] - 4 Sep 2026", "", "- a thing"},
			"is not YYYY-MM-DD",
		},
		{
			"entries out of order",
			[]string{
				"# Changelog", "",
				"## [1.0.0] - 2026-08-01", "", "- a thing", "",
				"## [1.1.0] - 2026-09-04", "", "- another",
			},
			"does not come before",
		},
		{
			"same version twice",
			[]string{
				"# Changelog", "",
				"## [1.0.0] - 2026-08-01", "", "- a thing", "",
				"## [1.0.0] - 2026-07-01", "", "- another",
			},
			"does not come before",
		},
		{
			"version with no entries",
			[]string{"# Changelog", "", "## [1.0.0] - 2026-08-01", "", "## [0.9.0] - 2026-07-01", "", "- a thing"},
			"has no entries under it",
		},
		{
			"section outside the vocabulary",
			[]string{"# Changelog", "", "## [1.0.0] - 2026-08-01", "", "### Improved", "", "- a thing"},
			`section "Improved" is not one of`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := lint(t, nil, tc.lines...)
			if len(got) == 0 {
				t.Fatalf("no findings, want one matching %q", tc.want)
			}
			for _, f := range got {
				if strings.Contains(f.Msg, tc.want) {
					return
				}
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
