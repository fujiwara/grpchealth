package grpchealth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func TestRunServer(t *testing.T) {
	tests := []struct {
		name    string
		opt     CLIServer
		wantErr bool
	}{
		{
			name: "plaintext server",
			opt: CLIServer{
				Address: ":0", // Use dynamic port
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a listener for testing
			lis, err := net.Listen("tcp", ":0")
			if err != nil {
				t.Fatalf("Failed to create listener: %v", err)
			}
			defer lis.Close()

			// Update the address with the actual port
			tt.opt.Address = lis.Addr().String()

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			// Start server in goroutine
			errCh := make(chan error, 1)
			go func() {
				// Close the listener since runServer will create its own
				lis.Close()
				errCh <- runServer(ctx, tt.opt)
			}()

			// Give server time to start
			time.Sleep(100 * time.Millisecond)

			// Test connection
			conn, err := grpc.NewClient(tt.opt.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				if !tt.wantErr {
					t.Errorf("Failed to connect: %v", err)
				}
				return
			}
			defer conn.Close()

			client := grpc_health_v1.NewHealthClient(conn)
			resp, err := client.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
			if err != nil {
				if !tt.wantErr {
					t.Errorf("Health check failed: %v", err)
				}
				return
			}

			if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
				t.Errorf("Expected SERVING status, got %v", resp.Status)
			}

			// Cancel context to stop server
			cancel()

			// Wait for server to finish
			select {
			case err := <-errCh:
				if err != nil && !tt.wantErr {
					t.Errorf("runServer() error = %v, wantErr %v", err, tt.wantErr)
				}
			case <-time.After(3 * time.Second):
				t.Error("Server did not shut down gracefully")
			}
		})
	}
}

func TestRunServerWithTLS(t *testing.T) {
	// Create temporary certificate files
	certFile, keyFile, cleanup := createTempCertFiles(t)
	defer cleanup()

	opt := CLIServer{
		Address:  ":0",
		CertFile: certFile,
		KeyFile:  keyFile,
	}

	// Create a listener for testing
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer lis.Close()

	opt.Address = lis.Addr().String()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		lis.Close() // Close since runServer creates its own
		errCh <- runServer(ctx, opt)
	}()

	// Give server time to start
	time.Sleep(200 * time.Millisecond)

	// Test TLS connection
	creds := credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})
	conn, err := grpc.NewClient(opt.Address, grpc.WithTransportCredentials(creds))
	if err != nil {
		t.Fatalf("Failed to connect with TLS: %v", err)
	}
	defer conn.Close()

	client := grpc_health_v1.NewHealthClient(conn)
	resp, err := client.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}

	if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
		t.Errorf("Expected SERVING status, got %v", resp.Status)
	}

	// Cancel context to stop server
	cancel()

	// Wait for server to finish
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("runServer() error = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("Server did not shut down gracefully")
	}
}

// createTempCertFiles creates temporary certificate and key files for testing
func createTempCertFiles(t *testing.T) (certFile, keyFile string, cleanup func()) {
	// Generate a private key
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"Test"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"Test"},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		DNSNames:    []string{"localhost"},
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	// Create temporary files
	certTempFile, err := os.CreateTemp("", "cert-*.pem")
	if err != nil {
		t.Fatalf("Failed to create temp cert file: %v", err)
	}

	keyTempFile, err := os.CreateTemp("", "key-*.pem")
	if err != nil {
		os.Remove(certTempFile.Name())
		t.Fatalf("Failed to create temp key file: %v", err)
	}

	// Write certificate
	if err := pem.Encode(certTempFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		os.Remove(certTempFile.Name())
		os.Remove(keyTempFile.Name())
		t.Fatalf("Failed to write certificate: %v", err)
	}
	certTempFile.Close()

	// Write private key
	privDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		os.Remove(certTempFile.Name())
		os.Remove(keyTempFile.Name())
		t.Fatalf("Failed to marshal private key: %v", err)
	}

	if err := pem.Encode(keyTempFile, &pem.Block{Type: "PRIVATE KEY", Bytes: privDER}); err != nil {
		os.Remove(certTempFile.Name())
		os.Remove(keyTempFile.Name())
		t.Fatalf("Failed to write private key: %v", err)
	}
	keyTempFile.Close()

	cleanup = func() {
		os.Remove(certTempFile.Name())
		os.Remove(keyTempFile.Name())
	}

	return certTempFile.Name(), keyTempFile.Name(), cleanup
}

func TestRunServerInvalidCertificate(t *testing.T) {
	opt := CLIServer{
		Address:  ":0",
		CertFile: "nonexistent.crt",
		KeyFile:  "nonexistent.key",
	}

	ctx := context.Background()
	err := runServer(ctx, opt)
	if err == nil {
		t.Error("Expected error for invalid certificate files, got nil")
	}
}

// Benchmarks
func BenchmarkHealthCheck(b *testing.B) {
	// Setup logging for benchmark
	cleanup := setupBenchmarkLogger()
	defer cleanup()

	// Setup server
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		b.Fatalf("Failed to listen: %v", err)
	}
	defer lis.Close()

	s := grpc.NewServer()

	// Register health service
	grpc_health_v1.RegisterHealthServer(s, &healthServer{})

	go func() {
		if err := s.Serve(lis); err != nil {
			// Only log errors in benchmark mode
		}
	}()
	defer s.Stop()

	// Setup client
	conn, err := grpc.NewClient(lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		b.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := grpc_health_v1.NewHealthClient(conn)
	req := &grpc_health_v1.HealthCheckRequest{}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := client.Check(context.Background(), req)
			if err != nil {
				b.Fatalf("Health check failed: %v", err)
			}
		}
	})
}

// Mock health server for benchmarks
type healthServer struct {
	grpc_health_v1.UnimplementedHealthServer
}

func (h *healthServer) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	return &grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	}, nil
}
