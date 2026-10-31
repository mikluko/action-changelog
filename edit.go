package changelog

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"sarak.sh/mikluko/changelog/semver"
)

// layout is where the parts of a document sit in its source, by byte offset.
type layout struct {
	entries []entrySpan
	refs    []refSpan
}

// entrySpan is one level-2 heading and the content under it.
type entrySpan struct {
	raw string
	// head is the first byte of the heading line, headEnd the newline ending it.
	head, headEnd int
	// end is the newline ending the entry's last content line, or headEnd for
	// an entry with none. Link reference definitions are not content.
	end      int
	sections []sectionSpan
}

// sectionSpan is one level-3 heading under an entry and the content under it,
// with offsets as in entrySpan.
type sectionSpan struct {
	name          string
	head, headEnd int
	end           int
}

// refSpan is one link reference definition: the label as written, and its line.
type refSpan struct {
	label, dest string
	start, end  int
}

func locate(src []byte) layout {
	root := goldmark.New().Parser().Parse(text.NewReader(src))
	var (
		out     layout
		entry   *entrySpan
		section *sectionSpan
	)
	for n := root.FirstChild(); n != nil; n = n.NextSibling() {
		if d, ok := n.(*ast.LinkReferenceDefinition); ok {
			if s, e, ok := span(d); ok {
				out.refs = append(out.refs, refSpan{
					label: string(d.Label),
					dest:  string(d.Destination),
					start: lineStart(src, s),
					end:   lineEnd(src, e),
				})
			}
			continue
		}
		if h, ok := n.(*ast.Heading); ok && h.Level <= 2 {
			section = nil
			entry = nil
			if h.Level == 2 {
				if start := headingStart(src, h); start >= 0 {
					end := lineEnd(src, start)
					out.entries = append(out.entries, entrySpan{raw: headingText(src, h), head: start, headEnd: end, end: end})
					entry = &out.entries[len(out.entries)-1]
				}
			}
			continue
		}
		if entry == nil {
			continue
		}
		if h, ok := n.(*ast.Heading); ok && h.Level == 3 {
			if start := headingStart(src, h); start >= 0 {
				end := lineEnd(src, start)
				entry.sections = append(entry.sections, sectionSpan{name: headingText(src, h), head: start, headEnd: end, end: end})
				section = &entry.sections[len(entry.sections)-1]
				entry.end = end
			}
			continue
		}
		_, e, ok := span(n)
		if !ok {
			continue
		}
		e = lineEnd(src, e)
		entry.end = max(entry.end, e)
		if section != nil {
			section.end = max(section.end, e)
		}
	}
	return out
}

// unreleased returns the index of the [Unreleased] entry, or -1.
func (l layout) unreleased() int {
	for i, e := range l.entries {
		name, _ := splitHeading(e.raw)
		if strings.EqualFold(name, "unreleased") {
			return i
		}
	}
	return -1
}

func splice(src []byte, at int, insert string) []byte {
	out := make([]byte, 0, len(src)+len(insert))
	out = append(out, src[:at]...)
	out = append(out, insert...)
	return append(out, src[at:]...)
}

func replace(src []byte, from, to int, with string) []byte {
	out := make([]byte, 0, len(src)-(to-from)+len(with))
	out = append(out, src[:from]...)
	out = append(out, with...)
	return append(out, src[to:]...)
}

// item writes note as a list item, indenting continuation lines under the
// marker so a multi-line note stays one item.
func item(note string) string {
	lines := strings.Split(strings.TrimSpace(note), "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
		if i > 0 && lines[i] != "" {
			lines[i] = "  " + lines[i]
		}
	}
	return "- " + strings.Join(lines, "\n")
}

// rank is a section's place in the Keep a Changelog order, with every other
// name after all six.
func rank(name string) int {
	if i := slices.IndexFunc(DefaultSections, func(s string) bool { return strings.EqualFold(s, name) }); i >= 0 {
		return i
	}
	return len(DefaultSections)
}

