// Package output writes the values an action reports in the format
// $GITHUB_OUTPUT accepts, which is also what a local run prints.
package output

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Output is one name and the value reported under it.
type Output struct {
	Name  string
	Value string
}

// Write emits outs, one per line where the value allows it and in the heredoc
// form where it does not.
//
// The delimiter is drawn at random and verified absent from the value, and a
// name carrying a character the format reads is refused. Values here come from
// a file the repository under test controls: with a fixed delimiter such a file
// ends the block early, and with the "name=value" form a newline in it declares
// further outputs outright.
func Write(w io.Writer, outs []Output) error {
	for _, o := range outs {
		if o.Name == "" || strings.ContainsAny(o.Name, "\n\r=<") {
			return fmt.Errorf("output name %q is empty or carries a character the format reads", o.Name)
		}
		if !strings.ContainsAny(o.Value, "\n\r") {
			if _, err := fmt.Fprintf(w, "%s=%s\n", o.Name, o.Value); err != nil {
				return err
			}
			continue
		}
		delim, err := delimiter(o.Value)
		if err != nil {
			return fmt.Errorf("output %s: %w", o.Name, err)
		}
		if _, err := fmt.Fprintf(w, "%s<<%s\n%s\n%s\n", o.Name, delim, o.Value, delim); err != nil {
			return err
		}
	}
	return nil
}

// delimiter returns a token that does not occur in value.
//
// Sixteen random bytes do not occur in a changelog, so the redraw is a
// formality. It is here because the failure it stands in the way of is an
// output the value forged rather than an output that came out wrong.
func delimiter(value string) (string, error) {
	for range 8 {
		var b [16]byte
		if _, err := rand.Read(b[:]); err != nil {
			return "", err
		}
		d := "delimiter_" + hex.EncodeToString(b[:])
		if !strings.Contains(value, d) {
			return d, nil
		}
	}
	return "", errors.New("no delimiter is absent from the value")
}
