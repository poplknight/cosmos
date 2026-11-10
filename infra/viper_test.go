package infra

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func TestLoadConfigWithPathSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.yaml")
	if err := os.WriteFile(path, []byte("logger:\n  level: warn\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(viper.Reset)

	v, err := LoadConfig(PathSource(path))
	if err != nil {
		t.Fatal(err)
	}
	if got := v.GetString("logger.level"); got != "warn" {
		t.Fatalf("logger.level = %q, want warn", got)
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	t.Cleanup(viper.Reset)
	if _, err := LoadConfig(PathSource(filepath.Join(t.TempDir(), "nope.yaml"))); err == nil {
		t.Fatal("missing file should fail")
	}
}

func TestLoadConfigNilSource(t *testing.T) {
	if _, err := LoadConfig(nil); err == nil {
		t.Fatal("nil source should fail")
	}
}

func TestParseContent(t *testing.T) {
	v := viper.New()
	content := "logger:\n  level: warn\n  with:\n    app: demo\n"
	if err := parseContent(v, content, "yaml"); err != nil {
		t.Fatal(err)
	}
	if got := v.GetString("logger.level"); got != "warn" {
		t.Fatalf("logger.level = %q, want warn", got)
	}
	if got := v.GetString("logger.with.app"); got != "demo" {
		t.Fatalf("logger.with.app = %q, want demo", got)
	}
}

func TestParseContentDefaultType(t *testing.T) {
	v := viper.New()
	if err := parseContent(v, "logger:\n  level: info\n", ""); err != nil {
		t.Fatal(err)
	}
	if got := v.GetString("logger.level"); got != "info" {
		t.Fatalf("logger.level = %q, want info", got)
	}
}

func TestNacosSourceEmptyServers(t *testing.T) {
	if err := NacosSource(nil, "", "", "", "")(viper.New()); err == nil {
		t.Fatal("empty servers should fail")
	}
	// 全是空白 → 清洗后为空 → 同样失败（不会尝试连接）
	if err := NacosSource([]string{" ", "\t", ""}, "", "", "", "")(viper.New()); err == nil {
		t.Fatal("all-blank servers should fail after sanitizing")
	}
}

func TestEtcdSourceEmptyConfig(t *testing.T) {
	if err := EtcdSource(nil, "", "")(viper.New()); err == nil {
		t.Fatal("empty endpoints should fail")
	}
	if err := EtcdSource([]string{"localhost:2379"}, "", "")(viper.New()); err == nil {
		t.Fatal("empty key should fail")
	}
	// 全是空白 → 清洗后为空 → 同样失败（不会尝试连接）
	if err := EtcdSource([]string{"  ", "\t", ""}, "", "")(viper.New()); err == nil {
		t.Fatal("all-blank endpoints should fail after sanitizing")
	}
	// 含有效 endpoint → 清洗后非空 → 错误不再是 "endpoints is empty"
	err := EtcdSource([]string{"localhost:2379", ""}, "", "")(viper.New())
	if err != nil && strings.Contains(err.Error(), "endpoints is empty") {
		t.Fatal("valid endpoint should not be treated as empty")
	}
}

func TestFlagPathSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.yaml")
	if err := os.WriteFile(path, []byte("logger:\n  level: warn\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldArgs := os.Args
	oldCommandLine := flag.CommandLine
	os.Args = []string{"test", "-p", path}
	flag.CommandLine = flag.NewFlagSet("test", flag.ExitOnError)
	t.Cleanup(func() {
		os.Args = oldArgs
		flag.CommandLine = oldCommandLine
	})

	v := viper.New()
	if err := FlagPathSource()(v); err != nil {
		t.Fatal(err)
	}
	if got := v.GetString("logger.level"); got != "warn" {
		t.Fatalf("logger.level = %q, want warn", got)
	}
}

func TestFlagNacosSourceEmpty(t *testing.T) {
	// 默认（未设 -server）→ Servers 为空 → 加载时报错。
	if err := FlagNacosSource()(viper.New()); err == nil {
		t.Fatal("empty nacos servers should fail")
	}
}

func TestCmdPathSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.yaml")
	if err := os.WriteFile(path, []byte("logger:\n  level: warn\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("p", "", "path")
	if err := cmd.Flags().Set("p", path); err != nil {
		t.Fatal(err)
	}

	source, err := CmdPathSource(cmd)
	if err != nil {
		t.Fatal(err)
	}
	v := viper.New()
	if err := source(v); err != nil {
		t.Fatal(err)
	}
	if got := v.GetString("logger.level"); got != "warn" {
		t.Fatalf("logger.level = %q, want warn", got)
	}
}

func TestCmdNacosSourceMissingFlag(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	if _, err := CmdNacosSource(cmd); err == nil {
		t.Fatal("missing server flag should fail")
	}
}