// Append adds note as a list item at the end of section under the [Unreleased]
// entry and returns the edited document; src is not modified.
//
// Note is the item's text without its list marker. Where the entry has no such
// section one is created, ordered among the Keep a Changelog six; where the
// document has no [Unreleased] entry one is created above the newest entry.
// A section already carrying the item is left as it is, so a rerun adds
// nothing. Every byte outside the lines written is preserved.
func Append(src []byte, section, note string) ([]byte, error) {
	section = strings.TrimSpace(section)
	if section == "" || strings.ContainsAny(section, "\r\n") {
		return nil, fmt.Errorf("section %q is not a heading", section)
	}
	if strings.TrimSpace(note) == "" {
		return nil, errors.New("the note is empty")
	}
	li := item(note)

	l := locate(src)
	u := l.unreleased()
	if u < 0 {
		block := "## [Unreleased]\n\n### " + section + "\n\n" + li + "\n"
		if len(l.entries) > 0 {
			return splice(src, l.entries[0].head, block+"\n"), nil
		}
		return appendBlock(src, l, block), nil
	}

	e := l.entries[u]
	for _, s := range e.sections {
		if strings.EqualFold(s.name, section) {
			if has(src[s.headEnd:s.end], li) {
				return slices.Clone(src), nil
			}
			if s.end == s.headEnd {
				return splice(src, s.headEnd, "\n\n"+li), nil
			}
			return splice(src, s.end, "\n"+li), nil
		}
	}
	want := rank(section)
	for _, s := range e.sections {
		if rank(s.name) > want {
			return splice(src, s.head, "### "+section+"\n\n"+li+"\n\n"), nil
		}
	}
	return splice(src, e.end, "\n\n### "+section+"\n\n"+li), nil
}

// has reports whether body carries item as whole lines.
func has(body []byte, item string) bool {
	s := string(body) + "\n"
	return strings.Contains(s, "\n"+item+"\n")
}

// appendBlock places block after the document's last content and before its
// link reference definitions, separated by a blank line.
func appendBlock(src []byte, l layout, block string) []byte {
	if len(l.refs) > 0 {
		return splice(src, l.refs[0].start, block+"\n")
	}
	trimmed := strings.TrimRight(string(src), "\n")
	if trimmed == "" {
		return []byte(block)
	}
	return append([]byte(trimmed+"\n\n"), block...)
}

// Release turns the [Unreleased] entry into one naming version, dated date
// unless date is empty, which leaves it an open entry, and puts an empty
// [Unreleased] entry above it. It returns the edited document; src is not
// modified.
//
// It refuses an [Unreleased] entry with no notes, a version not above every
// version the document names, and a document whose newest versioned entry is
// still open. Where the [Unreleased] link reference is a compare URL ending
// in HEAD, it is moved to start at the new tag and a definition comparing the
// previous tag with the new one is added below it, spelling the tag with a
// leading "v" exactly when the previous one has one.
func Release(src []byte, version semver.Version, date string) ([]byte, error) {
	if date != "" && !isDate(date) {
		return nil, fmt.Errorf("date %q is not YYYY-MM-DD", date)
	}
	doc := Parse(src)
	var notes string
	found := false
	for _, e := range doc.Entries {
		if e.Unreleased {
			notes, found = e.Body, true
			continue
		}
		if e.Version == "" {
			continue
		}
		if e.Date == "" {
			return nil, fmt.Errorf("the newest entry, %s, is open; date it before releasing [Unreleased]", e.Semver)
		}
		break
	}
	if !found {
		return nil, errors.New("the changelog has no [Unreleased] entry")
	}
	if notes == "" {
		return nil, errors.New("the [Unreleased] entry has no notes to release")
	}
	for _, e := range doc.Entries {
		if e.Version != "" && semver.Compare(version, e.Semver) <= 0 {
			return nil, fmt.Errorf("%s is not above %s, which the changelog already names", version, e.Semver)
		}
	}

	l := locate(src)
	u := l.entries[l.unreleased()]
	_, bracketed := headingLabel(u.raw)
	heading := version.String()
	if bracketed {
		heading = "[" + heading + "]"
	}
	if date != "" {
		heading += " - " + date
	}
	// Refs sit below every entry, so they are rewritten first and the heading
	// offsets stay valid.
	out, err := releaseRefs(src, l.refs, version)
	if err != nil {
		return nil, err
	}
	unreleased := "## Unreleased"
	if bracketed {
		unreleased = "## [Unreleased]"
	}
	return replace(out, u.head, u.headEnd, unreleased+"\n\n## "+heading), nil
}

