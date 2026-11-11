package infra

import (
	"testing"

	"github.com/spf13/viper"
	gormmysql "gorm.io/driver/mysql"
)

func TestGormConnectionError(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	// 指向无服务的地址：gorm.Open 连接必然失败并返回错误。
	dial := gormmysql.Open("root:@tcp(127.0.0.1:1)/nope")
	if _, err := Gorm(dial); err == nil {
		t.Fatal("should fail to connect to an unreachable mysql")
	}
}
