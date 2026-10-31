package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"sarak.sh/actions/changelog/internal/output"
	"sarak.sh/actions/changelog/pkg/changelog"
	"sarak.sh/actions/changelog/pkg/semver"
)

// The two references a workflow resolves this action by: the GitHub mirror,
// and the Forgejo repository it is developed in.
const (
	action        = "mikluko/action-changelog@"
	forgejoAction = "https://forge.sarak.sh/actions/changelog@"
)

// actions is every reference a documented invocation may use: the reader at
// the root and each writer in its subdirectory, under either name.
var actions = []string{
	action, forgejoAction,
	"mikluko/action-changelog/append@", "https://forge.sarak.sh/actions/changelog/append@",
	"mikluko/action-changelog/release@", "https://forge.sarak.sh/actions/changelog/release@",
}

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

// actionFiles are the actions this repository publishes, each with the
// outputs its binary writes.
var actionFiles = []struct {
	path    string
	outputs []output.Output
}{
	{"action.yaml", outputs(changelog.Parse(nil), repoState{}, nil, "")},
	{"append/action.yaml", nil},
	{"release/action.yaml", releaseOutputs(semver.Version{}, "")},
}

// readAction returns the action at path, the step installing the binary, and
// the step running it, whose outputs are the action's.
func readAction(t *testing.T, path string) (action actionFile, install, run actionStep) {
	t.Helper()
	if err := yaml.Unmarshal(read(t, path), &action); err != nil {
		t.Fatal(err)
	}
	for _, step := range action.Runs.Steps {
		switch step.ID {
		case "install":
			install = step
		case "run":
			run = step
		}
	}
	if install.ID == "" || run.ID == "" {
		t.Fatalf("%s has no step with id install and one with id run", path)
	}
	return action, install, run
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

	for _, a := range actionFiles {
		_, install, _ := readAction(t, a.path)
		if got, want := install.Env["VERSION"], strings.TrimPrefix(latest.Version, "v"); got != want {
			t.Errorf("%s downloads release %q, CHANGELOG.md's newest released entry is %s",
				a.path, got, latest.Version)
		}
	}

	// Locally there is no tag and nothing to check against.
	if tag := os.Getenv("GITHUB_REF_NAME"); strings.HasPrefix(tag, "v") && tag != latest.Version {
		t.Errorf("the tag being cut is %s, CHANGELOG.md's newest released entry is %s",
			tag, latest.Version)
	}
}

// Every action installs the binary with the one script at the repository root,
// reached from the action's own directory.
func TestActionInstallsWithTheSharedScript(t *testing.T) {
	for _, a := range actionFiles {
		_, install, _ := readAction(t, a.path)
		rel, err := filepath.Rel(filepath.Dir(a.path), "install.sh")
		if err != nil {
			t.Fatal(err)
		}
		if want := `bash "$ACTION_PATH/` + filepath.ToSlash(rel) + `"`; strings.TrimSpace(install.Run) != want {
			t.Errorf("%s installs with %q, want %q", a.path, install.Run, want)
		}
		if install.Env["ACTION_PATH"] != "${{ github.action_path }}" {
			t.Errorf("%s sets ACTION_PATH to %q", a.path, install.Env["ACTION_PATH"])
		}
	}
}

// An output a consuming workflow cannot reference is one the binary writes into
// the void, so the declaration and what emit writes are held to each other.
func TestActionDeclaresEveryOutput(t *testing.T) {
	for _, a := range actionFiles {
		t.Run(a.path, func(t *testing.T) {
			action, _, step := readAction(t, a.path)
			declared := action.Outputs

			written := map[string]bool{}
			for _, o := range a.outputs {
				written[o.Name] = true
				if _, ok := declared[o.Name]; !ok {
					t.Errorf("declares no output %q", o.Name)
				}
			}
			for name, o := range declared {
				if !written[name] {
					t.Errorf("declares an output %q nothing writes", name)
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
		})
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
				// where sarak.sh/actions/changelog/cmd/changelog@latest is the
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
