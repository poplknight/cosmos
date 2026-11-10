// Package output provides pluggable log output backends. Each backend is
// described by an OutputConfig whose Kind selects the backend and whose Config
// holds backend-specific options. Supported kinds are "stderr", "file" and
// "outputs".
package output

import (
	"errors"
	"fmt"
	"io"

	"github.com/go-viper/mapstructure/v2"
)

// OutputConfig describes a single log output and dispatches NewWriteCloser to
// the selected backend. The Kind and Config fields mirror the "kind" and
// "config" keys in configuration files, so they are read directly rather than
// through the Outputer interface (only the concrete backends implement it).
type OutputConfig struct {
	// Kind selects the backend, one of the Kind* constants.
	Kind Kind `json:"kind" yaml:"kind" mapstructure:"kind"`
	// Config holds backend-specific options. Its concrete type depends on
	// Kind, e.g. FileConfig for KindFile.
	Config any `json:"config" yaml:"config" mapstructure:"config"`
}

// NewWriteCloser builds the writer for the configured backend.
func (o OutputConfig) NewWriteCloser(prefix string) (io.WriteCloser, error) {
	switch o.Kind {
	case KindStderr:
		return StderrConfig{}.NewWriteCloser(prefix)
	case KindFile:
		var c FileConfig
		if o.Config != nil {
			if err := decodeConfig(o.Config, &c); err != nil {
				return nil, err
			}
		}
		return c.NewWriteCloser(prefix)
	case KindOutputs:
		var c OutputsConfig
		if err := decodeConfig(o.Config, &c); err != nil {
			return nil, err
		}
		return c.NewWriteCloser(prefix)
	default:
		return nil, fmt.Errorf("unknown output kind %q", o.Kind)
	}
}

// decodeConfig decodes an arbitrary config value into dst with mapstructure.
// Config parsers produce maps for the generic Config field; mapstructure maps
// them onto the destination struct directly, no JSON round-trip needed.
func decodeConfig(raw any, dst any) error {
	if raw == nil {
		return errors.New("missing output config")
	}
	return mapstructure.Decode(raw, dst)
}
