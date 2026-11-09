package upgrade

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestCommandUpgrade(t *testing.T) {
	store := &memStore{}
	ran := false
	m := New(store).Register("v0.0.1", func(ctx context.Context) error {
		ran = true
		return nil
	})

	root := &cobra.Command{}
	root.AddCommand(Command(m))
	root.SetArgs([]string{"upgrade"})
	var out bytes.Buffer
	root.SetOut(&out)

	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !ran {
		t.Fatal("upgrade function did not run")
	}
	if got, _ := store.GetVersion(context.Background()); got != "v0.0.1" {
		t.Fatalf("version = %q, want v0.0.1", got)
	}
	if !strings.Contains(out.String(), "upgraded to v0.0.1") {
		t.Fatalf("output should report the upgrade result, got %q", out.String())
	}
}

func TestCommandCurrent(t *testing.T) {
	store := &memStore{version: "v0.0.2"}
	root := &cobra.Command{}
	root.AddCommand(Command(New(store)))
	root.SetArgs([]string{"upgrade", "current"})
	var out bytes.Buffer
	root.SetOut(&out)

	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if out.String() != "v0.0.2\n" {
		t.Fatalf("current output = %q, want %q", out.String(), "v0.0.2\n")
	}
}

func TestCommandCurrentEmptyShowsZero(t *testing.T) {
	store := &memStore{}
	root := &cobra.Command{}
	root.AddCommand(Command(New(store)))
	root.SetArgs([]string{"upgrade", "current"})
	var out bytes.Buffer
	root.SetOut(&out)

	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if out.String() != "v0.0.0\n" {
		t.Fatalf("empty version current output = %q, want %q", out.String(), "v0.0.0\n")
	}
}
