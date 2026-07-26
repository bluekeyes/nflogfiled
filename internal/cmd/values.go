package cmd

import (
	"fmt"
	"strings"
)

// StringSliceValue is a [flag.Value] that builds a slice of strings. Each call
// to Set adds the value to the slice.
type StringSliceValue struct {
	Values []string
}

func (v *StringSliceValue) String() string {
	if v == nil || len(v.Values) == 0 {
		return ""
	}

	var s strings.Builder
	s.WriteString("[")
	for i, val := range v.Values {
		if i > 0 {
			s.WriteString(", ")
		}
		fmt.Fprintf(&s, "%q", val)
	}
	s.WriteString("]")
	return s.String()
}

func (v *StringSliceValue) Set(s string) error {
	v.Values = append(v.Values, s)
	return nil
}
