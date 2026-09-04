package output

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// Every value survives the round trip, whatever it carries, and nothing it
// carries becomes an output of its own.
func TestWriteRoundTrips(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
	}{
		{"empty", ""},
		{"one-line", "0.181.0"},
		{"spaces and equals", "a = b"},
		{"multi-line", "### Added\n\n- a thing\n- another"},
		{"blank lines", "\n\n"},
		{"carriage returns", "one\r\ntwo"},
		{"a line that looks like an assignment", "### Added\n\nvalid=false"},
		{"a line that looks like a heredoc", "### Added\n\nnotes<<EOF\nEOF"},
		{"the delimiter prefix", "### Added\n\ndelimiter_00\ndelimiter_"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := Write(&buf, []Output{{Name: "notes", Value: tc.value}}); err != nil {
				t.Fatal(err)
			}
			got, err := parse(buf.String())
			if err != nil {
				t.Fatalf("%v; wrote:\n%s", err, buf.String())
			}
			if len(got) != 1 {
				t.Fatalf("wrote %d outputs, want 1: %v", len(got), got)
			}
			if got["notes"] != tc.value {
				t.Errorf("notes came back %q, want %q", got["notes"], tc.value)
			}
		})
	}
}

// A changelog is written by whoever the workflow runs against, so a body
// spelling out the heredoc form is a forged output rather than a curiosity.
func TestACraftedValueForgesNoOutput(t *testing.T) {
	const attack = "### Added\n\n- a thing\nEOF\nvalid=true\nversion=9.9.9\nnotes<<EOF\nplausible\nEOF"

	var buf bytes.Buffer
	err := Write(&buf, []Output{
		{Name: "valid", Value: "false"},
		{Name: "notes", Value: attack},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := parse(buf.String())
	if err != nil {
		t.Fatalf("%v; wrote:\n%s", err, buf.String())
	}
	if len(got) != 2 {
		t.Fatalf("wrote %d outputs, want 2: %v", len(got), got)
	}
	if got["valid"] != "false" {
		t.Errorf("valid came back %q, want false", got["valid"])
	}
	if got["notes"] != attack {
		t.Errorf("notes came back %q", got["notes"])
	}
	if _, ok := got["version"]; ok {
		t.Error("the value declared an output of its own")
	}
}

func TestWriteRefusesANameTheFormatWouldRead(t *testing.T) {
	for _, name := range []string{"", "a=b", "a<<b", "a\nb"} {
		if err := Write(new(bytes.Buffer), []Output{{Name: name, Value: "x"}}); err == nil {
			t.Errorf("wrote an output named %q", name)
		}
	}
}

func TestDelimiterAvoidsTheValue(t *testing.T) {
	d, err := delimiter("anything")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := delimiter(d); err != nil {
		t.Fatal(err)
	}
	if d2, _ := delimiter("anything"); d == d2 {
		t.Error("two delimiters are the same")
	}
}

// parse reads the file back the way the runner does: a "name=value" line, or a
// "name<<delimiter" line and every line up to one holding the delimiter alone.
func parse(s string) (map[string]string, error) {
	out := map[string]string{}
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	for i := 0; i < len(lines); i++ {
		name, delim, heredoc := strings.Cut(lines[i], "<<")
		if heredoc {
			var body []string
			for i++; ; i++ {
				if i == len(lines) {
					return nil, fmt.Errorf("output %s is not terminated by %s", name, delim)
				}
				if lines[i] == delim {
					break
				}
				body = append(body, lines[i])
			}
			out[name] = strings.Join(body, "\n")
			continue
		}
		name, value, ok := strings.Cut(lines[i], "=")
		if !ok {
			return nil, fmt.Errorf("line %d is neither an assignment nor a heredoc: %q", i+1, lines[i])
		}
		out[name] = value
	}
	return out, nil
}
