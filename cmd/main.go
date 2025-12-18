package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/cavenine/queryops/cmd/web"
	"github.com/cavenine/queryops/config"

	"github.com/spf13/cobra"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: config.Global.LogLevel,
	}))
	slog.SetDefault(logger)

	root := &cobra.Command{
		Use:   "queryops",
		Short: "QueryOps server and tools",
	}

	root.AddCommand(
		web.NewWebCommand(),
		NewMigrationCommand(),
		NewWorkerCommand(),
	)

	if err := root.ExecuteContext(ctx); err != nil {
		slog.Error("command failed", "error", err)
		os.Exit(1)
	}
}
