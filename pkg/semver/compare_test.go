package semver

import (
	"sort"
	"testing"
)

func mustParse(t *testing.T, s string) Version {
	t.Helper()
	v, err := Parse(s)
	if err != nil {
		t.Fatalf("Parse(%q) = %v", s, err)
	}
	return v
}

// ascending is the specification's own worked example of precedence, section 11,
// extended with the numeric cases the prose calls out but does not list.
var ascending = []string{
	"1.0.0-alpha",
	"1.0.0-alpha.1",
	"1.0.0-alpha.beta",
	"1.0.0-beta",
	"1.0.0-beta.2",
	"1.0.0-beta.11",
	"1.0.0-rc.1",
	"1.0.0",
	"1.0.1",
	"1.1.0",
	"2.0.0",
	"2.1.0",
	"2.1.1",
	"10.0.0",
}

func TestCompareOrdersTheSpecificationsExample(t *testing.T) {
	for i := range ascending {
		for j := range ascending {
			a, b := mustParse(t, ascending[i]), mustParse(t, ascending[j])
			want := cmpInt(i, j)
			if got := Compare(a, b); got != want {
				t.Errorf("Compare(%q, %q) = %d, want %d", ascending[i], ascending[j], got, want)
			}
		}
	}
}

func TestSortUsesCompare(t *testing.T) {
	shuffled := []string{
		"2.1.1", "1.0.0-beta.11", "1.0.0", "1.0.0-alpha", "2.0.0",
		"1.0.0-rc.1", "1.0.1", "1.0.0-beta.2", "10.0.0", "1.0.0-alpha.beta",
		"1.1.0", "1.0.0-beta", "1.0.0-alpha.1", "2.1.0",
	}
	vs := make([]Version, len(shuffled))
	for i, s := range shuffled {
		vs[i] = mustParse(t, s)
	}
	sort.Slice(vs, func(i, j int) bool { return Compare(vs[i], vs[j]) < 0 })
	for i, want := range ascending {
		if got := vs[i].String(); got != want {
			t.Errorf("sorted[%d] = %q, want %q", i, got, want)
		}
	}
}

// Section 10: build metadata is ignored for precedence, so two versions
// differing only there are equal and are still different strings.
func TestCompareIgnoresBuildMetadata(t *testing.T) {
	for _, pair := range [][2]string{
		{"1.0.0+build.1", "1.0.0"},
		{"1.0.0+a", "1.0.0+b"},
		{"1.0.0-rc.1+x", "1.0.0-rc.1+y"},
		{"1.0.0+001", "1.0.0+1"},
	} {
		a, b := mustParse(t, pair[0]), mustParse(t, pair[1])
		if got := Compare(a, b); got != 0 {
			t.Errorf("Compare(%q, %q) = %d, want 0", pair[0], pair[1], got)
		}
		if a.String() == b.String() {
			t.Errorf("%q and %q compare equal and should still be different strings", pair[0], pair[1])
		}
	}
}

// A numeric identifier has no upper bound, and it is compared numerically. It
// carries no leading zero, so the longer of two is the larger without the value
// ever being read as a number.
func TestCompareNumericIdentifiersHaveNoBound(t *testing.T) {
	for _, asc := range [][]string{
		{"1.0.0-1", "1.0.0-2", "1.0.0-11", "1.0.0-99", "1.0.0-100"},
		{"1.0.0-9", "1.0.0-10", "1.0.0-99999999999999999999", "1.0.0-999999999999999999999999999999"},
	} {
		for i := 1; i < len(asc); i++ {
			a, b := mustParse(t, asc[i-1]), mustParse(t, asc[i])
			if Compare(a, b) >= 0 {
				t.Errorf("Compare(%q, %q) >= 0, want %q to sort first", asc[i-1], asc[i], asc[i-1])
			}
		}
	}
}

// A numeric identifier ranks below one that is not, whatever their sizes.
func TestCompareNumericRanksBelowAlphanumeric(t *testing.T) {
	for _, pair := range [][2]string{
		{"1.0.0-1", "1.0.0-alpha"},
		{"1.0.0-99999999999999999999", "1.0.0-a"},
		{"1.0.0-alpha.1", "1.0.0-alpha.a"},
		{"1.0.0-0", "1.0.0--"},
	} {
		a, b := mustParse(t, pair[0]), mustParse(t, pair[1])
		if Compare(a, b) != -1 {
			t.Errorf("Compare(%q, %q) = %d, want -1", pair[0], pair[1], Compare(a, b))
		}
	}
}

