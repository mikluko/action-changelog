package changelog

import (
	"fmt"
	"time"

	"golang.org/x/mod/semver"
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
	sections := opts.Sections
	if len(sections) == 0 {
		sections = DefaultSections
	}
	allowed := make(map[string]bool, len(sections))
	for _, s := range sections {
		allowed[s] = true
	}

	severities := opts.Severities
	if severities == nil {
		severities = DefaultSeverities()
	}
	f := findings{severities: severities}

	linked := anyLinkRef(c.Entries)
	asOf := today()
	newestLine := -1
	if latest, ok := c.Latest(); ok {
		newestLine = latest.Line
	}

	var prev Entry
	for _, e := range c.Entries {
		switch {
		case e.Unreleased:
		case e.Version == "":
			f.add(CheckHeadingForm, e.Line,
				"heading %q is neither [Unreleased] nor a version and date, as in [1.2.3] - 2006-01-02", e.Raw)
		default:
			switch {
			case e.Date == "" && e.Line == newestLine:
				f.add(CheckUndatedEntry, e.Line,
					"version %s carries no date; write it as [%s] - 2006-01-02, or switch this check off while the release is still accumulating on a branch of its own",
					e.Version[1:], e.Version[1:])
			case e.Date == "":
				f.add(CheckHeadingForm, e.Line,
					"version %s carries no date; write it as [%s] - 2006-01-02", e.Version[1:], e.Version[1:])
			case !isDate(e.Date):
				f.add(CheckDateFormat, e.Line, "date %q is not YYYY-MM-DD", e.Date)
			}
			if prev.Version != "" && semver.Compare(e.Version, prev.Version) >= 0 {
				f.add(CheckVersionOrder, e.Line,
					"version %s does not come before %s, which is above it; entries run newest first",
					e.Version[1:], prev.Version[1:])
			}
			if isDate(e.Date) && isDate(prev.Date) && e.Date > prev.Date {
				f.add(CheckDateOrder, e.Line,
					"version %s is dated %s, later than %s above it; entries run newest first",
					e.Version[1:], e.Date, prev.Date)
			}
			if isDate(e.Date) && e.Date > asOf {
				f.add(CheckDateFuture, e.Line,
					"version %s is dated %s, later than today, %s", e.Version[1:], e.Date, asOf)
			}
			if e.Body == "" {
				f.add(CheckEmptyEntry, e.Line, "version %s has no entries under it", e.Version[1:])
			}
			if linked && !e.LinkRef {
				f.add(CheckPartialLinkRef, e.Line,
					"version %s has no link reference definition, while others do; it renders as literal text",
					e.Version[1:])
			}
			if semver.Prerelease(e.Version) != "" {
				f.add(CheckPrereleaseEntry, e.Line,
					"version %s is a pre-release", e.Version[1:])
			}
			prev = e
		}
		for _, s := range e.Sections {
			if !allowed[s.Name] {
				f.add(CheckUnknownSection, s.Line, "section %q is not one of %v", s.Name, sections)
			}
		}
	}
	f.git(c, opts.Git)
	return f.out
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
