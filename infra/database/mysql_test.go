package database

import (
	"testing"

	"github.com/spf13/viper"
)

func TestMySQLNoConfig(t *testing.T) {
	viper.Reset() // 全局为空：无 database.driver，无 mysql 配置节
	t.Cleanup(viper.Reset)
	if _, err := MySQL(); err == nil {
		t.Fatal("missing database.mysql section should fail")
	}
}

func TestMySQLBuildsDialector(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	v := viper.GetViper()
	v.Set("database.driver", "mysql")
	v.Set("database.mysql.addr", "127.0.0.1:3306")
	v.Set("database.mysql.user", "root")
	v.Set("database.mysql.database", "app")

	dial, err := MySQL() // 只构造 Dialector，不建立连接
	if err != nil {
		t.Fatal(err)
	}
	if dial == nil {
		t.Fatal("dialector should not be nil")
	}
}
