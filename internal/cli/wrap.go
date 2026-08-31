package cli

import (
	"bytes"
	"strings"
)

// WrapText formats a single line of text in to multiple lines where each line
// contains no more than length characters. It replaces all whitespace with
// single space characters.
func WrapText(text string, length int) string {
	var s strings.Builder

	n := 0
	for w := range strings.FieldsSeq(text) {
		if n > 0 && n+1+len(w) > length {
			s.WriteString("\n")
			n = 0
		}
		if n > 0 {
			s.WriteString(" ")
			n++
		}
		s.WriteString(w)
		n += len(w)
	}

	return s.String()
}

// IndentText formats text so that each non-empty line starts with indent
// spaces. If skipFirstLine is true, it only indents lines after the first.
func IndentText(text string, indent int, skipFirstLine bool) string {
	var s strings.Builder

	indentBytes := bytes.Repeat([]byte(" "), indent)

	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if i > 0 {
			s.WriteString("\n")
		}
		if (i > 0 || !skipFirstLine) && line != "" {
			s.Write(indentBytes)
		}
		s.WriteString(line)
	}

	return s.String()
}
