package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"sarak.sh/mikluko/changelog/internal/output"
	"sarak.sh/mikluko/changelog/pkg/changelog"
	"sarak.sh/mikluko/changelog/pkg/semver"
)

// appendCmd adds one note under a section of the [Unreleased] entry and writes
// the document back where it was read.
func appendCmd(args []string, stderr io.Writer) error {
	fs := flag.NewFlagSet("append", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		path    = fs.String("changelog", "CHANGELOG.md", "path to the changelog")
		section = fs.String("section", "", "level-3 heading to add the note under, as in Added")
		note    = fs.String("note", "", "the note, without its list marker")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("append takes no arguments, got %q", fs.Args())
	}
	return rewrite(*path, func(src []byte) ([]byte, error) {
		return changelog.Append(src, *section, *note)
	})
}

// releaseCmd folds the fragments into the [Unreleased] entry, turns it into a
// versioned one, writes the document back, deletes the fragments, and reports
// the version and the notes it released.
func releaseCmd(args []string, stdout, stderr io.Writer, now func() time.Time) error {
	fs := flag.NewFlagSet("release", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		path      = fs.String("changelog", "CHANGELOG.md", "path to the changelog")
		fragments = fs.String("fragments", ".changelog", "directory of fragments to fold into [Unreleased] and delete; none when empty")
		version   = fs.String("version", "", "version to release; computed from the notes when empty")
		date      = fs.String("date", "", "release date as YYYY-MM-DD; today in UTC when empty")
		open      = fs.Bool("open", false, "write an open entry, naming the version and no date")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("release takes no arguments, got %q", fs.Args())
	}
	if *open && *date != "" {
		return errors.New("-open writes no date, so -date has nothing to set")
	}
	day := *date
	switch {
	case *open:
		day = ""
	case day == "":
		day = now().UTC().Format("2006-01-02")
	}

	given, err := releaseVersion(*version)
	if err != nil {
		return err
	}
	consumed, frags, err := readFragments(*fragments)
	if err != nil {
		return err
	}
	var released semver.Version
	err = rewrite(*path, func(src []byte) ([]byte, error) {
		out, v, err := changelog.ReleaseFragments(src, frags, given, day)
		released = v
		return out, err
	})
	if err != nil {
		return err
	}
	for _, p := range consumed {
		if err := os.Remove(p); err != nil {
			return fmt.Errorf("%s is released and the fragment is not deleted: %w", *path, err)
		}
	}

	src, err := os.ReadFile(*path)
	if err != nil {
		return err
	}
	entry, _ := changelog.Parse(src).Find(released.String())
	return emit(releaseOutputs(released, entry.Body), stdout)
}

// releaseOutputs is what a workflow reads after a release: the version
// released, the notes it carries, and whether it is a pre-release.
func releaseOutputs(v semver.Version, notes string) []output.Output {
	return []output.Output{
		{Name: "version", Value: v.String()},
		{Name: "notes", Value: notes},
		{Name: "prerelease", Value: strconv.FormatBool(v.Prerelease())},
	}
}

// releaseVersion is the version given, read without the leading "v" a tag
// carries, or nil where none is, which leaves it to the notes.
func releaseVersion(given string) (*semver.Version, error) {
	if given = strings.TrimSpace(given); given == "" {
		return nil, nil
	}
	v, err := semver.Parse(strings.TrimPrefix(given, "v"))
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// rewrite applies edit to the file at path and writes the result back with the
// file's mode.
func rewrite(path string, edit func([]byte) ([]byte, error)) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	out, err := edit(src)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return os.WriteFile(path, out, info.Mode().Perm())
}
