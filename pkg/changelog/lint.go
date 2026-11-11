package changelog

import (
	"errors"
	"fmt"
	"time"

	"sarak.sh/mikluko/changelog/v2/pkg/semver"
)

// Finding is one departure from the format, addressed to the line that carries
// it so a caller can annotate a diff, and naming the check that raised it so a
// reader knows which one to reconfigure.
type Finding struct {
	Check    string
	Severity Severity
	Line     int
	Msg      string
}

func (f Finding) String() string {
	return fmt.Sprintf("%d: %s: %s (%s)", f.Line, f.Severity, f.Msg, f.Check)
}

// versionReason says why an entry's heading states no version, in the words of
// the rule it broke. It falls back to the general shape for an entry carrying
// no error, which Parse does not produce and a caller building an Entry by hand
// can.
func versionReason(e Entry) string {
	var se *semver.SyntaxError
	if errors.As(e.VersionErr, &se) {
		if se.Part == "" {
			return se.Note
		}
		return fmt.Sprintf("%s (%q)", se.Note, se.Part)
	}
	return "a version states all three of major, minor and patch"
}

// DefaultSections is the vocabulary Keep a Changelog mandates, unchanged between
// 1.1.0 and 2.0.0.
//
// Breaking is deliberately absent: 2.0.0 marks a breaking change inline as
// "**Breaking:**" inside the section it belongs to, which keeps the grouping a
// dedicated section would discard. Repositories that already use it as a heading
// pass it to Lint rather than the default.
var DefaultSections = []string{
	"Added", "Changed", "Deprecated", "Removed", "Fixed", "Security",
}

// Options configures a lint run.
type Options struct {
	// Sections is the accepted set of level-3 headings; DefaultSections is
	// used when it is empty.
	Sections []string
	// Severities is the severity in force per check; DefaultSeverities is
	// used when it is nil. A check at Off raises nothing.
	Severities Severities
	// Git is what the repository says about its tags. Nil runs none of the
	// checks that read it.
	Git *Git
}

// Lint reports every way c departs from the format, the findings about the
// document in document order and those about the repository after them.
//
// An empty result means no check above Off had anything to say. What the file
// alone can be held to is all that is checked unless opts.Git is set: the
// checks that compare it against the repository's tags need the caller to have
// read them.
func (c *Changelog) Lint(opts Options) []Finding {
	l := newLinter(c, opts)
	var prev Entry
	for _, e := range c.Entries {
		switch {
		case e.Unreleased:
		case e.Version == "":
			l.f.add(CheckUnreadableVersion, e.Line,
				"heading %q is neither [Unreleased] nor a version and date, as in [1.2.3] - 2006-01-02: %s", e.Raw, versionReason(e))
		default:
			l.versioned(e, prev)
			prev = e
		}
		l.sections(e)
	}
	l.f.git(c, opts.Git)
	return l.f.out
}

// linter is one Lint run: the findings so far and what every entry is read
// against.
type linter struct {
	f        *findings
	git      *Git
	vocab    []string
	allowed  map[string]bool
	linked   bool
	frozen   map[string]bool
	asOf     string
	newest   int
	newestAt string
}

func newLinter(c *Changelog, opts Options) *linter {
	vocab := opts.Sections
	if len(vocab) == 0 {
		vocab = DefaultSections
	}
	allowed := make(map[string]bool, len(vocab))
	for _, s := range vocab {
		allowed[s] = true
	}
	severities := opts.Severities
	if severities == nil {
		severities = DefaultSeverities()
	}
	l := &linter{
		f:       &findings{severities: severities},
		git:     opts.Git,
		vocab:   vocab,
		allowed: allowed,
		linked:  anyLinkRef(c.Entries),
		frozen:  opts.Git.frozen(),
		asOf:    today(),
		newest:  -1,
	}
	// An entry with no date is one of two things and a final tag naming it
	// tells them apart: a release still being written, or a release that
	// shipped and nobody dated. A candidate tag decides neither, because a
	// candidate ships its tag and stays undated by design, so it leaves the
	// entry open. tag answers "" where the tags could not be read, so the case
	// nothing can decide is reported as the first.
	if latest, ok := c.Latest(); ok {
		l.newest, l.newestAt = latest.Line, opts.Git.tag(latest)
	}
	return l
}

// versioned runs every check on e, an entry naming a version, with prev the
// versioned entry above it.
func (l *linter) versioned(e, prev Entry) {
	l.dated(e)
	l.ordered(e, prev)
	l.taggedDay(e)
	l.carries(e)
}

