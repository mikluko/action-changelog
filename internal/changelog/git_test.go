package changelog_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/mikluko/action-changelog/internal/changelog"
)

const tagged = `# Changelog

## [0.2.0] - 2026-02-01

### Added

- The second thing.

## [0.1.0] - 2026-01-01

### Added

- The first thing.
`

func TestGitChecks(t *testing.T) {
	for _, tc := range []struct {
		name string
		doc  string
		git  *changelog.Git
		want []string
	}{
		{
			name: "no repository offered runs none of them",
			doc:  tagged,
			git:  nil,
		},
		{
			name: "an unreadable history is one finding",
			doc:  tagged,
			git:  &changelog.Git{Err: errors.New("no git repository")},
			want: []string{changelog.CheckNoGitTags},
		},
		{
			name: "a history read to the end that names no version says nothing",
			doc:  tagged,
			git:  &changelog.Git{},
		},
		{
			name: "an entry level with the tag passes",
			doc:  tagged,
			git:  &changelog.Git{ReferenceTag: "v0.2.0", TaggedChangelog: []byte(tagged)},
		},
		{
			name: "an entry ahead of the tag passes",
			doc:  strings.Replace(tagged, "## [0.2.0]", "## [0.3.0]", 1),
			git:  &changelog.Git{ReferenceTag: "v0.2.0"},
		},
		{
			name: "an entry behind the tag fires",
			doc:  tagged,
			git:  &changelog.Git{ReferenceTag: "v0.9.0"},
			want: []string{changelog.CheckVersionBehindTag},
		},
		{
			name: "a tag written without its v is read the same",
			doc:  tagged,
			git:  &changelog.Git{ReferenceTag: "0.9.0"},
			want: []string{changelog.CheckVersionBehindTag},
		},
		{
			name: "a pre-release entry orders against its release",
			doc:  strings.Replace(tagged, "## [0.2.0]", "## [0.2.0-rc.1]", 1),
			git:  &changelog.Git{ReferenceTag: "v0.2.0"},
			want: []string{changelog.CheckVersionBehindTag},
		},
		{
			name: "notes rewritten under a released entry fire",
			doc:  strings.Replace(tagged, "- The first thing.", "- The first thing, restated.", 1),
			git:  &changelog.Git{ReferenceTag: "v0.2.0", TaggedChangelog: []byte(tagged)},
			want: []string{changelog.CheckReleaseEntryModified},
		},
		{
			name: "a released entry's date changing fires",
			doc:  strings.Replace(tagged, "0.1.0] - 2026-01-01", "0.1.0] - 2026-01-02", 1),
			git:  &changelog.Git{ReferenceTag: "v0.2.0", TaggedChangelog: []byte(tagged)},
			want: []string{changelog.CheckReleaseEntryModified},
		},
		{
			name: "a released entry deleted fires",
			doc: `# Changelog

## [0.2.0] - 2026-02-01

### Added

- The second thing.
`,
			git:  &changelog.Git{ReferenceTag: "v0.2.0", TaggedChangelog: []byte(tagged)},
			want: []string{changelog.CheckReleaseEntryModified},
		},
		{
			name: "an entry added below the oldest does not fire",
			doc: tagged + `
## [0.0.1] - 2025-12-01

### Added

- The thing nobody wrote down at the time.
`,
			git: &changelog.Git{ReferenceTag: "v0.2.0", TaggedChangelog: []byte(tagged)},
		},
		{
			name: "a link-reference block added at the foot does not fire",
			doc: tagged + `
[0.2.0]: https://example.invalid/compare/v0.1.0...v0.2.0
[0.1.0]: https://example.invalid/releases/tag/v0.1.0
`,
			git: &changelog.Git{ReferenceTag: "v0.2.0", TaggedChangelog: []byte(tagged)},
		},
		{
			name: "a rewritten pre-release entry fires the other check",
			doc:  strings.Replace(prereleased, "- The candidate.", "- The candidate, folded in.", 1),
			git:  &changelog.Git{ReferenceTag: "v0.2.0", TaggedChangelog: []byte(prereleased)},
			want: []string{changelog.CheckPrereleaseEntryModified},
		},
		{
			name: "collapsing a pre-release entry fires only the pre-release check",
			doc:  tagged,
			git:  &changelog.Git{ReferenceTag: "v0.2.0", TaggedChangelog: []byte(prereleased)},
			want: []string{changelog.CheckPrereleaseEntryModified},
		},
		{
			name: "a tag carrying no changelog compares nothing",
			doc:  strings.Replace(tagged, "- The first thing.", "- Rewritten freely.", 1),
			git:  &changelog.Git{ReferenceTag: "v0.2.0"},
		},
		{
			name: "an undated entry no tag names is a release still being written",
			doc:  undated,
			git:  &changelog.Git{ReferenceTag: "v0.1.0", Tags: []string{"v0.1.0"}},
			want: []string{changelog.CheckUndatedEntry},
		},
		{
			name: "an undated entry a tag already names is a release nobody dated",
			doc:  undated,
			git:  &changelog.Git{ReferenceTag: "v0.2.0", Tags: []string{"v0.1.0", "v0.2.0"}},
			want: []string{changelog.CheckUndatedRelease},
		},
		{
			name: "the tag naming it need not be the reference",
			doc:  undated,
			git:  &changelog.Git{Tags: []string{"v0.2.0"}},
			want: []string{changelog.CheckUndatedRelease},
		},
		{
			name: "a tag written without its v names the release just the same",
			doc:  undated,
			git:  &changelog.Git{ReferenceTag: "0.2.0", Tags: []string{"0.2.0"}},
			want: []string{changelog.CheckUndatedRelease},
		},
		{
			name: "tags nobody could read accuse nobody of shipping it undated",
			doc:  undated,
			git:  &changelog.Git{Err: errors.New("no git repository")},
			want: []string{changelog.CheckUndatedEntry, changelog.CheckNoGitTags},
		},
		{
			name: "a caller offering no repository reads it as still being written",
			doc:  undated,
			git:  nil,
			want: []string{changelog.CheckUndatedEntry},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := checks(changelog.Parse([]byte(tc.doc)).Lint(changelog.Options{Git: tc.git}))
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("checks fired %v, want %v", got, tc.want)
			}
		})
	}
}

