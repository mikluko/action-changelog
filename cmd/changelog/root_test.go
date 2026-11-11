package main

import (
	"os"
	"testing"
)

// TestMain runs the tests from the repository root, which is where the files
// they hold to the code live: actions/, CHANGELOG.md and examples/.
func TestMain(m *testing.M) {
	if err := os.Chdir("../.."); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}
