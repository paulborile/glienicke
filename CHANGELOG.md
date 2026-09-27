# Changelog

## 0.20.3 - 2026-09-27

### Fixed
- REQ replies could overflow a client's own send queue and get it disconnected as a "slow client" on its very first subscription, even when the client was healthy and reading immediately: `defaultMaxEventsPerREQ` (100) was larger than the per-client send queue (`sendQueueSize`, 64), and `HandleReq` enqueues its entire capped reply in one uninterrupted loop with no pacing. Any subscription matching 65+ stored events overflowed the queue before the write goroutine could drain a single message, tripping the queue-full disconnect and logging the remaining queued sends as "client closed" — visible in production as a repeating burst of "Send queue full ... disconnecting slow client" followed by dozens of "Failed to send stored event to client: client closed" lines, especially for clients that auto-reconnect and immediately resubscribe. `defaultMaxEventsPerREQ` is now 50, kept with headroom under the send queue so a REQ's entire reply always fits in one burst.

## 0.20.2 - 2026-09-27

### Fixed
- NIP-42 `AUTH` message type was never handled: `MessageTypeAuth` was defined but the message dispatcher only handled `EVENT`/`REQ`/`CLOSE`/`COUNT`, so a client replying to an auth challenge with the spec's own `["AUTH", <event>]` message (rather than wrapping it in `["EVENT", ...]`) got `unknown message type: AUTH` and never authenticated. This is a real protocol gap fixed here, though investigation of a subsequent production log (after this fix was deployed) showed the reconnect-storm symptom actually seen in prod was a separate bug — see 0.20.3. The pre-auth handshake gate and the message dispatcher now accept the auth event via either message shape.

## 0.20.1 - 2026-06-17

### Fixed
- Out-of-memory restarts under the container memory limit: production was OOM-killed (cgroup memory limit) roughly hourly to twice-daily, every kill at ~375 MiB against a 384 MiB limit. The growth was in cgo SQLite memory (invisible to `GOMEMLIMIT`) driven by unbounded query result sets on a multi-GB database. Two fixes:
  - REQ filter limits are now clamped server-side to `maxEventsPerREQ` (100) before querying, so a filter with no client-supplied limit no longer materializes the entire matching set (thousands of full events) into memory. Previously the cap was applied in Go only after loading every matching row.
  - SQLite `MaxOpenConns` reduced from 25 to 8. Each connection can hold its own page cache and in-memory temp working set, so the pool size directly multiplies worst-case query memory under concurrent load.

## 0.20.0 - 2026-06-15

### Fixed
- Silent process crashes: a panic in any per-connection goroutine (handling untrusted client input) or in a broadcast send goroutine would take down the whole relay with no log line. Added panic recovery to the read/write pumps and routed broadcast and retention goroutines through a `safeGo` helper that recovers and logs the stack. Investigation of production logs showed ~80 unlogged restarts over 8 days driven by this.
- Data race in the `/health` handler: it held the metrics read lock while `updateMetrics` wrote `packetCount`/`packetsPerSecond`/`dbStatus`. `updateMetrics` now takes its own write lock (and runs the DB ping outside the lock to avoid stalling connection handlers); the handler snapshots fields under a short read lock.
- Slow clients could block sender goroutines indefinitely: every send blocked until the client's 256-deep queue drained. Sends are now non-blocking and a client whose queue is full is disconnected, bounding per-client memory under backpressure.

### Added
- Global connection cap (1024 concurrent connections across all IPs): new connections are rejected with HTTP 503 before the WebSocket upgrade. Hard memory-safety ceiling, enforced even when rate limiting is disabled.
- Per-IP connection cap (32 concurrent connections per source IP): excess connections rejected with HTTP 429 before the upgrade, preventing a single source from exhausting goroutines/memory.

### Changed
- Bounded the per-client outbound send queue from 256 to 64 messages, with disconnect-on-full, to cap per-connection memory.
- Bounded broadcast fan-out: a single event broadcast now dispatches sends through a semaphore (max 64 concurrent) inside one background goroutine, instead of spawning an unbounded goroutine per client per event. The publishing client is no longer blocked by slow subscribers.

## 0.19.8 - 2026-04-29

### Added
- NIP-36 content-warning enforcement: events containing terms from a configurable vocabulary file are rejected unless they carry a `content-warning` tag
- Vocabulary file is reloaded automatically when its mtime changes (polled every 30s)
- New `--nip36-vocab <path>` CLI flag to enable enforcement and point at the vocabulary file
- Default vocabulary at `resources/nip36-vocab.txt` covers known NSFW domains, hashtags, and Telegram/anime image-board patterns
- New `nip36` package with `Policy` API: `New`, `StartWatcher`, `ShouldReject`, `MatchedTerm`, `HasContentWarning`

