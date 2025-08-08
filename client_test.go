package grpchealth

import (
	"context"
	"crypto/tls"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

func TestRunClient(t *testing.T) {
	// Setup a real TCP server for testing
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer lis.Close()

	s := grpc.NewServer()
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus("testservice", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(s, healthServer)

	go func() {
		if err := s.Serve(lis); err != nil {
			t.Logf("Server stopped: %v", err)
		}
	}()
	defer s.Stop()

	address := lis.Addr().String()

	tests := []struct {
		name    string
		opt     CLIClient
		wantErr bool
	}{
		{
			name: "plaintext connection - default service",
			opt: CLIClient{
				Address: address,
				Service: "",
			},
			wantErr: false,
		},
		{
			name: "plaintext connection - specific service",
			opt: CLIClient{
				Address: address,
				Service: "testservice",
			},
			wantErr: false,
		},
		{
			name: "service not serving",
			opt: CLIClient{
				Address: address,
				Service: "nonexistent",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err := runClient(ctx, tt.opt)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
		})
	}
}

func TestRunClientTLS(t *testing.T) {
	// Create temporary certificate files
	certFile, keyFile, cleanup := createTempCertFiles(t)
	defer cleanup()

	// Setup TLS server
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		t.Fatalf("Failed to load key pair: %v", err)
	}

	creds := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
	})

	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer lis.Close()

	s := grpc.NewServer(grpc.Creds(creds))
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(s, healthServer)

	go func() {
		if err := s.Serve(lis); err != nil {
			t.Errorf("Server failed: %v", err)
		}
	}()
	defer s.Stop()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	tests := []struct {
		name    string
		opt     CLIClient
		wantErr bool
	}{
		{
			name: "TLS with insecure mode",
			opt: CLIClient{
				Address:  lis.Addr().String(),
				TLS:      true,
				Insecure: true,
				Service:  "",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err := runClient(ctx, tt.opt)
			if (err != nil) != tt.wantErr {
				t.Errorf("runClient() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRunClientConnectionFailure(t *testing.T) {
	opt := CLIClient{
		Address: "localhost:99999", // Non-existent port
		Service: "",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := runClient(ctx, opt)
	if err == nil {
		t.Error("Expected connection failure, got nil error")
	}
}

func TestRunClientServiceNotServing(t *testing.T) {
	// Setup test server with NOT_SERVING status
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer lis.Close()

	s := grpc.NewServer()
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	grpc_health_v1.RegisterHealthServer(s, healthServer)

	go func() {
		if err := s.Serve(lis); err != nil {
			t.Logf("Server stopped: %v", err)
		}
	}()
	defer s.Stop()

	// Test with actual connection
	conn, err := grpc.NewClient(lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := grpc_health_v1.NewHealthClient(conn)
	resp, err := client.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}

	if resp.Status == grpc_health_v1.HealthCheckResponse_SERVING {
		t.Error("Expected NOT_SERVING status, got SERVING")
	}
}

func TestRunClientInvalidAddress(t *testing.T) {
	opt := CLIClient{
		Address: "invalid-address",
		Service: "",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := runClient(ctx, opt)
	if err == nil {
		t.Error("Expected error for invalid address, got nil")
	}
}

func TestHealthCheckWithPeerInfo(t *testing.T) {
	// Setup test server
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer lis.Close()

	s := grpc.NewServer()
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(s, healthServer)

	go func() {
		if err := s.Serve(lis); err != nil {
			t.Logf("Server stopped: %v", err)
		}
	}()
	defer s.Stop()

	conn, err := grpc.NewClient(lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := grpc_health_v1.NewHealthClient(conn)
	req := &grpc_health_v1.HealthCheckRequest{}

	ctx := context.Background()
	resp, err := client.Check(ctx, req)
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}

	if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
		t.Errorf("Expected SERVING status, got %v", resp.Status)
	}
}

// Benchmark for client operations
func BenchmarkClientHealthCheck(b *testing.B) {
	// Setup logging for benchmark
	cleanup := setupBenchmarkLogger()
	defer cleanup()

	// Setup test server
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		b.Fatalf("Failed to listen: %v", err)
	}
	defer lis.Close()

	s := grpc.NewServer()
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(s, healthServer)

	go func() {
		if err := s.Serve(lis); err != nil {
			// Suppress logs in benchmark mode
		}
	}()
	defer s.Stop()

	address := lis.Addr().String()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			err := runBenchmarkClient(address)
			if err != nil {
				b.Fatalf("Health check failed: %v", err)
			}
		}
	})
}

func TestRunClientUnixSocket(t *testing.T) {
	// Create temporary socket path
	tempDir := t.TempDir()
	socketPath := filepath.Join(tempDir, "test.sock")

	// Setup Unix socket server
	lis, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("Failed to listen on unix socket: %v", err)
	}
	defer lis.Close()
	defer os.RemoveAll(socketPath)

	s := grpc.NewServer()
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(s, healthServer)

	go func() {
		if err := s.Serve(lis); err != nil {
			t.Logf("Server stopped: %v", err)
		}
	}()
	defer s.Stop()

	// Test Unix socket client
	opt := CLIClient{
		Address: "unix:" + socketPath,
		Service: "",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = runClient(ctx, opt)
	if err != nil {
		t.Errorf("Unix socket client failed: %v", err)
	}
}

// Test helper functions
func TestGRPCStatusCodes(t *testing.T) {
	// Setup test server that returns different status codes
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer lis.Close()

	s := grpc.NewServer()

	// Register a mock health server that can return errors
	grpc_health_v1.RegisterHealthServer(s, &mockHealthServer{})

	go func() {
		if err := s.Serve(lis); err != nil {
			t.Logf("Server stopped: %v", err)
		}
	}()
	defer s.Stop()

	conn, err := grpc.NewClient(lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := grpc_health_v1.NewHealthClient(conn)

	// Test with service that returns NOT_FOUND
	req := &grpc_health_v1.HealthCheckRequest{Service: "notfound"}
	_, err = client.Check(context.Background(), req)
	if err == nil {
		t.Error("Expected error for not found service")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("Expected gRPC status error")
	}

	if st.Code() != codes.NotFound {
		t.Errorf("Expected NOT_FOUND status, got %v", st.Code())
	}
}

// Mock health server for testing error conditions
type mockHealthServer struct {
	grpc_health_v1.UnimplementedHealthServer
}

func (m *mockHealthServer) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	switch req.Service {
	case "notfound":
		return nil, status.Error(codes.NotFound, "service not found")
	case "":
		return &grpc_health_v1.HealthCheckResponse{
			Status: grpc_health_v1.HealthCheckResponse_SERVING,
		}, nil
	default:
		return &grpc_health_v1.HealthCheckResponse{
			Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
		}, nil
	}
}
