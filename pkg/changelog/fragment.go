package changelog

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"sarak.sh/mikluko/changelog/pkg/semver"
)

// Fragment is a file of notes waiting to be folded into the [Unreleased] entry:
// level-3 section headings, each followed by a list of notes, as in
//
//	### Fixed
//
//	- The parser no longer reads a fenced heading as an entry.
type Fragment struct {
	Sections []FragmentSection
}

// FragmentSection is one level-3 heading of a fragment and the notes listed
// under it, each without its list marker.
type FragmentSection struct {
	Name  string
	Line  int
	Notes []string
}

// FragmentError is why a file is not a fragment, at the line that shows it.
type FragmentError struct {
	Line int
	Msg  string
}

func (e *FragmentError) Error() string { return fmt.Sprintf("line %d: %s", e.Line, e.Msg) }

// ParseFragment reads src as a fragment. Anything but level-3 headings and the
// bullet lists under them is a *FragmentError, as are a list under no heading,
// a heading with no notes and a document with neither, because folding would
// drop it or move it under a section it was not written for.
func ParseFragment(src []byte) (Fragment, error) {
	root := goldmark.New().Parser().Parse(text.NewReader(src))
	var out Fragment
	for n := root.FirstChild(); n != nil; n = n.NextSibling() {
		var err error
		switch n := n.(type) {
		case *ast.Heading:
			err = out.heading(src, n)
		case *ast.List:
			err = out.list(src, n)
		default:
			err = &FragmentError{Line: nodeLine(src, n), Msg: fmt.Sprintf("a %s; a fragment carries level-3 section headings and the notes listed under them only", kindName(n))}
		}
		if err != nil {
			return Fragment{}, err
		}
	}
	if err := out.closeLast(); err != nil {
		return Fragment{}, err
	}
	if len(out.Sections) == 0 {
		return Fragment{}, &FragmentError{Line: 1, Msg: "no notes; a fragment carries at least one level-3 section heading and a note listed under it"}
	}
	return out, nil
}

// heading opens the section h names, refusing any other level and a section
// above it that lists nothing.
func (f *Fragment) heading(src []byte, h *ast.Heading) error {
	line := nodeLine(src, h)
	if h.Level != 3 {
		return &FragmentError{Line: line, Msg: fmt.Sprintf("a level-%d heading; a fragment carries level-3 section headings only", h.Level)}
	}
	if err := f.closeLast(); err != nil {
		return err
	}
	name := headingText(src, h)
	if name == "" {
		return &FragmentError{Line: line, Msg: "a section heading names no section"}
	}
	f.Sections = append(f.Sections, FragmentSection{Name: name, Line: line})
	return nil
}

// list adds the items of l as notes of the last section.
func (f *Fragment) list(src []byte, l *ast.List) error {
	line := nodeLine(src, l)
	if l.IsOrdered() {
		return &FragmentError{Line: line, Msg: "an ordered list; notes are a bullet list"}
	}
	if len(f.Sections) == 0 {
		return &FragmentError{Line: line, Msg: "notes under no section; put a level-3 heading such as ### Fixed above them"}
	}
	section := &f.Sections[len(f.Sections)-1]
	at := line
	for li := l.FirstChild(); li != nil; li = li.NextSibling() {
		note, ok := itemText(src, li)
		if !ok {
			return &FragmentError{Line: at, Msg: "an empty note"}
		}
		section.Notes = append(section.Notes, note)
		_, end, _ := span(li)
		at = lineOf(src, end) + 1
	}
	return nil
}

// closeLast refuses a last section that lists no notes.
func (f *Fragment) closeLast() error {
	if n := len(f.Sections); n > 0 && len(f.Sections[n-1].Notes) == 0 {
		s := f.Sections[n-1]
		return &FragmentError{Line: s.Line, Msg: fmt.Sprintf("section %q lists no notes", s.Name)}
	}
	return nil
}

// LintFragment reports what keeps src from folding cleanly: a fragment-form
// finding where it is not a fragment, and otherwise an unknown-section finding
// for each heading outside the accepted vocabulary. Only those two checks run.
func LintFragment(src []byte, opts Options) []Finding {
	severities := opts.Severities
	if severities == nil {
		severities = DefaultSeverities()
	}
	f := findings{severities: severities}
	frag, err := ParseFragment(src)
	var fe *FragmentError
	if errors.As(err, &fe) {
		f.add(CheckFragmentForm, fe.Line, "%s", fe.Msg)
		return f.out
	}
	sections := opts.Sections
	if len(sections) == 0 {
		sections = DefaultSections
	}
	allowed := make(map[string]bool, len(sections))
	for _, s := range sections {
		allowed[s] = true
	}
	for _, s := range frag.Sections {
		if !allowed[s.Name] {
			f.add(CheckUnknownSection, s.Line, "section %q is not one of %s", s.Name, strings.Join(sections, ", "))
		}
	}
	return f.out
}

