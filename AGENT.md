# Glienicke Nostr Relay - Development Guidelines

## Build, Lint, and Test Commands

### Build
```bash
# Build the relay binary
go build -o bin/relay ./cmd/relay

# Build for different platforms
GOOS=linux GOARCH=amd64 go build -o bin/relay-linux ./cmd/relay
GOOS=darwin GOARCH=amd64 go build -o bin/relay-mac ./cmd/relay
GOOS=windows GOARCH=amd64 go build -o bin/relay.exe ./cmd/relay
```

### Run Tests
```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test ./... -v

# Run tests with race detection
go test -race ./...

# Run tests with coverage
go test -cover ./...

# Clean test cache (recommended after major changes)
go clean -testcache

# Run specific test packages
go test ./pkg/event -v
go test ./pkg/nips/nip02 -v
go test ./internal/store/memory -v
go test ./internal/store/sqlite -v
go test ./test/integration -v

# Run single test
go test ./test/integration -run TestNIP01_BasicProtocol -v

# Run tests with timeout
go test ./test/integration -v -timeout 60s
```

### Development Tools
```bash
# Format code
go fmt ./...

# Run linter (if golangci-lint is installed)
golangci-lint run

# Vet code for potential issues
go vet ./...

# Tidy dependencies
go mod tidy

# Download dependencies
go mod download

# Verify dependencies
go mod verify
```

## Code Style Guidelines

### Import Organization
- Group imports: standard library, third-party packages, local packages
- Use blank lines between import groups
- Local packages use relative imports within the project

```go
import (
    "context"
    "fmt"                     // standard library
    "time"

    "github.com/gorilla/websocket"  // third-party
    "github.com/nbd-wtf/go-nostr"

    "github.com/paul/glienicke/pkg/event"  // local packages
    "github.com/paul/glienicke/pkg/storage"
)
```

### Naming Conventions
- **Packages**: lowercase, single word (e.g., `relay`, `event`, `storage`)
- **Constants**: `CamelCase` with descriptive names (e.g., `Version`, `KindTextNote`)
- **Variables**: `camelCase` (e.g., `relayStore`, `clientConn`)
- **Functions**: `CamelCase` (e.g., `HandleEvent`, `SaveEvent`)
- **Interfaces**: Usually end with `-er` suffix (e.g., `Storage`, `Handler`)
- **Private**: unexported (lowercase) within packages
- **Test Functions**: `TestPackageName_FunctionName` with descriptive subtests

### Error Handling
- Use error wrapping with `fmt.Errorf("context: %w", err)`
- Return errors from functions, don't panic unless unrecoverable
- Use custom error types when beneficial (e.g., `storage.ErrNotFound`)
- Log errors at appropriate levels using `log.Printf()`

### Types and Formatting
- Use concrete types instead of `interface{}` where possible
- Prefer `string` over `[]byte` for text data
- Use `int64` for timestamps (Unix timestamps)
- Follow Go formatting conventions (`go fmt ./...`)
- Keep lines under 120 characters where practical

### Testing Guidelines
- Write tests before implementation (TDD) for new features
- Use table-driven tests for multiple test cases
- Include both unit tests and integration tests
- Use `require.NoError(t, err)` for setup, `assert.Equal(t, a, b)` for assertions
- Clean up resources in `defer` statements
- Test both success and failure paths

### NIP Implementation Pattern
1. Create package under `pkg/nips/nipXX/`
2. Implement NIP logic with validation
3. Add unit tests with mock storage
4. Add integration tests in `test/integration/`
5. Wire into relay orchestrator in `pkg/relay/relay.go`
6. Update supported NIPs list in NIP-11 response
7. Bump version and update CHANGELOG

### Storage Interface
- All storage backends must implement the `storage.Store` interface
- Handle `context.Context` for cancellation and timeouts
- Use prepared statements for database operations
- Implement proper transaction handling for SQLite backend
- Maintain compatibility between memory and SQLite implementations

### Dependencies
- Minimize external dependencies
- Prefer standard library when possible
- Use well-maintained, actively developed packages
- Keep dependencies up-to-date with `go get -u ./...`
- Vendor dependencies only when necessary (currently not vendored)