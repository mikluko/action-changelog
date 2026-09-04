// Command gen-readme writes the register of checks into the README between its
// markers.
//
// CI runs it and fails on a diff, so a check added to the register without the
// README being regenerated is caught in the pull request that adds it.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/mikluko/action-changelog/internal/readme"
)

func main() {
	path := flag.String("readme", "README.md", "path to the README")
	flag.Parse()

	if err := run(*path); err != nil {
		fmt.Fprintf(os.Stderr, "gen-readme: %v\n", err)
		os.Exit(1)
	}
}

func run(path string) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	out, err := readme.Render(src)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return os.WriteFile(path, out, 0o644)
}