## 0.19.7 - 2026-04-07

### Added
- NIP-16: Ephemeral events (kinds 20000-29999) are now broadcast to subscribers but never stored in the database
- NIP-01: Replaceable events (kind 0, 3, 10000-19999) — SQLite store now enforces keeping only the newest event per pubkey+kind, deleting older duplicates on save
- Event retention: background goroutine deletes events older than 30 days (configurable via `SetRetentionDays`), runs every hour
- Retention exempts long-lived identity events: kind 0 (profile metadata), kind 3 (contact list), kind 10002 (relay lists), kind 10050 (DM relay lists)

## 0.19.6 - 2026-04-01

### Changed
- NIP-42 AUTH reverted to opt-in (default off) — most Nostr clients don't support NIP-42, requiring it blocks all legitimate traffic

## 0.19.5 - 2026-03-30

### Changed
- NIP-42 AUTH now required by default (was opt-in)
- AUTH check now blocks all unauthenticated messages (EVENT, REQ, COUNT), not just REQ/COUNT. Only AUTH events (kind 22242) and CLOSE pass through without authentication.
- Ban log messages now include authenticated pubkeys for the banned IP
- Rate limit check tracks pubkey per IP for abuse attribution

## 0.19.4 - 2026-03-30

### Added
- Auto-close subscriptions after EOSE to free slots (opt-in via `SetCloseAfterEOSE`)
- Log User-Agent and Origin headers on new WebSocket connections

### Changed
- Reduce ban violation threshold from 50 to 10 (abusers hit it in under a second anyway)
- Remove noisy log messages: routine subscription closes and normal stored event counts
- Only log stored event counts when capped by `maxEventsPerREQ`

## 0.19.3 - 2026-03-30

### Added
- Per-IP rate limiting on all message types (10 msg/sec with burst of 20) to prevent abuse
- Per-connection max concurrent subscriptions limit (20) with CLOSED message on rejection
- Auto-ban IPs after 50 rate limit violations (24-hour ban duration)
- Max events per REQ response cap (default 100) to limit per-request resource usage
- Real client IP logging from X-Forwarded-For/X-Real-IP headers when running behind a reverse proxy
- Rate limited request count exposed in health endpoint metrics
- `GLIENICKE_RATE_LIMIT_ENABLED` env var to disable rate limiting (for load testing)
- NIP-42 AUTH support: optional authentication requirement with challenge/response handshake (opt-in via `SetRequireAuth`)
- `--version` flag to print version and exit

### Fixed
- Suppress spurious "WebSocket read error: close 1000 (normal)" log messages

## 0.19.2 - 2026-03-18

### Fixed
- Remove incorrect WebSocket proxy detection that rejected all connections arriving via Traefik (`X-Forwarded-Proto: wss` is standard SSL termination behavior, not an error)

## 0.19.1 - 2026-03-12

### Fixed

* **SQLite tag parser (`internal/store/sqlite`):** Fixed panic `slice bounds out of range [1:0]` in `parseTagString` when a tag part is a single `"` character. Added `len(part) >= 2` guard before unquoting.

## 0.19.0 - 2026-02-23

### Added NIP-28 Relay Integration and Channel Deletion

*   **Relay Integration (`pkg/relay`):**
    *   NIP-28 events now wired into event pipeline.
    *   Channel events validated and stored in `channel_events` table.
    *   Channel subscriptions supported via `#channel_id` filter in REQ messages.
    *   Automatic broadcast to subscribers.

*   **Memory Store Channel Support (`internal/store/memory`):**
    *   Added in-memory channel event storage for testing.
    *   Full support for `SaveChannelEvent()`, `QueryChannelEvents()`, `DeleteChannelEvents()`.

*   **NIP-09 Channel Deletion:**
    *   Deleting a channel creation event (kind 40) now cleans up ALL channel events.
    *   Messages, metadata, hide, and mute events are cascade deleted.
    *   Works with both SQLite and memory storage.

*   **Integration Tests:**
    *   `TestNIP28_ChannelLifecycle` - Full channel flow test.
    *   `TestNIP28_Validation` - Input validation tests.
    *   `TestNIP28_ChannelSubscription` - Multi-client broadcast test.
    *   `TestNIP28_ChannelDeletion` - Channel deletion cleanup test.

