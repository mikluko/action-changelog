package changelog

import (
	"fmt"
	"sort"
	"strings"
)

// Severity is how bad a finding is. Off suppresses the check entirely: a check
// at Off produces no finding at all rather than a finding a caller must filter.
type Severity int

const (
	Off Severity = iota
	Warning
	Error
)

// String returns the spelling the flags and the printed register use.
func (s Severity) String() string {
	switch s {
	case Off:
		return "off"
	case Warning:
		return "warning"
	case Error:
		return "error"
	}
	return fmt.Sprintf("Severity(%d)", int(s))
}

// ParseSeverity reads the spelling String writes.
func ParseSeverity(s string) (Severity, error) {
	switch strings.TrimSpace(s) {
	case "off":
		return Off, nil
	case "warning":
		return Warning, nil
	case "error":
		return Error, nil
	}
	return Off, fmt.Errorf("severity %q is not one of off, warning, error", s)
}

// The name of each check, as it is written in the register, in a finding, in
// the GitHub Actions annotation, and in the flags that reconfigure it.
const (
	CheckHeadingForm    = "heading-form"
	CheckDateFormat     = "date-format"
	CheckVersionOrder   = "version-order"
	CheckEmptyEntry     = "empty-entry"
	CheckUnknownSection = "unknown-section"
)

// Check is one thing the linter looks for.
type Check struct {
	Name        string
	Description string
	Default     Severity
}

// Checks is the register: every check this package knows how to run, its
// one-line description and the severity it carries unless a caller says
// otherwise.
//
// It is the single source. The command's --list-checks prints it, the README
// table is generated from it, and a finding names a row of it, so a check added
// here needs no second declaration anywhere.
var Checks = []Check{
	{
		Name:        CheckHeadingForm,
		Description: "An entry heading states a version and a date, as in [1.2.3] - 2006-01-02.",
		Default:     Error,
	},
	{
		Name:        CheckDateFormat,
		Description: "An entry's date is written YYYY-MM-DD.",
		Default:     Error,
	},
	{
		Name:        CheckVersionOrder,
		Description: "Entries run newest first, each version strictly below the one above it.",
		Default:     Error,
	},
	{
		Name:        CheckEmptyEntry,
		Description: "A released entry carries something under it.",
		Default:     Error,
	},
	{
		Name:        CheckUnknownSection,
		Description: "A level-3 heading is one of the accepted section vocabulary.",
		Default:     Error,
	},
}

// Lookup returns the registered check by name.
func Lookup(name string) (Check, bool) {
	for _, c := range Checks {
		if c.Name == name {
			return c, true
		}
	}
	return Check{}, false
}

// Severities is the severity in force for each check, keyed by check name.
type Severities map[string]Severity

// DefaultSeverities is every registered check at the severity the register
// gives it.
func DefaultSeverities() Severities {
	out := make(Severities, len(Checks))
	for _, c := range Checks {
		out[c.Name] = c.Default
	}
	return out
}

// Set puts name at sev, reporting an error naming the register's contents when
// no such check exists. A name nobody registered is a typo in a workflow rather
// than a check that happens to be silent, so it is refused rather than stored.
func (s Severities) Set(name string, sev Severity) error {
	if _, ok := Lookup(name); !ok {
		return fmt.Errorf("no check named %q; the checks are %s", name, strings.Join(names(), ", "))
	}
	s[name] = sev
	return nil
}

func names() []string {
	out := make([]string, 0, len(Checks))
	for _, c := range Checks {
		out = append(out, c.Name)
	}
	sort.Strings(out)
	return out
}

// findings accumulates what a lint run reports, dropping whatever the caller
// turned off and stamping each finding with the check that raised it.
type findings struct {
	severities Severities
	out        []Finding
}

// add records a finding against a registered check.
//
// A check name absent from the register is a programming error rather than
// anything a document can provoke, so it panics: the alternative is a finding
// whose severity nothing configures, reported as though it were configurable.
func (f *findings) add(check string, line int, format string, args ...any) {
	sev, ok := f.severities[check]
	if !ok {
		if _, registered := Lookup(check); !registered {
			panic("changelog: finding raised against unregistered check " + check)
		}
		sev = Off
	}
	if sev == Off {
		return
	}
	f.out = append(f.out, Finding{
		Check:    check,
		Severity: sev,
		Line:     line,
		Msg:      fmt.Sprintf(format, args...),
	})
}
