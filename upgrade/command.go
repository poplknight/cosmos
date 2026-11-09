package upgrade

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Command returns a cobra subcommand for running upgrades manually from the user's root command.
// The *Upgrade must already have its upgrade list configured via Register at initialization.
//
//	m := upgrade.New(store).Register("v0.0.1", fn1).Register("v0.0.2", fn2)
//	rootCmd.AddCommand(upgrade.Command(m))
//
// Subcommands:
//
//	upgrade         run upgrades from the current version to the latest, then print the current version
//	upgrade current print the current version
func Command(u *Upgrade) *cobra.Command {
	root := &cobra.Command{
		Use:   "upgrade",
		Short: "run upgrades up to the latest version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := u.Up(cmd.Context()); err != nil {
				return err
			}
			cur, err := u.CurrentVersion(cmd.Context())
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "upgraded to %s\n", cur)
			return err
		},
	}
	root.AddCommand(&cobra.Command{
		Use:   "current",
		Short: "print the current version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cur, err := u.CurrentVersion(cmd.Context())
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), cur)
			return err
		},
	})
	return root
}
