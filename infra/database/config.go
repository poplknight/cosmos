// Package database provides concrete database drivers built from the global
// viper config source.
package database

import (
	"fmt"

	"github.com/spf13/viper"
)

// Config holds the common connection parameters of the "database.<driver>"
// section in the global viper.
type Config struct {
	// Addr 是 host:port 形式的连接地址。
	Addr string `json:"addr" yaml:"addr" mapstructure:"addr"`
	// User 是登录用户名。
	User string `json:"user" yaml:"user" mapstructure:"user"`
	// Password 是登录密码。
	Password string `json:"password" yaml:"password" mapstructure:"password"`
	// Database 是库名。
	Database string `json:"database" yaml:"database" mapstructure:"database"`
	// Params 是额外的 DSN 参数，如 charset、parseTime。
	Params map[string]string `json:"params" yaml:"params" mapstructure:"params"`
}

// LoadConfig reads the database driver config from the global viper. The
// section is "database.<driver>", where <driver> is the value of
// "database.driver". Database is required; driver-specific fields (e.g. Addr
// for mysql) are validated by each driver.
func LoadConfig() (Config, error) {
	v := viper.GetViper()
	driverName := v.GetString("database.driver")
	db := v.Sub("database")
	if db == nil {
		return Config{}, fmt.Errorf("no database config section")
	}
	section := db.Sub(driverName)
	if section == nil {
		return Config{}, fmt.Errorf("no database.%s config section", driverName)
	}
	c := Config{
		Addr:     section.GetString("addr"),
		User:     section.GetString("user"),
		Password: section.GetString("password"),
		Database: section.GetString("database"),
		Params:   section.GetStringMapString("params"),
	}
	if c.Database == "" {
		return Config{}, fmt.Errorf("database.%s database is required", driverName)
	}
	return c, nil
}
