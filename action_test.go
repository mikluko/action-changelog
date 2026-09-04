package main

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/mikluko/action-changelog/internal/changelog"
)

// imageRepository is the reference action.yml pins, up to and including the
// colon that introduces the tag.
const imageRepository = "docker://ghcr.io/mikluko/action-changelog:"

// TestActionPinsTheChangelogVersion holds action.yml's image tag to the version
// CHANGELOG.md names.
//
// A Docker container action is resolved from the action.yml committed at the
// ref a consumer names, so the file tagged v1.2.3 has to already pin v1.2.3.
// Nothing about the tag can enforce that after the fact, which is why the pair
// is written before the release and checked here.
func TestActionPinsTheChangelogVersion(t *testing.T) {
	var action struct {
		Runs struct {
			Image string `yaml:"image"`
		} `yaml:"runs"`
	}
	src, err := os.ReadFile("action.yml")
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(src, &action); err != nil {
		t.Fatal(err)
	}
	ref, ok := strings.CutPrefix(action.Runs.Image, imageRepository)
	if !ok {
		t.Fatalf("action.yml pins %q, which is not %s<tag>", action.Runs.Image, imageRepository)
	}

	src, err = os.ReadFile("CHANGELOG.md")
	if err != nil {
		t.Fatal(err)
	}
	latest, ok := changelog.Parse(src).Latest()
	if !ok {
		t.Fatal("CHANGELOG.md names no released version")
	}
	if ref != latest.Version {
		t.Errorf("action.yml pins %s, CHANGELOG.md's newest released entry is %s", ref, latest.Version)
	}

	// Under the release workflow the tag being cut joins the pair: it is the
	// image tag that will exist, so a mismatch publishes an image no consumer
	// of that tag can resolve. Locally there is no tag and nothing to check.
	if tag := os.Getenv("GITHUB_REF_NAME"); tag != "" && strings.HasPrefix(tag, "v") && tag != ref {
		t.Errorf("the tag being cut is %s, action.yml pins %s", tag, ref)
	}
}