// dated reports a missing or malformed date, by which of the undated states e
// is in.
func (l *linter) dated(e Entry) {
	v := e.Version[1:]
	switch {
	// The tag is asked about through the entry rather than parsed again: tag
	// matches by precedence, and section 11 counts the pre-release
	// identifiers, so a tag naming this entry carries the same ones it does.
	case e.Date == "" && e.Line == l.newest && l.newestAt != "" && !e.Semver.Prerelease():
		l.f.add(CheckUndatedRelease, e.Line,
			"version %s carries no date and tag %s already names it; the release shipped, so write its date as [%s] - 2006-01-02",
			v, l.newestAt, v)
	case e.Date == "" && e.Line == l.newest:
		l.f.add(CheckUndatedEntry, e.Line,
			"version %s carries no date; write it as [%s] - 2006-01-02, or switch this check off while the release is still accumulating on a branch of its own",
			v, v)
	case e.Date == "":
		l.f.add(CheckHeadingForm, e.Line,
			"version %s carries no date; write it as [%s] - 2006-01-02", v, v)
	case !isDate(e.Date):
		l.f.add(CheckDateFormat, e.Line, "date %q is not YYYY-MM-DD", e.Date)
	}
}

// ordered reports e out of order against prev, and dated later than today.
func (l *linter) ordered(e, prev Entry) {
	if prev.Version != "" && semver.Compare(e.Semver, prev.Semver) >= 0 {
		l.f.add(CheckVersionOrder, e.Line,
			"version %s does not come before %s, which is above it; entries run newest first",
			e.Version[1:], prev.Version[1:])
	}
	if isDate(e.Date) && isDate(prev.Date) && e.Date > prev.Date {
		l.f.add(CheckDateOrder, e.Line,
			"version %s is dated %s, later than %s above it; entries run newest first",
			e.Version[1:], e.Date, prev.Date)
	}
	if isDate(e.Date) && e.Date > l.asOf {
		l.f.add(CheckDateFuture, e.Line,
			"version %s is dated %s, later than today, %s", e.Version[1:], e.Date, l.asOf)
	}
}

// taggedDay reports e dated other than the day the tag naming it was cut. A
// tag names the version, so the entry is history and its date a record rather
// than a prediction; an entry no tag names is a release pending, not a date
// that disagrees.
func (l *linter) taggedDay(e Entry) {
	if !isDate(e.Date) {
		return
	}
	name := l.git.tag(e)
	if name == "" {
		return
	}
	if day, ok := l.git.day(name); ok && day != e.Date {
		l.f.add(CheckDateMismatch, e.Line,
			"version %s is dated %s and tag %s was cut on %s",
			e.Version[1:], e.Date, name, day)
	}
}

// carries reports what e holds or lacks: no notes, no link reference
// definition where others have one, a pre-release version, and one that cannot
// be an OCI tag.
func (l *linter) carries(e Entry) {
	if e.Body == "" {
		l.f.add(CheckEmptyEntry, e.Line, "version %s has no entries under it", e.Version[1:])
	}
	if l.linked && !e.LinkRef {
		l.f.add(CheckPartialLinkRef, e.Line,
			"version %s has no link reference definition, while others do; it renders as literal text",
			e.Version[1:])
	}
	if e.Semver.Prerelease() {
		l.f.add(CheckPrereleaseEntry, e.Line,
			"version %s is a pre-release", e.Version[1:])
	}
	// A fact about the version rather than about the document: it is valid
	// Semantic Versioning and the registry grammar is narrower.
	if reason := ociTagReason(e.Semver.String()); reason != "" {
		l.f.add(CheckOCIIncompatible, e.Line,
			"version %s cannot be an OCI tag: %s", e.Version[1:], reason)
	}
}

// sections reports e's level-3 headings outside the vocabulary, and those
// repeating one above them under an entry the reference tag has not frozen.
func (l *linter) sections(e Entry) {
	seen := make(map[string]bool, len(e.Sections))
	for _, s := range e.Sections {
		if !l.allowed[s.Name] {
			l.f.add(CheckUnknownSection, s.Line, "section %q is not one of %v", s.Name, l.vocab)
		}
		if seen[s.Name] && !l.frozen[e.Version] {
			l.f.add(CheckDuplicateSection, s.Line, "section %q appears more than once under this entry", s.Name)
		}
		seen[s.Name] = true
	}
}

// anyLinkRef reports whether any versioned entry carries a link reference
// definition, which is what turns the partial-link-refs check on: a document
// carrying none is written without them and is not partly done.
func anyLinkRef(entries []Entry) bool {
	for _, e := range entries {
		if e.Version != "" && e.LinkRef {
			return true
		}
	}
	return false
}

// now is the clock today reads. Tests replace it.
var now = time.Now

// today returns the date the date-future check compares an entry against: the
// current date in UTC, whatever zone the author or the runner sits in.
//
// A heading a day out either side of a local midnight is not what the check is
// for; a year typed wrong is.
func today() string { return now().UTC().Format("2006-01-02") }

// isDate reports whether s is a YYYY-MM-DD calendar date.
//
// The check is on shape rather than on validity as an instant: whether the date
// is in the future is date-future's question, and whether it is a real calendar
// day is nobody's.
func isDate(s string) bool {
	if len(s) != 10 || s[4] != '-' || s[7] != '-' {
		return false
	}
	for i, r := range s {
		if i == 4 || i == 7 {
			continue
		}
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
