package groups

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
)

type Group uint16

// ParseSelector parses a group selector string. A group selector string is a
// comma separated list of positive integers with optional ranges. Ranges take
// the form `start-end` and are inclusive. For example, the the selector string
// "1,3,5-7" selects the groups 1, 3, 5, 6, and 7. Parsing ignores all
// whitespace in the selector. Selectors must specify at least one group; the
// empty selector is not valid.
//
// The resulting list of groups has no duplicates and is in ascending order.
func ParseSelector(selector string) ([]Group, error) {
	groups := make(map[Group]struct{})

	selector = strings.TrimSpace(selector)
	if selector == "" {
		return nil, fmt.Errorf("must select at least one group")
	}

	for part := range strings.SplitSeq(selector, ",") {
		part = strings.TrimSpace(part)
		if start, end, ok := strings.Cut(part, "-"); ok {
			s, err := trimAndParseGroup(start)
			if err != nil {
				return nil, err
			}
			e, err := trimAndParseGroup(end)
			if err != nil {
				return nil, err
			}
			if s > e {
				return nil, fmt.Errorf("invalid group range: %d is after %d", s, e)
			}
			for g := s; g <= e; g++ {
				groups[g] = struct{}{}
			}
		} else {
			g, err := trimAndParseGroup(part)
			if err != nil {
				return nil, err
			}
			groups[g] = struct{}{}
		}
	}
	return slices.Sorted(maps.Keys(groups)), nil
}

func trimAndParseGroup(g string) (Group, error) {
	n, err := strconv.ParseUint(strings.TrimSpace(g), 10, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid group value: %s", g)
	}
	if n == 0 {
		return 0, fmt.Errorf("invalid group value: groups must be positive")
	}
	return Group(n), nil
}
