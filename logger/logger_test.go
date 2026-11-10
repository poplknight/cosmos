package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/poplknight/cosmos/logger/output"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

func fileOutput(t *testing.T, path string) output.OutputsConfig {
	t.Helper()
	return output.OutputsConfig{
		{Kind: output.KindFile, Config: output.FileConfig{Path: path}},
	}
}

func TestLevelFiltering(t *testing.T) {
	path := filepath.Join(t.TempDir(), "level.log")
	z, err := (Config{Level: "warn", Outputs: fileOutput(t, path)}).InitLogger()
	if err != nil {
		t.Fatal(err)
	}
	z.Debug().Msg("debug 1")
	z.Info().Msg("info")
	z.Warn().Msg("warn 2")
	z.Error().Msg("error boom")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if strings.Contains(s, "debug") || strings.Contains(s, "info") {
		t.Fatalf("debug/info should be filtered, got %q", s)
	}
	if !strings.Contains(s, "warn 2") || !strings.Contains(s, "error boom") {
		t.Fatalf("warn/error should be emitted, got %q", s)
	}
}

func TestWithFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "with.log")
	z, err := (Config{
		Level:   "info",
		With:    map[string]string{"app": "demo", "env": "prod"},
		Outputs: fileOutput(t, path),
	}).InitLogger()
	if err != nil {
		t.Fatal(err)
	}
	z.Info().Msg("hello world")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{`"level":"info"`, `"app":"demo"`, `"env":"prod"`, `"hello world"`} {
		if !strings.Contains(s, want) {
			t.Fatalf("output %q should contain %q", s, want)
		}
	}
}

func TestEmptyLevelDefaultsToInfo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "default.log")
	z, err := (Config{Outputs: fileOutput(t, path)}).InitLogger()
	if err != nil {
		t.Fatal(err)
	}
	z.Info().Msg("ready")
	z.Debug().Msg("hidden")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "ready") || strings.Contains(s, "hidden") {
		t.Fatalf("empty level should default to info, got %q", s)
	}
}

func TestReloadLoggerInstallsGlobal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reload.log")
	v := viper.New()
	v.Set("logger.level", "warn")
	v.Set("logger.with", map[string]string{"app": "demo"})
	v.Set("logger.outputs", []any{
		map[string]any{"kind": "file", "config": map[string]any{"path": path}},
	})
	if err := ReloadLogger(v); err != nil {
		t.Fatal(err)
	}

	log.Warn().Msg("hello")
	log.Info().Msg("skip")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, `"level":"warn"`) || !strings.Contains(s, "hello") {
		t.Fatalf("should contain the warn line, got %q", s)
	}
	if strings.Contains(s, "skip") {
		t.Fatalf("info should be filtered, got %q", s)
	}
}

func TestReloadLoggerWithPrefix(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prefix.log")
	v := viper.New()
	v.Set("app.logger.level", "error")
	v.Set("app.logger.outputs", []any{
		map[string]any{"kind": "file", "config": map[string]any{"path": path}},
	})
	if err := ReloadLogger(v, WithReloadPrefix("app")); err != nil {
		t.Fatal(err)
	}

	log.Error().Msg("boom")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"level":"error"`) || !strings.Contains(string(data), "boom") {
		t.Fatalf("should contain the error line, got %q", string(data))
	}
}

func TestReloadLoggerMissingSection(t *testing.T) {
	v := viper.New()
	if err := ReloadLogger(v, WithReloadPrefix("nope")); err == nil {
		t.Fatal("missing section should fail")
	}
}
