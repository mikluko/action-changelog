// Package semver implements Semantic Versioning 2.0.0.
//
// The accepted set is the specification's. golang.org/x/mod/semver is not that
// set and says so in its own package doc: it requires a leading "v" and
// recognises "vMAJOR" and "vMAJOR.MINOR" as alternatives to the three-component
// form. Parse accepts neither, so a version this package reads is one the
// specification's own suggested regular expression also matches.
//
// One departure is this package's own: Major, Minor and Patch are uint64, where
// the specification sets no upper bound. A component past that range is
// rejected as RuleRange.
package semver

import (
	"fmt"
	"strconv"
	"strings"
)

// Version is a parsed semantic version. Pre and Build hold the dot-separated
// identifiers as written, because build metadata that differs only in leading
// zeroes is two different strings and section 10 still ranks them equal.
type Version struct {
	Major, Minor, Patch uint64
	Pre                 []string
	Build               []string
}

// String writes the version back in the form Parse reads.
func (v Version) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d.%d.%d", v.Major, v.Minor, v.Patch)
	if len(v.Pre) > 0 {
		b.WriteByte('-')
		b.WriteString(strings.Join(v.Pre, "."))
	}
	if len(v.Build) > 0 {
		b.WriteByte('+')
		b.WriteString(strings.Join(v.Build, "."))
	}
	return b.String()
}

// Prerelease reports whether the version carries a pre-release part, which
// section 9 says denotes a version not yet stable.
func (v Version) Prerelease() bool { return len(v.Pre) > 0 }

// Rule names the requirement a version string broke, so a caller reporting a
// rejection can say which rule it hit rather than restating the grammar.
type Rule string

const (
	RuleEmpty       Rule = "empty"                 // nothing to parse
	RuleVPrefix     Rule = "v-prefix"              // a leading "v", which is not the specification's
	RuleCore        Rule = "version-core"          // not three dot-separated components
	RuleNumeric     Rule = "numeric-identifier"    // a component that is not a number
	RuleLeadingZero Rule = "leading-zero"          // a numeric identifier with a leading zero
	RuleEmptyIdent  Rule = "empty-identifier"      // a dot-separated identifier with nothing in it
	RuleIdentChars  Rule = "identifier-characters" // an identifier outside [0-9A-Za-z-]
	RuleRange       Rule = "range"                 // a component past what this package represents
)

// SyntaxError reports a string the specification does not accept, naming the
// rule it broke and the fragment that broke it.
//
// Part is empty where the whole input is at fault. Note is the requirement in
// words, and it is what a message addressed to a human should carry.
type SyntaxError struct {
	Input string
	Rule  Rule
	Part  string
	Note  string
}

func (e *SyntaxError) Error() string {
	if e.Part == "" {
		return fmt.Sprintf("%q is not a semantic version: %s", e.Input, e.Note)
	}
	return fmt.Sprintf("%q is not a semantic version: %s (%q)", e.Input, e.Note, e.Part)
}

// Parse reads a semantic version. It accepts exactly what Semantic Versioning
// 2.0.0 accepts, so "1.2" and "v1.2.3" are both errors, and returns a
// *SyntaxError naming the rule that rejected it.
func Parse(s string) (Version, error) {
	if s == "" {
		return Version{}, &SyntaxError{Input: s, Rule: RuleEmpty, Note: "it is empty"}
	}
	if s[0] == 'v' || s[0] == 'V' {
		return Version{}, &SyntaxError{
			Input: s, Rule: RuleVPrefix, Part: s[:1],
			Note: `it carries a leading "v", which the specification does not use`,
		}
	}

	// Build metadata is split off first: it may contain "-", so a version core
	// separated on "-" before "+" would take part of the build for a
	// pre-release. The core itself contains neither.
	rest, build, hasBuild := strings.Cut(s, "+")
	core, pre, hasPre := strings.Cut(rest, "-")

	var v Version
	fields := strings.Split(core, ".")
	if len(fields) != 3 {
		return Version{}, &SyntaxError{
			Input: s, Rule: RuleCore, Part: core,
			Note: fmt.Sprintf("a version states major.minor.patch, three components, and this states %d", len(fields)),
		}
	}
	for i, name := range [3]string{"major", "minor", "patch"} {
		n, err := parseNumeric(s, name, fields[i])
		if err != nil {
			return Version{}, err
		}
		switch i {
		case 0:
			v.Major = n
		case 1:
			v.Minor = n
		case 2:
			v.Patch = n
		}
	}

	// A numeric pre-release identifier may not carry a leading zero, while a
	// build identifier may be bare digits: "1.0.0-00" is invalid and
	// "1.0.0+00" is valid. That asymmetry is the whole difference between the
	// two calls below.
	if hasPre {
		ids, err := parseIdents(s, "pre-release", pre, true)
		if err != nil {
			return Version{}, err
		}
		v.Pre = ids
	}
	if hasBuild {
		ids, err := parseIdents(s, "build", build, false)
		if err != nil {
			return Version{}, err
		}
		v.Build = ids
	}
	return v, nil
}

// Valid reports whether s is a semantic version, for callers with nothing to
// say about why it is not.
func Valid(s string) bool {
	_, err := Parse(s)
	return err == nil
}

// parseNumeric reads one component of the version core.
func parseNumeric(input, name, s string) (uint64, *SyntaxError) {
	switch {
	case s == "":
		return 0, &SyntaxError{
			Input: input, Rule: RuleNumeric,
			Note: fmt.Sprintf("the %s is empty, and every component of a version is a number", name),
		}
	case !allDigits(s):
		return 0, &SyntaxError{
			Input: input, Rule: RuleNumeric, Part: s,
			Note: fmt.Sprintf("the %s is not a number", name),
		}
	case len(s) > 1 && s[0] == '0':
		return 0, &SyntaxError{
			Input: input, Rule: RuleLeadingZero, Part: s,
			Note: fmt.Sprintf("the %s has a leading zero", name),
		}
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, &SyntaxError{
			Input: input, Rule: RuleRange, Part: s,
			Note: fmt.Sprintf("the %s is larger than this tool represents", name),
		}
	}
	return n, nil
}

