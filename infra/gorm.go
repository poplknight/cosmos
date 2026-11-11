package infra

import (
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

// Gorm opens a gorm.DB from the given dialector, tuning gorm with the viper
// "database.gorm" config section. opts further customize the session.
//
//	dial, err := database.MySQL()
//	db, err := infra.Gorm(dial)
func Gorm(dial gorm.Dialector, opts ...gorm.Option) (*gorm.DB, error) {
	v := viper.GetViper()
	conf := &gorm.Config{
		// SkipDefaultTransaction 关闭 gorm 为每条写操作自动开启的事务。
		SkipDefaultTransaction: v.GetBool("database.gorm.skip_default_transaction"),
		// FullSaveAssociations 保存时对关联也做全量更新。
		FullSaveAssociations: v.GetBool("database.gorm.full_save_associations"),
		// PrepareStmt 为执行的 SQL 开启预处理语句缓存。
		PrepareStmt: v.GetBool("database.gorm.prepare_stmt"),
		// DryRun 只生成 SQL 不执行，用于打印调试。
		DryRun: v.GetBool("database.gorm.dry_run"),
		// DisableNestedTransaction 禁用嵌套事务。
		DisableNestedTransaction: v.GetBool("database.gorm.disable_nested_transaction"),
		// AllowGlobalUpdate 允许不带条件的全局 Update/Delete。
		AllowGlobalUpdate: v.GetBool("database.gorm.allow_global_update"),
		// DisableAutomaticPing 关闭打开连接后的自动 Ping。
		DisableAutomaticPing: v.GetBool("database.gorm.disable_automatic_ping"),
		// DisableForeignKeyConstraintWhenMigrating 迁移时不创建外键约束。
		DisableForeignKeyConstraintWhenMigrating: v.GetBool("database.gorm.disable_foreign_key_constraint_when_migrating"),
		// IgnoreRelationshipsWhenMigrating 迁移时忽略关联关系。
		IgnoreRelationshipsWhenMigrating: v.GetBool("database.gorm.ignore_relationships_when_migrating"),
		// QueryFields 查询时始终 SELECT 全字段。
		QueryFields: v.GetBool("database.gorm.query_fields"),
		// TranslateError 把底层驱动错误翻译成 gorm 错误类型。
		TranslateError: v.GetBool("database.gorm.translate_error"),
	}
	return gorm.Open(dial, append([]gorm.Option{conf}, opts...)...)
}
