package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"sarak.sh/mikluko/changelog/internal/output"
	"sarak.sh/mikluko/changelog/pkg/changelog"
	"sarak.sh/mikluko/changelog/pkg/semver"
)

// The two references a workflow resolves this action by: the GitHub mirror,
// and the Forgejo repository it is developed in.
const (
	action        = "mikluko/action-changelog/actions/validate@"
	forgejoAction = "https://sarak.sh/mikluko/changelog/actions/validate@"
)

// actions is every reference a documented invocation may use: the reader at
// the root and each writer in its subdirectory, under either name.
var actions = []string{
	action, forgejoAction,
	"mikluko/action-changelog/actions/append@", "https://sarak.sh/mikluko/changelog/actions/append@",
	"mikluko/action-changelog/actions/release@", "https://sarak.sh/mikluko/changelog/actions/release@",
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
	{"actions/validate/action.yaml", outputs(changelog.Parse(nil), repoState{}, nil, "")},
	{"actions/append/action.yaml", nil},
	{"actions/release/action.yaml", releaseOutputs(semver.Version{}, "")},
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
// one version: CHANGELOG.md, and actions/VERSION, the release every action
// downloads its binary from.
//
// Both are written in the one release commit, before the tag exists, because
// an action has to name an immutable release at the moment it is committed: a
// workflow pinning it by SHA reads the files as they stand at that commit, and
// nothing can be rewritten onto it afterwards. Under the release workflow the
// tag being cut joins them, which is the last moment a mismatch can be caught
// for free.
func TestVersionMatchesTheChangelog(t *testing.T) {
	latest, ok := changelog.Parse(read(t, "CHANGELOG.md")).Latest()
	if !ok {
		t.Fatal("CHANGELOG.md names no released version")
	}

	if got, want := strings.TrimSpace(string(read(t, "actions/VERSION"))), strings.TrimPrefix(latest.Version, "v"); got != want {
		t.Errorf("actions/VERSION names release %q, CHANGELOG.md's newest released entry is %s", got, latest.Version)
	}
	for _, a := range actionFiles {
		_, install, _ := readAction(t, a.path)
		if v, ok := install.Env["VERSION"]; ok {
			t.Errorf("%s sets VERSION to %q; install.sh reads the release from actions/VERSION", a.path, v)
		}
	}

	// Locally there is no tag and nothing to check against.
	if tag := os.Getenv("GITHUB_REF_NAME"); strings.HasPrefix(tag, "v") && tag != latest.Version {
		t.Errorf("the tag being cut is %s, CHANGELOG.md's newest released entry is %s",
			tag, latest.Version)
	}
}

// Every action installs the binary with the one script in actions/, reached
// from the action's own directory.
func TestActionInstallsWithTheSharedScript(t *testing.T) {
	for _, a := range actionFiles {
		_, install, _ := readAction(t, a.path)
		rel, err := filepath.Rel(filepath.Dir(a.path), "actions/install.sh")
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
			checkOutputs(t, action, a.outputs)
			checkInputs(t, action, step)
		})
	}
}

// checkOutputs holds the outputs action declares to those the binary writes,
// each mapped from the run step's output of the same name: a composite action's
// output is empty unless it is mapped from a step's, and one mapped from
// another name reports the wrong value.
func checkOutputs(t *testing.T, action actionFile, outputs []output.Output) {
	t.Helper()
	written := map[string]bool{}
	for _, o := range outputs {
		written[o.Name] = true
		if _, ok := action.Outputs[o.Name]; !ok {
			t.Errorf("declares no output %q", o.Name)
		}
	}
	for name, o := range action.Outputs {
		if !written[name] {
			t.Errorf("declares an output %q nothing writes", name)
		}
		if o.Description == "" {
			t.Errorf("output %q carries no description", name)
		}
		if v := o.Value; v != "${{ steps.run.outputs."+name+" }}" && v != "${{ steps.run.outputs['"+name+"'] }}" {
			t.Errorf("output %q is %q, not the run step's output of that name", name, v)
		}
	}
}

// checkInputs holds every input of action to reaching the binary as the flag
// of the same name, through a variable of step's environment: one the script
// never passes is a documented setting that does nothing.
func checkInputs(t *testing.T, action actionFile, step actionStep) {
	t.Helper()
	for name := range action.Inputs {
		if !passes(step, name) {
			t.Errorf("input %q reaches no -%s argument", name, name)
		}
	}
}

// passes reports whether step hands input name to the binary as -name.
func passes(step actionStep, name string) bool {
	for key, v := range step.Env {
		if v != "${{ inputs."+name+" }}" && v != "${{ inputs['"+name+"'] }}" {
			continue
		}
		if strings.Contains(step.Run, "-"+name+`="$`+key+`"`) {
			return true
		}
	}
	return false
}

// Every documented invocation names the major tag the release workflow moves:
// that of the release the pending fragments call for, or of the newest entry
// where there are none, so a breaking change documents its major in the pull
// request that makes it.
// A reference the repository carries no tag for fails on the uses: line before
// the action runs, so a reader copying one gets a broken workflow rather than
// an old one.
func TestDocumentedInvocationsNameTheCurrentMajor(t *testing.T) {
	major := pendingMajor(t)
	found := invocations(t)
	if len(found) == 0 {
		t.Fatalf("no invocation of %q found; this test is checking nothing", actions)
	}
	for _, in := range found {
		if want := in.prefix + major; in.ref != want {
			t.Errorf("%s:%d names %q, want %q", in.path, in.line, in.ref, want)
		}
	}
}

// invocation is one uses: line naming an action of this repository.
type invocation struct {
	path        string
	line        int
	prefix, ref string
}

// invocations returns every uses: line naming one of actions in the Markdown
// and YAML files outside dot-directories.
func invocations(t *testing.T) []invocation {
	t.Helper()
	var out []invocation
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
			out = append(out, invocationsIn(path, string(read(t, path)))...)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// invocationsIn returns the invocations in src, the file at path. Matching on
// the uses: prefix leaves the go run lines alone, where
// sarak.sh/mikluko/changelog/cmd/changelog@latest is the correct reference.
func invocationsIn(path, src string) []invocation {
	var out []invocation
	for i, line := range strings.Split(src, "\n") {
		for _, prefix := range actions {
			at := strings.Index(line, "uses: "+prefix)
			if at < 0 {
				continue
			}
			ref := strings.Fields(line[at+len("uses: "):])[0]
			out = append(out, invocation{path: path, line: i + 1, prefix: prefix, ref: ref})
		}
	}
	return out
}

// pendingMajor is the major tag of the release folding .changelog/ would cut,
// or of the newest entry where .changelog/ holds no fragments.
func pendingMajor(t *testing.T) string {
	t.Helper()
	src := read(t, "CHANGELOG.md")
	_, frags, err := readFragments(".changelog")
	if err != nil {
		t.Fatal(err)
	}
	if len(frags) > 0 {
		folded, err := changelog.Fold(src, frags...)
		if err != nil {
			t.Fatal(err)
		}
		next, err := changelog.Parse(folded).Next()
		if err != nil {
			t.Fatal(err)
		}
		return next.MajorTag()
	}
	latest, ok := changelog.Parse(src).Latest()
	if !ok {
		t.Fatal("CHANGELOG.md names no released version")
	}
	return latest.Semver.MajorTag()
}

func read(t *testing.T, name string) []byte {
	t.Helper()
	src, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return src
}
