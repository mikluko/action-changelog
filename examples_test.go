package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/mikluko/action-changelog/internal/changelog"
)

// The example under ./examples is executed rather than described. Its two
// workflows are read for the inputs they carry, those inputs are resolved
// through the same flag handling the command uses, and both example documents
// are validated under the result.
//
// The tag-dependent checks are left out, which is deliberate: they compare a
// document against the tags of the repository it lives in, and these documents
// live in this one, whose tags describe this action rather than the fictional
// project the example is written for.
const (
	exampleGood     = "examples/CHANGELOG.md"
	exampleBroken   = "examples/CHANGELOG.broken.md"
	workflowMain    = "examples/workflows/main.yaml"
	workflowRequest = "examples/workflows/pull-request.yaml"
)

// The example claims a policy that costs one input on one of the two
// invocations, so a reader can copy both and see where they part. A second
// difference would make the claim false without any test noticing.
func TestExampleWorkflowsDifferInOneInput(t *testing.T) {
	on, off := exampleInputs(t, workflowMain), exampleInputs(t, workflowRequest)

	if got := on["error"]; got != changelog.CheckPrereleaseEntry {
		t.Errorf("the main-branch invocation raises %q, not %s", got, changelog.CheckPrereleaseEntry)
	}
	if got, ok := off["error"]; ok {
		t.Errorf("the pull-request invocation raises %q; it is meant to raise nothing", got)
	}

	delete(on, "error")
	if !reflect.DeepEqual(on, off) {
		t.Errorf("the two invocations differ in more than prerelease-entry:\n%v\n%v", on, off)
	}
}

// The example changelog passes under the example configuration. It is written
// to a policy the default vocabulary refuses, so this also holds the sections
// input to the document it is there for.
func TestExampleChangelogPasses(t *testing.T) {
	for _, workflow := range []string{workflowMain, workflowRequest} {
		t.Run(filepath.Base(workflow), func(t *testing.T) {
			var log bytes.Buffer
			findings := lintExample(t, exampleGood, workflow, &log)
			if len(findings) != 0 {
				t.Errorf("the example changelog does not pass its own configuration:\n%s", log.String())
			}
		})
	}
}

// The broken copy fails with the findings the example's README lists, in the
// order the report prints them. Asserting the set rather than the count is what
// keeps the README honest: a check that stopped firing would otherwise be
// covered by one that started.
func TestBrokenExampleFailsWithTheFindingsItClaims(t *testing.T) {
	for _, tc := range []struct {
		workflow string
		want     []string
	}{
		{
			workflowMain,
			[]string{
				changelog.CheckUnknownSection,
				changelog.CheckPrereleaseEntry,
				changelog.CheckVersionOrder,
				changelog.CheckPartialLinkRef,
			},
		},
		{
			workflowRequest,
			[]string{
				changelog.CheckUnknownSection,
				changelog.CheckVersionOrder,
				changelog.CheckPartialLinkRef,
			},
		},
	} {
		t.Run(filepath.Base(tc.workflow), func(t *testing.T) {
			var log bytes.Buffer
			findings := lintExample(t, exampleBroken, tc.workflow, &log)

			var got []string
			for _, f := range findings {
				got = append(got, f.Check)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("the broken example raises %v, want %v:\n%s", got, tc.want, log.String())
			}
			if valid(findings) {
				t.Error("the broken example is reported valid")
			}
		})
	}
}

// lintExample validates path under the inputs workflow carries, resolved
// through the command's own flag handling so the example cannot pass here by a
// route the action does not take.
func lintExample(t *testing.T, path, workflow string, log *bytes.Buffer) []changelog.Finding {
	t.Helper()

	with := exampleInputs(t, workflow)
	severities, err := severities(with["error"], with["warn"], with["off"])
	if err != nil {
		t.Fatal(err)
	}
	_, findings, err := run(path, changelog.Options{
		Sections:   split(with["sections"]),
		Severities: severities,
	}, log, log)
	if err != nil {
		t.Fatal(err)
	}
	return findings
}

// exampleInputs returns the with: block of the step running this action, which
// is what a reader copying the workflow would copy.
func exampleInputs(t *testing.T, workflow string) map[string]string {
	t.Helper()

	var parsed struct {
		Jobs map[string]struct {
			Steps []struct {
				Uses string            `yaml:"uses"`
				With map[string]string `yaml:"with"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	src, err := os.ReadFile(workflow)
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(src, &parsed); err != nil {
		t.Fatal(err)
	}
	for _, job := range parsed.Jobs {
		for _, step := range job.Steps {
			if step.Uses == "mikluko/action-changelog@v0" {
				return step.With
			}
		}
	}
	t.Fatalf("%s runs no step using this action", workflow)
	return nil
}
