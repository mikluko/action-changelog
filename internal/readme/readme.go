// Package readme generates the README's table of checks from the register in
// package changelog, so the documented set and the implemented set cannot
// disagree.
package readme

import (
	"fmt"
	"strings"

	"github.com/mikluko/action-changelog/internal/changelog"
)

// The markers delimiting the generated region. They are HTML comments, so they
// are invisible wherever the README is rendered.
const (
	Start = "<!-- checks:start -->"
	End   = "<!-- checks:end -->"
)

// Table is the register rendered as a Markdown table.
func Table() string {
	var b strings.Builder
	b.WriteString("| Check | Default | Description |\n")
	b.WriteString("|---|---|---|\n")
	for _, c := range changelog.Checks {
		fmt.Fprintf(&b, "| `%s` | `%s` | %s |\n", c.Name, c.Default, escape(c.Description))
	}
	return b.String()
}

// Render returns src with the region between the markers replaced by Table.
//
// It reports an error when a marker is missing or they are the wrong way round,
// rather than appending a second table to a document that has lost one.
func Render(src []byte) ([]byte, error) {
	s := string(src)
	i := strings.Index(s, Start)
	if i < 0 {
		return nil, fmt.Errorf("no %s marker", Start)
	}
	j := strings.Index(s, End)
	if j < 0 {
		return nil, fmt.Errorf("no %s marker", End)
	}
	if j < i {
		return nil, fmt.Errorf("%s comes before %s", End, Start)
	}
	return []byte(s[:i] + Start + "\n\n" + Table() + "\n" + s[j:]), nil
}

// escape protects the table's own delimiter, which no description carries today
// and which would otherwise split a cell in silence.
func escape(s string) string { return strings.ReplaceAll(s, "|", `\|`) }