## 0.18.0 - 2026-02-20

### Added NIP-28 Public Chat Support

*   **NIP-28 Validation Package (`pkg/nips/nip28`):**
    *   New package for NIP-28 Public Chat validation.
    *   Support for kinds 40-44 (Channel Create, Metadata, Message, Hide, Mute).
    *   Validation functions for each event kind.
    *   `ParseChannelMetadata()` for extracting channel info.
    *   Helper functions: `IsNIP28Event()`, `IsReplaceableKind()`.

*   **Database Storage (`internal/store/sqlite`):**
    *   New `channel_events` table for storing channel events.
    *   New methods: `SaveChannelEvent()`, `GetChannelEvent()`, `QueryChannelEvents()`, `GetChannelMetadata()`, `ListChannels()`.
    *   Optimized indexes for channel queries.

*   **Database Migrations:**
    *   Added migration system for safe schema upgrades.
    *   `schema_migrations` table tracks applied versions.
    *   Automatic migration on startup preserves existing data.
    *   Tested for idempotency and old DB upgrades.

## 0.17.0 - 2026-02-18

### Added Central Configuration System

*   **Configuration Package (`pkg/config`):**
    *   New `pkg/config` package for centralized relay configuration.
    *   YAML/JSON configuration file support.
    *   Environment variable overrides (`GLIENICKE_*` prefix).
    *   Backward-compatible CLI flags (`-addr`, `-db`, `-cert`, `-key`).

*   **Configuration Sections:**
    *   **Network:** Address, TLS certificate/key, read/write timeouts.
    *   **Database:** Path, connection pool settings (max open/idle connections, lifetime).
    *   **Rate Limit:** Enable/disable, events/sec, REQ/sec, max connections, max event size.
    *   **Logging:** Log level (debug/info/warn/error) and format (text/json).
    *   **Features:** Feature flags for NIP-11, NIP-42, NIP-28.

*   **Validation:**
    *   Configuration validation on startup.
    *   TLS certificate/key pair validation.
    *   Required field checks.

*   **Example Configuration:**
    *   Added `config/relay.yaml.example` with all available options.

### Added Database Performance Optimizations

*   **SQLite Options (`internal/store/sqlite`):**
    *   New `Options` struct for database configuration.
    *   `NewWithOptions()` for custom database settings.

*   **Connection Pool:**
    *   Configurable `MaxOpenConns`, `MaxIdleConns`, `ConnMaxLifetime`.
    *   Uses config package settings.

*   **Performance Pragmas:**
    *   **WAL Mode:** Write-Ahead Logging for better concurrency (enabled by default).
    *   **Cache Size:** Default 2MB cache.
    *   **Busy Timeout:** Default 5 seconds.
    *   **Synchronous Mode:** NORMAL for balanced safety/performance.
    *   **Temp Store:** MEMORY for better performance.

*   **Batch Operations:**
    *   New `SaveEvents()` for bulk inserts using transactions.

*   **Maintenance:**
    *   `DeleteEventsOlderThan()` for event retention policies.
    *   `PruneDeletedEvents()` to clean up deleted_events table.
    *   `Vacuum()` to reclaim database space.
    *   `GetStats()` for monitoring (event count, size, etc.).

## 0.16.2 2026 Feb 2nd

Added Icon to nip11 information

## 0.16.1 2026 Feb 2nd

Changes for using traefik in front of glienicke-relay

## 0.16.0 - 2026-01-16

### Added Health Monitoring Endpoint

*   **HTTP Health Endpoint (`/health`):**
    *   New `/health` endpoint provides comprehensive relay monitoring metrics.
    *   Returns JSON response with real-time operational data.
    *   Suitable for monitoring systems, load balancers, and administrative dashboards.

*   **Monitoring Metrics:**
    *   **Connection Monitoring:** Active WebSocket connections count and total connections history.
    *   **Activity Tracking:** Total events processed, requests handled, and packets per second rate.
    *   **System Metrics:** Memory usage in MB, database connectivity status, and uptime in seconds.
    *   **Version Information:** Relay version and build details for tracking.
    *   **Performance Indicators:** Rate-limited requests count and response timestamp.

*   **HTTP Status Codes:**
    *   Returns `200 OK` when relay is healthy and operational.
    *   Returns `503 Service Unavailable` when critical issues are detected (e.g., database problems).
    *   Proper `Content-Type: application/json` header for monitoring system integration.

