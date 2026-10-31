package changelog

import (
	"strings"
	"testing"

	"sarak.sh/actions/changelog/pkg/semver"
)

const head = "# Changelog\n\nPreamble.\n\n"

func TestAppend(t *testing.T) {
	for _, tc := range []struct {
		name, src, section, note, want string
	}{
		{
			name:    "existing section gets the item at its end",
			src:     head + "## [Unreleased]\n\n### Added\n\n- one\n\n## [1.0.0] - 2020-01-01\n\n- x\n",
			section: "Added", note: "two",
			want: head + "## [Unreleased]\n\n### Added\n\n- one\n- two\n\n## [1.0.0] - 2020-01-01\n\n- x\n",
		},
		{
			name:    "section matched case-insensitively",
			src:     head + "## [Unreleased]\n\n### Fixed\n\n- one\n",
			section: "fixed", note: "two",
			want: head + "## [Unreleased]\n\n### Fixed\n\n- one\n- two\n",
		},
		{
			name:    "empty section",
			src:     head + "## [Unreleased]\n\n### Added\n\n## [1.0.0] - 2020-01-01\n\n- x\n",
			section: "Added", note: "two",
			want: head + "## [Unreleased]\n\n### Added\n\n- two\n\n## [1.0.0] - 2020-01-01\n\n- x\n",
		},
		{
			name:    "new section before a later one",
			src:     head + "## [Unreleased]\n\n### Fixed\n\n- f\n",
			section: "Added", note: "a",
			want: head + "## [Unreleased]\n\n### Added\n\n- a\n\n### Fixed\n\n- f\n",
		},
		{
			name:    "new section after an earlier one",
			src:     head + "## [Unreleased]\n\n### Added\n\n- a\n\n## [1.0.0] - 2020-01-01\n\n- x\n",
			section: "Fixed", note: "f",
			want: head + "## [Unreleased]\n\n### Added\n\n- a\n\n### Fixed\n\n- f\n\n## [1.0.0] - 2020-01-01\n\n- x\n",
		},
		{
			name:    "empty unreleased entry",
			src:     head + "## [Unreleased]\n\n## [1.0.0] - 2020-01-01\n\n- x\n",
			section: "Added", note: "a",
			want: head + "## [Unreleased]\n\n### Added\n\n- a\n\n## [1.0.0] - 2020-01-01\n\n- x\n",
		},
		{
			name:    "unreleased entry created above the newest",
			src:     head + "## [1.0.0] - 2020-01-01\n\n- x\n",
			section: "Added", note: "a",
			want: head + "## [Unreleased]\n\n### Added\n\n- a\n\n## [1.0.0] - 2020-01-01\n\n- x\n",
		},
		{
			name:    "unreleased entry created in a document with none",
			src:     head,
			section: "Added", note: "a",
			want: head + "## [Unreleased]\n\n### Added\n\n- a\n",
		},
		{
			name:    "last entry before link references",
			src:     head + "## [Unreleased]\n\n### Added\n\n- a\n\n[Unreleased]: https://h/compare/v1.0.0...HEAD\n",
			section: "Added", note: "b",
			want: head + "## [Unreleased]\n\n### Added\n\n- a\n- b\n\n[Unreleased]: https://h/compare/v1.0.0...HEAD\n",
		},
		{
			name:    "multi-line note stays one item",
			src:     head + "## [Unreleased]\n\n### Added\n\n- one\n",
			section: "Added", note: "**Thing.** First line\nsecond line\n\nNew paragraph.\n",
			want: head + "## [Unreleased]\n\n### Added\n\n- one\n- **Thing.** First line\n  second line\n\n  New paragraph.\n",
		},
		{
			name:    "an item already there is not added twice",
			src:     head + "## [Unreleased]\n\n### Added\n\n- one\n- two\n\n## [1.0.0] - 2020-01-01\n\n- x\n",
			section: "Added", note: "one",
			want: head + "## [Unreleased]\n\n### Added\n\n- one\n- two\n\n## [1.0.0] - 2020-01-01\n\n- x\n",
		},
		{
			name:    "the same item under another section is added",
			src:     head + "## [Unreleased]\n\n### Added\n\n- one\n\n### Fixed\n\n- f\n",
			section: "Fixed", note: "one",
			want: head + "## [Unreleased]\n\n### Added\n\n- one\n\n### Fixed\n\n- f\n- one\n",
		},
		{
			name:    "a fenced heading is content",
			src:     head + "## [Unreleased]\n\n### Added\n\n```\n### Fixed\n```\n",
			section: "Fixed", note: "f",
			want: head + "## [Unreleased]\n\n### Added\n\n```\n### Fixed\n```\n\n### Fixed\n\n- f\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Append([]byte(tc.src), tc.section, tc.note)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Errorf("got\n%s\nwant\n%s", got, tc.want)
			}
		})
	}
}

