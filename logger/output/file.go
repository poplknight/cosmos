package output

import (
	"io"
	"os"

	"gopkg.in/natefinch/lumberjack.v2"
)

// FileConfig configures a file output with rotation, backed by lumberjack.
type FileConfig struct {
	// Path is the log file path. The file is created if it does not exist.
	Path string `json:"path" yaml:"path" mapstructure:"path"`
	// MaxSize is the maximum size of a single file in megabytes before it
	// rotates.
	MaxSize int `json:"max_size" yaml:"max_size" mapstructure:"max_size"`
	// MaxBackups is the maximum number of rotated files to keep, 0 means
	// keeping all of them.
	MaxBackups int `json:"max_backups" yaml:"max_backups" mapstructure:"max_backups"`
	// MaxAge is the maximum number of days to keep a rotated file, 0 means
	// never removing them by age.
	MaxAge int `json:"max_age" yaml:"max_age" mapstructure:"max_age"`
	// Compress controls whether rotated files are gzip-compressed.
	Compress bool `json:"compress" yaml:"compress" mapstructure:"compress"`
}

// NewWriteCloser returns a lumberjack writer that appends to Path and rotates
// the file once it grows past MaxSize, keeping at most MaxBackups rotated files
// for up to MaxAge days. When no Path is set, it falls back to stderr so the
// file output never fails on a missing path.
func (c FileConfig) NewWriteCloser(prefix string) (io.WriteCloser, error) {
	if c.Path == "" {
		return os.Stderr, nil
	}
	return &lumberjack.Logger{
		Filename:   c.Path,
		MaxSize:    c.MaxSize,
		MaxBackups: c.MaxBackups,
		MaxAge:     c.MaxAge,
		Compress:   c.Compress,
	}, nil
}
