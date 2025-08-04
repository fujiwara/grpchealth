package grpchealth

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type CLIServer struct {
	Address  string `help:"gRPC server address" arg:"" required:""`
	CertFile string `help:"Path to the server certificate file" short:"c"`
	KeyFile  string `help:"Path to the server key file" short:"k"`
}

func runServer(ctx context.Context, opt CLIServer) error {
	lis, err := net.Listen("tcp", opt.Address)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	var opts []grpc.ServerOption
	if opt.CertFile != "" && opt.KeyFile != "" {
		// TLS設定
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
