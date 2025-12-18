package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/cavenine/queryops/background"
	"github.com/cavenine/queryops/config"
	"github.com/cavenine/queryops/db"

	"github.com/spf13/cobra"
)

func NewWorkerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worker",
		Short: "Run background workers only",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			if config.Global.DatabaseURL == "" {
				return errors.New("DATABASE_URL must be set")
			}

			pool, err := db.NewPool(ctx, config.Global)
			if err != nil {
				return fmt.Errorf("creating database pool: %w", err)
			}
			defer pool.Close()

			clientCfg := background.DefaultClientConfig()
			slog.InfoContext(ctx, "starting dedicated river worker process")
			if err := background.RunWorker(ctx, pool, clientCfg); err != nil && !errors.Is(err, context.Canceled) {
				return err
			}

			return nil
		},
	}

	return cmd
}
