# grpchealth

A simple gRPC health check tool for testing and monitoring gRPC services.

## Overview

grpchealth provides both server and client implementations of the gRPC Health Checking Protocol. It allows you to quickly spin up a health check server for testing or verify the health status of existing gRPC services.

## Features

- **Server Mode**: Run a standalone gRPC health check server
  - Support for TLS/SSL connections
  - Graceful shutdown on interrupt signals
  - Configurable server address

- **Client Mode**: Check health status of gRPC services
  - Support for TLS connections with certificate verification
  - Insecure mode for testing (skip certificate verification)
  - Service-specific health checks
  - Connection timing information
  - Peer certificate information display

## Installation

```bash
go install github.com/fujiwara/grpchealth/cmd/grpchealth@latest
```

Or build from source:

```bash
git clone https://github.com/fujiwara/grpchealth.git
cd grpchealth
make
```

## Usage

```
Usage: grpchealth <command>

Flags:
  -h, --help    Show context-sensitive help.

Commands:
  server <address> [flags]
    Run gRPC health check server

  client <address> [flags]
    Run gRPC health check client

Run "grpchealth <command> --help" for more information on a command.
```

### Server Mode

Start a basic gRPC health check server:

```bash
grpchealth server localhost:50051
```

Start a server with TLS:

```bash
grpchealth server localhost:50051 --cert-file server.crt --key-file server.key
```

#### Server Options

```
Usage: grpchealth server <address> [flags]

Run gRPC health check server

Arguments:
  <address>    gRPC server address

Flags:
  -h, --help                Show context-sensitive help.

  -c, --cert-file=STRING    Path to the server certificate file
  -k, --key-file=STRING     Path to the server key file
```

### Client Mode

Check health of a gRPC service:

```bash
grpchealth client localhost:50051
```

Check with TLS:

```bash
grpchealth client localhost:50051 --tls
```

Check with TLS (skip certificate verification):

```bash
grpchealth client localhost:50051 --tls --insecure
```

Check specific service health:

```bash
grpchealth client localhost:50051 --service myservice
```

#### Client Options

```
Usage: grpchealth client <address> [flags]

Run gRPC health check client

Arguments:
  <address>    gRPC client address

Flags:
  -h, --help          Show context-sensitive help.

  -t, --tls           Use TLS for connection
  -k, --insecure      Use insecure connection
  -s, --service=""    Service name to check health status
```

## Examples

### Testing with a local server

1. Start the health check server:
```bash
grpchealth server :50051
```

2. In another terminal, check its health:
```bash
grpchealth client localhost:50051
```

### Testing with TLS

1. Generate self-signed certificates (for testing):
```bash
openssl req -x509 -newkey rsa:4096 -keyout server.key -out server.crt -days 365 -nodes
```

2. Start server with TLS:
```bash
grpchealth server :50051 --cert-file server.crt --key-file server.key
```

3. Check health with TLS:
```bash
grpchealth client localhost:50051 --tls --insecure
```

## Development

### Building

```bash
make
```

### Running tests

```bash
make test
```

### Installing locally

```bash
make install
```

### Creating distribution packages

```bash
make dist
```

## License

See [LICENSE](LICENSE) file for details.

## Author

Fujiwara Shunichiro
