# Glienicke Nostr Relay - Development Guidelines

## Development Guidelines

Every new feature implemented will follow these steps : 

1. Identify features to be implemented
2. Write an integrations test for the new feature : at this stage the test should fail. This is called TDD (Test Driven Design) and includes developing
basic empty stubs for the new code so that intengration/unit tests can be written.
3. Implement the features : if nostr specific features are needed check if available in github.com/nbd-wtf/go-nostr, otherwise implement. Keep dependency from external code at minimum, implement unit tests for new features
4. Run unit and integration test / debug / fix / until the both test work without failures
5. Bump the version, update README and CHANGELOG
6. NEVER REMOVE FILES WITHOUT CONFIRMATION

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
7. Bump version and update CHANGELOG and README

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

## Issue Tracking with bd (beads)

**IMPORTANT**: This project uses **bd (beads)** for ALL issue tracking. Do NOT use markdown TODOs, task lists, or other tracking methods.

### Why bd?

- Dependency-aware: Track blockers and relationships between issues
- Git-friendly: Auto-syncs to JSONL for version control
- Agent-optimized: JSON output, ready work detection, discovered-from links
- Prevents duplicate tracking systems and confusion

### Quick Start

**Check for ready work:**
```bash
bd ready --json
```

**Create new issues:**
```bash
bd create "Issue title" -t bug|feature|task -p 0-4 --json
bd create "Issue title" -p 1 --deps discovered-from:bd-123 --json
bd create "Subtask" --parent <epic-id> --json  # Hierarchical subtask (gets ID like epic-id.1)
```

**Claim and update:**
```bash
bd update bd-42 --status in_progress --json
bd update bd-42 --priority 1 --json
```

**Complete work:**
```bash
bd close bd-42 --reason "Completed" --json
```

### Issue Types

- `bug` - Something broken
- `feature` - New functionality
- `task` - Work item (tests, docs, refactoring)
- `epic` - Large feature with subtasks
- `chore` - Maintenance (dependencies, tooling)

### Priorities

- `0` - Critical (security, data loss, broken builds)
- `1` - High (major features, important bugs)
- `2` - Medium (default, nice-to-have)
- `3` - Low (polish, optimization)
- `4` - Backlog (future ideas)

### Workflow for AI Agents

1. **Check ready work**: `bd ready` shows unblocked issues
2. **Claim your task**: `bd update <id> --status in_progress`
3. **Work on it**: Implement, test, document
4. **Discover new work?** Create linked issue:
   - `bd create "Found bug" -p 1 --deps discovered-from:<parent-id>`
5. **Complete**: `bd close <id> --reason "Done"`
6. **Commit together**: Always commit the `.beads/issues.jsonl` file together with the code changes so issue state stays in sync with code state

### Auto-Sync

bd automatically syncs with git:
- Exports to `.beads/issues.jsonl` after changes (5s debounce)
- Imports from JSONL when newer (e.g., after `git pull`)
- No manual export/import needed!

### GitHub Copilot Integration

If using GitHub Copilot, also create `.github/copilot-instructions.md` for automatic instruction loading.
Run `bd onboard` to get the content, or see step 2 of the onboard instructions.

### MCP Server (Recommended)

If using Claude or MCP-compatible clients, install the beads MCP server:

```bash
pip install beads-mcp
```

Add to MCP config (e.g., `~/.config/claude/config.json`):
```json
{
  "beads": {
    "command": "beads-mcp",
    "args": []
  }
}
```

Then use `mcp__beads__*` functions instead of CLI commands.

### Managing AI-Generated Planning Documents

AI assistants often create planning and design documents during development:
- PLAN.md, IMPLEMENTATION.md, ARCHITECTURE.md
- DESIGN.md, CODEBASE_SUMMARY.md, INTEGRATION_PLAN.md
- TESTING_GUIDE.md, TECHNICAL_DESIGN.md, and similar files

**Best Practice: Use a dedicated directory for these ephemeral files**

**Recommended approach:**
- Create a `history/` directory in the project root
- Store ALL AI-generated planning/design docs in `history/`
- Keep the repository root clean and focused on permanent project files
- Only access `history/` when explicitly asked to review past planning

**Example .gitignore entry (optional):**
```
# AI planning documents (ephemeral)
history/
```

**Benefits:**
- ✅ Clean repository root
- ✅ Clear separation between ephemeral and permanent documentation
- ✅ Easy to exclude from version control if desired
- ✅ Preserves planning history for archeological research
- ✅ Reduces noise when browsing the project

### CLI Help

Run `bd <command> --help` to see all available flags for any command.
For example: `bd create --help` shows `--parent`, `--deps`, `--assignee`, etc.

### Important Rules

- ✅ Use bd for ALL task tracking
- ✅ Always use `--json` flag for programmatic use
- ✅ Link discovered work with `discovered-from` dependencies
- ✅ Check `bd ready` before asking "what should I work on?"
- ✅ Store AI planning docs in `history/` directory
- ✅ Run `bd <cmd> --help` to discover available flags
- ❌ Do NOT create markdown TODO lists
- ❌ Do NOT use external issue trackers
- ❌ Do NOT duplicate tracking systems
- ❌ Do NOT clutter repo root with planning documents

