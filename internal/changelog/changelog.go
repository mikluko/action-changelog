// Package changelog parses Keep a Changelog documents into their version
// entries and checks that a document conforms to the format the release
// ceremony reads.
//
// Parsing goes through a Markdown parser rather than line matching: a fenced
// code block containing what looks like a heading is content, not structure.
package changelog

import (
	"bytes"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/mikluko/action-changelog/internal/semver"
)

// Section is a level-3 heading under an entry, such as "Added".
type Section struct {
	Name string
	Line int
}

// Entry is one level-2 heading of a changelog together with everything under
// it, up to the next level-2 heading.
//
// Version is empty for the Unreleased entry and for a heading that does not
// state a parseable version: Unreleased distinguishes the two.
type Entry struct {
	Raw        string
	Version    string
	Date       string
	Unreleased bool
	Sections   []Section
	Body       string
	Line       int
	// LinkRef reports whether the document defines a link reference matching
	// this entry's bracketed heading token, which is what decides whether the
	// heading renders as a live link or as literal "[1.2.3]" text.
	LinkRef bool
	// VersionErr is why the heading states no version, for an entry that is
	// neither Unreleased nor versioned. It is a *semver.SyntaxError naming the
	// rule the heading broke, so a finding can report that rule rather than
	// restating the grammar, and it is nil for every other entry.
	VersionErr error
}

// Changelog is a parsed document. Entries keep document order, so the first
// released entry is the one a release ceremony acts on.
type Changelog struct {
	Title   string
	Entries []Entry
}

// Released returns the entries that name a version, in document order.
func (c *Changelog) Released() []Entry {
	out := make([]Entry, 0, len(c.Entries))
	for _, e := range c.Entries {
		if e.Version != "" {
			out = append(out, e)
		}
	}
	return out
}

// Latest returns the newest entry naming a version, which is the one the
// release ceremony publishes. It reports false for a document holding none,
// and for one whose newest entry names a version that cannot be read: an entry
// under such a heading is not the newest entry, and reporting it as one hands
// the ceremony a release nobody wrote.
func (c *Changelog) Latest() (Entry, bool) {
	for _, e := range c.Entries {
		if e.Unreleased {
			continue
		}
		if e.Version == "" {
			return Entry{}, false
		}
		return e, true
	}
	return Entry{}, false
}

// Find returns the entry for version, which may be given with or without the
// leading "v".
func (c *Changelog) Find(version string) (Entry, bool) {
	want := canonical(version)
	for _, e := range c.Entries {
		if e.Version != "" && e.Version == want {
			return e, true
		}
	}
	return Entry{}, false
}

// Parse reads a Keep a Changelog document. Malformed headings become entries
// with an empty Version and a populated Raw, so Lint can report them by line
// rather than the parse failing as a whole.
func Parse(src []byte) *Changelog {
	root := goldmark.New().Parser().Parse(text.NewReader(src))

	var (
		out      Changelog
		headings []*ast.Heading
	)
	for n := root.FirstChild(); n != nil; n = n.NextSibling() {
		h, ok := n.(*ast.Heading)
		if !ok {
			continue
		}
		switch h.Level {
		case 1:
			if out.Title == "" {
				out.Title = headingText(src, h)
			}
		case 2:
			headings = append(headings, h)
		}
	}

	defined := linkRefs(root)
	for i, h := range headings {
		var next ast.Node
		if i+1 < len(headings) {
			next = headings[i+1]
		}
		e := Entry{
			Raw:  headingText(src, h),
			Line: lineOf(src, headingStart(src, h)),
		}
		e.Version, e.Date, e.Unreleased, e.VersionErr = parseHeading(e.Raw)
		e.Sections, e.Body = collect(src, h, next)
		if label, bracketed := headingLabel(e.Raw); bracketed {
			e.LinkRef = defined[strings.ToLower(label)]
		}
		out.Entries = append(out.Entries, e)
	}
	return &out
}

// linkRefs returns the labels the document defines a link reference for, folded
// to lower case because a Markdown reference label matches case-insensitively.
func linkRefs(root ast.Node) map[string]bool {
	out := map[string]bool{}
	_ = ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if d, ok := n.(*ast.LinkReferenceDefinition); ok {
			out[strings.ToLower(string(d.Label))] = true
		}
		return ast.WalkContinue, nil
	})
	return out
}

// headingLabel returns the text between the brackets of an entry heading, and
// whether the heading was bracketed at all. An unbracketed heading names no
// reference label, so nothing can define one for it.
func headingLabel(raw string) (label string, bracketed bool) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "[") {
		return "", false
	}
	i := strings.IndexByte(raw, ']')
	if i < 0 {
		return "", false
	}
	return raw[1:i], true
}

