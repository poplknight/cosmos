// Package infra is the infrastructure layer that assembles runtime handles
// from a viper config source. It currently builds the logger handle; other
// handles (database connections, caches, ...) can be added here later.
package infra

import (
	"fmt"

	"github.com/poplknight/cosmos/logger"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// LoggerConfig carries the options that control how the logger is loaded.
type LoggerConfig struct {
	// Prefix is the config section above the logger keys, e.g. "app"
	// when the config looks like "app.logger.level".
	Prefix string
}

// LoggerOption customizes how the logger is loaded from a viper instance.
type LoggerOption func(*LoggerConfig)

// WithLoggerPrefix sets the config section above the logger keys. For example,
// with prefix "app" the keys become "app.logger.level", "app.logger.with" and
// "app.logger.outputs". Without it, the keys are "logger.level" and friends.
func WithLoggerPrefix(prefix string) LoggerOption {
	return func(rc *LoggerConfig) {
		rc.Prefix = prefix
	}
}

// Logger reads the logger config from a viper instance, builds the zerolog
// logger, and installs it as the global logger of github.com/rs/zerolog/log.
// After a successful call the rest of the program can use log.Info().Msg(...)
// directly.
//
//	cfg := viper.New()
//	cfg.SetConfigFile("app.yaml")   // contains a "logger:" section
//	if err := infra.Logger(cfg); err != nil {
//		panic(err)
//	}
func Logger(v *viper.Viper, opts ...LoggerOption) error {
	var conf LoggerConfig
	for _, opt := range opts {
		if opt != nil {
			opt(&conf)
		}
	}

	sub := v
	if conf.Prefix != "" {
		sub = v.Sub(conf.Prefix)
		if sub == nil {
			return fmt.Errorf("no config section %q", conf.Prefix)
		}
	}
	loggerSub := sub.Sub("logger")
	if loggerSub == nil {
		return fmt.Errorf("no logger section")
	}
	var c logger.Config
	if err := loggerSub.Unmarshal(&c); err != nil {
		return err
	}

	z, err := c.InitLogger()
	if err != nil {
		return err
	}
	log.Logger = z
	return nil
}
