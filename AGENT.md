## Project Overview

This project is a modular Nostr relay implementation in Go, named "Glienicke". It is designed with a clean architecture, separating concerns into distinct packages for event handling, storage, WebSocket protocol, and the main relay orchestrator.

The relay is NIP-01 compliant, supporting basic protocol flow for EVENT, REQ, and CLOSE messages. It includes event validation with Schnorr signature verification (BIP-340).

The project uses an in-memory store for development and testing, but the storage interface is designed to be pluggable, allowing for different storage backends to be used in production.

## Building and Running

### Build

```bash
go build -o bin/relay ./cmd/relay
```

### Run

```bash
./bin/relay -addr :8080
```

Or run directly:

```bash
go run ./cmd/relay -addr :8080
```

### Testing

Run all tests:

```bash
go test ./...
```

Run integration tests only:

```bash
go test ./test/integration/... -v
```

Run with race detection:

```bash
go test -race ./...
```

## Development Conventions

The project follows a modular design, with key components located in the `pkg/` directory:

-   `pkg/event`: Core Nostr event primitives and validation.
-   `pkg/storage`: Storage interface (implementation-agnostic).
-   `pkg/protocol`: WebSocket protocol handler.
-   `pkg/relay`: Main relay orchestrator.

The main application entry point is in `cmd/relay/main.go`. Integration tests are located in `test/integration/`.

When adding new NIPs, the convention is to create a new package under `pkg/nips/nipXX/`, implement the NIP-specific logic, add unit and integration tests, and then wire it into the relay orchestrator.

## Development Guidelines

Every new feature implemented will follow these steps : 

1. Identify features of the new NIP to be implemented
2. Write an integrations test for the new feature : at this stage the test should fail
3. Implement the features : if nostr specific features are needed check if available in github.com/nbd-wtf/go-nostr, otherwise implement. Keep dependency from external code at minimum
4. Run the integration test / debug / fix / until the integration test works
5. Bump the version, update README and CHANGELOG
6. Commit and create pull request

