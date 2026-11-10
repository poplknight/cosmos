package output

import (
	"errors"
	"io"
)

// Outputer is implemented by each log output backend. It builds the writer
// that log lines are written to. Backends are identified by the Kind* constants
// in kinds.go rather than through the interface.
type Outputer interface {
	// NewWriteCloser builds the writer for the backend. prefix can be used
	// by backends that need a per-instance identifier (e.g. a file name).
	NewWriteCloser(prefix string) (io.WriteCloser, error)
}

// wrapWriteCloser adapts a plain write function into an io.WriteCloser.
type wrapWriteCloser func(p []byte) (n int, err error)

func (w wrapWriteCloser) Write(p []byte) (n int, err error) { return w(p) }
func (w wrapWriteCloser) Close() error                      { return nil }

// emptyWrite discards everything written to it.
var emptyWrite io.WriteCloser = wrapWriteCloser(func(p []byte) (n int, err error) {
	return len(p), nil
})

// errWrite reports an error on every write. It is the fallback writer used
// when a backend has not been configured yet.
var errWrite io.WriteCloser = wrapWriteCloser(func(p []byte) (n int, err error) {
	return len(p), errors.New("write not configured")
})
