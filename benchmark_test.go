package grpchealth

import (
	"context"
	"io"
	"log/slog"
)

// setupBenchmarkLogger sets up a logger that discards output for benchmarks
func setupBenchmarkLogger() func() {
	// Save original logger
	originalLogger := slog.Default()

	// Create a logger that discards output
	discardLogger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		Level: slog.LevelError, // Only log errors, suppress info/debug
	}))
	slog.SetDefault(discardLogger)

	// Return cleanup function
	return func() {
		slog.SetDefault(originalLogger)
	}
}

// runBenchmarkClient runs a client health check without logging
func runBenchmarkClient(address string) error {
	opt := CLIClient{
		Address: address,
		TLS:     false,
		Service: "",
	}

	// Temporarily disable logging for client operations
	originalLogger := slog.Default()
	discardLogger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))
	slog.SetDefault(discardLogger)
	defer slog.SetDefault(originalLogger)

	return runClient(context.Background(), opt)
}

// runBenchmarkUnixClient runs a Unix socket client health check without logging
func runBenchmarkUnixClient(socketPath string) error {
	opt := CLIClient{
		Address: socketPath,
		TLS:     false,
		Service: "",
	}

	// Temporarily disable logging for client operations
	originalLogger := slog.Default()
	discardLogger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))
	slog.SetDefault(discardLogger)
	defer slog.SetDefault(originalLogger)

	return runClient(context.Background(), opt)
}
