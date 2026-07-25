package groups

import (
	"fmt"
	"strconv"
	"strings"
)

type Group uint16

type Mapping struct {
	Groups []Group
	File   string
}

// ParseMapping parses a group mapping string. A mapping string consists of a
// group selector (see [ParseSelector]) separated from a file path by a colon.
// The file path may contain the literal string "[group]" at zero or more
// locations. ParseMapping replaces each instance of this string in the file
// path with each group in the selector, creating a unique file path for each
// selected group. If the file path does not contain the group marker, then all
// of the selected groups map to the same file path.
func ParseMapping(mapping string) ([]Mapping, error) {
	const (
		groupMarker = "[group]"
	)

	selector, path, _ := strings.Cut(strings.TrimSpace(mapping), ":")
	if selector == "" || path == "" {
		return nil, fmt.Errorf("group mapping must contain a group selector and a file path")
	}

	groups, err := ParseSelector(selector)
	if err != nil {
		return nil, fmt.Errorf("invalid group selector: %w", err)
	}

	var mappings []Mapping
	if strings.Contains(path, groupMarker) {
		for _, g := range groups {
			mappings = append(mappings, Mapping{
				Groups: []Group{g},
				File:   strings.ReplaceAll(path, groupMarker, strconv.FormatUint(uint64(g), 10)),
			})
		}
	} else {
		mappings = append(mappings, Mapping{
			Groups: groups,
			File:   path,
		})
	}

	return mappings, nil
}