// undated is the tagged document with the date struck from its newest entry.
// A release still accumulating on a branch of its own leaves it in that state
// and so does a release nobody dated, and only the tags tell the two apart.
var undated = strings.Replace(tagged, "## [0.2.0] - 2026-02-01", "## [0.2.0]", 1)

// The two are separately overridable, which is the whole of the split: a branch
// accumulating a release switches undated-entry off, and a release that shipped
// without its date is still reported on that same invocation.
func TestUndatedChecksAreSeparatelyConfigurable(t *testing.T) {
	sev := changelog.DefaultSeverities()
	if err := sev.Set(changelog.CheckUndatedEntry, changelog.Off); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		git  *changelog.Git
		want []string
	}{
		{
			"a version a tag already names",
			&changelog.Git{ReferenceTag: "v0.2.0", Tags: []string{"v0.2.0"}},
			[]string{changelog.CheckUndatedRelease},
		},
		{
			"a version no tag names yet",
			&changelog.Git{ReferenceTag: "v0.1.0", Tags: []string{"v0.1.0"}},
			nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := checks(changelog.Parse([]byte(undated)).Lint(changelog.Options{Severities: sev, Git: tc.git}))
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("checks fired %v, want %v", got, tc.want)
			}
		})
	}
}

// prereleased is the tagged document with a release candidate above the release
// it became.
const prereleased = `# Changelog

## [0.2.0] - 2026-02-01

### Added

- The second thing.

## [0.2.0-rc.1] - 2026-01-20

### Added

- The candidate.

## [0.1.0] - 2026-01-01

### Added

- The first thing.
`

func TestPrereleaseEntryModifiedIsSeparatelyConfigurable(t *testing.T) {
	doc := strings.Replace(prereleased, "- The candidate.", "- The candidate, folded in.", 1)

	sev := changelog.DefaultSeverities()
	if err := sev.Set(changelog.CheckPrereleaseEntryModified, changelog.Off); err != nil {
		t.Fatal(err)
	}
	got := checks(changelog.Parse([]byte(doc)).Lint(changelog.Options{
		Severities: sev,
		Git:        &changelog.Git{ReferenceTag: "v0.2.0", TaggedChangelog: []byte(prereleased)},
	}))
	if len(got) != 0 {
		t.Errorf("checks fired %v with the pre-release check off, want none", got)
	}
}

// Two causes fire this check and neither remedy answers the other, so one
// message carries both and the cause it interpolates says which is this run's.
func TestNoGitTagsNamesTheFix(t *testing.T) {
	for _, cause := range []string{
		"the checkout is shallow and carries no tags",
		".git names the git directory /elsewhere/.git/worktrees/x, which cannot be read",
	} {
		got := changelog.Parse([]byte(tagged)).Lint(changelog.Options{
			Git: &changelog.Git{Err: errors.New(cause)},
		})
		if len(got) != 1 {
			t.Fatalf("findings are %v, want one", got)
		}
		if !strings.Contains(got[0].Msg, cause) {
			t.Errorf("message %q does not name the cause %q", got[0].Msg, cause)
		}
		if !strings.Contains(got[0].Msg, "fetch-depth: 0") {
			t.Errorf("message %q does not name fetch-depth: 0", got[0].Msg)
		}
		if !strings.Contains(got[0].Msg, "cannot see") {
			t.Errorf("message %q does not name the unreachable git directory", got[0].Msg)
		}
	}
}

func checks(findings []changelog.Finding) []string {
	out := make([]string, 0, len(findings))
	for _, f := range findings {
		out = append(out, f.Check)
	}
	return out
}
