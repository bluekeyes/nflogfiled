package logfile

import (
	"io"
	"os"
	"path/filepath"
)

// Open opens the file at name for writing, creating it and any parent
// directories if they doesn't exist.
func Open(name string, mode os.FileMode) (io.WriteCloser, error) {
	dir := filepath.Dir(name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(name, os.O_CREATE|os.O_APPEND|os.O_WRONLY, mode)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// Stdout returns an io.WriteCloser for standard output. Closing the writer
// does not close underlying stdout handle.
func Stdout() io.WriteCloser {
	return stdoutWriteCloser{}
}

type stdoutWriteCloser struct{}

func (w stdoutWriteCloser) Write(p []byte) (n int, err error) {
	return os.Stdout.Write(p)
}

func (w stdoutWriteCloser) Close() error {
	return nil
}
