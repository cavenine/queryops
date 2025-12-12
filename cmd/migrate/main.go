package migrate

import (
	"fmt"
	"strconv"

	"queryops/background"
	"queryops/config"
	"queryops/migrations"

	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "migrate",
		Short: "Run database migrations",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if config.Global.DatabaseURL == "" {
				return fmt.Errorf("DATABASE_URL must be set")
			}
			return nil
		},
	}

	root.AddCommand(
		newUpCmd(),
		newDownCmd(),
		newVersionCmd(),
		newForceCmd(),
		newToCmd(),
	)

	return root
}

func newUpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Apply all available migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := migrations.Up(config.Global.DatabaseURL); err != nil {
				return err
			}

			// Always run River's migrations alongside application migrations.
			ctx := cmd.Context()
			if err := background.MigrateRiver(ctx, config.Global); err != nil {
				return err
			}

			return nil
		},
	}
}

func newDownCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "down",
		Short: "Roll back migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			steps, err := strconv.Atoi(cmd.Flag("steps").Value.String())
			if err != nil {
				return fmt.Errorf("invalid steps: %w", err)
			}
			return migrations.Down(config.Global.DatabaseURL, steps)
		},
	}

	cmd.Flags().Int("steps", 1, "number of steps to roll back")
	return cmd
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print current migration version",
		RunE: func(cmd *cobra.Command, args []string) error {
			v, dirty, err := migrations.Version(config.Global.DatabaseURL)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "version=%d dirty=%v\n", v, dirty)
			return nil
		},
	}
}

func newForceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "force [version]",
		Short: "Force set migration version",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			v, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid version: %w", err)
			}
			return migrations.Force(config.Global.DatabaseURL, v)
		},
	}
}

func newToCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "to [version]",
		Short: "Migrate to a specific version",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			v64, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid version: %w", err)
			}
			return migrations.ToVersion(config.Global.DatabaseURL, uint(v64))
		},
	}
}
