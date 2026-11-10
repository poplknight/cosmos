package infra

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

func TestLoggerInstallsGlobal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reload.log")
	v := viper.New()
	v.Set("logger.level", "warn")
	v.Set("logger.with", map[string]string{"app": "demo"})
	v.Set("logger.outputs", []any{
		map[string]any{"kind": "file", "config": map[string]any{"path": path}},
	})
	if err := Logger(v); err != nil {
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

func TestLoggerWithPrefix(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prefix.log")
	v := viper.New()
	v.Set("app.logger.level", "error")
	v.Set("app.logger.outputs", []any{
		map[string]any{"kind": "file", "config": map[string]any{"path": path}},
	})
	if err := Logger(v, WithLoggerPrefix("app")); err != nil {
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

func TestLoggerMissingSection(t *testing.T) {
	v := viper.New()
	if err := Logger(v, WithLoggerPrefix("nope")); err == nil {
		t.Fatal("missing section should fail")
	}
}
