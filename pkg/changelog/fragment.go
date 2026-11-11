package changelog

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
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
	var (
		out     Fragment
		section *FragmentSection
	)
	closeSection := func() error {
		if section != nil && len(section.Notes) == 0 {
			return &FragmentError{Line: section.Line, Msg: fmt.Sprintf("section %q lists no notes", section.Name)}
		}
		return nil
	}
	for n := root.FirstChild(); n != nil; n = n.NextSibling() {
		line := nodeLine(src, n)
		switch n := n.(type) {
		case *ast.Heading:
			if n.Level != 3 {
				return Fragment{}, &FragmentError{Line: line, Msg: fmt.Sprintf("a level-%d heading; a fragment carries level-3 section headings only", n.Level)}
			}
			if err := closeSection(); err != nil {
				return Fragment{}, err
			}
			name := headingText(src, n)
			if name == "" {
				return Fragment{}, &FragmentError{Line: line, Msg: "a section heading names no section"}
			}
			out.Sections = append(out.Sections, FragmentSection{Name: name, Line: line})
			section = &out.Sections[len(out.Sections)-1]
		case *ast.List:
			if n.IsOrdered() {
				return Fragment{}, &FragmentError{Line: line, Msg: "an ordered list; notes are a bullet list"}
			}
			if section == nil {
				return Fragment{}, &FragmentError{Line: line, Msg: "notes under no section; put a level-3 heading such as ### Fixed above them"}
			}
			at := line
			for li := n.FirstChild(); li != nil; li = li.NextSibling() {
				note, ok := itemText(src, li)
				if !ok {
					return Fragment{}, &FragmentError{Line: at, Msg: "an empty note"}
				}
				section.Notes = append(section.Notes, note)
				_, end, _ := span(li)
				at = lineOf(src, end) + 1
			}
		default:
			return Fragment{}, &FragmentError{Line: line, Msg: fmt.Sprintf("a %s; a fragment carries level-3 section headings and the notes listed under them only", kindName(n))}
		}
	}
	if err := closeSection(); err != nil {
		return Fragment{}, err
	}
	if len(out.Sections) == 0 {
		return Fragment{}, &FragmentError{Line: 1, Msg: "no notes; a fragment carries at least one level-3 section heading and a note listed under it"}
	}
	return out, nil
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
	out := slices.Clone(src)
	for _, frag := range frags {
		for _, s := range frag.Sections {
			for _, note := range s.Notes {
				var err error
				if out, err = Append(out, s.Name, note); err != nil {
					return nil, err
				}
			}
		}
	}
	return out, nil
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
