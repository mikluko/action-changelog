package semver

import (
	"errors"
	"regexp"
	"testing"
)

// oracle is the regular expression the specification suggests in its FAQ, which
// is the only conformance artifact it ships: semver.org lists no reference
// implementation and no test suite. It sits inside RE2 — named groups spelled
// the Python way, no backreference, no lookaround — so Go compiles it as
// written.
//
// It is the oracle and not the implementation. It returns a boolean, and this
// package exists so a rejection names the rule it hit.
var oracle = regexp.MustCompile(`^(?P<major>0|[1-9]\d*)\.(?P<minor>0|[1-9]\d*)\.(?P<patch>0|[1-9]\d*)(?:-(?P<prerelease>(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+(?P<buildmetadata>[0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)

// valid is the corpus of accepted forms, grouped by the production each
// exercises. Every entry is asserted against the oracle as well as against
// Parse, so a form believed valid that the specification rejects is caught here
// rather than shipped as a fixture.
var valid = []struct {
	production string
	in         string
}{
	{"version core", "0.0.0"},
	{"version core", "0.0.4"},
	{"version core", "1.2.3"},
	{"version core", "10.20.30"},
	{"version core", "1.0.0"},
	{"version core", "2.0.0"},
	{"version core", "999999.999999.999999"},
	{"version core", "18446744073709551615.0.0"},

	{"pre-release, alphanumeric", "1.0.0-alpha"},
	{"pre-release, alphanumeric", "1.0.0-beta"},
	{"pre-release, alphanumeric", "1.0.0-rc"},
	{"pre-release, alphanumeric", "1.0.0-SNAPSHOT"},
	{"pre-release, alphanumeric", "1.0.0-alpha.beta"},
	{"pre-release, alphanumeric", "1.0.0-alpha.beta.gamma"},
	{"pre-release, alphanumeric", "1.0.0-0a"},
	{"pre-release, alphanumeric", "1.0.0-0A"},
	{"pre-release, alphanumeric", "1.0.0-00a"},

	{"pre-release, numeric", "1.0.0-0"},
	{"pre-release, numeric", "1.0.0-1"},
	{"pre-release, numeric", "1.0.0-999"},
	{"pre-release, numeric", "1.0.0-alpha.0"},
	{"pre-release, numeric", "1.0.0-alpha.1"},
	{"pre-release, numeric", "1.0.0-0.3.7"},
	{"pre-release, numeric", "1.0.0-rc.1"},
	{"pre-release, numeric", "1.0.0-99999999999999999999"},

	{"pre-release, hyphens", "1.0.0-alpha-beta"},
	{"pre-release, hyphens", "1.0.0--"},
	{"pre-release, hyphens", "1.0.0---RC-SNAPSHOT.12.9.1--.12"},
	{"pre-release, hyphens", "1.0.0-x.7.z.92"},
	{"pre-release, hyphens", "1.0.0-x-y-z.--"},

	{"build", "1.0.0+build"},
	{"build", "1.0.0+build.1"},
	{"build", "1.0.0+001"},
	{"build", "1.0.0+00"},
	{"build", "1.0.0+0"},
	{"build", "1.0.0+exp.sha.5114f85"},
	{"build", "1.0.0+21AF26D3---117B344092BD"},
	{"build", "1.0.0+-"},
	{"build", "1.0.0+build.0.0"},

	{"pre-release and build", "1.0.0-alpha+001"},
	{"pre-release and build", "1.0.0-beta+exp.sha.5114f85"},
	{"pre-release and build", "1.2.3-rc.1+build.123"},
	{"pre-release and build", "1.0.0-alpha.beta+21AF26D3"},
	{"pre-release and build", "1.0.0-0+0"},
	{"pre-release and build", "1.0.0-alpha-beta+build-1"},

	{"specification's own examples", "1.0.0-alpha.1"},
	{"specification's own examples", "1.0.0-alpha.beta.1"},
	{"specification's own examples", "1.0.0-rc.1"},
	{"specification's own examples", "1.9.0"},
	{"specification's own examples", "1.10.0"},
	{"specification's own examples", "1.11.0"},
	{"specification's own examples", "1.0.0-x.7.z.92"},
}

// invalid is the corpus of rejected forms, each with the rule it breaks. Every
// entry is asserted against the oracle too: a form believed invalid that the
// specification accepts is a defect in the belief.
var invalid = []struct {
	in   string
	rule Rule
}{
	{"", RuleEmpty},

	{"v1.2.3", RuleVPrefix},
	{"V1.2.3", RuleVPrefix},
	{"v1.0.0-alpha", RuleVPrefix},

	{"1", RuleCore},
	{"1.2", RuleCore},
	{"1.2.3.4", RuleCore},
	{"1.2.3.4.5", RuleCore},
	{"1-alpha", RuleCore},
	{"1.2-alpha", RuleCore},

	{"01.2.3", RuleLeadingZero},
	{"1.02.3", RuleLeadingZero},
	{"1.2.03", RuleLeadingZero},
	{"00.0.0", RuleLeadingZero},
	{"1.0.0-00", RuleLeadingZero},
	{"1.0.0-01", RuleLeadingZero},
	{"1.0.0-alpha.0011", RuleLeadingZero},
	{"1.0.0-0.01", RuleLeadingZero},

	{"a.b.c", RuleNumeric},
	{"1.2.x", RuleNumeric},
	{"1.x.3", RuleNumeric},
	{"x.2.3", RuleNumeric},
	{"1..3", RuleNumeric},
	{"..", RuleNumeric},
	{"1.2.3 ", RuleNumeric},
	{" 1.2.3", RuleNumeric},
	{"1.2.-3", RuleNumeric},
	{"1.2.3a", RuleNumeric},

	{"1.2.3-", RuleEmptyIdent},
	{"1.2.3+", RuleEmptyIdent},
	{"1.2.3-alpha..1", RuleEmptyIdent},
	{"1.2.3-.alpha", RuleEmptyIdent},
	{"1.2.3-alpha.", RuleEmptyIdent},
	{"1.2.3+build..1", RuleEmptyIdent},
	{"1.2.3+.build", RuleEmptyIdent},
	{"1.2.3-alpha+", RuleEmptyIdent},

	{"1.2.3-alpha_1", RuleIdentChars},
	{"1.2.3+build_1", RuleIdentChars},
	{"1.2.3-alpha+build+more", RuleIdentChars},
	{"1.2.3-alp?ha", RuleIdentChars},
	{"1.2.3-α", RuleIdentChars},
	{"1.2.3+buïld", RuleIdentChars},
}

// departures are forms the specification accepts and this package does not.
// They are asserted as disagreements so the departure is a fixture rather than
// a surprise: the specification sets no upper bound on a numeric component and
// this package reads them into uint64.
var departures = []struct {
	in   string
	rule Rule
}{
	{"18446744073709551616.0.0", RuleRange},
	{"1.18446744073709551616.0", RuleRange},
	{"0.0.99999999999999999999999", RuleRange},
}

func TestValidCorpusMatchesOracle(t *testing.T) {
	for _, c := range valid {
		t.Run(c.production+"/"+c.in, func(t *testing.T) {
			if !oracle.MatchString(c.in) {
				t.Fatalf("corpus claims %q valid, the specification's regex rejects it", c.in)
			}
			if _, err := Parse(c.in); err != nil {
				t.Fatalf("Parse(%q) = %v, want no error", c.in, err)
			}
		})
	}
}

func TestInvalidCorpusMatchesOracle(t *testing.T) {
	for _, c := range invalid {
		t.Run(c.rule.String()+"/"+c.in, func(t *testing.T) {
			if oracle.MatchString(c.in) {
				t.Fatalf("corpus claims %q invalid, the specification's regex accepts it", c.in)
			}
			_, err := Parse(c.in)
			var se *SyntaxError
			if !errors.As(err, &se) {
				t.Fatalf("Parse(%q) = %v, want a *SyntaxError", c.in, err)
			}
			if se.Rule != c.rule {
				t.Errorf("Parse(%q) broke %s, want %s: %v", c.in, se.Rule, c.rule, se)
			}
		})
	}
}

// TestDeparturesAreDisagreements pins the one place this package is narrower
// than the specification, so widening or removing the bound has to change this
// test deliberately.
func TestDeparturesAreDisagreements(t *testing.T) {
	for _, c := range departures {
		t.Run(c.in, func(t *testing.T) {
			if !oracle.MatchString(c.in) {
				t.Fatalf("%q is not a departure: the specification's regex rejects it too", c.in)
			}
			_, err := Parse(c.in)
			var se *SyntaxError
			if !errors.As(err, &se) || se.Rule != c.rule {
				t.Fatalf("Parse(%q) = %v, want %s", c.in, err, c.rule)
			}
		})
	}
}

// TestComponentsAgreeWithOracle checks the parse against the oracle's own
// capture groups, which is what makes the regex a differential oracle rather
// than a second yes/no opinion.
func TestComponentsAgreeWithOracle(t *testing.T) {
	names := oracle.SubexpNames()
	for _, c := range valid {
		t.Run(c.in, func(t *testing.T) {
			m := oracle.FindStringSubmatch(c.in)
			if m == nil {
				t.Fatalf("oracle rejects %q", c.in)
			}
			got, err := Parse(c.in)
			if err != nil {
				t.Fatalf("Parse(%q) = %v", c.in, err)
			}
			for i, name := range names {
				want := m[i]
				var have string
				switch name {
				case "major":
					have = itoa(got.Major)
				case "minor":
					have = itoa(got.Minor)
				case "patch":
					have = itoa(got.Patch)
				case "prerelease":
					have = join(got.Pre)
				case "buildmetadata":
					have = join(got.Build)
				default:
					continue
				}
				if have != want {
					t.Errorf("%s of %q = %q, oracle captured %q", name, c.in, have, want)
				}
			}
		})
	}
}

// TestStringRoundTrips checks that a parsed version writes back as it was read,
// which is what lets a caller store the parse and print the original.
func TestStringRoundTrips(t *testing.T) {
	for _, c := range valid {
		t.Run(c.in, func(t *testing.T) {
			v, err := Parse(c.in)
			if err != nil {
				t.Fatalf("Parse(%q) = %v", c.in, err)
			}
			if got := v.String(); got != c.in {
				t.Errorf("Parse(%q).String() = %q", c.in, got)
			}
		})
	}
}

// TestNumericAsymmetry is the rule no amount of reading the prose makes
// obvious: a pre-release numeric identifier may not carry a leading zero, and a
// build identifier may be bare digits.
func TestNumericAsymmetry(t *testing.T) {
	if Valid("1.0.0-00") {
		t.Error(`"1.0.0-00" parsed; a numeric pre-release identifier may not lead with zero`)
	}
	if !Valid("1.0.0+00") {
		t.Error(`"1.0.0+00" rejected; a build identifier may be bare digits`)
	}
	if !Valid("1.0.0-00a") {
		t.Error(`"1.0.0-00a" rejected; it is alphanumeric, not numeric, so the zero is fine`)
	}
}

func TestPrerelease(t *testing.T) {
	for in, want := range map[string]bool{
		"1.0.0":            false,
		"1.0.0+build":      false,
		"1.0.0-rc.1":       true,
		"1.0.0-0":          true,
		"1.0.0-rc.1+build": true,
	} {
		v, err := Parse(in)
		if err != nil {
			t.Fatalf("Parse(%q) = %v", in, err)
		}
		if got := v.Prerelease(); got != want {
			t.Errorf("Parse(%q).Prerelease() = %v, want %v", in, got, want)
		}
	}
}

// TestSyntaxErrorNamesThePart is B1722's requirement: a rejection points at the
// fragment at fault rather than restating the grammar.
func TestSyntaxErrorNamesThePart(t *testing.T) {
	for _, c := range []struct{ in, part string }{
		{"01.2.3", "01"},
		{"1.02.3", "02"},
		{"1.0.0-01", "01"},
		{"1.2.x", "x"},
		{"v1.2.3", "v"},
		{"1.2.3-alpha_1", "alpha_1"},
	} {
		var se *SyntaxError
		if _, err := Parse(c.in); !errors.As(err, &se) {
			t.Fatalf("Parse(%q) = %v, want a *SyntaxError", c.in, err)
		} else if se.Part != c.part {
			t.Errorf("Parse(%q).Part = %q, want %q", c.in, se.Part, c.part)
		}
	}
}

func (r Rule) String() string { return string(r) }

func itoa(n uint64) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func join(ids []string) string {
	out := ""
	for i, id := range ids {
		if i > 0 {
			out += "."
		}
		out += id
	}
	return out
}
