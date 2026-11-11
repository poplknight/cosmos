package database

import (
	"net/url"

	"github.com/spf13/viper"
	sqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// SQLite returns the gorm sqlite dialector built from the global viper config.
// The database file path comes from LoadConfig.Database (the "database" key of
// the "database.<driver>" section); extra params are appended to the DSN.
func SQLite() (gorm.Dialector, error) {
	c, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	// sqlite 的库名即数据库文件路径。
	dsn := c.Database
	if len(c.Params) > 0 {
		vals := url.Values{}
		for k, v := range c.Params {
			vals.Set(k, v)
		}
		dsn += "?" + vals.Encode()
	}
	conf := sqlite.Config{
		DriverName: viper.GetViper().GetString("database.driver"),
		DSN:        dsn,
	}
	return sqlite.New(conf), nil
}
