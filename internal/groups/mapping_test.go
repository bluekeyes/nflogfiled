package groups

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseMapping(t *testing.T) {
	for name, test := range map[string]struct {
		Input    string
		Mappings []Mapping
		Err      bool
	}{
		"singleGroupLiteralFile": {
			Input: "3:/var/log/drop.log",
			Mappings: []Mapping{
				{Groups: []Group{3}, File: "/var/log/drop.log"},
			},
		},
		"multipleGroupsLiteralFile": {
			Input: "3-5:/var/log/drop.log",
			Mappings: []Mapping{
				{Groups: []Group{3, 4, 5}, File: "/var/log/drop.log"},
			},
		},
		"singleGroupTemplateFile": {
			Input: "3:/var/log/drop-[group].log",
			Mappings: []Mapping{
				{Groups: []Group{3}, File: "/var/log/drop-3.log"},
			},
		},
		"multipleGroupsTemplateFile": {
			Input: "3-5:/var/log/drop-[group].log",
			Mappings: []Mapping{
				{Groups: []Group{3}, File: "/var/log/drop-3.log"},
				{Groups: []Group{4}, File: "/var/log/drop-4.log"},
				{Groups: []Group{5}, File: "/var/log/drop-5.log"},
			},
		},
		"multipleTemplateMarkers": {
			Input: "3:/var/log/group-[group]/[group].log",
			Mappings: []Mapping{
				{Groups: []Group{3}, File: "/var/log/group-3/3.log"},
			},
		},
		"invalidMissingColon": {
			Input: "/path/to/file.log",
			Err:   true,
		},
		"invalidMissingSelector": {
			Input: ":/path/to/file.log",
			Err:   true,
		},
		"invalidMissingPath": {
			Input: "3:",
			Err:   true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := ParseMapping(test.Input)
			if test.Err {
				if err == nil {
					t.Fatalf("expected parse error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("expected nil parse error, got: %v", err)
			}

			want := test.Mappings
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("incorrect mappings (-want, +got):\n%s", diff)
			}
		})
	}
}