func TestAppendRefuses(t *testing.T) {
	for _, tc := range []struct{ name, section, note string }{
		{"empty note", "Added", " \n"},
		{"empty section", "", "a"},
		{"multi-line section", "Added\nFixed", "a"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Append([]byte(head), tc.section, tc.note); err == nil {
				t.Error("no error")
			}
		})
	}
}

func TestRelease(t *testing.T) {
	v := semver.Version{Major: 1, Minor: 1}
	for _, tc := range []struct {
		name, src, date, want string
	}{
		{
			name: "dated, with compare links",
			src: head + "## [Unreleased]\n\n### Added\n\n- a\n\n## [1.0.0] - 2020-01-01\n\n- x\n\n" +
				"[Unreleased]: https://h/o/r/compare/v1.0.0...HEAD\n[1.0.0]: https://h/o/r/releases/tag/v1.0.0\n",
			date: "2020-02-02",
			want: head + "## [Unreleased]\n\n## [1.1.0] - 2020-02-02\n\n### Added\n\n- a\n\n## [1.0.0] - 2020-01-01\n\n- x\n\n" +
				"[Unreleased]: https://h/o/r/compare/v1.1.0...HEAD\n[1.1.0]: https://h/o/r/compare/v1.0.0...v1.1.0\n[1.0.0]: https://h/o/r/releases/tag/v1.0.0\n",
		},
		{
			name: "open, no links, tags without v",
			src:  head + "## [Unreleased]\n\n### Added\n\n- a\n\n## [1.0.0] - 2020-01-01\n\n- x\n",
			want: head + "## [Unreleased]\n\n## [1.1.0]\n\n### Added\n\n- a\n\n## [1.0.0] - 2020-01-01\n\n- x\n",
		},
		{
			name: "tag spelling follows the previous tag",
			src:  head + "## [Unreleased]\n\n- a\n\n[Unreleased]: https://h/compare/1.0.0...HEAD\n",
			date: "2020-02-02",
			want: head + "## [Unreleased]\n\n## [1.1.0] - 2020-02-02\n\n- a\n\n[Unreleased]: https://h/compare/1.1.0...HEAD\n[1.1.0]: https://h/compare/1.0.0...1.1.0\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Release([]byte(tc.src), v, tc.date)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Errorf("got\n%s\nwant\n%s", got, tc.want)
			}
		})
	}
}

