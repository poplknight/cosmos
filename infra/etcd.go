package infra

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// EtcdSource returns a Source that reads the config content from an etcd key
// and parses it into the viper instance. It is the bottom layer: call it
// directly with explicit parameters from in-process code or tests. Input is
// sanitized here (each endpoint is trimmed, blanks are dropped), so callers
// may pass raw values without cleaning them first.
func EtcdSource(endpoints []string, key, configType string) Source {
	return func(v *viper.Viper) error {
		var clean []string
		for _, e := range endpoints {
			if e = strings.TrimSpace(e); e != "" {
				clean = append(clean, e)
			}
		}
		if len(clean) == 0 {
			return errors.New("etcd endpoints is empty")
		}
		if key == "" {
			return errors.New("etcd key is empty")
		}
		cli, err := clientv3.New(clientv3.Config{
			Endpoints:   clean,
			DialTimeout: 5 * time.Second,
		})
		if err != nil {
			return err
		}
		defer cli.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		resp, err := cli.Get(ctx, key)
		if err != nil {
			return err
		}
		if len(resp.Kvs) == 0 {
			return fmt.Errorf("etcd key %q not found", key)
		}
		return parseContent(v, string(resp.Kvs[0].Value), configType)
	}
}

// FlagEtcdSource is the entry that registers and parses the etcd standard
// flags itself, then returns EtcdSource. Calling it twice panics (the flags
// are redefined), which is intentional. Only this source's flags are touched.
func FlagEtcdSource() Source {
	var endpoint, key, configType string
	flag.StringVar(&endpoint, "endpoint", "", "etcd endpoint, host:port (comma separated)")
	flag.StringVar(&key, "key", "", "etcd config key")
	flag.StringVar(&configType, "type", "yaml", "config format: yaml, json, toml")
	flag.Parse()
	return EtcdSource(strings.Split(endpoint, ","), key, configType)
}

// CmdEtcdSource reads the etcd parameters from a cobra command's flags and
// returns EtcdSource.
func CmdEtcdSource(cmd *cobra.Command) (Source, error) {
	endpoint, err := cmd.Flags().GetString("endpoint")
	if err != nil {
		return nil, err
	}
	key, err := cmd.Flags().GetString("key")
	if err != nil {
		return nil, err
	}
	configType, err := cmd.Flags().GetString("type")
	if err != nil {
		return nil, err
	}
	return EtcdSource(strings.Split(endpoint, ","), key, configType), nil
}
