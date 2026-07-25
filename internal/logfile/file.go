package logfile

import (
	"bufio"
	"os"
)

type File struct {
	f   *os.File
	w   *bufio.Writer
	std bool
}

// Open opens the file at name for writing, creating if it doesn't exist. If
// bufferred is true, writes are bufferred in memory.
func Open(name string, mode os.FileMode, bufferred bool) (*File, error) {
	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY, mode)
	if err != nil {
		return nil, err
	}

	var w *bufio.Writer
	if bufferred {
		w = bufio.NewWriter(f)
	}

	return &File{f: f, w: w}, nil
}

func (f *File) Write(p []byte) (n int, err error) {
	if f.w != nil {
		return f.w.Write(p)
	} else {
		return f.f.Write(p)
	}
}

func (f *File) Close() error {
	if f.w != nil {
		if err := f.w.Flush(); err != nil {
			return err
		}
	}
	if f.std {
		return nil
	}
	return f.f.Close()
}

// Stdout returns a File for standard output. Closing the File flushes pending
// writes but does not close underlying stdout handle.
func Stdout(bufferred bool) *File {
	var w *bufio.Writer
	if bufferred {
		w = bufio.NewWriter(os.Stdout)
	}

	return &File{
		f:   os.Stdout,
		w:   w,
		std: true,
	}
}
