package main

import (
	"context"
	"fmt"

	"github.com/poplknight/cosmos/upgrade"
	"github.com/spf13/cobra"
)

// memStore keeps the current version in memory and is only for demonstration.
// In a real deployment, replace it with a persistent implementation (file, database, etc.)
// so that Up can resume after a restart.
type memStore struct {
	version string
}

func (m *memStore) GetVersion(context.Context) (string, error) { return m.version, nil }
func (m *memStore) SetVersion(_ context.Context, v string) error {
	m.version = v
	return nil
}

func main() {
	// Register the upgrade list: step from v0.0.0 up to v0.0.2.
	m := upgrade.New(&memStore{}).
		Register("v0.0.1", func(ctx context.Context) error {
			fmt.Println("upgrading to v0.0.1")
			return nil
		}).
		Register("v0.0.2", func(ctx context.Context) error {
			fmt.Println("upgrading to v0.0.2")
			return nil
		})

	// Mount the upgrade subcommand on a cobra root command:
	//   go run . upgrade          run the upgrades up to the latest version
	//   go run . upgrade current  print the current version
	// Calling m.Up(ctx) directly is equivalent to the "upgrade" subcommand.
	root := &cobra.Command{Use: "app"}
	root.AddCommand(upgrade.Command(m))
	if err := root.ExecuteContext(context.Background()); err != nil {
		fmt.Println(err)
		return
	}
}