*   **Production Features:**
    *   **Lightweight Design:** Fast response times (< 100ms) with minimal computational overhead.
    *   **Thread-Safe:** All metrics collection uses mutex protection for concurrent access.
    *   **Database Health:** Automatic connectivity checking with 5-second timeout.
    *   **Memory Tracking:** Real-time memory usage monitoring using Go runtime statistics.

*   **Monitoring Integration:**
    *   **Prometheus/Grafana Compatible:** JSON format easily consumed by monitoring infrastructure.
    *   **Load Balancer Ready:** Suitable for health checks in production load balancer configurations.
    *   **DevOps Friendly:** Comprehensive operational visibility for troubleshooting and capacity planning.

*   **Testing Infrastructure:**
    *   **Comprehensive Integration Tests:** Full test suite covering all health endpoint functionality.
    *   **Performance Validation:** Response time testing ensures sub-100ms performance targets.
    *   **Activity Simulation:** Tests verify metrics collection and real-time updates.
    *   **Format Validation:** JSON schema validation and field completeness testing.

*   **Technical Implementation:**
    *   **HTTP Multiplexer:** Uses `http.ServeMux` for clean routing alongside WebSocket handler.
    *   **Metrics Collection:** Real-time counters, gauges, and rate calculations.
    *   **Database Integration:** Health checks verify storage layer connectivity and responsiveness.
    *   **Backward Compatibility:** Preserves existing NIP-11 endpoint and WebSocket functionality.

*   **Integration Points:**
    *   **Connection Handler:** Metrics collected during WebSocket upgrade and client lifecycle.
    *   **Event Processing:** Counters incremented for EVENT and REQ message processing.
    *   **Packet Rate Calculation:** Sliding window calculation for packets per second.
    *   **Memory Sampling:** Runtime memory statistics collection on each health request.

## 0.15.1 - 2025-12-15

### Fixed Gossip Relay JSON Serialization Bug

*   **JSON Response Format Fix:**
    *   Fixed critical bug where relay returned `null` instead of empty array `[]` when no events matched a filter.
    *   Issue was in SQLite storage layer returning `nil` slice instead of empty slice for empty filter sets.
    *   This caused gossip clients to fail with "invalid type: null, expected a sequence" error.

*   **Root Cause:**
    *   SQLite `QueryEvents()` function returned `nil` when no filters provided instead of `[]*event.Event{}`.
    *   JSON marshaling of `nil` slice produces `null`, violating NIP-01 specification.

*   **Fix Details:**
    *   Updated `internal/store/sqlite/sqlite.go` line 118 to return empty slice instead of `nil`.
    *   Ensures compliance with NIP-01 requirement for array responses in EVENT messages.
    *   All gossip relay tests now pass successfully.

*   **Technical Details:**
    *   Fixed SQLite storage layer to initialize Event.Tags with empty slice instead of nil.
    *   Updated parseTagsJSON() to return empty slice instead of nil for invalid JSON.
    *   Ensured all Event struct initialization includes Tags field initialization.

*   **Impact:**
    *   Gossip protocol compatibility restored.
    *   No more JSON parsing errors in gossip clients.
    *   Proper handling of empty result sets across all query types.
    *   Full compliance with NIP-01 JSON array requirements.

## 0.15.0 - 2025-12-13

### Implemented NIP-22 Comment Threads

*   **NIP-22 Comment Events (Kind 1111):**
    *   Complete implementation of comment threading for various content types.
    *   Support for comments on blog posts, files, web URLs, and podcasts.
    *   Proper validation of root scope (uppercase tags: E, A, I, K, P) and parent scope (lowercase tags: e, a, i, k, p).
    *   Thread structure analysis with top-level comment vs reply detection.
    *   Prevention of comments on kind 1 notes (redirects to NIP-10).

*   **Tag Relationship Validation:**
    *   Mandatory K and k tags for root and parent kind specification.
    *   Validation of tag relationships and consistency.
    *   Support for special kinds like "web" for URL-based comments.
    *   Proper handling of event addresses vs event IDs.

*   **Content and Structure Validation:**
    *   Plaintext content requirement (no HTML/Markdown).
    *   Non-empty content validation.
    *   Comprehensive tag structure validation.
    *   Error messages for invalid comment structures.

*   **Integration and Testing:**
    *   Full integration with relay event processing pipeline.
    *   Comprehensive unit tests covering all validation scenarios.
    *   Integration tests for end-to-end comment workflows.
    *   Test coverage for top-level comments, replies, and edge cases.

