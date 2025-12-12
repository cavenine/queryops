package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"queryops/background"
	"queryops/config"
	"queryops/db"

	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worker",
		Short: "Run background workers only",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if config.Global.DatabaseURL == "" {
				return fmt.Errorf("DATABASE_URL must be set")
			}

			pool, err := db.NewPool(ctx, config.Global)
			if err != nil {
				return fmt.Errorf("creating database pool: %w", err)
			}
			defer pool.Close()

			clientCfg := background.DefaultClientConfig()
			slog.Info("starting dedicated river worker process")
			if err := background.RunWorker(ctx, pool, clientCfg); err != nil && !errors.Is(err, context.Canceled) {
				return err
			}

			return nil
		},
	}

	return cmd
}
