package changelog

import (
	"golang.org/x/mod/semver"
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

	if latest, ok := c.Latest(); ok && semver.Compare(latest.Version, canonical(g.ReferenceTag)) < 0 {
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
		if semver.Prerelease(was.Version) != "" {
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