func TestReleaseRefuses(t *testing.T) {
	v := semver.Version{Major: 1, Minor: 1}
	for _, tc := range []struct{ name, src, date, want string }{
		{"no unreleased entry", head + "## [1.0.0] - 2020-01-01\n\n- x\n", "", "no [Unreleased]"},
		{"empty unreleased entry", head + "## [Unreleased]\n\n## [1.0.0] - 2020-01-01\n\n- x\n", "", "no notes"},
		{"version not above", head + "## [Unreleased]\n\n- a\n\n## [1.1.0] - 2020-01-01\n\n- x\n", "", "not above"},
		{"open entry below", head + "## [Unreleased]\n\n- a\n\n## [1.0.0]\n\n- x\n", "", "is open"},
		{"bad date", head + "## [Unreleased]\n\n- a\n", "02/02/2020", "YYYY-MM-DD"},
		{"refs without unreleased", head + "## [Unreleased]\n\n- a\n\n## [1.0.0] - 2020-01-01\n\n- x\n\n[1.0.0]: https://h\n", "", "none for [Unreleased]"},
		{"unreleased ref not a compare", head + "## [Unreleased]\n\n- a\n\n[Unreleased]: https://h/commits/main\n", "", "not a compare URL"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Release([]byte(tc.src), v, tc.date)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want one containing %q", err, tc.want)
			}
		})
	}
}

// What Release writes is a document Lint has nothing new to say about, which
// is the point of writing it rather than leaving it to a person.
func TestReleaseLintsClean(t *testing.T) {
	src := head + "## [Unreleased]\n\n### Added\n\n- a\n\n## [1.0.0] - 2020-01-01\n\n### Fixed\n\n- x\n\n" +
		"[Unreleased]: https://h/compare/v1.0.0...HEAD\n[1.0.0]: https://h/releases/tag/v1.0.0\n"
	if f := Parse([]byte(src)).Lint(Options{}); len(f) != 0 {
		t.Fatalf("the input already has findings: %v", f)
	}
	out, err := Release([]byte(src), semver.Version{Major: 1, Minor: 1}, "2020-02-02")
	if err != nil {
		t.Fatal(err)
	}
	if f := Parse(out).Lint(Options{}); len(f) != 0 {
		t.Errorf("findings after release: %v\n%s", f, out)
	}
}

func TestNext(t *testing.T) {
	const prior = "## [1.2.3] - 2020-01-01\n\n- x\n"
	for _, tc := range []struct{ name, src, want string }{
		{"fixes only", "## [Unreleased]\n\n### Fixed\n\n- f\n\n### Security\n\n- s\n\n" + prior, "1.2.4"},
		{"an addition", "## [Unreleased]\n\n### Fixed\n\n- f\n\n### Added\n\n- a\n\n" + prior, "1.3.0"},
		{"no sections", "## [Unreleased]\n\n- a\n\n" + prior, "1.2.4"},
		{"marked breaking", "## [Unreleased]\n\n### Changed\n\n- **Breaking:** b\n\n" + prior, "2.0.0"},
		{"breaking marked in a fix", "## [Unreleased]\n\n### Fixed\n\n- **breaking** b\n\n" + prior, "2.0.0"},
		{"breaking section", "## [Unreleased]\n\n### Breaking\n\n- b\n\n" + prior, "2.0.0"},
		{"emphasis that is not the marker", "## [Unreleased]\n\n### Fixed\n\n- **Not** breaking\n\n" + prior, "1.2.4"},
		{"breaking below 1.0.0", "## [Unreleased]\n\n- **Breaking:** b\n\n## [0.4.1] - 2020-01-01\n\n- x\n", "0.5.0"},
		{"bumps from the newest final", "## [Unreleased]\n\n### Fixed\n\n- f\n\n## [1.3.0-rc.1] - 2020-02-01\n\n- r\n\n" + prior, "1.2.4"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse([]byte(head + tc.src)).Next()
			if err != nil {
				t.Fatal(err)
			}
			if got.String() != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestNextRefuses(t *testing.T) {
	for _, tc := range []struct{ name, src string }{
		{"no unreleased notes", "## [Unreleased]\n\n## [1.0.0] - 2020-01-01\n\n- x\n"},
		{"nothing to bump from", "## [Unreleased]\n\n- a\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Parse([]byte(head + tc.src)).Next(); err == nil {
				t.Error("no error")
			}
		})
	}
}