*   **Updated NIP-11 Support:**
    *   Added NIP-22 to supported NIPs list in relay information document.
    *   Updated documentation to reflect new comment threading capabilities.

## 0.14.0 - 2025-12-13

### Implemented NIP-04 and NIP-17 Private Messaging

*   **NIP-04 Encrypted Direct Messages (Legacy Support):**
    *   Complete AES-256-CBC encryption/decryption implementation.
    *   Content parsing and validation for encrypted direct messages.
    *   Recipient extraction from event tags.
    *   Backward compatibility with existing NIP-04 clients.
    *   Comprehensive test suite with edge case handling.

*   **NIP-17 Private Direct Messages (Modern Standard):**
    *   Full implementation of modern private messaging standard.
    *   Private direct message creation (kind 14) and file message support (kind 15).
    *   Multiple recipient support with proper tag management.
    *   Reply threading support with conversation context.
    *   Subject extraction for message organization.
    *   Rumor validation and unsigned event handling.

*   **Enhanced NIP-59 Gift Wrapping:**
    *   Complete integration with NIP-17 for secure message delivery.
    *   Multiple recipient gift wrapping functionality.
    *   Random timestamp generation for privacy protection.
    *   Full unwrapping workflow for message recipients.
    *   Enhanced validation for sealed events and gift wraps.

*   **Security Implementation:**
    *   Modern NIP-44 encryption (XChaCha20-Poly1305 AEAD) for NIP-17.
    *   Legacy NIP-04 encryption with proper deprecation warnings.
    *   Metadata protection through layered encryption.
    *   Forward secrecy and replay protection support.
    *   Comprehensive security analysis and best practices documentation.

*   **Testing Infrastructure:**
    *   Complete test suites for both NIP-04 and NIP-17 implementations.
    *   Integration tests for end-to-end private messaging workflows.
    *   Security-focused test cases for encryption/decryption validation.
    *   Performance and compatibility testing with existing clients.

*   **Documentation and Migration:**
    *   Comprehensive implementation guide with security analysis.
    *   Client integration examples and usage patterns.
    *   Migration strategy from NIP-04 to NIP-17.
    *   Best practices for secure private messaging implementation.

## 0.13.0 - 2025-12-13

### Added WSS/TLS Support

*   **Secure WebSocket (WSS) Support:**
    *   Relay now supports secure WebSocket connections with TLS encryption.
    *   Added `StartTLS()` method for HTTPS/WSS server functionality.
    *   New CLI flags: `-cert` and `-key` for TLS certificate and private key paths.

*   **Certificate Management:**
    *   Comprehensive certificate generation and management tools.
    *   Support for both OpenSSL and mkcert certificate generation.
    *   Automatic certificate configuration with proper Subject Alternative Names (SANs).
    *   Production-ready certificates for `relay.paulstephenborile.com`.

*   **Testing Infrastructure:**
    *   Complete end-to-end WSS testing suite.
    *   TLS certificate generation for development and testing.
    *   Enhanced test client with TLS dialer support.

*   **Documentation:**
    *   Complete TLS certificate setup guide in `Certificates.md`.
    *   Quick start instructions in `QUICKSTART.md`.
    *   Gossip client debugging guide in `GOSSIP_DEBUG.md`.

*   **Security and Compatibility:**
    *   Proper certificate validation and error handling.
    *   Support for both development and production certificate setups.
    *   Backward compatibility with existing non-TLS connections.

## 0.12.0 - 2025-12-12

### Implemented NIP-62: Request to Vanish

*   **Request to Vanish Support (Kind 62 Events):**
    *   Relay now supports NIP-62 Request to Vanish events (kind 62).
    *   Request to Vanish allows users to request complete deletion of all their events from a relay.
    *   Supports both relay-specific requests and global requests (ALL_RELAYS).
    *   Content field may include a reason or legal notice for the deletion request.

*   **Relay-Specific Deletion:**
    *   Events with `["relay", "relay-url"]` tags request deletion from specific relays.
    *   Relays only process requests that explicitly include their URL.
    *   Requests for other relays are ignored but still accepted and stored.

*   **Global Deletion Requests:**
    *   Events with `["relay", "ALL_RELAYS"]` tags request deletion from all relays.
    *   All relays should process such requests regardless of their specific URL.
    *   Global requests enable coordinated deletion across the Nostr network.

