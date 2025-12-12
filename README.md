# Glienicke - Nostr Relay in Go

A modular Nostr relay implementation in Go with clean architecture and comprehensive testing.

## Features

- **NIP-01 Compliant**: Full support for basic protocol flow (EVENT, REQ, CLOSE messages)
- **Event Validation**: Schnorr signature verification (BIP-340)
- **WebSocket Protocol**: Real-time bidirectional communication
- **Modular Architecture**: Clean separation of concerns with pluggable components
- **Comprehensive Testing**: Integration tests for all protocol aspects

## Quick Start

### Build and Run

```bash
# Build the relay
go build -o bin/relay ./cmd/relay

# Run the relay (creates relay.db automatically)
./bin/relay -addr :8080

# Run with custom database path
./bin/relay -addr :8080 -db /path/to/myrelay.db
```

Or run directly:

```bash
go run ./cmd/relay -addr :8080
```

### Database Configuration

The relay uses SQLite for persistent storage with autoconfiguration:

- **Default**: Creates `relay.db` in current directory
- **Custom path**: Use `-db` flag to specify database location
- **Auto-create**: Database is created automatically if it doesn't exist
- **Path expansion**: Supports `~/path.db`, relative and absolute paths

### Run Tests

```bash
# Run all tests
go test ./...

# Run integration tests only
go test ./test/integration/... -v

# Run with race detection
go test -race ./...
```

## Architecture

The relay follows a black box modular design:

```
┌─────────────────────────────────────┐
│       WebSocket Protocol Layer      │  ← Handles WS connections & messages
└─────────────────────────────────────┘
              ↓
┌─────────────────────────────────────┐
│         Relay Orchestrator          │  ← Coordinates components
└─────────────────────────────────────┘
              ↓
┌─────────────────────────────────────┐
│      Storage Interface (Black Box)  │  ← Swappable storage backend
└─────────────────────────────────────┘
```

### Key Components

- **`pkg/event`**: Core Nostr event primitives and validation
- **`pkg/storage`**: Storage interface (implementation-agnostic)
- **`pkg/protocol`**: WebSocket protocol handler
- **`pkg/relay`**: Main relay orchestrator
- **`pkg/nips`**: NIP-specific implementations (e.g., NIP-09, NIP-11)
- **`internal/store/memory`**: In-memory storage (for testing)
- **`internal/store/sqlite`**: SQLite storage implementation (production)

## Integration Tests

The integration tests demonstrate all core protocol functionality:

### Test Coverage

1. **TestEventMessage**: Posting events and receiving OK responses
2. **TestEventValidation**: Invalid event rejection with error messages
3. **TestReqMessage**: Subscription requests with filters
4. **TestCloseMessage**: Closing subscriptions
5. **TestEventBroadcast**: Real-time event broadcasting to subscribed clients
6. **TestMultipleFilters**: OR-ing multiple filters in subscriptions
7. **TestEventWithTags**: Events with tags (e, p, t, etc.)

### Running Specific Tests

```bash
# Test event posting
go test ./test/integration -run TestEventMessage -v

# Test event broadcasting
go test ./test/integration -run TestEventBroadcast -v

# Test validation
go test ./test/integration -run TestEventValidation -v
```

## Project Structure

```
glienicke/
├── cmd/
│   └── relay/              # Main application entry point
├── pkg/
│   ├── event/              # Event primitives & validation
│   ├── storage/            # Storage interface
│   ├── protocol/           # WebSocket protocol handler
│   ├── nips/               # NIP-specific implementations
│   │   ├── nip02/          # NIP-02 (Follow Lists)
│   │   ├── nip09/          # NIP-09 (Event Deletion)
│   │   ├── nip11/          # NIP-11 (Relay Information Document)
│   │   ├── nip42/          # NIP-42 (Authentication)
│   │   └── nip65/          # NIP-65 (Relay List Metadata)
│   └── relay/              # Relay orchestrator
├── internal/
│   ├── store/
│   │   ├── memory/         # In-memory storage implementation
│   │   └── sqlite/         # SQLite storage implementation
│   └── testutil/           # Test utilities (key generation, WS client)
└── test/
    └── integration/        # Integration tests
```

## Development

### Adding New NIPs

1. Create package under `pkg/nips/nipXX/`
2. Implement the NIP-specific logic
3. Add unit tests with mock storage
4. Add integration tests in `test/integration/`
5. Wire into relay orchestrator

### Crypto Details

- **Signature Scheme**: Schnorr signatures (BIP-340)
- **Public Keys**: 32-byte x-only format
- **Signatures**: 64-byte format
- **Library**: `github.com/btcsuite/btcd/btcec/v2/schnorr`

## Implemented NIPs

- **NIP-02: Follow Lists**: Handles `kind:3` follow list events with proper validation and replaceable event support. Includes support for petnames and relay hints in `p` tags.
- **NIP-09: Event Deletions**: Handles `kind:5` events to delete referenced events, as specified in NIP-09.
- **NIP-11: Relay Information Document**: Serves a JSON document at relay's root URL containing metadata about the relay, including supported NIPs, name, description, and version.
- **NIP-17: Private Direct Messages**: 
  - **NIP-59 Gift Wrap**
  - **NIP-44 Encrypted Payloads (Versioned)**
- **NIP-40: Event Expiration**: Supports `expiration` tag to automatically expire and filter events based on timestamp.
- **NIP-42: Authentication**: Handles `kind:22242` AUTH events for client authentication with signature verification.
- **NIP-50: Search Capability**: Supports full-text search across event content and tags with support for basic operators (AND, OR, NOT) and extensions like domain filtering.
- **NIP-65: Relay List Metadata**: Handles `kind:10002` relay list events for advertising preferred relays with read/write markers and proper validation.

## Planned NIPs

For compliance with gossip :
- NIP-56 Reporting
- NIP-62 Vanish requests

- NIP-70: Protected events
- NIP-77: Negentropy sync

## License

MIT
