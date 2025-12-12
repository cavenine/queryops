package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"queryops/cmd/migrate"
	"queryops/cmd/web"
	"queryops/cmd/worker"
	"queryops/config"

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
		web.NewCommand(),
		migrate.NewCommand(),
		worker.NewCommand(),
	)

	if err := root.ExecuteContext(ctx); err != nil {
		slog.Error("command failed", "error", err)
		os.Exit(1)
	}
}
