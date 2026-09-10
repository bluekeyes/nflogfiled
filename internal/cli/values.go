package cli

import (
	"fmt"
	"io/fs"
	"log/slog"
	"strconv"
	"strings"

	"github.com/bluekeyes/nflogfiled/internal/groups"
	"github.com/bluekeyes/nflogfiled/internal/message"
)

// GroupMappingValue is a [flag.Value] that builds a slice of [groups.Mapping].
// Each call to Set adds additional mappings to the slice.
type GroupMappingValue struct {
	Values []groups.Mapping
}

func (v *GroupMappingValue) String() string {
	if v == nil {
		return ""
	}

	var s strings.Builder
	s.WriteString("[")
	for i, mapping := range v.Values {
		if i > 0 {
			s.WriteString(", ")
		}
		s.WriteString(groups.FormatSelector(mapping.Groups))
		s.WriteString(":")
		s.WriteString(mapping.File)
	}
	s.WriteString("]")
	return s.String()
}

func (v *GroupMappingValue) Set(s string) error {
	mapping, err := groups.ParseMapping(s)
	if err != nil {
		return err
	}
	v.Values = append(v.Values, mapping...)
	return nil
}

// FieldSetValue is a [flag.Value] that parses a comma-separated list of
// output field names into a [message.FieldSet]. Each call to Set replaces
// the stored value.
type FieldSetValue struct {
	Value message.FieldSet
}

func (v *FieldSetValue) String() string {
	if v == nil {
		return ""
	}
	return v.Value.String()
}

func (v *FieldSetValue) Set(s string) error {
	fs, err := message.ParseFieldSet(s)
	if err != nil {
		return err
	}
	v.Value = fs
	return nil
}

// FileModeValue is a [flag.Value] that parses an octal string into a
// [fs.FileMode]. Each call to Set replaces the stored value.
type FileModeValue struct {
	Value fs.FileMode
}

func (v *FileModeValue) String() string {
	if v == nil {
		return ""
	}
	return strconv.FormatUint(uint64(v.Value), 8)
}

func (v *FileModeValue) Set(s string) error {
	mode, err := strconv.ParseUint(s, 8, 32)
	if err != nil {
		return fmt.Errorf("invalid file mode: %q", s)
	}
	v.Value = fs.FileMode(mode).Perm()
	return nil
}

// LogLevelValue is a [flag.Value] that parses the string representation of an
// [slog.Level]. Each call to Set replaces the stored value.
type LogLevelValue struct {
	Value slog.Level
}

func (v *LogLevelValue) String() string {
	if v == nil {
		return ""
	}
	return v.Value.String()
}

func (v *LogLevelValue) Set(s string) error {
	return v.Value.UnmarshalText([]byte(s))
}
