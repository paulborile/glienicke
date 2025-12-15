# Glienicke - Nostr Relay in Go

A modular Nostr relay implementation in Go with clean architecture and comprehensive testing.

## Features

- **NIP-01 Compliant**: Full support for basic protocol flow (EVENT, REQ, CLOSE messages)
- **Secure WebSocket (WSS)**: TLS encryption support with certificate management for production deployments
- **Private Messaging**: Complete support for both legacy (NIP-04) and modern (NIP-17) encrypted direct messages
- **Event Validation**: Schnorr signature verification (BIP-340) with comprehensive event validation
- **Search & Filtering**: Full-text search capability with advanced filtering options (NIP-50)
- **Authentication**: Client authentication with challenge-response protocol (NIP-42)
- **Event Management**: Event deletion, expiration, and bulk operations (NIP-09, NIP-40, NIP-62)
- **Social Features**: Reactions, comments, and long-form content support (NIP-22, NIP-25)
- **WebSocket Protocol**: Real-time bidirectional communication with efficient broadcasting
- **Modular Architecture**: Clean separation of concerns with pluggable storage backends
- **Comprehensive Testing**: Integration tests for all protocol aspects with extensive coverage

## Quick Start

### Build and Run

```bash
# Build the relay
go build -o bin/relay ./cmd/relay

# Run the relay (creates relay.db automatically)
./bin/relay -addr :8080

# Run with TLS/WSS support
./bin/relay -cert resources/relay-cert.pem -key resources/relay-key.pem -addr :8443

# Run with custom database path
./bin/relay -addr :8080 -db /path/to/myrelay.db
```

Or run directly:

```bash
# Non-TLS
go run ./cmd/relay -addr :8080

# With TLS/WSS
go run ./cmd/relay -cert resources/relay-cert.pem -key resources/relay-key.pem -addr :8443
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
│   │   ├── nip04/          # NIP-04 (Encrypted Direct Messages - Legacy)
│   │   ├── nip09/          # NIP-09 (Event Deletion)
│   │   ├── nip11/          # NIP-11 (Relay Information Document)
│   │   ├── nip17/          # NIP-17 (Private Direct Messages - Modern)
│   │   ├── nip22/          # NIP-22 (Comment Threads)
│   │   ├── nip40/          # NIP-40 (Event Expiration)
│   │   ├── nip42/          # NIP-42 (Authentication)
│   │   ├── nip44/          # NIP-44 (Encrypted Payloads)
│   │   ├── nip45/          # NIP-45 (Event Counts)
│   │   ├── nip50/          # NIP-50 (Search Capability)
│   │   ├── nip56/          # NIP-56 (Reporting)
│   │   ├── nip59/          # NIP-59 (Gift Wrapping)
│   │   ├── nip62/          # NIP-62 (Request to Vanish)
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

### **Core Protocol**
- **NIP-01: Basic Protocol Flow**: Full support for EVENT, REQ, CLOSE messages with proper WebSocket communication and event broadcasting.

### **Social Features**
- **NIP-02: Follow Lists**: Handles `kind:3` follow list events with proper validation and replaceable event support. Includes support for petnames and relay hints in `p` tags.
- **NIP-22: Comment Threads**: Handles `kind:1111` comment events for threaded discussions on various content types including blog posts, files, and web URLs. Includes proper validation of root/parent tag relationships and prevents comments on kind 1 notes (which should use NIP-10 instead).

### **Content Management**
- **NIP-09: Event Deletions**: Handles `kind:5` events to delete referenced events with proper authorization checks.
- **NIP-40: Event Expiration**: Supports `expiration` tag to automatically expire and filter events based on timestamp.

### **Private Messaging**
- **NIP-04: Encrypted Direct Messages (Legacy)**: AES-256-CBC encrypted direct messages with backward compatibility. Includes content parsing, recipient extraction, and proper encryption/decryption workflows.
- **NIP-17: Private Direct Messages (Modern)**: Complete implementation of modern private messaging with:
  - **NIP-44 Encryption**: XChaCha20-Poly1305 AEAD encryption for strong security
  - **NIP-59 Gift Wrapping**: Secure message delivery with metadata protection
  - **Multiple Recipients**: Support for group conversations
  - **File Messages**: Kind 15 support for file sharing
  - **Reply Threading**: Conversation context and threading support

### **Security & Authentication**
- **NIP-42: Authentication**: Handles `kind:22242` AUTH events for client authentication with signature verification and challenge-response protocol.

### **Advanced Features**
- **NIP-11: Relay Information Document**: Serves JSON metadata at root URL including supported NIPs, name, description, version, and relay capabilities.
- **NIP-45: Event Counts**: Supports COUNT message type for efficient event counting with filters, returning `{"count": <integer>}` responses for performance optimization.
- **NIP-50: Search Capability**: Full-text search across event content and tags with support for basic operators (AND, OR, NOT) and domain filtering extensions.
- **NIP-56: Reporting**: Handles `kind:1984` report events for flagging objectionable content including profiles, notes, and blobs with comprehensive validation.
- **NIP-62: Request to Vanish**: Handles `kind:62` events for requesting complete deletion of all events from a specific pubkey, supporting both relay-specific and global deletion requests.
- **NIP-65: Relay List Metadata**: Handles `kind:10002` relay list events for advertising preferred relays with read/write markers and proper validation.

### **Infrastructure**
- **WSS/TLS Support**: Complete secure WebSocket implementation with:
  - TLS certificate management and automatic HTTPS/WSS support
  - Certificate generation tools for development and production
  - Backward compatibility with non-TLS connections
  - Production-ready security with proper certificate validation

## Testing

### Comprehensive Test Coverage

The project includes extensive testing at multiple levels:

#### **Unit Tests**
- **Storage Layer**: Comprehensive unit tests for both memory and SQLite storage backends
  - CRUD operations (Create, Read, Update, Delete)
  - Event filtering and querying
  - NIP-45 COUNT functionality
  - NIP-62 bulk deletion operations
  - Replaceable events handling
  - Concurrent access and thread safety
  - Data persistence and edge cases
- **Event Validation**: Unit tests for NIP implementations
- **Protocol Layer**: Message handling and parsing tests

#### **Integration Tests**
- Full protocol testing with real WebSocket connections
- NIP compliance validation for all implemented NIPs
- End-to-end event flow testing
- Relay Information Document (NIP-11) verification

#### **Test Commands**

```bash
# Run all tests
go test ./...

# Run storage unit tests specifically
go test ./internal/store/memory/ -v
go test ./internal/store/sqlite/ -v

# Run integration tests only
go test ./test/integration/... -v

# Run with race detection
go test -race ./...

# Clean test cache (recommended after major changes)
go clean -testcache
```

#### **Test Coverage**
- **Memory Storage**: 17 test functions covering all major functionality
- **SQLite Storage**: 16 test functions with real database behavior validation
- **Integration Tests**: Tests for all implemented NIPs (01, 02, 09, 11, 17, 22, 40, 42, 44, 45, 50, 56, 62, 65)

## Planned NIPs

For enhanced functionality and ecosystem compliance:
- NIP-28: Public Chat channels and communities
- NIP-70: Protected events and access control
- NIP-77: Negentropy sync for efficient synchronization

## License

MIT
