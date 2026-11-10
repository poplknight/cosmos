package logger

import (
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// reloadConfig carries the options that control how the logger is loaded.
type reloadConfig struct {
	// Prefix is the config section that holds the logger keys, e.g. "app"
	// when the config looks like "app.logger.level".
	Prefix string
}

// ReloadOption customizes how the logger is loaded from a viper instance.
type ReloadOption func(*reloadConfig)

// WithReloadPrefix sets the config section above the logger keys. For example,
// with prefix "app" the keys become "app.logger.level", "app.logger.with" and
// "app.logger.outputs". Without it, the keys are "logger.level" and friends.
func WithReloadPrefix(prefix string) ReloadOption {
	return func(rc *reloadConfig) {
		rc.Prefix = prefix
	}
}

// ReloadLogger reads the logger config from a viper instance, builds the
// zerolog logger, and installs it as the global logger of
// github.com/rs/zerolog/log. After a successful call the rest of the program
// can use log.Info().Msg(...) directly.
//
//	cfg := viper.New()
//	cfg.SetConfigFile("app.yaml")   // contains a "logger:" section
//	if err := ReloadLogger(cfg); err != nil {
//		panic(err)
//	}
func ReloadLogger(v *viper.Viper, opts ...ReloadOption) error {
	var conf reloadConfig
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
	var c Config
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
