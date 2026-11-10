// Package logger provides a leveled, structured logger. Every log line is
// emitted as JSON by zerolog, carries an optional set of fixed key-value
// fields, and is written to one or more pluggable outputs.
package logger

import (
	"fmt"
	"sort"

	"github.com/poplknight/cosmos/logger/output"
	"github.com/rs/zerolog"
)

// Config configures a logger from a config file.
type Config struct {
	// Level is the minimum level to emit, e.g. "info".
	Level string `json:"level" yaml:"level" mapstructure:"level"`
	// With adds fixed key-value fields to every log line.
	With map[string]string `json:"with" yaml:"with" mapstructure:"with"`
	// Outputs is the list of log destinations.
	Outputs output.OutputsConfig `json:"outputs" yaml:"outputs" mapstructure:"outputs"`
}

// InitLogger builds a zerolog.Logger from the config: the output list is
// resolved into one writer, the minimum level is applied, a timestamp is
// attached, and the fixed fields are injected in stable (sorted) order.
//
// The caller is free to install the result as the global logger of
// github.com/rs/zerolog/log.
func (c Config) InitLogger() (zerolog.Logger, error) {
	lvl, err := levelFrom(c.Level)
	if err != nil {
		return zerolog.Logger{}, err
	}
	w, err := c.Outputs.NewWriteCloser("")
	if err != nil {
		return zerolog.Logger{}, err
	}
	z := zerolog.New(w).Level(lvl).With().Timestamp().Logger()
	for _, k := range sortedKeys(c.With) {
		z = z.With().Str(k, c.With[k]).Logger()
	}
	return z, nil
}

// levelFrom parses a level name, defaulting an empty string to InfoLevel.
func levelFrom(s string) (zerolog.Level, error) {
	if s == "" {
		return zerolog.InfoLevel, nil
	}
	lvl, err := zerolog.ParseLevel(s)
	if err != nil {
		return zerolog.InfoLevel, fmt.Errorf("unknown log level %q", s)
	}
	return lvl, nil
}

// sortedKeys returns the keys of m in sorted order so the injected fields are
// deterministic.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
