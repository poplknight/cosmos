package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/poplknight/cosmos/logger/output"
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
