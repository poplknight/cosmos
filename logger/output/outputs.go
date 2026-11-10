package output

import (
	"errors"
	"io"
)

// OutputsConfig aggregates several outputs into one logical destination:
// every log line is written to each sub-output. It is a plain list of
// OutputConfigs, typically bound to Config.Outputs. It can also appear as a
// nested output (KindOutputs) whose config is the list itself.
type OutputsConfig []OutputConfig

// NewWriteCloser builds every sub-output and fans each write out to all of
// them. Closing the result closes each sub-output in order.
func (c OutputsConfig) NewWriteCloser(prefix string) (io.WriteCloser, error) {
	if len(c) == 0 {
		return nil, errors.New("no outputs configured")
	}
	multi := &multiWriteCloser{}
	for _, oc := range c {
		w, err := oc.NewWriteCloser(prefix)
		if err != nil {
			_ = multi.Close()
			return nil, err
		}
		multi.writers = append(multi.writers, w)
		multi.closers = append(multi.closers, w)
	}
	return multi, nil
}

// multiWriteCloser fans writes to all sub-writers and closes them in order.
type multiWriteCloser struct {
	writers []io.Writer
	closers []io.Closer
}

func (m *multiWriteCloser) Write(p []byte) (n int, err error) {
	for _, w := range m.writers {
		if _, err := w.Write(p); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

func (m *multiWriteCloser) Close() error {
	var errs []error
	for _, c := range m.closers {
		if err := c.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
