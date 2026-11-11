package database

import (
	"fmt"
	"net/url"

	"github.com/spf13/viper"
	mysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// MySQL returns the gorm mysql dialector built from the global viper config.
// Connection parameters come from driver.LoadConfig; mysql-specific options
// are read from the "database.<driver>" section.
func MySQL() (gorm.Dialector, error) {
	c, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	if c.Addr == "" {
		return nil, fmt.Errorf("mysql addr is empty")
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s", c.User, c.Password, c.Addr, c.Database)
	if len(c.Params) > 0 {
		vals := url.Values{}
		for k, v := range c.Params {
			vals.Set(k, v)
		}
		dsn += "?" + vals.Encode()
	}

	v := viper.GetViper()
	prefix := "database." + v.GetString("database.driver")
	conf := mysql.Config{
		// DriverName 是注册的驱动名。
		DriverName: v.GetString("database.driver"),
		// ServerVersion 是服务器版本号，用于跳过某些版本相关的初始化检查。
		ServerVersion: v.GetString(prefix + ".server_version"),
		// DSN 是连接串，由 LoadConfig 组装。
		DSN: dsn,
		// SkipInitializeWithVersion 初始化时跳过版本检查。
		SkipInitializeWithVersion: v.GetBool(prefix + ".skip_initialize_with_version"),
		// DefaultStringSize 是 string 类型字段的默认长度。
		DefaultStringSize: uint(v.GetInt(prefix + ".default_string_size")),
		// DisableWithReturning 禁用 RETURNING 子句（当驱动不支持时）。
		DisableWithReturning: v.GetBool(prefix + ".disable_with_returning"),
		// DisableDatetimePrecision 禁用 datetime 精度。
		DisableDatetimePrecision: v.GetBool(prefix + ".disable_datetime_precision"),
		// DontSupportRenameIndex 不支持索引重命名（改用 DROP+ADD 替代）。
		DontSupportRenameIndex: v.GetBool(prefix + ".dont_support_rename_index"),
		// DontSupportRenameColumn 不支持列重命名。
		DontSupportRenameColumn: v.GetBool(prefix + ".dont_support_rename_column"),
		// DontSupportForShareClause 不支持 FOR SHARE 子句。
		DontSupportForShareClause: v.GetBool(prefix + ".dont_support_for_share_clause"),
		// DontSupportNullAsDefaultValue NULL 不能作为默认值。
		DontSupportNullAsDefaultValue: v.GetBool(prefix + ".dont_support_null_as_default_value"),
		// DontSupportRenameColumnUnique 不支持唯一约束列的重命名。
		DontSupportRenameColumnUnique: v.GetBool(prefix + ".dont_support_rename_column_unique"),
		// DontSupportDropConstraint 不支持 DROP CONSTRAINT。
		DontSupportDropConstraint: v.GetBool(prefix + ".dont_support_drop_constraint"),
	}
	if v.IsSet(prefix + ".default_datetime_precision") {
		// DefaultDatetimePrecision 是 datetime 精度（指针，未设置时为 nil）。
		p := v.GetInt(prefix + ".default_datetime_precision")
		conf.DefaultDatetimePrecision = &p
	}
	return mysql.New(conf), nil
}