// collect walks the nodes between an entry's heading and the next one,
// returning its level-3 sections and the source they span.
//
// The span stops at the last node rather than at the next heading, and link
// reference definitions are skipped outright: the "[1.2.3]: …compare…" block at
// the foot of a Keep a Changelog document belongs to the document, not to the
// release notes of whichever entry happens to be last.
func collect(src []byte, from, to ast.Node) ([]Section, string) {
	var (
		sections   []Section
		start, end = -1, -1
	)
	for n := from.NextSibling(); n != nil && n != to; n = n.NextSibling() {
		if n.Kind() == ast.KindLinkReferenceDefinition {
			continue
		}
		if h, ok := n.(*ast.Heading); ok && h.Level == 3 {
			sections = append(sections, Section{
				Name: headingText(src, h),
				Line: lineOf(src, headingStart(src, h)),
			})
		}
		s, e, ok := span(n)
		if !ok {
			continue
		}
		if start < 0 || s < start {
			start = s
		}
		if e > end {
			end = e
		}
	}
	if start < 0 {
		return sections, ""
	}
	// A heading's span excludes the "### " that introduces it, so the body is
	// taken from the start of that line rather than from the span itself.
	return sections, strings.TrimSpace(string(src[lineStart(src, start):end]))
}

// span reports the source range a node covers, recursing into children for the
// container nodes whose own Lines() are empty.
//
// Inline nodes are skipped: Lines panics on them, and the block that holds
// them already spans their source.
func span(n ast.Node) (start, end int, ok bool) {
	if n.Type() == ast.TypeInline {
		return 0, 0, false
	}
	if l := n.Lines(); l.Len() > 0 {
		start, end = l.At(0).Start, l.At(l.Len()-1).Stop
		ok = true
	}
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		s, e, o := span(c)
		if !o {
			continue
		}
		if !ok || s < start {
			start = s
		}
		if !ok || e > end {
			end = e
		}
		ok = true
	}
	return start, end, ok
}

// headingText returns a heading's line as written, without its "#" markers.
//
// It reads the source rather than the heading's inline children because
// "## [1.2.3]" parses as a link when a matching reference definition sits at
// the foot of the document and as literal text when it does not, and the two
// yield different text.
func headingText(src []byte, h *ast.Heading) string {
	start := headingStart(src, h)
	if start < 0 {
		return ""
	}
	end := lineEnd(src, start)
	line := strings.TrimSpace(string(src[start:end]))
	line = strings.TrimLeft(line, "#")
	return strings.TrimSpace(strings.TrimRight(line, "# \t"))
}

// headingStart returns the offset of the first byte of a heading's line.
func headingStart(src []byte, h *ast.Heading) int {
	if h == nil {
		return -1
	}
	l := h.Lines()
	if l.Len() == 0 {
		// A heading with no inline content still occupies a line, but goldmark
		// records no segment for it. Nothing downstream can place it.
		return -1
	}
	return lineStart(src, l.At(0).Start)
}

func lineStart(src []byte, off int) int {
	if off > len(src) {
		off = len(src)
	}
	if i := bytes.LastIndexByte(src[:off], '\n'); i >= 0 {
		return i + 1
	}
	return 0
}

func lineEnd(src []byte, off int) int {
	if i := bytes.IndexByte(src[off:], '\n'); i >= 0 {
		return off + i
	}
	return len(src)
}

func lineOf(src []byte, off int) int {
	if off < 0 {
		return 0
	}
	if off > len(src) {
		off = len(src)
	}
	return bytes.Count(src[:off], []byte("\n")) + 1
}

// parseHeading reads an entry heading of the form "[1.2.3] - 2026-09-04" or
// "[Unreleased]". Brackets are optional, and so is the date.
//
// The version token is read as Semantic Versioning 2.0.0 with no shorthand and
// no leading "v", so a heading naming "1.2" or "v1.2.3" states no version. The
// error says which rule it broke.
func parseHeading(raw string) (version, date string, unreleased bool, err error) {
	name, rest := splitHeading(raw)
	if strings.EqualFold(name, "unreleased") {
		return "", strings.TrimSpace(rest), true, nil
	}
	v, err := semver.Parse(name)
	if err != nil {
		return "", "", false, err
	}
	return "v" + v.String(), strings.TrimSpace(rest), false, nil
}

// splitHeading separates the version token from whatever follows it: the text
// inside brackets when they are present, otherwise the first field.
func splitHeading(raw string) (name, rest string) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "[") {
		if i := strings.IndexByte(raw, ']'); i > 0 {
			return raw[1:i], strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw[i+1:]), "-"))
		}
		return "", ""
	}
	name, after, found := strings.Cut(raw, " ")
	if !found {
		return raw, ""
	}
	return name, strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(after), "-"))
}

// canonical adds the "v" that golang.org/x/mod/semver requires.
func canonical(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if v[0] != 'v' && v[0] != 'V' {
		return "v" + v
	}
	return "v" + v[1:]
}
