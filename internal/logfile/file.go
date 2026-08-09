package logfile

import (
	"os"
	"path/filepath"
)

type File struct {
	f   *os.File
	std bool
}

// Open opens the file at name for writing, creating it and any parent
// directories if they doesn't exist.
func Open(name string, mode os.FileMode) (*File, error) {
	dir := filepath.Dir(name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY, mode)
	if err != nil {
		return nil, err
	}

	return &File{f: f}, nil
}

func (f *File) Write(p []byte) (n int, err error) {
	return f.f.Write(p)
}

func (f *File) Close() error {
	if f.std {
		return nil
	}
	return f.f.Close()
}

// Stdout returns a File for standard output. Closing the File does not close
// underlying stdout handle.
func Stdout() *File {
	return &File{
		f:   os.Stdout,
		std: true,
	}
}
