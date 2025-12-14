# Changelog

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