*   **Complete Event Deletion:**
    *   When processed, all events from the requesting pubkey are permanently deleted.
    *   This includes all event types and kinds from the specified public key.
    *   Deleted events cannot be recovered or re-broadcasted to the relay.
    *   The relay may store the signed Request to Vanish for bookkeeping purposes.

*   **Validation and Security:**
    *   Request to Vanish events must include at least one `relay` tag.
    *   Relay tag values cannot be empty or consist only of whitespace.
    *   Events are validated for correct structure before processing.
    *   Only the pubkey owner can create valid Request to Vanish events (via signature verification).

*   **Storage Interface Extensions:**
    *   Added `DeleteAllEventsByPubKey()` method to storage interface.
    *   Implemented for both memory and SQLite storage backends.
    *   Efficient bulk deletion operations for handling large pubkey event sets.

*   **Utility Functions:**
    *   `IsRequestToVanishEvent()` - Checks if an event is a Request to Vanish.
    *   `ValidateRequestToVanish()` - validates Request to Vanish events.
    *   `HandleRequestToVanish()` - Processes deletion requests for specific relays.
    *   `IsGlobalRequest()` - Identifies global (ALL_RELAYS) deletion requests.
    *   `GetRelayTags()` - Extracts all relay URLs from Request to Vanish events.

*   **Integration:**
    *   NIP-62 support is now advertised in relay information documents.
    *   Request to Vanish events integrate seamlessly with existing event validation.
    *   Comprehensive unit and integration tests ensure correct behavior and security.

## 0.11.0 - 2025-12-12

### Implemented NIP-62: Request to Vanish

*   **Request to Vanish Support (Kind 62 Events):**
    *   Relay now supports NIP-62 Request to Vanish events (kind 62).
    *   Request to Vanish allows users to request complete deletion of all their events from a relay.
    *   Supports both relay-specific requests and global requests (ALL_RELAYS).
    *   Content field may include a reason or legal notice for the deletion request.

*   **Relay-Specific Deletion:**
    *   Events with `["relay", "relay-url"]` tags request deletion from specific relays.
    *   Relays only process requests that explicitly include their URL.
    *   Requests for other relays are ignored but still accepted and stored.

*   **Global Deletion Requests:**
    *   Events with `["relay", "ALL_RELAYS"]` tags request deletion from all relays.
    *   All relays should process such requests regardless of their specific URL.
    *   Global requests enable coordinated deletion across the Nostr network.

*   **Complete Event Deletion:**
    *   When processed, all events from the requesting pubkey are permanently deleted.
    *   This includes all event types and kinds from the specified public key.
    *   Deleted events cannot be recovered or re-broadcasted to the relay.
    *   The relay may store the signed Request to Vanish for bookkeeping purposes.

*   **Validation and Security:**
    *   Request to Vanish events must include at least one `relay` tag.
    *   Relay tag values cannot be empty or consist only of whitespace.
    *   Events are validated for correct structure before processing.
    *   Only the pubkey owner can create valid Request to Vanish events (via signature verification).

*   **Storage Interface Extensions:**
    *   Added `DeleteAllEventsByPubKey()` method to storage interface.
    *   Implemented for both memory and SQLite storage backends.
    *   Efficient bulk deletion operations for handling large pubkey event sets.

*   **Utility Functions:**
    *   `IsRequestToVanishEvent()` - Checks if an event is a Request to Vanish.
    *   `ValidateRequestToVanish()` - validates Request to Vanish events.
    *   `HandleRequestToVanish()` - Processes deletion requests for specific relays.
    *   `IsGlobalRequest()` - Identifies global (ALL_RELAYS) deletion requests.
    *   `GetRelayTags()` - Extracts all relay URLs from Request to Vanish events.

*   **Integration:**
    *   NIP-62 support is now advertised in relay information documents.
    *   Request to Vanish events integrate seamlessly with existing event validation.
    *   Comprehensive unit and integration tests ensure correct behavior and security.

## 0.10.0 - 2025-12-12

### Implemented NIP-56: Reporting

*   **Report Event Support (Kind 1984 Events):**
    *   Relay now supports NIP-56 report events (kind 1984).
    *   Report events allow users to flag objectionable content including profiles, notes, and blobs.
    *   Supports all NIP-56 report types: nudity, malware, profanity, illegal, spam, impersonation, other.
    *   Content field may contain additional information about the report.

*   **Report Tag Validation:**
    *   Reports must include a `p` tag referencing the reported user's pubkey.
    *   Optional `e` tags can reference specific note/event IDs being reported.
    *   Optional `x` tags can reference blob hashes with associated server information.
    *   Report type must be specified as the 3rd element in reported tags.
    *   Supports NIP-32 `l` and `L` tags for additional categorization.