// Alphanumeric identifiers compare in ASCII order, which puts every upper-case
// letter below every lower-case one and the hyphen below both.
func TestCompareAlphanumericIsASCIIOrder(t *testing.T) {
	for _, asc := range [][]string{
		{"1.0.0--", "1.0.0-A", "1.0.0-Z", "1.0.0-a", "1.0.0-z"},
		{"1.0.0-RC", "1.0.0-rc"},
	} {
		for i := 1; i < len(asc); i++ {
			a, b := mustParse(t, asc[i-1]), mustParse(t, asc[i])
			if Compare(a, b) >= 0 {
				t.Errorf("Compare(%q, %q) >= 0, want %q first", asc[i-1], asc[i], asc[i-1])
			}
		}
	}
}

func TestParseTagExpandsShorthand(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"v1", "1.0.0"},
		{"V1", "1.0.0"},
		{"1", "1.0.0"},
		{"v1.2", "1.2.0"},
		{"1.2", "1.2.0"},
		{"v1.2.3", "1.2.3"},
		{"1.2.3", "1.2.3"},
		{"v1.2.3-rc.1", "1.2.3-rc.1"},
		{"v1-rc.1", "1.0.0-rc.1"},
		{"v1.2+build", "1.2.0+build"},
		{" v1.2.3 ", "1.2.3"},
	} {
		got, err := ParseTag(tc.in)
		if err != nil {
			t.Errorf("ParseTag(%q) = %v", tc.in, err)
			continue
		}
		if got.String() != tc.want {
			t.Errorf("ParseTag(%q) = %q, want %q", tc.in, got.String(), tc.want)
		}
	}
}

// The moving tag the GitHub Actions convention keeps beside a full one reads as
// the same version, which is what leaves it a candidate for the reference tag.
func TestParseTagMovingTagEqualsFullTag(t *testing.T) {
	short, err := ParseTag("v1")
	if err != nil {
		t.Fatal(err)
	}
	full, err := ParseTag("v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if Compare(short, full) != 0 {
		t.Errorf("v1 and v1.0.0 compare %d, want 0", Compare(short, full))
	}
}

// Shorthand is the only thing ParseTag relaxes. Everything the specification
// rejects inside a component or an identifier is still rejected.
func TestParseTagRelaxesNothingElse(t *testing.T) {
	for _, tc := range []struct {
		in   string
		rule Rule
	}{
		{"", RuleEmpty},
		{"v", RuleNumeric},
		{"vv1.2.3", RuleVPrefix},
		{"v01.2.3", RuleLeadingZero},
		{"v1.2.3-01", RuleLeadingZero},
		{"v1.2.3.4", RuleCore},
		{"v1.x", RuleNumeric},
		{"v1.2.3-", RuleEmptyIdent},
		{"v1.2.3-a_b", RuleIdentChars},
	} {
		_, err := ParseTag(tc.in)
		se, ok := err.(*SyntaxError)
		if !ok {
			t.Errorf("ParseTag(%q) = %v, want a *SyntaxError", tc.in, err)
			continue
		}
		if se.Rule != tc.rule {
			t.Errorf("ParseTag(%q) broke %s, want %s", tc.in, se.Rule, tc.rule)
		}
	}
}

func TestCanonicalDropsBuildMetadata(t *testing.T) {
	v := mustParse(t, "1.2.3-rc.1+build.9")
	if got := v.Canonical().String(); got != "1.2.3-rc.1" {
		t.Errorf("Canonical() = %q, want %q", got, "1.2.3-rc.1")
	}
	if got := v.String(); got != "1.2.3-rc.1+build.9" {
		t.Errorf("Canonical() mutated the receiver: %q", got)
	}
	if got := v.Canonical().Tag(); got != "v1.2.3-rc.1" {
		t.Errorf("Tag() = %q, want %q", got, "v1.2.3-rc.1")
	}
}
