package grpchealth

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"
)

// TestIntegrationServerClient tests the complete server-client interaction
func TestIntegrationServerClient(t *testing.T) {
	tests := []struct {
		name       string
		serverOpts CLIServer
		clientOpts CLIClient
		wantErr    bool
	}{
		{
			name: "plaintext server and client",
			serverOpts: CLIServer{
				Address: ":0", // Dynamic port
			},
			clientOpts: CLIClient{
				TLS:     false,
				Service: "",
			},
			wantErr: false,
		},
		{
			name: "plaintext server and client with specific service",
			serverOpts: CLIServer{
				Address: ":0",
			},
			clientOpts: CLIClient{
				TLS:     false,
				Service: "testservice",
			},
			wantErr: true, // Default health server doesn't register specific services
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get available port
			lis, err := net.Listen("tcp", ":0")
			if err != nil {
				t.Fatalf("Failed to get available port: %v", err)
			}
			address := lis.Addr().String()
			lis.Close()

			// Update server and client options with actual address
			tt.serverOpts.Address = address
			tt.clientOpts.Address = address

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			var wg sync.WaitGroup
			serverErrCh := make(chan error, 1)

			// Start server
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := runServer(ctx, tt.serverOpts)
				if err != nil {
					serverErrCh <- err
				}
			}()

			// Give server time to start
			time.Sleep(200 * time.Millisecond)

			// Run client
			clientErr := runClient(context.Background(), tt.clientOpts)

			// Check client result
			if (clientErr != nil) != tt.wantErr {
				t.Errorf("runClient() error = %v, wantErr %v", clientErr, tt.wantErr)
			}

			// Stop server
			cancel()

			// Wait for server to finish
			done := make(chan struct{})
			go func() {
				wg.Wait()
				close(done)
			}()

			select {
			case <-done:
				// Server stopped gracefully
			case err := <-serverErrCh:
				if err != nil {
					t.Errorf("Server error: %v", err)
				}
			case <-time.After(3 * time.Second):
				t.Error("Server did not shut down gracefully")
			}
		})
	}
}

// TestIntegrationTLSServerClient tests TLS communication
func TestIntegrationTLSServerClient(t *testing.T) {
	// Create temporary certificate files
	certFile, keyFile, cleanup := createTempCertFiles(t)
	defer cleanup()

	// Get available port
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to get available port: %v", err)
	}
	address := lis.Addr().String()
	lis.Close()

	serverOpts := CLIServer{
		Address:  address,
		CertFile: certFile,
		KeyFile:  keyFile,
	}

	clientOpts := CLIClient{
		Address:  address,
		TLS:      true,
		Insecure: true, // Skip certificate verification for test
		Service:  "",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	serverErrCh := make(chan error, 1)

	// Start server
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := runServer(ctx, serverOpts)
		if err != nil {
			serverErrCh <- err
		}
	}()

	// Give server more time to start with TLS
	time.Sleep(500 * time.Millisecond)

	// Run client
	clientErr := runClient(context.Background(), clientOpts)
	if clientErr != nil {
		t.Errorf("TLS client failed: %v", clientErr)
	}

	// Stop server
	cancel()

	// Wait for server to finish
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Server stopped gracefully
	case err := <-serverErrCh:
		if err != nil {
			t.Errorf("TLS Server error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("TLS Server did not shut down gracefully")
	}
}

// TestIntegrationMultipleClients tests multiple concurrent clients
func TestIntegrationMultipleClients(t *testing.T) {
	// Get available port
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to get available port: %v", err)
	}
	address := lis.Addr().String()
	lis.Close()

	serverOpts := CLIServer{
		Address: address,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	serverErrCh := make(chan error, 1)

	// Start server
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := runServer(ctx, serverOpts)
		if err != nil {
			serverErrCh <- err
		}
	}()

	// Give server time to start
	time.Sleep(200 * time.Millisecond)

	// Run multiple clients concurrently
	numClients := 10
	clientErrors := make(chan error, numClients)

	for i := range numClients {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			clientOpts := CLIClient{
				Address: address,
				TLS:     false,
				Service: "",
			}
			err := runClient(context.Background(), clientOpts)
			if err != nil {
				clientErrors <- err
			}
		}(i)
	}

	// Wait a bit for clients to complete
	time.Sleep(1 * time.Second)

	// Check for client errors
	close(clientErrors)
	for err := range clientErrors {
		if err != nil {
			t.Errorf("Client error: %v", err)
		}
	}

	// Stop server
	cancel()

	// Wait for all goroutines to finish
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All goroutines finished
	case err := <-serverErrCh:
		if err != nil {
			t.Errorf("Server error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("Test did not complete within timeout")
	}
}

// TestIntegrationServerShutdown tests graceful server shutdown
func TestIntegrationServerShutdown(t *testing.T) {
	// Get available port
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to get available port: %v", err)
	}
	address := lis.Addr().String()
	lis.Close()

	serverOpts := CLIServer{
		Address: address,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	serverStarted := make(chan struct{})
	serverDone := make(chan struct{})

	// Start server
	go func() {
		defer close(serverDone)

		// Signal that server is about to start
		close(serverStarted)

		err := runServer(ctx, serverOpts)
		if err != nil {
			t.Errorf("Server error: %v", err)
		}
	}()

	// Wait for server to signal it's starting
	<-serverStarted

	// Give server time to actually start listening
	time.Sleep(200 * time.Millisecond)

	// Test that server is responding
	clientOpts := CLIClient{
		Address: address,
		TLS:     false,
		Service: "",
	}

	err = runClient(context.Background(), clientOpts)
	if err != nil {
		t.Errorf("Initial client request failed: %v", err)
	}

	// Cancel context to trigger shutdown
	cancel()

	// Wait for server to shut down
	select {
	case <-serverDone:
		// Server shut down successfully
	case <-time.After(2 * time.Second):
		t.Error("Server did not shut down within timeout")
	}

	// Verify server is no longer responding
	err = runClient(context.Background(), clientOpts)
	if err == nil {
		t.Error("Expected connection failure after server shutdown, got success")
	}
}

// TestIntegrationErrorScenarios tests various error conditions
func TestIntegrationErrorScenarios(t *testing.T) {
	tests := []struct {
		name        string
		serverOpts  CLIServer
		clientOpts  CLIClient
		startServer bool
		description string
	}{
		{
			name: "client connection to non-existent server",
			clientOpts: CLIClient{
				Address: "localhost:99999",
				TLS:     false,
				Service: "",
			},
			startServer: false,
			description: "Should fail to connect to non-existent server",
		},
		{
			name: "invalid server address",
			serverOpts: CLIServer{
				Address: "invalid-address",
			},
			startServer: true,
			description: "Should fail to start server with invalid address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.startServer {
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
				defer cancel()

				err := runServer(ctx, tt.serverOpts)
				if err == nil {
					t.Error("Expected server to fail with invalid configuration")
				}
			} else {
				err := runClient(context.Background(), tt.clientOpts)
				if err == nil {
					t.Error("Expected client to fail with unreachable server")
				}
			}
		})
	}
}

// Benchmark integration test
func BenchmarkIntegrationHealthCheck(b *testing.B) {
	// Setup logging for benchmark
	cleanup := setupBenchmarkLogger()
	defer cleanup()

	// Get available port
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		b.Fatalf("Failed to get available port: %v", err)
	}
	address := lis.Addr().String()
	lis.Close()

	serverOpts := CLIServer{
		Address: address,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start server
	go func() {
		runServer(ctx, serverOpts)
	}()

	// Give server time to start
	time.Sleep(200 * time.Millisecond)

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
