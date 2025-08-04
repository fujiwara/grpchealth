package grpchealth

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/peer"
)

type CLIClient struct {
	Address  string `help:"gRPC client address" arg:"" required:""`
	TLS      bool   `help:"Use TLS for connection" short:"t"`
	Insecure bool   `help:"Use insecure connection" short:"k"`
	Service  string `help:"Service name to check health status" default:"" short:"s"`
}

func runClient(ctx context.Context, opt CLIClient) error {
	dialOpts := []grpc.DialOption{}
	if opt.TLS {
		var creds credentials.TransportCredentials
		if opt.Insecure {
			creds = credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})
			slog.Info("Using TLS with insecure mode (certificate verification disabled)")
		} else {
			creds = credentials.NewTLS(nil)
			slog.Info("Using TLS with certificate verification")
		}
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
	} else {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		slog.Info("Using plaintext connection")
	}

	conn, err := grpc.NewClient(opt.Address, dialOpts...)
	if err != nil {
		return fmt.Errorf("failed to connect to gRPC server: %w", err)
	}
	defer conn.Close()

	client := grpc_health_v1.NewHealthClient(conn)
	req := &grpc_health_v1.HealthCheckRequest{
		Service: opt.Service,
	}
	slog.Info("Sending health check request",
		"address", opt.Address,
		"service", opt.Service,
	)
	var pe peer.Peer
	callerOpts := []grpc.CallOption{
		grpc.Peer(&pe),
	}
	start := time.Now()
	resp, err := client.Check(ctx, req, callerOpts...)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}
	duration := time.Since(start)
	status := resp.GetStatus().String()
	slog.Info("Received health check response",
		"service", opt.Service,
		"status", status,
		"duration", duration,
		"peer", pe.Addr.String(),
	)

	if pe.AuthInfo != nil {
		if tlsInfo, ok := pe.AuthInfo.(credentials.TLSInfo); ok {
			if len(tlsInfo.State.PeerCertificates) > 0 {
				cert := tlsInfo.State.PeerCertificates[0]
				slog.Info("Peer certificate information",
					"subject", cert.Subject,
					"issuer", cert.Issuer,
					"notBefore", cert.NotBefore,
					"notAfter", cert.NotAfter,
				)
			}
		}
	}

	if resp.GetStatus() == grpc_health_v1.HealthCheckResponse_SERVING {
		return nil
	}
	return fmt.Errorf("service %s is not serving: %s", opt.Service, status)
}