*   **Validation and Error Handling:**
    *   Report events are validated for correct structure and required tags.
    *   Invalid report types are rejected with descriptive error messages.
    *   Missing required `p` tags are rejected.
    *   Blob reports require corresponding event references when present.

*   **Utility Functions:**
    *   `ValidateReportEvent()` - Validates report events according to NIP-56 specification.
    *   `IsReportEvent()` - Checks if an event is a report event.
    *   `GetReportedPubKey()` - Extracts the pubkey being reported.
    *   `GetReportedEventIDs()` - Extracts reported event IDs.
    *   `GetReportedBlobs()` - Extracts blob report information with associated metadata.
    *   `IsValidReportType()` and `GetReportTypes()` - Report type validation utilities.

*   **Integration:**
    *   NIP-56 support is now advertised in relay information documents.
    *   Report events integrate seamlessly with existing event query and broadcast systems.
    *   Comprehensive unit and integration tests ensure correct behavior.

## 0.10.0 - 2025-12-12

### Implemented NIP-65: Relay List Metadata

*   **Relay List Support (Kind 10002 Events):**
    *   Relay now supports NIP-65 relay list metadata events (kind 10002).
    *   Relay lists contain `r` tags specifying relay URLs with optional read/write markers.
    *   Content field must be empty for valid relay list events.
    *   Supports default relays (read+write), read-only relays, and write-only relays.

*   **Read/Write Markers:**
    *   Relays without a marker default to both read and write access.
    *   `read` marker indicates relay is used only for reading events about the user.
    *   `write` marker indicates relay is used only for publishing events by the user.
    *   Unknown markers are treated as default (read+write) for forward compatibility.

*   **Validation and Error Handling:**
    *   Relay list events are validated for correct structure and content.
    *   Relay URLs must be non-empty and should start with `ws://` or `wss://`.
    *   Invalid relay lists are rejected with descriptive error messages.
    *   Relay lists must contain at least one valid `r` tag.

*   **Utility Functions:**
    *   `ExtractReadRelays()` - Extract read relay URLs from relay list events.
    *   `ExtractWriteRelays()` - Extract write relay URLs from relay list events.
    *   `ExtractAllRelays()` - Extract all relay URLs from relay list events.
    *   `ExtractRelayInfo()` - Extract detailed relay information with read/write flags.

*   **Integration:**
    *   NIP-65 support is now advertised in relay information documents.
    *   Relay list events integrate seamlessly with existing event query and broadcast systems.
    *   Comprehensive unit and integration tests ensure correct behavior.

## 0.9.0 - 2025-12-12

### Implemented NIP-02: Follow Lists

*   **Follow List Support (Kind 3 Events):**
    *   Relay now supports NIP-02 follow list events (kind 3).
    *   Follow lists contain `p` tags specifying followed users with optional relay URLs and petnames.
    *   Content field must be empty for valid follow list events.
    *   Comprehensive validation ensures proper `p` tag format and pubkey validation.

*   **Replaceable Event Handling:**
    *   Follow lists are replaceable events - new follow lists replace older ones from the same author.
    *   Storage layer properly handles replaceable events (kinds 0, 3, 10000-19999).
    *   Only the most recent follow list for each user is stored and returned.

*   **Validation and Error Handling:**
    *   Follow list events are validated for correct structure and content.
    *   Invalid follow lists are rejected with descriptive error messages.
    *   Follow lists must contain at least one valid `p` tag.

*   **Integration:**
    *   NIP-02 support is now advertised in relay information documents.
    *   Follow list events integrate seamlessly with existing event query and broadcast systems.
    *   Comprehensive integration tests ensure correct behavior.

## 0.8.0 - 2025-12-09

### SQLite Persistence with Autoconfiguration

*   **Database Storage:**
    *   Migrated from in-memory storage to SQLite for persistent data storage.
    *   Events are now stored across relay restarts, preventing data loss.
    *   SQLite database includes proper schema with indexes for efficient querying.

*   **Autoconfiguration:**
    *   Automatic database creation if no database file exists.
    *   Opens and loads existing database if present.
    *   New `-db` flag for custom database path (default: `relay.db`).
    *   Supports path expansion (`~/path.db`) and relative/absolute paths.

*   **Storage Architecture:**
    *   Maintains clean storage interface - no changes to business logic.
    *   SQLite implementation fully compatible with existing storage interface.
    *   All existing tests pass without modification.

