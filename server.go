package grpchealth

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type CLIServer struct {
	Address  string `help:"gRPC server address (e.g., :50051 or unix:///tmp/grpc.sock)" arg:"" required:""`
	CertFile string `help:"Path to the server certificate file" short:"c"`
	KeyFile  string `help:"Path to the server key file" short:"k"`
}

func runServer(ctx context.Context, opt CLIServer) error {
	var lis net.Listener
	var err error
	var network, address string
	
	// Check if address is Unix Domain Socket
	if isUnixSocket(opt.Address) {
		network = "unix"
		address = parseUnixSocketPath(opt.Address)
		// Remove existing socket file if it exists
		if err := os.RemoveAll(address); err != nil {
			slog.Warn("Failed to remove existing socket file", "path", address, "error", err)
		}
		lis, err = net.Listen(network, address)
		if err != nil {
			return fmt.Errorf("failed to listen on unix socket: %w", err)
		}
		// Cleanup socket file on exit
		defer func() {
			if err := os.RemoveAll(address); err != nil {
				slog.Warn("Failed to cleanup socket file", "path", address, "error", err)
			}
		}()
	} else {
		network = "tcp"
		address = opt.Address
		lis, err = net.Listen(network, address)
		if err != nil {
			return fmt.Errorf("failed to listen: %w", err)
		}
	}
	var opts []grpc.ServerOption
	
	// TLS is not applicable for Unix Domain Sockets
	if network == "unix" {
		slog.Info("Starting gRPC server on Unix Domain Socket",
			"address", opt.Address,
			"socket_path", address,
		)
	} else if opt.CertFile != "" && opt.KeyFile != "" {
		// TLS設定 (TCP only)
		cert, err := tls.LoadX509KeyPair(opt.CertFile, opt.KeyFile)
		if err != nil {
			return fmt.Errorf("failed to load key pair: %w", err)
		}
		creds := credentials.NewTLS(&tls.Config{
			Certificates: []tls.Certificate{cert},
		})
		opts = append(opts, grpc.Creds(creds))
		slog.Info("Starting gRPC server with TLS",
			"address", opt.Address,
			"certFile", opt.CertFile,
			"keyFile", opt.KeyFile,
		)
	} else {
		slog.Info("Starting gRPC server without TLS",
			"address", opt.Address,
		)
	}

	sv := grpc.NewServer(opts...)

	// register health check service
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(sv, healthServer)

	go func() {
		<-ctx.Done()
		slog.Info("Stopping gRPC server")
		sv.GracefulStop()
	}()

	if err := sv.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}
	return nil
}

