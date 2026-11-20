package cli

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestExpandArgs(t *testing.T) {
	dir := t.TempDir()

	writeFile := func(name, content string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		return path
	}

	basic := writeFile("basic", "-format=json\n-map=1:/var/log/a.log\n")
	comments := writeFile("comments", "# header comment\n\n-format=json\n   # indented comment\n\n\n-log-level=debug\n")
	whitespace := writeFile("whitespace", "  -format=json  \n\t-log-level=debug\t\r\n-file-mode=0640")
	spaces := writeFile("spaces", "-map=1:/var/log/with space.log\n-format json\n")
	nested := writeFile("nested", "-format=json\n@"+basic+"\n")
	empty := writeFile("empty", "# nothing here\n\n")

	tests := map[string]struct {
		Args []string
		Want []string
	}{
		"noFiles": {
			Args: []string{"-format=json", "-map=1:-"},
			Want: []string{"-format=json", "-map=1:-"},
		},
		"onlyFile": {
			Args: []string{"@" + basic},
			Want: []string{"-format=json", "-map=1:/var/log/a.log"},
		},
		"ordering": {
			Args: []string{"-format=text", "@" + basic, "-map=2:-"},
			Want: []string{"-format=text", "-format=json", "-map=1:/var/log/a.log", "-map=2:-"},
		},
		"repeatedFile": {
			Args: []string{"@" + basic, "@" + basic},
			Want: []string{"-format=json", "-map=1:/var/log/a.log", "-format=json", "-map=1:/var/log/a.log"},
		},
		"commentsAndBlankLines": {
			Args: []string{"@" + comments},
			Want: []string{"-format=json", "-log-level=debug"},
		},
		"whitespaceTrimming": {
			Args: []string{"@" + whitespace},
			Want: []string{"-format=json", "-log-level=debug", "-file-mode=0640"},
		},
		"valuesWithSpaces": {
			Args: []string{"@" + spaces},
			Want: []string{"-map=1:/var/log/with space.log", "-format json"},
		},
		"nestedIsLiteral": {
			Args: []string{"@" + nested},
			Want: []string{"-format=json", "@" + basic},
		},
		"emptyFile": {
			Args: []string{"-format=json", "@" + empty},
			Want: []string{"-format=json"},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := ExpandArgs(test.Args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(test.Want, got); diff != "" {
				t.Errorf("incorrect args (-want +got):\n%s", diff)
			}
		})
	}
}

func TestExpandArgsErrors(t *testing.T) {
	t.Run("missingFile", func(t *testing.T) {
		_, err := ExpandArgs([]string{"@" + filepath.Join(t.TempDir(), "missing")})
		if !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("expected not exist error, got: %v", err)
		}
	})

	t.Run("emptyPath", func(t *testing.T) {
		if _, err := ExpandArgs([]string{"@"}); err == nil {
			t.Fatal("expected error for empty path, got nil")
		}
	})

	t.Run("directory", func(t *testing.T) {
		if _, err := ExpandArgs([]string{"@" + t.TempDir()}); err == nil {
			t.Fatal("expected error for directory, got nil")
		}
	})
}
