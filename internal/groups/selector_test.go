package groups

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseSelector(t *testing.T) {
	for name, test := range map[string]struct {
		Selector string
		Groups   []Group
		Err      bool
	}{
		"single": {
			Selector: "3",
			Groups:   []Group{3},
		},
		"multiple": {
			Selector: "3,1,8,9",
			Groups:   []Group{1, 3, 8, 9},
		},
		"singleRange": {
			Selector: "4-12",
			Groups:   []Group{4, 5, 6, 7, 8, 9, 10, 11, 12},
		},
		"mixed": {
			Selector: "2,4,7-10",
			Groups:   []Group{2, 4, 7, 8, 9, 10},
		},
		"duplicateLiteral": {
			Selector: "3,5,7,3,1,5",
			Groups:   []Group{1, 3, 5, 7},
		},
		"duplicateRange": {
			Selector: "1,2,5-9,6,7,8-10",
			Groups:   []Group{1, 2, 5, 6, 7, 8, 9, 10},
		},
		"whitespace": {
			Selector: "  2,4 ,7 -  10  ",
			Groups:   []Group{2, 4, 7, 8, 9, 10},
		},
		"empty": {
			Selector: "",
			Err:      true,
		},
		"invalidNumbers": {
			Selector: "1,3,four",
			Err:      true,
		},
		"negativeNumbers": {
			Selector: "-3,-8",
			Err:      true,
		},
		"zeroGroup": {
			Selector: "0,1,2",
			Err:      true,
		},
		"flippedRange": {
			Selector: "5-2,3",
			Err:      true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := ParseSelector(test.Selector)
			if test.Err {
				if err == nil {
					t.Fatalf("expected parse error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("expected nil parse error, got: %v", err)
			}

			want := test.Groups
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("incorrect groups (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestFormatSelector(t *testing.T) {
	for name, test := range map[string]struct {
		Groups   []Group
		Selector string
	}{
		"single": {
			Groups:   []Group{3},
			Selector: "3",
		},
		"mixed": {
			Groups:   []Group{3, 1, 8, 9, 10},
			Selector: "1,3,8-10",
		},
		"singleRange": {
			Groups:   []Group{4, 5, 6, 7, 8, 9, 10, 11, 12},
			Selector: "4-12",
		},
		"dualRange": {
			Groups:   []Group{1, 2, 5, 6, 7, 8, 9, 10},
			Selector: "1-2,5-10",
		},
		"duplicatesOutOfOrder": {
			Groups:   []Group{3, 5, 7, 3, 1, 5},
			Selector: "1,3,5,7",
		},
		"empty": {
			Groups:   nil,
			Selector: "",
		},
	} {
		t.Run(name, func(t *testing.T) {
			got := FormatSelector(test.Groups)
			if want := test.Selector; got != want {
				t.Errorf("incorrect selector, want %q, got %q", want, got)
			}
		})
	}
}
