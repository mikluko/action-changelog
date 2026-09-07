package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/mikluko/action-changelog/internal/changelog"
)

// The examples under ./examples are executed rather than described. Each
// strategy's workflows are read for the inputs they carry, those inputs are
// resolved through the same flag handling the command uses, and that strategy's
// two documents are validated under the result.
//
// The checks that read the repository are left out, which is deliberate: they
// compare a document against the tags of the repository it lives in, and these
// documents live in this one, whose tags describe this action rather than the
// fictional project the examples are written for. undated-release is one of
// them, so what holds it is TestReleaseBranchKeepsUndatedReleaseOn rather than a
// document provoking it.
const (
	trunkGood    = "examples/release-trunk/CHANGELOG.md"
	trunkBroken  = "examples/release-trunk/CHANGELOG.broken.md"
	trunkMain    = "examples/release-trunk/workflows/main.yaml"
	trunkRequest = "examples/release-trunk/workflows/pull-request.yaml"

	branchGood      = "examples/release-branch/CHANGELOG.md"
	branchBroken    = "examples/release-branch/CHANGELOG.broken.md"
	branchCI        = "examples/release-branch/workflows/ci.yaml"
	branchCandidate = "examples/release-branch/workflows/release-candidate.yaml"
	branchRelease   = "examples/release-branch/workflows/release.yaml"
	branchPublish   = "examples/release-branch/workflows/publish.yaml"
)

// exampleSections is the vocabulary both strategies accept, which is the Keep a
// Changelog six plus the Breaking heading each policy adds.
const exampleSections = "Added,Changed,Deprecated,Removed,Fixed,Security,Breaking"

