package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/mikluko/action-changelog/internal/changelog"
)

// The two references a workflow resolves this action by: the GitHub mirror,
// and the Forgejo repository it is developed in.
const (
	action        = "mikluko/action-changelog@"
	forgejoAction = "https://forge.sarak.sh/actions/changelog@"
)

var actions = []string{action, forgejoAction}

// actionFile is the part of action.yaml the tests hold to the code.
type actionFile struct {
	Inputs  map[string]yaml.Node `yaml:"inputs"`
	Outputs map[string]struct {
		Description string `yaml:"description"`
		Value       string `yaml:"value"`
	} `yaml:"outputs"`
	Runs struct {
		Steps []actionStep `yaml:"steps"`
	} `yaml:"runs"`
}

type actionStep struct {
	ID  string            `yaml:"id"`
	Env map[string]string `yaml:"env"`
	Run string            `yaml:"run"`
}

// readAction returns action.yaml and the step that runs the binary, whose
// outputs are the action's.
func readAction(t *testing.T) (actionFile, actionStep) {
	t.Helper()
	var action actionFile
	if err := yaml.Unmarshal(read(t, "action.yaml"), &action); err != nil {
		t.Fatal(err)
	}
	for _, step := range action.Runs.Steps {
		if step.ID == "run" {
			return action, step
		}
	}
	t.Fatal("action.yaml has no step with id run")
	return action, actionStep{}
}

// TestVersionMatchesTheChangelog holds the two places a release names itself to
// one version: CHANGELOG.md, and the release action.yaml downloads the binary
// from.
//
// Both are written together in the release pull request, before the tag exists,
// because action.yaml has to name an immutable release at the moment it is
// committed: a workflow pinning this action by SHA reads that file as it stands
// at that commit, and nothing can be rewritten onto it afterwards. Under the
// release workflow the tag being cut joins them, which is the last moment a
// mismatch can be caught for free.
func TestVersionMatchesTheChangelog(t *testing.T) {
	latest, ok := changelog.Parse(read(t, "CHANGELOG.md")).Latest()
	if !ok {
		t.Fatal("CHANGELOG.md names no released version")
	}

	_, step := readAction(t)
	if got, want := step.Env["VERSION"], strings.TrimPrefix(latest.Version, "v"); got != want {
		t.Errorf("action.yaml downloads release %q, CHANGELOG.md's newest released entry is %s",
			got, latest.Version)
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
	action, step := readAction(t)
	declared := action.Outputs

	written := map[string]bool{}
	for _, o := range outputs(changelog.Parse(nil), repoState{}, nil, "") {
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
		// A composite action's output is empty unless it is mapped from a
		// step's, and one mapped from another name reports the wrong value.
		if v := o.Value; v != "${{ steps.run.outputs."+name+" }}" && v != "${{ steps.run.outputs['"+name+"'] }}" {
			t.Errorf("output %q is %q, not the run step's output of that name", name, v)
		}
	}

	// An input reaches the binary as the flag of the same name and nowhere
	// else, through a variable of the step's environment, so one the script
	// never passes is a documented setting that does nothing.
	for name := range action.Inputs {
		var passed bool
		for key, v := range step.Env {
			if v != "${{ inputs."+name+" }}" && v != "${{ inputs['"+name+"'] }}" {
				continue
			}
			passed = passed || strings.Contains(step.Run, "-"+name+`="$`+key+`"`)
		}
		if !passed {
			t.Errorf("input %q reaches no -%s argument", name, name)
		}
	}
}

// Every documented invocation names the major tag the release workflow moves.
// A reference the repository carries no tag for fails on the uses: line before
// the action runs, so a reader copying one gets a broken workflow rather than
// an old one.
func TestDocumentedInvocationsNameTheCurrentMajor(t *testing.T) {
	latest, ok := changelog.Parse(read(t, "CHANGELOG.md")).Latest()
	if !ok {
		t.Fatal("CHANGELOG.md names no released version")
	}
	major := latest.Semver.MajorTag()

	var found int
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); path != "." && strings.HasPrefix(name, ".") {
				return fs.SkipDir
			}
			return nil
		}
		switch filepath.Ext(path) {
		case ".md", ".yaml", ".yml":
		default:
			return nil
		}
		for i, line := range strings.Split(string(read(t, path)), "\n") {
			for _, prefix := range actions {
				// Matching on the uses: prefix leaves the go run lines alone,
				// where github.com/mikluko/action-changelog@latest is the
				// correct reference.
				at := strings.Index(line, "uses: "+prefix)
				if at < 0 {
					continue
				}
				found++
				ref := strings.Fields(line[at+len("uses: "):])[0]
				if want := prefix + major; ref != want {
					t.Errorf("%s:%d names %q, want %q", path, i+1, ref, want)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if found == 0 {
		t.Fatalf("no invocation of %q found; this test is checking nothing", actions)
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