// releaseRefs moves the [Unreleased] compare link to start at version's tag and
// adds the definition for version below it. A document defining no link
// references is returned as it is.
func releaseRefs(src []byte, refs []refSpan, version semver.Version) ([]byte, error) {
	if len(refs) == 0 {
		return src, nil
	}
	i := slices.IndexFunc(refs, func(r refSpan) bool { return strings.EqualFold(r.label, "unreleased") })
	if i < 0 {
		return nil, errors.New("the changelog defines link references but none for [Unreleased], so none can be written for the release")
	}
	r := refs[i]
	base, prev, ok := compareHEAD(r.dest)
	if !ok {
		return nil, fmt.Errorf("the [Unreleased] link %q is not a compare URL ending in ...HEAD", r.dest)
	}
	tag := version.String()
	if strings.HasPrefix(prev, "v") {
		tag = "v" + tag
	}
	lines := "[" + r.label + "]: " + base + tag + "...HEAD\n" +
		"[" + version.String() + "]: " + base + prev + "..." + tag
	return replace(src, r.start, r.end, lines), nil
}

// compareHEAD splits a ".../compare/<prev>...HEAD" URL into everything up to
// and including "/compare/", and prev.
func compareHEAD(dest string) (base, prev string, ok bool) {
	i := strings.LastIndex(dest, "/compare/")
	if i < 0 || !strings.HasSuffix(dest, "...HEAD") {
		return "", "", false
	}
	base, rest := dest[:i+len("/compare/")], strings.TrimSuffix(dest[i+len("/compare/"):], "...HEAD")
	if rest == "" || strings.Contains(rest, "...") {
		return "", "", false
	}
	return base, rest, true
}

// Next returns the version releasing the [Unreleased] entry calls for, bumped
// from the newest entry naming a final version: major where a note is marked
// breaking, minor where any section is other than Fixed or Security, patch
// otherwise. Below 1.0.0 a breaking change bumps the minor version.
//
// A note is marked breaking by a "Breaking" section or by strong emphasis
// opening with "Breaking", as in "**Breaking:**". It is an error for the
// document to have no [Unreleased] notes or no final version to bump from.
func (c *Changelog) Next() (semver.Version, error) {
	var (
		notes string
		found bool
		base  semver.Version
		has   bool
	)
	var sections []Section
	for _, e := range c.Entries {
		if e.Unreleased && !found {
			notes, sections, found = e.Body, e.Sections, true
			continue
		}
		if e.Version != "" && !e.Semver.Prerelease() && !has {
			base, has = e.Semver, true
		}
	}
	if !found || notes == "" {
		return semver.Version{}, errors.New("the [Unreleased] entry has no notes to release")
	}
	if !has {
		return semver.Version{}, errors.New("the changelog names no final version to bump from")
	}

	next := semver.Version{Major: base.Major, Minor: base.Minor, Patch: base.Patch}
	switch {
	case breaking(notes, sections) && base.Major > 0:
		next.Major, next.Minor, next.Patch = base.Major+1, 0, 0
	case breaking(notes, sections) || slices.ContainsFunc(sections, func(s Section) bool {
		return !strings.EqualFold(s.Name, "Fixed") && !strings.EqualFold(s.Name, "Security")
	}):
		next.Minor, next.Patch = base.Minor+1, 0
	default:
		next.Patch = base.Patch + 1
	}
	return next, nil
}

func breaking(body string, sections []Section) bool {
	if slices.ContainsFunc(sections, func(s Section) bool { return strings.EqualFold(s.Name, "Breaking") }) {
		return true
	}
	src := []byte(body)
	root := goldmark.New().Parser().Parse(text.NewReader(src))
	found := false
	_ = ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if em, ok := n.(*ast.Emphasis); ok && em.Level == 2 {
			if strings.HasPrefix(strings.ToLower(inlineText(src, em)), "breaking") {
				found = true
				return ast.WalkStop, nil
			}
		}
		return ast.WalkContinue, nil
	})
	return found
}

func inlineText(src []byte, n ast.Node) string {
	var b strings.Builder
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			b.Write(t.Segment.Value(src))
			continue
		}
		b.WriteString(inlineText(src, c))
	}
	return b.String()
}
