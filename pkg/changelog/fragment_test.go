package changelog

import (
	"errors"
	"reflect"
	"testing"

	"sarak.sh/mikluko/changelog/pkg/semver"
)

func TestParseFragment(t *testing.T) {
	src := "### Added\n\n- one\n- two\n  continued\n\n  second paragraph\n\n### Fixed\n\n* three\n  - nested\n"
	got, err := ParseFragment([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	want := Fragment{Sections: []FragmentSection{
		{Name: "Added", Line: 1, Notes: []string{"one", "two\ncontinued\n\nsecond paragraph"}},
		{Name: "Fixed", Line: 9, Notes: []string{"three\n- nested"}},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v\nwant %#v", got, want)
	}
}

func TestParseFragmentRefuses(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		line      int
	}{
		{"empty", "", 1},
		{"blank", "\n\n", 1},
		{"notes under no section", "- one\n", 1},
		{"section with no notes", "### Added\n\n### Fixed\n\n- f\n", 1},
		{"last section with no notes", "### Fixed\n\n- f\n\n### Added\n", 5},
		{"prose", "### Fixed\n\n- f\n\nSome prose.\n", 5},
		{"level-2 heading", "## [1.0.0]\n\n### Fixed\n\n- f\n", 1},
		{"ordered list", "### Fixed\n\n1. f\n", 3},
		{"empty note", "### Fixed\n\n- f\n-\n", 4},
		{"link reference definition", "### Fixed\n\n- [f]\n\n[f]: https://example.com\n", 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseFragment([]byte(tc.src))
			var fe *FragmentError
			if !errors.As(err, &fe) {
				t.Fatalf("err = %v, want a *FragmentError", err)
			}
			if fe.Line != tc.line {
				t.Errorf("line %d, want %d: %v", fe.Line, tc.line, fe)
			}
		})
	}
}

func TestLintFragment(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		opts      Options
		want      []string
	}{
		{"clean", "### Fixed\n\n- f\n", Options{}, nil},
		{"form", "- f\n", Options{}, []string{CheckFragmentForm}},
		{"unknown section", "### Fixes\n\n- f\n", Options{}, []string{CheckUnknownSection}},
		{"section accepted by the caller", "### Breaking\n\n- f\n", Options{Sections: []string{"Breaking"}}, nil},
		{"form switched off", "- f\n", Options{Severities: Severities{CheckFragmentForm: Off}}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, f := range LintFragment([]byte(tc.src), tc.opts) {
				got = append(got, f.Check)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFold(t *testing.T) {
	src := head + "## [Unreleased]\n\n### Fixed\n\n- existing\n\n## [1.0.0] - 2020-01-01\n\n- x\n"
	a, err := ParseFragment([]byte("### Fixed\n\n- from a\n\n### Added\n\n- added\n  over two lines\n"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := ParseFragment([]byte("### Fixed\n\n- from b\n- existing\n"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := Fold([]byte(src), a, b)
	if err != nil {
		t.Fatal(err)
	}
	want := head + "## [Unreleased]\n\n### Added\n\n- added\n  over two lines\n\n### Fixed\n\n- existing\n- from a\n- from b\n\n## [1.0.0] - 2020-01-01\n\n- x\n"
	if string(got) != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestFoldNothing(t *testing.T) {
	src := []byte(head + "## [Unreleased]\n")
	got, err := Fold(src)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(src) {
		t.Errorf("got:\n%s", got)
	}
	got[0] = 'X'
	if src[0] == 'X' {
		t.Error("Fold returned src itself")
	}
}

func TestReleaseFragments(t *testing.T) {
	src := head + "## [Unreleased]\n\n## [1.0.0] - 2020-01-01\n\n- x\n"
	frag, err := ParseFragment([]byte("### Added\n\n- a\n"))
	if err != nil {
		t.Fatal(err)
	}
	got, v, err := ReleaseFragments([]byte(src), []Fragment{frag}, nil, "2020-02-02")
	if err != nil {
		t.Fatal(err)
	}
	if v.String() != "1.1.0" {
		t.Errorf("version %s, want 1.1.0", v)
	}
	if want := head + "## [Unreleased]\n\n## [1.1.0] - 2020-02-02\n\n### Added\n\n- a\n\n## [1.0.0] - 2020-01-01\n\n- x\n"; string(got) != want {
		t.Errorf("got:\n%s", got)
	}

	given := semver.Version{Major: 2}
	if _, v, err := ReleaseFragments([]byte(src), []Fragment{frag}, &given, "2020-02-02"); err != nil || v.String() != "2.0.0" {
		t.Errorf("given 2.0.0: %v, %v", v, err)
	}
	if _, _, err := ReleaseFragments([]byte(src), nil, nil, "2020-02-02"); err == nil {
		t.Error("no error with nothing to release")
	}
}
