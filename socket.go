package grpchealth

import "strings"

// isUnixSocket checks if the given address is a Unix Domain Socket
func isUnixSocket(address string) bool {
	// unix: prefix (e.g., unix:/tmp/grpc.sock)
	if strings.HasPrefix(address, "unix:") {
		return true
	}
	// Absolute path (e.g., /tmp/grpc.sock)
	if strings.HasPrefix(address, "/") {
		return true
	}
	return false
}

// parseUnixSocketPath extracts the socket path from various formats
func parseUnixSocketPath(address string) string {
	// Remove unix: prefix if present
	if strings.HasPrefix(address, "unix:") {
		return strings.TrimPrefix(address, "unix:")
	}
	// Return as-is for absolute paths
	return address
}