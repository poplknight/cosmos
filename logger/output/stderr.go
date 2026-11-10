package output

import (
	"io"
	"os"
)

// StderrConfig is the config for an output that writes to the process stderr.
type StderrConfig struct{}

// NewWriteCloser returns the process stderr as the writer.
func (c StderrConfig) NewWriteCloser(prefix string) (io.WriteCloser, error) {
	return os.Stderr, nil
}