// Fold appends every note of frags under its section of the [Unreleased]
// entry, in order, exactly as Append would one at a time, and returns the
// edited document; src is not modified.
func Fold(src []byte, frags ...Fragment) ([]byte, error) {
	return foldWith(Append, src, frags)
}

// FoldOpen appends every note of frags to the open entry as Fold does to
// [Unreleased]. It is an error for the newest versioned entry not to be open.
func FoldOpen(src []byte, frags ...Fragment) ([]byte, error) {
	if _, err := locate(src).open(); err != nil {
		return nil, err
	}
	return foldWith(AppendOpen, src, frags)
}

func foldWith(add func([]byte, string, string) ([]byte, error), src []byte, frags []Fragment) ([]byte, error) {
	out := slices.Clone(src)
	for _, frag := range frags {
		for _, s := range frag.Sections {
			for _, note := range s.Notes {
				var err error
				if out, err = add(out, s.Name, note); err != nil {
					return nil, err
				}
			}
		}
	}
	return out, nil
}

// ReleasePending releases what the document has pending, dated date, with frags
// folded in: the open entry where the newest versioned entry is one, closed as
// Close does, and otherwise the [Unreleased] entry as ReleaseFragments does at
// the version its notes call for. It returns the edited document and the
// version released.
func ReleasePending(src []byte, frags []Fragment, date string) ([]byte, semver.Version, error) {
	open, ok := Parse(src).Latest()
	if !ok || open.Date != "" {
		return ReleaseFragments(src, frags, nil, date)
	}
	folded, err := FoldOpen(src, frags...)
	if err != nil {
		return nil, semver.Version{}, err
	}
	out, err := Close(folded, date)
	if err != nil {
		return nil, semver.Version{}, err
	}
	return out, open.Semver, nil
}

// itemText returns a list item's source without its marker, continuation lines
// dedented by the marker's width, which is the note Append re-indents under a
// marker of its own.
func itemText(src []byte, li ast.Node) (string, bool) {
	start, end, ok := span(li)
	if !ok {
		return "", false
	}
	indent := start - lineStart(src, start)
	lines := strings.Split(string(src[start:lineEnd(src, end)]), "\n")
	for i := 1; i < len(lines); i++ {
		l := lines[i]
		n := 0
		for n < indent && n < len(l) && l[n] == ' ' {
			n++
		}
		lines[i] = l[n:]
	}
	note := strings.TrimSpace(strings.Join(lines, "\n"))
	return note, note != ""
}

// nodeLine is the line a block node starts on.
func nodeLine(src []byte, n ast.Node) int {
	if h, ok := n.(*ast.Heading); ok {
		if s := headingStart(src, h); s >= 0 {
			return lineOf(src, s)
		}
	}
	if s, _, ok := span(n); ok {
		return lineOf(src, s)
	}
	return 1
}

func kindName(n ast.Node) string {
	switch n.Kind() {
	case ast.KindParagraph:
		return "paragraph"
	case ast.KindFencedCodeBlock, ast.KindCodeBlock:
		return "code block"
	case ast.KindThematicBreak:
		return "thematic break"
	case ast.KindHTMLBlock:
		return "HTML block"
	case ast.KindBlockquote:
		return "blockquote"
	case ast.KindLinkReferenceDefinition:
		return "link reference definition"
	}
	return strings.ToLower(n.Kind().String())
}

// ReleaseFragments folds frags into the [Unreleased] entry and releases it as
// Release does, at version or, where version is nil, at the version Next calls
// for. It returns the edited document and the version released; src is not
// modified.
func ReleaseFragments(src []byte, frags []Fragment, version *semver.Version, date string) ([]byte, semver.Version, error) {
	folded, err := Fold(src, frags...)
	if err != nil {
		return nil, semver.Version{}, err
	}
	var v semver.Version
	if version != nil {
		v = *version
	} else if v, err = Parse(folded).Next(); err != nil {
		return nil, semver.Version{}, err
	}
	out, err := Release(folded, v, date)
	if err != nil {
		return nil, semver.Version{}, err
	}
	return out, v, nil
}

// Notes returns every list item of the entry's top-level lists, without its
// marker, in document order, as a fragment's notes are read.
func (e Entry) Notes() []string {
	src := []byte(e.Body)
	root := goldmark.New().Parser().Parse(text.NewReader(src))
	var out []string
	for n := root.FirstChild(); n != nil; n = n.NextSibling() {
		if _, ok := n.(*ast.List); !ok {
			continue
		}
		for li := n.FirstChild(); li != nil; li = li.NextSibling() {
			if note, ok := itemText(src, li); ok {
				out = append(out, note)
			}
		}
	}
	return out
}
