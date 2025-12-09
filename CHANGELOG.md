# Changelog

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