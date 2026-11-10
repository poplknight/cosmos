package infra

import (
	"errors"
	"flag"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Source describes where the config comes from. It applies the source onto a
// viper instance. Every source comes in three layers: the parameterized one
// (PathSource/NacosSource/EtcdSource, for in-process calls and tests), the
// standard-flag entry (Flag*Source, which registers and parses its own flags),
// and the cobra one (Cmd*Source, reading a command's flags).
type Source func(v *viper.Viper) error

// LoadConfig loads the config into viper's global instance using the given
// source and returns the global viper. It is designed to be a wire provider.
func LoadConfig(source Source) (*viper.Viper, error) {
	if source == nil {
		return nil, errors.New("config source is nil")
	}
	if err := source(viper.GetViper()); err != nil {
		return nil, err
	}
	return viper.GetViper(), nil
}

// PathSource returns a Source that loads the config file at path. An empty
// path falls back to searching for "config.yaml" in the working directory.
// It is the bottom layer: call it directly from in-process code or tests.
func PathSource(path string) Source {
	return func(v *viper.Viper) error {
		if path != "" {
			v.SetConfigFile(path)
		} else {
			v.SetConfigName("config")
			v.AddConfigPath(".")
		}
		return v.ReadInConfig()
	}
}

// FlagPathSource is the entry that registers and parses the "-p" standard flag
// itself, then returns PathSource. Calling it twice panics (the flag is
// redefined), which is intentional: an infra startup that misuses its config
// should fail loudly. Only this source's flag is ever touched, so the three
// sources never interfere.
func FlagPathSource() Source {
	var path string
	flag.StringVar(&path, "p", "", "config file path")
	flag.Parse()
	return PathSource(path)
}

// CmdPathSource reads the "-p" flag of a cobra command and returns PathSource.
func CmdPathSource(cmd *cobra.Command) (Source, error) {
	p, err := cmd.Flags().GetString("p")
	if err != nil {
		return nil, err
	}
	return PathSource(p), nil
}

// parseContent parses remote config content into v. configType defaults to
// yaml when empty.
func parseContent(v *viper.Viper, content, configType string) error {
	if configType == "" {
		configType = "yaml"
	}
	v.SetConfigType(configType)
	return v.ReadConfig(strings.NewReader(content))
}
