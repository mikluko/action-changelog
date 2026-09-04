package main

import (
	"os"
	"strings"
	"testing"

	"github.com/mikluko/action-changelog/internal/changelog"
)

// TestVersionMatchesTheChangelog holds the VERSION file to the version
// CHANGELOG.md names.
//
// VERSION is what script/install.sh downloads when a workflow pins this action
// by SHA rather than by tag, so the file committed at v1.2.3 has to already
// name v1.2.3. Nothing about the tag can enforce that after the fact, which is
// why the pair is written before the release and checked here.
func TestVersionMatchesTheChangelog(t *testing.T) {
	src, err := os.ReadFile("VERSION")
	if err != nil {
		t.Fatal(err)
	}
	pinned := strings.TrimSpace(string(src))

	src, err = os.ReadFile("CHANGELOG.md")
	if err != nil {
		t.Fatal(err)
	}
	latest, ok := changelog.Parse(src).Latest()
	if !ok {
		t.Fatal("CHANGELOG.md names no released version")
	}
	if pinned != latest.Version {
		t.Errorf("VERSION is %s, CHANGELOG.md's newest released entry is %s", pinned, latest.Version)
	}

	// Under the release workflow the tag being cut joins the pair: it names the
	// release assets install.sh will ask for, so a mismatch publishes a release
	// that the VERSION committed inside it does not point at. Locally there is
	// no tag and nothing to check.
	if tag := os.Getenv("GITHUB_REF_NAME"); strings.HasPrefix(tag, "v") && tag != pinned {
		t.Errorf("the tag being cut is %s, VERSION is %s", tag, pinned)
	}
}
