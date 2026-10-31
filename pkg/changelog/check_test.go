package changelog

import (
	"strings"
	"testing"
)

// The register is what --list-checks prints and what the README table is
// generated from, so a row missing either half ships a check nobody can look up.
func TestEveryCheckIsNamedAndDescribed(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range Checks {
		if c.Name == "" {
			t.Errorf("check with description %q has no name", c.Description)
			continue
		}
		if seen[c.Name] {
			t.Errorf("check %q is registered twice", c.Name)
		}
		seen[c.Name] = true

		if strings.TrimSpace(c.Description) == "" {
			t.Errorf("check %q has no description", c.Name)
		}
		if c.Default == Off {
			continue
		}
		if c.Default != Warning && c.Default != Error {
			t.Errorf("check %q defaults to %s", c.Name, c.Default)
		}
	}
}

func TestSeveritiesRefusesAnUnregisteredCheck(t *testing.T) {
	s := DefaultSeverities()
	if err := s.Set("no-such-check", Off); err == nil {
		t.Fatal("Set accepted a check nobody registered")
	}
	if err := s.Set(CheckDateFormat, Warning); err != nil {
		t.Fatalf("Set refused a registered check: %v", err)
	}
	if s[CheckDateFormat] != Warning {
		t.Errorf("date-format is %s after being set to warning", s[CheckDateFormat])
	}
}

// A check at Off raises nothing at all, which is what lets a workflow disable
// one and still read a clean result rather than filtering findings itself.
func TestLintDropsWhatIsSwitchedOff(t *testing.T) {
	lines := []string{"# Changelog", "", "## [1.0.0] - 4 Sep 2026", "", "- a thing"}

	sev := DefaultSeverities()
	if got := Parse([]byte(strings.Join(lines, "\n"))).Lint(Options{Severities: sev}); len(got) != 1 {
		t.Fatalf("findings %v, want exactly the date-format one", got)
	}
	if err := sev.Set(CheckDateFormat, Off); err != nil {
		t.Fatal(err)
	}
	if got := Parse([]byte(strings.Join(lines, "\n"))).Lint(Options{Severities: sev}); len(got) != 0 {
		t.Errorf("findings %v with date-format off, want none", got)
	}
}

func TestLintCarriesTheConfiguredSeverity(t *testing.T) {
	sev := DefaultSeverities()
	if err := sev.Set(CheckDateFormat, Warning); err != nil {
		t.Fatal(err)
	}
	src := "# Changelog\n\n## [1.0.0] - 4 Sep 2026\n\n- a thing\n"
	got := Parse([]byte(src)).Lint(Options{Severities: sev})
	if len(got) != 1 {
		t.Fatalf("findings %v, want exactly one", got)
	}
	if got[0].Severity != Warning {
		t.Errorf("finding %v at %s, want warning", got[0], got[0].Severity)
	}
}

func TestSeverityRoundTrips(t *testing.T) {
	for _, s := range []Severity{Off, Warning, Error} {
		got, err := ParseSeverity(s.String())
		if err != nil {
			t.Errorf("ParseSeverity(%q): %v", s, err)
			continue
		}
		if got != s {
			t.Errorf("ParseSeverity(%q) is %s", s, got)
		}
	}
	if _, err := ParseSeverity("fatal"); err == nil {
		t.Error("ParseSeverity accepted \"fatal\"")
	}
}