## 0.7.0 - 2025-12-11

### Implemented NIP-50: Search Capability

*   **Search Filter Support:**
    *   Relay now supports the `search` field in REQ filter objects.
    *   Full-text search across event content and tag values.
    *   Support for basic search operators:
        *   AND logic (multiple terms must all be present)
        *   NOT logic (terms prefixed with `-` are excluded)
        *   OR logic (using `OR` keyword between terms)
    *   Support for search extensions:
        *   `domain:` - filter by NIP-05 domain
        *   `language:` - filter by language tag
        *   `nsfw:` - filter content warning status
    *   Search results are returned in order of storage query (future implementations may sort by relevance).
    *   Search is integrated with existing filter criteria (kinds, authors, etc.).

## 0.6.0 - 2025-12-09

### Implemented NIP-42: Authentication

*   **AUTH Event Support:**
    *   Relay now processes `EVENT` messages of kind 22242 (AUTH events).
    *   AUTH events are validated for correct kind, non-empty content, and valid signature.
    *   AUTH events are not stored but are used for client authentication.
    *   Successful authentication returns an OK message with "authenticated" status.
    *   Invalid AUTH events are rejected with appropriate error messages.

## 0.5.0 - 2025-12-03

### Implemented NIP-40: Event Expiration

*   **Expiration Tag Support:**
    *   Events can now include an `expiration` tag with a Unix timestamp.
    *   Events with expiration timestamps in the past are rejected during publishing.
    *   Expired events are filtered out from query responses and broadcasts.
    *   Normal events without expiration tags continue to work as before.

## 0.4.0 - 2025-11-28

### Implemented NIP-17: Private Direct Messages  : 
  . NIP-59 Gift Wrap
  . NIP-44 Encrypted Payloads (Versioned)


## 0.3.0 - 2025-11-20

### Implemented NIP-11 Features

*   **Relay Information Document:**
    *   The relay now serves a NIP-11 relay information document at the root URL.
    *   The document is served when the `Accept` header is `application/nostr+json`.
    *   The document includes the relay's name, description, software, version, and supported NIPs.

## 0.2.0 - 2025-11-20

### Implemented NIP-09 Features

*   **Event Deletion Request (`EVENT` kind 5):**
    *   Relay now processes `EVENT` messages of kind 5 (deletion events).
    *   Deletion requests specify event IDs to be deleted using 'e' tags.
    *   Only the original author of an event can request its deletion.
    *   Deleted events are marked as such in storage and are no longer returned by `REQ` queries.
    *   The relay does not send an `OK` message for deletion events, aligning with common client expectations.

## 0.1.0 - 2025-11-18

### Implemented NIP-01 Features

*   **Event Publishing (`EVENT` message):**
    *   Clients can send `EVENT` messages to the relay.
    *   Events undergo validation, including checking for missing public key, signature, invalid kind, and verifying the event ID against its computed hash.
    *   Schnorr signature verification (BIP-340) is performed.
    *   Duplicate event checking is implemented; if an event with the same ID already exists, it's not saved again, and a "duplicate" status is returned.
    *   Valid events are saved to the configured storage (currently in-memory).
    *   An `OK` message is sent back to the client indicating whether the event was accepted or rejected, along with a message.

*   **Event Subscription (`REQ` message):**
    *   Clients can send `REQ` messages with filters to subscribe to events.
    *   Filters support matching by:
        *   Event IDs (full or prefix).
        *   Author public keys (full or prefix).
        *   Event Kinds.
        *   `since` and `until` timestamps.
        *   Generic tags (e.g., `#e`, `#p`) are now correctly parsed and matched in `REQ` messages.
    *   Stored events matching the subscription filters are sent to the client.
    *   An `EOSE` (End of Stored Events) message is sent to indicate the completion of initial event transmission for a subscription.

*   **Subscription Closing (`CLOSE` message):**
    *   Clients can send `CLOSE` messages to end a specific subscription.
    *   The relay correctly removes the specified subscription, preventing further events from being sent to that client for that subscription ID.

*   **Basic Relay Communication:**
    *   WebSocket connections are handled, including upgrading HTTP requests to WebSocket.
    *   `NOTICE` messages can be sent to clients for human-readable information or errors.
    *   The relay broadcasts new events to all currently connected clients whose active subscriptions match the event.

### Known Issues

*   Tag filtering for generic tags (e.g., `#e`, `#p`) in `REQ` messages is not working correctly and is currently being debugged.