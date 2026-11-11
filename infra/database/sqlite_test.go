package database

import (
	"testing"

	"github.com/spf13/viper"
)

func TestSQLiteNoConfig(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	if _, err := SQLite(); err == nil {
		t.Fatal("missing database.sqlite section should fail")
	}
}

func TestSQLiteBuildsDialector(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	v := viper.GetViper()
	v.Set("database.driver", "sqlite")
	v.Set("database.sqlite.database", "test.db")

	dial, err := SQLite() // 只构造 Dialector，不建立连接
	if err != nil {
		t.Fatal(err)
	}
	if dial == nil {
		t.Fatal("dialector should not be nil")
	}
}
