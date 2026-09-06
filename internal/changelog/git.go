package changelog

import (
	"github.com/mikluko/action-changelog/internal/semver"
)

// Git is what the tag-dependent checks read about the repository the document
// lives in. Reading it is the caller's job; this package compares.
//
// A nil Options.Git runs none of those checks. A caller that offered no
// repository is not the same as a repository whose tags cannot be read, and
// only the second is a finding.
type Git struct {
	// Err is why the tag history could not be read, where it could not. It is
	// the whole of what no-git-tags fires on: a history read to the end that
	// holds no tag is a repository before its first release, and reporting
	// that as a defect would fail every such repository until it made one.
	Err error
	// ReferenceTag is the tag every check below compares against, empty where
	// the checkout reaches no eligible version tag. Which tag that is belongs
	// to the caller: this package compares against whatever it is handed.
	ReferenceTag string
	// TaggedChangelog is the changelog as it stood at ReferenceTag, nil where
	// that tag carries no such file.
	TaggedChangelog []byte
	// Tags is every tag the repository carries, spelled as it writes them.
	// It is the whole set rather than ReferenceTag alone, because a release is
	// tagged whether or not the tag naming it is reachable from HEAD, which is
	// the same set already-tagged is answered from.
	Tags []string
	// TagDays is the day each tag was cut, as YYYY-MM-DD, keyed by the name
	// Tags spells. Which date a tag carries and which timezone reads it are
	// settled by whoever fills this in; date-mismatch compares two calendar
	// days and knows nothing of either. A tag absent from the map has no day
	// that could be read, which is not evidence that a date disagrees.
	TagDays map[string]string
}

// day returns the day the named tag was cut, and whether it is known.
func (g *Git) day(name string) (string, bool) {
	if g == nil || g.Err != nil {
		return "", false
	}
	d, ok := g.TagDays[name]
	return d, ok
}

// tag returns the tag naming version, as the repository spells it, or "" where
// no tag does. Tags are compared by the version they name, so a repository
// tagging 1.2.3 answers for a heading reading 1.2.3 as one tagging v1.2.3 would.
//
// A nil Git offered no repository and a populated Err is a history that could
// not be read. Neither is evidence that a tag exists, so both answer "": a run
// that could not read the tags reports no-git-tags by name and accuses nobody of
// having shipped a release it left undated.
func (g *Git) tag(e Entry) string {
	if g == nil || g.Err != nil || e.Version == "" {
		return ""
	}
	// The fullest spelling wins. A repository keeping a moving "v1" beside the
	// "v1.0.0" it points at has two tags naming one version, and the release
	// was cut at the fuller one: the shorter moves to the next release, so its
	// date is when it last moved rather than when anything shipped.
	best, spelt := "", 0
	for _, name := range g.Tags {
		// Compare rather than string equality, so build metadata is ignored on
		// both sides the way section 10 says it is: a tag cannot carry a "+" at
		// all, and an entry naming one still names the version the tag names.
		t, err := semver.ParseTag(name)
		if err != nil || semver.Compare(t, e.Semver) != 0 {
			continue
		}
		if n := semver.TagComponents(name); n > spelt {
			best, spelt = name, n
		}
	}
	return best
}

// git runs the checks that read the repository rather than the document.
//
// A history that could not be read stops the rest: version-behind-tag and the
// immutability pair have nothing to compare against, so reporting the missing
// history once by its own name is the whole of what can be said. A history read
// to the end that names no version stops them just as quietly, and says nothing.
func (f *findings) git(c *Changelog, g *Git) {
	if g == nil {
		return
	}
	if g.Err != nil {
		// Two causes reach here and they want opposite remedies, so the message
		// carries both and the parenthesis says which one this run met: a
		// checkout that fetched no tags is fixed by fetching them, and a .git
		// naming a git directory out of reach is fixed by putting it in reach.
		f.add(CheckNoGitTags, 0,
			"the tag history cannot be read (%v); check out with fetch-depth: 0 where the checkout is shallow, "+
				"and bring the git directory into reach where .git names one this run cannot see",
			g.Err)
		return
	}
	if g.ReferenceTag == "" {
		return
	}

	reference, refErr := semver.ParseTag(g.ReferenceTag)
	if latest, ok := c.Latest(); ok && refErr == nil && semver.Compare(latest.Semver, reference) < 0 {
		f.add(CheckVersionBehindTag, latest.Line,
			"the newest entry is %s, which is behind the reference tag %s; an entry level with the tag is a release just made and one above it is a release pending",
			latest.Version[1:], g.ReferenceTag)
	}
	f.immutable(c, g)
}

// immutable reports released entries that have changed since the reference tag.
//
// It walks the entries the tagged document carried and looks each up in the
// current one, so an entry added since is not a finding: backfilling history
// nobody recorded is a repair rather than a rewrite. Walking parsed entries
// rather than raw text also leaves the link-reference block at the foot of the
// document exempt, since no entry's body carries it.
func (f *findings) immutable(c *Changelog, g *Git) {
	if len(g.TaggedChangelog) == 0 {
		return
	}
	now := make(map[string]Entry)
	for _, e := range c.Released() {
		now[e.Version] = e
	}
	for _, was := range Parse(g.TaggedChangelog).Released() {
		check := CheckReleaseEntryModified
		if was.Semver.Prerelease() {
			check = CheckPrereleaseEntryModified
		}
		is, ok := now[was.Version]
		switch {
		case !ok:
			f.add(check, 0, "version %s was released at tag %s and is no longer in the file",
				was.Version[1:], g.ReferenceTag)
		case is.Date != was.Date:
			f.add(check, is.Line, "version %s was dated %q at tag %s and is now dated %q",
				was.Version[1:], was.Date, g.ReferenceTag, is.Date)
		case is.Body != was.Body:
			f.add(check, is.Line, "the notes under version %s differ from the ones released at tag %s",
				was.Version[1:], g.ReferenceTag)
		}
	}
}
