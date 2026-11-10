package infra

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// NacosSource returns a Source that pulls the config from a Nacos config
// center and parses it into the viper instance. It is the bottom layer: call
// it directly with explicit parameters from in-process code or tests. Input is
// sanitized here (each server is trimmed, blanks are dropped), so callers may
// pass raw values without cleaning them first.
func NacosSource(servers []string, namespace, group, dataID, configType string) Source {
	return func(v *viper.Viper) error {
		var clean []string
		for _, s := range servers {
			if s = strings.TrimSpace(s); s != "" {
				clean = append(clean, s)
			}
		}
		if len(clean) == 0 {
			return errors.New("nacos servers is empty")
		}
		serverConfigs := make([]constant.ServerConfig, 0, len(clean))
		for _, addr := range clean {
			host, portStr, err := net.SplitHostPort(addr)
			if err != nil {
				return fmt.Errorf("invalid nacos server address %q: %w", addr, err)
			}
			port, err := strconv.ParseUint(portStr, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid port in %q: %w", addr, err)
			}
			serverConfigs = append(serverConfigs, *constant.NewServerConfig(host, port))
		}
		clientConfig := *constant.NewClientConfig(constant.WithNamespaceId(namespace))
		client, err := clients.NewConfigClient(vo.NacosClientParam{
			ClientConfig:  &clientConfig,
			ServerConfigs: serverConfigs,
		})
		if err != nil {
			return err
		}
		content, err := client.GetConfig(vo.ConfigParam{DataId: dataID, Group: group})
		if err != nil {
			return err
		}
		return parseContent(v, content, configType)
	}
}

// FlagNacosSource is the entry that registers and parses the nacos standard
// flags itself, then returns NacosSource. Calling it twice panics (the flags
// are redefined), which is intentional. Only this source's flags are touched.
func FlagNacosSource() Source {
	var server, namespace, group, dataID, configType string
	flag.StringVar(&server, "server", "", "nacos server, host:port (comma separated)")
	flag.StringVar(&namespace, "namespace", "", "nacos namespace id")
	flag.StringVar(&group, "group", "DEFAULT_GROUP", "nacos group")
	flag.StringVar(&dataID, "data-id", "", "nacos data id")
	flag.StringVar(&configType, "type", "yaml", "config format: yaml, json, toml")
	flag.Parse()
	return NacosSource(strings.Split(server, ","), namespace, group, dataID, configType)
}

// CmdNacosSource reads the nacos parameters from a cobra command's flags and
// returns NacosSource.
func CmdNacosSource(cmd *cobra.Command) (Source, error) {
	server, err := cmd.Flags().GetString("server")
	if err != nil {
		return nil, err
	}
	namespace, err := cmd.Flags().GetString("namespace")
	if err != nil {
		return nil, err
	}
	group, err := cmd.Flags().GetString("group")
	if err != nil {
		return nil, err
	}
	dataID, err := cmd.Flags().GetString("data-id")
	if err != nil {
		return nil, err
	}
	configType, err := cmd.Flags().GetString("type")
	if err != nil {
		return nil, err
	}
	return NacosSource(strings.Split(server, ","), namespace, group, dataID, configType), nil
}