// The example claims a policy that costs one input on one of the two
// invocations, so a reader can copy both and see where they part. A second
// difference would make the claim false without any test noticing.
func TestReleaseTrunkWorkflowsDifferInOneInput(t *testing.T) {
	on, off := exampleInputs(t, trunkMain), exampleInputs(t, trunkRequest)

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
func TestReleaseTrunkChangelogPasses(t *testing.T) {
	for _, workflow := range []string{trunkMain, trunkRequest} {
		t.Run(filepath.Base(workflow), func(t *testing.T) {
			var log bytes.Buffer
			findings := lintExample(t, trunkGood, workflow, &log)
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
func TestReleaseTrunkBrokenChangelogFailsWithTheFindingsItClaims(t *testing.T) {
	for _, tc := range []struct {
		workflow string
		want     []string
	}{
		{
			trunkMain,
			[]string{
				changelog.CheckUnknownSection,
				changelog.CheckPrereleaseEntry,
				changelog.CheckVersionOrder,
				changelog.CheckPartialLinkRef,
			},
		},
		{
			trunkRequest,
			[]string{
				changelog.CheckUnknownSection,
				changelog.CheckVersionOrder,
				changelog.CheckPartialLinkRef,
			},
		},
	} {
		t.Run(filepath.Base(tc.workflow), func(t *testing.T) {
			var log bytes.Buffer
			findings := lintExample(t, trunkBroken, tc.workflow, &log)

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
			if strings.HasPrefix(step.Uses, action) {
				return step.With
			}
		}
	}
	t.Fatalf("%s runs no step using this action", workflow)
	return nil
}

// The release-branch strategy is three invocations whose differences are the
// whole of it, and its README tabulates them. The literal here is that table:
// an input added to a workflow or dropped from one fails this test rather than
// leaving a README that has quietly stopped describing the tree.
func TestReleaseBranchWorkflowInputs(t *testing.T) {
	for _, tc := range []struct {
		workflow string
		want     map[string]string
	}{
		{branchCI, map[string]string{
			"sections": exampleSections,
			"error":    changelog.CheckPrereleaseEntry,
			"off":      changelog.CheckUndatedEntry,
		}},
		{branchCandidate, map[string]string{
			"sections":       exampleSections,
			"error":          changelog.CheckPrereleaseEntry,
			"off":            changelog.CheckUndatedEntry,
			"reference-tags": "final",
		}},
		{branchRelease, map[string]string{
			"sections": exampleSections,
			"error":    changelog.CheckPrereleaseEntry,
		}},
		{branchPublish, map[string]string{
			"sections": exampleSections,
			"error":    changelog.CheckPrereleaseEntry,
			"off":      changelog.CheckUndatedEntry,
		}},
	} {
		t.Run(filepath.Base(tc.workflow), func(t *testing.T) {
			if got := exampleInputs(t, tc.workflow); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("%s carries %v, want %v", tc.workflow, got, tc.want)
			}
		})
	}
}

// The four invocations differ in exactly one input: release.yaml keeps
// undated-entry at its default, and the other three switch it off. That is the
// whole of what the strategy costs in configuration, and a second difference
// would make the README's claim false without any test noticing.
func TestReleaseBranchInvocationsDifferInOneInput(t *testing.T) {
	release := exampleInputs(t, branchRelease)
	for _, other := range []string{branchCI, branchCandidate, branchPublish} {
		t.Run(filepath.Base(other), func(t *testing.T) {
			with := exampleInputs(t, other)
			delete(with, "reference-tags") // candidate alone states the default
			if got, want := with["off"], changelog.CheckUndatedEntry; got != want {
				t.Errorf("%s switches off %q, want %q", other, got, want)
			}
			delete(with, "off")
			if !reflect.DeepEqual(with, release) {
				t.Errorf("%s carries %v beyond that, want %v", other, with, release)
			}
		})
	}
}

// undated-entry is the check the stabilization branch switches off; the strategy
// breaks where undated-release goes off beside it. That one fires only where a
// tag already names the undated entry's version, which is a release that shipped
// and nobody dated, and no branch wants that. The two read alike in a workflow
// and mean opposite things, so the branch invocation is held to the distinction.
func TestReleaseBranchKeepsUndatedReleaseOn(t *testing.T) {
	with := exampleInputs(t, branchCandidate)
	sev, err := severities(with["error"], with["warn"], with["off"])
	if err != nil {
		t.Fatal(err)
	}
	if got := sev[changelog.CheckUndatedEntry]; got != changelog.Off {
		t.Errorf("the candidate invocation runs %s at %s, want off", changelog.CheckUndatedEntry, got)
	}
	if got := sev[changelog.CheckUndatedRelease]; got != changelog.Error {
		t.Errorf("the candidate invocation runs %s at %s, want error", changelog.CheckUndatedRelease, got)
	}
}

// One document under the four invocations is the whole strategy. The open entry
// names the release it is heading for and carries no date, which is legitimate
// everywhere but on the trunk: the merge is what dates it, so an entry reaching
// release.yaml still open is a release nobody finished. That file never reaches
// the trunk in that state.
func TestReleaseBranchOpenEntry(t *testing.T) {
	forEachInvocation(t, branchGood, []exampleCase{
		{branchCI, nil},
		{branchCandidate, nil},
		{branchPublish, nil},
		{branchRelease, []string{changelog.CheckUndatedEntry}},
	})
}

// The broken copy fails with the findings the README lists, per invocation.
// Asserting the set rather than the count is what keeps the README honest: a
// check that stopped firing would otherwise be covered by one that started.
//
// prerelease-entry is raised on every invocation, which is the strategy's own
// doing: the document names the release it heads for and never an attempt, so a
// candidate in a heading is wrong wherever it is read.
func TestReleaseBranchBrokenChangelogFailsWithTheFindingsItClaims(t *testing.T) {
	shared := []string{
		changelog.CheckPartialLinkRef,
		changelog.CheckHeadingForm,
		changelog.CheckPrereleaseEntry,
	}
	forEachInvocation(t, branchBroken, []exampleCase{
		{branchCI, shared},
		{branchCandidate, shared},
		{branchPublish, shared},
		{branchRelease, append([]string{changelog.CheckUndatedEntry}, shared...)},
	})
}

// exampleCase is a document validated under one workflow's inputs and the checks
// it is expected to raise, in the order the report prints them. An empty want is
// a document that passes, and the verdict is asserted beside the list so a
// finding at warning could not pass for one at error.
type exampleCase struct {
	workflow string
	want     []string
}

func forEachInvocation(t *testing.T, path string, cases []exampleCase) {
	t.Helper()

	for _, tc := range cases {
		t.Run(filepath.Base(tc.workflow), func(t *testing.T) {
			var log bytes.Buffer
			findings := lintExample(t, path, tc.workflow, &log)

			var got []string
			for _, f := range findings {
				got = append(got, f.Check)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("%s raises %v, want %v:\n%s", path, got, tc.want, log.String())
			}
			if valid(findings) != (len(tc.want) == 0) {
				t.Errorf("%s is reported valid=%v", path, valid(findings))
			}
		})
	}
}