// parseIdents splits a dot-separated run of identifiers and checks each.
//
// numeric says whether an all-digit identifier is a numeric identifier, which
// bars a leading zero. It holds for pre-release identifiers and not for build
// ones, per sections 9 and 10.
func parseIdents(input, kind, s string, numeric bool) ([]string, *SyntaxError) {
	ids := strings.Split(s, ".")
	for i, id := range ids {
		switch {
		case id == "":
			return nil, &SyntaxError{
				Input: input, Rule: RuleEmptyIdent, Part: s,
				Note: fmt.Sprintf("%s identifier %d is empty", kind, i+1),
			}
		case !identChars(id):
			return nil, &SyntaxError{
				Input: input, Rule: RuleIdentChars, Part: id,
				Note: fmt.Sprintf("%s identifier %d holds a character outside [0-9A-Za-z-]", kind, i+1),
			}
		case numeric && allDigits(id) && len(id) > 1 && id[0] == '0':
			return nil, &SyntaxError{
				Input: input, Rule: RuleLeadingZero, Part: id,
				Note: fmt.Sprintf("%s identifier %d is a number with a leading zero", kind, i+1),
			}
		}
	}
	return ids, nil
}

func allDigits(s string) bool {
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return s != ""
}

func identChars(s string) bool {
	for i := range len(s) {
		c := s[i]
		switch {
		case c >= '0' && c <= '9', c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c == '-':
		default:
			return false
		}
	}
	return true
}

// Canonical is the version without its build metadata. Section 10 says build
// metadata is ignored when determining precedence, so two versions differing
// only there are one version wherever versions are compared or matched.
func (v Version) Canonical() Version {
	v.Build = nil
	return v
}

// Tag writes the version as a tag name, with the leading "v" a repository's tag
// namespace carries by convention and the specification's own grammar does not.
func (v Version) Tag() string { return "v" + v.String() }

// ParseTag reads a version tag, which is deliberately laxer than Parse.
//
// A tag namespace is not the specification's: it carries a leading "v", and the
// GitHub Actions convention keeps a moving "v1" beside the "v1.0.0" it points
// at, so a caller reading tags has to understand both. A component the tag omits
// is zero, making "v1" and "v1.0.0" one version — which is what leaves a
// repository's moving tag a candidate for the reference tag.
//
// Only the core is expanded. A pre-release or build part rides along and is read
// exactly as Parse reads it, so "v1-rc.1" is the version 1.0.0-rc.1 and
// "v1.2.3-01" is still a leading zero in a numeric identifier.
func ParseTag(s string) (Version, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Version{}, &SyntaxError{Input: s, Rule: RuleEmpty, Note: "it is empty"}
	}
	if s[0] == 'v' || s[0] == 'V' {
		s = s[1:]
	}
	// Only the version core is expanded, so the shorthand cannot reach across a
	// pre-release or build separator and invent components a tag never spelled.
	core := s
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		core = s[:i]
	}
	switch strings.Count(core, ".") {
	case 0:
		s = core + ".0.0" + s[len(core):]
	case 1:
		s = core + ".0" + s[len(core):]
	}
	return Parse(s)
}

// Compare orders two versions by the precedence rules of section 11, returning
// -1, 0 or +1 as v sorts before, equal to, or after w.
//
// Build metadata is ignored, per section 10, so two versions differing only
// there compare equal while remaining different strings.
func Compare(v, w Version) int {
	if c := cmpUint(v.Major, w.Major); c != 0 {
		return c
	}
	if c := cmpUint(v.Minor, w.Minor); c != 0 {
		return c
	}
	if c := cmpUint(v.Patch, w.Patch); c != 0 {
		return c
	}
	return comparePre(v.Pre, w.Pre)
}

// comparePre orders two pre-release parts. A version with no pre-release part
// outranks one that has any, which is section 9's rule that a pre-release
// denotes something below the normal version.
func comparePre(a, b []string) int {
	switch {
	case len(a) == 0 && len(b) == 0:
		return 0
	case len(a) == 0:
		return 1
	case len(b) == 0:
		return -1
	}
	for i := 0; i < len(a) && i < len(b); i++ {
		if c := compareIdent(a[i], b[i]); c != 0 {
			return c
		}
	}
	// Every identifier they share is equal, so the longer run wins: a larger
	// set of pre-release fields outranks a smaller one.
	return cmpInt(len(a), len(b))
}

// compareIdent orders two pre-release identifiers.
//
// Numeric identifiers compare numerically and rank below every identifier that
// is not one. The numeric comparison is done on the text: a numeric identifier
// carries no leading zero, so the longer of two is the larger, and equal
// lengths order lexically. That holds at any width, which matters because the
// specification sets no bound on one.
func compareIdent(a, b string) int {
	an, bn := allDigits(a), allDigits(b)
	switch {
	case an && bn:
		if len(a) != len(b) {
			return cmpInt(len(a), len(b))
		}
		return strings.Compare(a, b)
	case an:
		return -1
	case bn:
		return 1
	default:
		return strings.Compare(a, b)
	}
}

func cmpUint(a, b uint64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// MajorTag is the moving tag a release workflow keeps for this version's major
// line, as in "v1". It is the reference a documented invocation names.
func (v Version) MajorTag() string { return "v" + strconv.FormatUint(v.Major, 10) }
