package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/evanw/esbuild/pkg/api"
	"golang.org/x/sync/errgroup"

	"github.com/cavenine/queryops/config"
	"github.com/cavenine/queryops/web/resources"
)

func main() {
	if err := runMain(); err != nil {
		slog.Error("failure", "error", err)
		os.Exit(1)
	}
}

func runMain() error {
	var watch bool
	flag.BoolVar(&watch, "watch", false, "Enable watcher mode")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
	defer stop()

	return run(ctx, watch)
}

func run(ctx context.Context, watch bool) error {
	eg, egctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		return build(egctx, watch)
	})

	return eg.Wait()
}

func build(ctx context.Context, watch bool) error {
	opts := api.BuildOptions{
		EntryPointsAdvanced: []api.EntryPoint{
			{
				InputPath:  resources.LibsDirectoryPath + "/web-components/reverse-component/index.ts",
				OutputPath: "libs/reverse-component",
			},
			/*
				uncomment the entrypoint below after running pnpm install in the resources.LibsDirectoryPath + /lit directory
				esbuild will only be able to find the lit + sortable libraries after doing so
			*/
			// {
			// 	InputPath:  resources.LibsDirectoryPath + "/lit/src/index.ts",
			// 	OutputPath: "libs/sortable-example",
			// },
		},
		Bundle:            true,
		Format:            api.FormatESModule,
		LogLevel:          api.LogLevelInfo,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
		MinifyWhitespace:  true,
		Outdir:            resources.StaticDirectoryPath,
		Sourcemap:         api.SourceMapLinked,
		Target:            api.ESNext,
		Write:             true,
	}

	if watch {
		slog.InfoContext(ctx, "watching...")

		opts.Plugins = append(opts.Plugins, hotReloadPlugin())

		buildCtx, err := api.Context(opts)
		if err != nil {
			return err
		}
		defer buildCtx.Dispose()

		if err := buildCtx.Watch(api.WatchOptions{}); err != nil {
			return err
		}

		<-ctx.Done()
		return nil
	}

	slog.InfoContext(ctx, "building...")

	result := api.Build(opts)

	if len(result.Errors) > 0 {
		errs := make([]error, len(result.Errors))
		for i, err := range result.Errors {
			errs[i] = errors.New(err.Text)
		}
		return errors.Join(errs...)
	}

	return nil
}

func hotReloadPlugin() api.Plugin {
	return api.Plugin{
		Name: "hotreload",
		Setup: func(build api.PluginBuild) {
			build.OnEnd(func(result *api.BuildResult) (api.OnEndResult, error) {
				slog.Info("build complete", "errors", len(result.Errors), "warnings", len(result.Warnings))
				if len(result.Errors) == 0 {
					hostPort := net.JoinHostPort(config.Global.Host, config.Global.Port)
					url := fmt.Sprintf("http://%s/hotreload", hostPort)
					// #nosec G107
					req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
					if err != nil {
						slog.Warn("failed to create hotreload request", "error", err)
						return api.OnEndResult{}, nil
					}
					resp, err := http.DefaultClient.Do(req)
					if err != nil {
						slog.Warn("failed to trigger hotreload", "error", err)
					} else {
						_ = resp.Body.Close()
					}
				}
				return api.OnEndResult{}, nil
			})
		},
	}
}
