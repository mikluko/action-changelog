package main

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/mikluko/action-changelog/internal/changelog"
)

const image = "docker://ghcr.io/mikluko/action-changelog:"

// TestVersionMatchesTheChangelog holds the two places a release names itself to
// one version: CHANGELOG.md, and the image tag in action.yaml.
//
// Both are written together in the release pull request, before the tag exists,
// because action.yaml has to name an immutable image at the moment it is
// committed: a workflow pinning this action by SHA reads that file as it stands
// at that commit, and nothing can be rewritten onto it afterwards. Under the
// release workflow the tag being cut joins them, which is the last moment a
// mismatch can be caught for free.
func TestVersionMatchesTheChangelog(t *testing.T) {
	latest, ok := changelog.Parse(read(t, "CHANGELOG.md")).Latest()
	if !ok {
		t.Fatal("CHANGELOG.md names no released version")
	}

	var action struct {
		Runs struct {
			Image string `yaml:"image"`
		} `yaml:"runs"`
	}
	if err := yaml.Unmarshal(read(t, "action.yaml"), &action); err != nil {
		t.Fatal(err)
	}
	if want := image + latest.Version; action.Runs.Image != want {
		t.Errorf("action.yaml runs %q, CHANGELOG.md's newest released entry is %s",
			action.Runs.Image, latest.Version)
	}

	// Locally there is no tag and nothing to check against.
	if tag := os.Getenv("GITHUB_REF_NAME"); strings.HasPrefix(tag, "v") && tag != latest.Version {
		t.Errorf("the tag being cut is %s, CHANGELOG.md's newest released entry is %s",
			tag, latest.Version)
	}
}

// An output a consuming workflow cannot reference is one the binary writes into
// the void, so the declaration and what emit writes are held to each other.
func TestActionDeclaresEveryOutput(t *testing.T) {
	var action struct {
		Inputs  map[string]yaml.Node `yaml:"inputs"`
		Outputs map[string]struct {
			Description string `yaml:"description"`
		} `yaml:"outputs"`
		Runs struct {
			Args []string `yaml:"args"`
		} `yaml:"runs"`
	}
	if err := yaml.Unmarshal(read(t, "action.yaml"), &action); err != nil {
		t.Fatal(err)
	}
	declared := action.Outputs

	written := map[string]bool{}
	for _, o := range outputs(changelog.Parse(nil), nil, "", nil) {
		written[o.Name] = true
		if _, ok := declared[o.Name]; !ok {
			t.Errorf("action.yaml declares no output %q", o.Name)
		}
	}
	for name, o := range declared {
		if !written[name] {
			t.Errorf("action.yaml declares an output %q nothing writes", name)
		}
		if o.Description == "" {
			t.Errorf("output %q carries no description", name)
		}
	}

	// An input reaches the binary as an argument and nowhere else, so one the
	// args never name is a documented setting that does nothing.
	args := strings.Join(action.Runs.Args, "\n")
	for name := range action.Inputs {
		if !strings.Contains(args, "inputs."+name) && !strings.Contains(args, "inputs['"+name+"']") {
			t.Errorf("input %q reaches no argument", name)
		}
	}
}

func read(t *testing.T, name string) []byte {
	t.Helper()
	src, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return src
}
