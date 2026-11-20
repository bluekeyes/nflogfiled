package cli

import (
	"fmt"
	"os"
	"strings"
)

// ExpandArgs replaces each argument that starts with '@' with the lines of the
// named file, after removing blank and comment lines.
func ExpandArgs(args []string) ([]string, error) {
	var expanded []string
	for _, arg := range args {
		path, ok := strings.CutPrefix(arg, "@")
		if !ok {
			expanded = append(expanded, arg)
			continue
		}

		fileArgs, err := readArgsFile(path)
		if err != nil {
			return nil, err
		}
		expanded = append(expanded, fileArgs...)
	}
	return expanded, nil
}

func readArgsFile(path string) ([]string, error) {
	if path == "" {
		return nil, fmt.Errorf("missing file path after '@'")
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read flags file: %w", err)
	}

	var args []string
	for line := range strings.Lines(string(b)) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		args = append(args, line)
	}
	return args, nil
}
