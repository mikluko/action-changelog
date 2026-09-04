package readme

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mikluko/action-changelog/internal/changelog"
)

const path = "../../README.md"

// The README documents the checks a workflow can name in -error, -warn and
// -off, so a register the file disagrees with sends readers after checks that
// do not exist. CI runs gen-readme and fails on a diff; this fails first, and
// without a working tree.
func TestREADMEMatchesTheRegister(t *testing.T) {
	src, err := os.ReadFile(filepath.FromSlash(path))
	if err != nil {
		t.Fatal(err)
	}
	want, err := Render(src)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(src, want) {
		t.Errorf("README.md is stale; run: go run ./cmd/gen-readme")
	}
}

func TestTableNamesEveryCheck(t *testing.T) {
	got := Table()
	for _, c := range changelog.Checks {
		if !strings.Contains(got, "`"+c.Name+"`") {
			t.Errorf("the table omits %q", c.Name)
		}
		if !strings.Contains(got, c.Description) {
			t.Errorf("the table omits the description of %q", c.Name)
		}
	}
}

func TestRenderNeedsBothMarkers(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
	}{
		{"no start", "# Title\n" + End + "\n"},
		{"no end", "# Title\n" + Start + "\n"},
		{"the wrong way round", "# Title\n" + End + "\n" + Start + "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Render([]byte(tc.src)); err == nil {
				t.Error("Render accepted it")
			}
		})
	}
}

func TestRenderReplacesRatherThanAppends(t *testing.T) {
	src := []byte("# Title\n\n" + Start + "\n\nstale\n\n" + End + "\n\ntail\n")
	once, err := Render(src)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(once, []byte("stale")) {
		t.Error("the previous table survived")
	}
	if !bytes.Contains(once, []byte("tail")) {
		t.Error("what followed the markers was lost")
	}
	twice, err := Render(once)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(once, twice) {
		t.Error("a second render changed the document")
	}
}
