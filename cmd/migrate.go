package main

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/cavenine/queryops/background"
	"github.com/cavenine/queryops/config"
	"github.com/cavenine/queryops/migrations"

	"github.com/spf13/cobra"
)

func NewMigrationCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "migrate",
		Short: "Run database migrations",
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			if config.Global.DatabaseURL == "" {
				return errors.New("DATABASE_URL must be set")
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
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := migrations.Up(config.Global.DatabaseURL); err != nil {
				return err
			}
			ctx := cmd.Context()
			slog.InfoContext(ctx, "migrations applied")

			// Always run River's migrations alongside application migrations.
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
		RunE: func(cmd *cobra.Command, _ []string) error {
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
		RunE: func(cmd *cobra.Command, _ []string) error {
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
		RunE: func(_ *cobra.Command, args []string) error {
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
		RunE: func(_ *cobra.Command, args []string) error {
			v64, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid version: %w", err)
			}
			return migrations.ToVersion(config.Global.DatabaseURL, uint(v64))
		},
	}
}
