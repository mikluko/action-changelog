package changelog

import "fmt"

// maxOCITagLen is the length limit the OCI distribution specification puts on a
// tag: a leading character and at most 127 more.
const maxOCITagLen = 128

// ociTagReason says why version cannot be an OCI tag, or "" where it can.
//
// The OCI distribution specification requires a tag to be at most 128
// characters and to match [a-zA-Z0-9_][a-zA-Z0-9._-]{0,127}. Semantic
// Versioning is the wider set: "+" introduces build metadata and appears
// nowhere in that grammar, and a pre-release run has no length bound at all.
//
// Git ref syntax permits "+", so a version carrying build metadata cuts a tag
// and is then not a name a registry will take. The two specifications disagree
// about the same string, which is why this is a fact about the version rather
// than about the document that names it.
func ociTagReason(version string) string {
	if version == "" {
		return ""
	}
	if n := len(version); n > maxOCITagLen {
		return fmt.Sprintf("an OCI tag is at most %d characters and this is %d", maxOCITagLen, n)
	}
	if c := version[0]; !isOCIFirst(c) {
		return fmt.Sprintf("an OCI tag opens with a letter, a digit or an underscore, and this opens with %q", string(c))
	}
	for i := 1; i < len(version); i++ {
		if c := version[i]; !isOCIRest(c) {
			return fmt.Sprintf(`an OCI tag holds only letters, digits, ".", "-" and "_", and this holds %q`, string(c))
		}
	}
	return ""
}

func isOCIFirst(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_'
}

func isOCIRest(c byte) bool {
	return isOCIFirst(c) || c == '.' || c == '-'
}
