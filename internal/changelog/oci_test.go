package changelog

import (
	"strings"
	"testing"
)

// The grammar is the OCI distribution specification's, and Semantic Versioning
// is the wider set in exactly two ways: "+" is in no part of the tag grammar,
// and a pre-release run has no length bound.
func TestOCITagReason(t *testing.T) {
	long := "1.0.0-" + strings.Repeat("a", 123) // 129 characters
	edge := "1.0.0-" + strings.Repeat("a", 122) // 128 characters, the limit itself

	for _, tc := range []struct {
		in   string
		want string // a fragment the reason must carry, or "" for no reason
	}{
		{"", ""},
		{"1.2.3", ""},
		{"1.2.3-rc.1", ""},
		{"1.2.3-alpha-beta", ""},
		{"0.0.0", ""},
		{edge, ""},

		{"1.2.3+build.1", `"+"`},
		{"1.2.3+001", `"+"`},
		{"1.0.0-rc.1+exp.sha.5114f85", `"+"`},
		{"1.0.0+a", `"+"`},

		{long, "at most 128 characters and this is 129"},

		// Not reachable from a parsed version, whose first character is always a
		// digit, but the function implements the grammar rather than the subset
		// this package happens to feed it.
		{"-1.2.3", "opens with"},
		{".1.2.3", "opens with"},
	} {
		t.Run(tc.in, func(t *testing.T) {
			got := ociTagReason(tc.in)
			switch {
			case tc.want == "" && got != "":
				t.Errorf("ociTagReason(%q) = %q, want no reason", tc.in, got)
			case tc.want != "" && !strings.Contains(got, tc.want):
				t.Errorf("ociTagReason(%q) = %q, want it to say %q", tc.in, got, tc.want)
			}
		})
	}
}

// The check is about what a version can become elsewhere, so it fires on the
// version and not on the shape of the document around it.
func TestLintOCIIncompatibleVersion(t *testing.T) {
	got := lint(t, nil,
		"# Changelog", "",
		"## [Unreleased]", "",
		"## [1.2.0+build.7] - 2026-02-01", "", "### Added", "", "- a thing", "",
		"## [1.1.0] - 2026-01-02", "", "### Added", "", "- fine", "",
		"## [1.0.0+a] - 2026-01-01", "", "### Added", "", "- also flagged",
	)
	if len(got) != 2 {
		t.Fatalf("findings %v, want two", got)
	}
	for i, want := range []int{5, 17} {
		if got[i].Check != CheckOCIIncompatible {
			t.Errorf("finding %d is %s, want %s", i, got[i].Check, CheckOCIIncompatible)
		}
		if got[i].Line != want {
			t.Errorf("finding %d is at line %d, want %d", i, got[i].Line, want)
		}
	}
	if !strings.Contains(got[0].Msg, "1.2.0+build.7") || !strings.Contains(got[0].Msg, `"+"`) {
		t.Errorf("finding %q names neither the version nor the character at fault", got[0].Msg)
	}
}

// A repository that publishes no image or chart switches it off, and the
// finding never fires again. That is the whole of the escape hatch the default
// leans on.
func TestLintOCIIncompatibleVersionIsSwitchableOff(t *testing.T) {
	doc := []string{
		"# Changelog", "",
		"## [1.2.0+build.7] - 2026-02-01", "", "### Added", "", "- a thing",
	}
	severities := DefaultSeverities()
	if err := severities.Set(CheckOCIIncompatible, Off); err != nil {
		t.Fatal(err)
	}
	got := Parse([]byte(strings.Join(doc, "\n"))).Lint(Options{Severities: severities})
	if len(got) != 0 {
		t.Errorf("findings %v with the check off, want none", got)
	}
}

// It is a separate check from the parser's verdict, which is the split the map
// this came from turned on: the parser says what a version is, and this says
// what it can become.
func TestLintOCIIncompatibleIsSeparateFromUnreadableVersion(t *testing.T) {
	severities := DefaultSeverities()
	if err := severities.Set(CheckOCIIncompatible, Off); err != nil {
		t.Fatal(err)
	}
	doc := strings.Join([]string{
		"# Changelog", "",
		"## [1.2.0+build.7] - 2026-02-01", "", "### Added", "", "- a thing",
	}, "\n")
	if got := Parse([]byte(doc)).Lint(Options{Severities: severities}); len(got) != 0 {
		t.Errorf("findings %v: build metadata is a version the specification accepts", got)
	}
}
