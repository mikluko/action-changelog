// Command action-changelog reads a Keep a Changelog document and reports where
// it departs from the format.
//
// It runs as a GitHub Action and as a local command; under GitHub Actions each
// finding is additionally emitted as a workflow annotation, so a failing check
// lands on the offending line of the diff rather than only in the log.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/mikluko/action-changelog/internal/changelog"
)

func main() {
	var (
		path     = flag.String("changelog", "CHANGELOG.md", "path to the changelog")
		validate = flag.Bool("validate", false, "report where the changelog departs from the format")
		sections = flag.String("sections", "", "comma-separated level-3 headings to accept; the Keep a Changelog six when empty")
	)
	flag.Parse()

	if !*validate {
		fmt.Fprintln(os.Stderr, "nothing to do: pass -validate")
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*path, split(*sections)); err != nil {
		fmt.Fprintf(os.Stderr, "action-changelog: %v\n", err)
		os.Exit(1)
	}
}

func run(path string, sections []string) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	findings := changelog.Parse(src).Lint(sections)
	if len(findings) == 0 {
		return nil
	}
	annotate := os.Getenv("GITHUB_ACTIONS") == "true"
	for _, f := range findings {
		fmt.Fprintf(os.Stderr, "%s:%d: %s\n", path, f.Line, f.Msg)
		if annotate {
			fmt.Printf("::error file=%s,line=%d::%s\n", path, f.Line, f.Msg)
		}
	}
	return fmt.Errorf("%s: %d findings", path, len(findings))
}

// split reads the comma-separated -sections value, dropping empty fields so a
// trailing comma is not a section named "".
func split(s string) []string {
	var out []string
	for _, f := range strings.Split(s, ",") {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}
