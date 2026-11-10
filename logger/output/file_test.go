package output

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileOutputWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.log")
	w, err := (FileConfig{Path: path}).NewWriteCloser("")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("line one\n")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "line one\n" {
		t.Fatalf("content = %q", data)
	}
}

func TestFileOutputEmptyPathFallsBackToStderr(t *testing.T) {
	w, err := (FileConfig{}).NewWriteCloser("")
	if err != nil {
		t.Fatal(err)
	}
	if w != os.Stderr {
		t.Fatalf("empty path should fall back to stderr, got %T", w)
	}
}

// TestFileConfigDecodeSnakeCase verifies that snake_case config keys map to
// the struct fields via the mapstructure tags.
func TestFileConfigDecodeSnakeCase(t *testing.T) {
	raw := map[string]any{
		"path":        "/tmp/app.log",
		"max_size":    50,
		"max_backups": 3,
		"max_age":     7,
		"compress":    true,
	}
	var c FileConfig
	if err := decodeConfig(raw, &c); err != nil {
		t.Fatal(err)
	}
	if c.Path != "/tmp/app.log" || c.MaxSize != 50 || c.MaxBackups != 3 || c.MaxAge != 7 || !c.Compress {
		t.Fatalf("decoded config = %+v", c)
	}
}

func TestOutputsAggregate(t *testing.T) {
	dir := t.TempDir()
	p1 := filepath.Join(dir, "a.log")
	p2 := filepath.Join(dir, "b.log")
	w, err := (OutputsConfig{
		{Kind: KindFile, Config: FileConfig{Path: p1}},
		{Kind: KindFile, Config: FileConfig{Path: p2}},
	}).NewWriteCloser("")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("shared\n")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{p1, p2} {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "shared") {
			t.Fatalf("%s should contain the shared line, got %q", p, data)
		}
	}
}

func TestStderrOutput(t *testing.T) {
	w, err := (StderrConfig{}).NewWriteCloser("")
	if err != nil {
		t.Fatal(err)
	}
	if w != os.Stderr {
		t.Fatalf("stderr output should be os.Stderr, got %T", w)
	}
}
