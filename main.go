package grpchealth

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/alecthomas/kong"
	"github.com/fujiwara/sloghandler"
)

type CLI struct {
	Server CLIServer `cmd:"" help:"Run gRPC health check server"`
	Client CLIClient `cmd:"" help:"Run gRPC health check client"`
}

func Run(ctx context.Context) error {
	opts := &sloghandler.HandlerOptions{
		HandlerOptions: slog.HandlerOptions{
			Level: slog.LevelDebug,
		},
		Color: true, // Colorize the output based on log level
	}
	handler := sloghandler.NewLogHandler(os.Stdout, opts)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	var cli CLI
	k := kong.Parse(&cli)
	switch k.Command() {
	case "server <address>":
		return runServer(ctx, cli.Server)
	case "client <address>":
		return runClient(ctx, cli.Client)
	default:
		return fmt.Errorf("unknown command: %s", k.Command())
	}
}
