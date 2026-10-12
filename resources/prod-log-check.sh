#!/bin/bash
# Fetches recent production relay logs over SSH and summarizes them into
# known categories (connections, errors, rate limiting, protocol gaps) so a
# health check doesn't require manually re-reading raw log lines each time.
#
# Usage:
#   ./resources/prod-log-check.sh [since] [container]
#
#   since       docker logs --since value (default: 1h). Examples: 30m, 6h, 2026-09-27T00:00:00
#   container   container name (default: nostr-relay-server)
#
# Requires SSH access configured for: ssh -p 50162 -l root fender
# (adjust SSH_HOST/SSH_PORT/SSH_USER below if the host changes).

set -euo pipefail

SSH_HOST="fender"
SSH_PORT="50162"
SSH_USER="root"
SINCE="${1:-1h}"
CONTAINER="${2:-nostr-relay-server}"

OUT_DIR="$(dirname "$0")/prod-log-check-results"
mkdir -p "$OUT_DIR"
TS="$(date -u +%Y%m%dT%H%M%SZ)"
LOG_FILE="$OUT_DIR/prod-logs-$TS.txt"

echo "Fetching logs from $SSH_USER@$SSH_HOST:$SSH_PORT (container: $CONTAINER, since: $SINCE)..."
ssh -p "$SSH_PORT" -l "$SSH_USER" "$SSH_HOST" \
    "docker logs --since '$SINCE' '$CONTAINER' 2>&1" > "$LOG_FILE"

echo "Saved to: $LOG_FILE"
echo ""

TOTAL_LINES=$(wc -l < "$LOG_FILE")
echo "=== Summary ($TOTAL_LINES lines) ==="
echo ""

count() { grep -c "$1" "$LOG_FILE" 2>/dev/null || true; }

echo "New connections:        $(count 'New WebSocket connection')"
echo "Rate limited events:    $(count 'Rate limited')"
echo "IP bans:                $(count 'BANNED IP')"
echo "Read errors:            $(count 'WebSocket read error')"
echo "Write errors:           $(count 'WebSocket write error')"
echo ""

echo "=== Known bug signatures (should be ZERO after a fix ships) ==="
SEND_QUEUE_FULL=$(count 'Send queue full')
UNKNOWN_AUTH=$(count 'unknown message type: AUTH')
echo "Send queue full (REQ burst overflow, fixed 0.20.3):     $SEND_QUEUE_FULL"
echo "unknown message type: AUTH (fixed 0.20.2):              $UNKNOWN_AUTH"
echo ""

echo "=== Unhandled / unsupported message types (candidates for new NIP support) ==="
grep -o "unknown message type: [A-Z-]*" "$LOG_FILE" 2>/dev/null | sort | uniq -c | sort -rn || echo "(none found)"
echo ""

echo "=== Malformed client messages ==="
count_invalid=$(count "invalid event:")
echo "invalid event (bad EVENT/AUTH payload): $count_invalid"
if [ "$count_invalid" -gt 0 ]; then
    echo "  Top offending IPs:"
    OFFENDING_IPS=$(grep -B1 "invalid event:" "$LOG_FILE" | grep "New WebSocket connection" | \
        sed -E 's/.*from ([0-9.]+).*/\1/' | sort | uniq -c | sort -rn)
    echo "$OFFENDING_IPS" | head -5 | sed 's/^/    /'
    echo "  User-Agents seen for these IPs (a UA is often only sent occasionally, so check across the whole window):"
    TOP_IPS=$(echo "$OFFENDING_IPS" | head -5 | awk '{print $2}')
    for ip in $TOP_IPS; do
        ua=$(grep "New WebSocket connection from $ip " "$LOG_FILE" | grep -oE "UA: [^,]*" | grep -v "UA: $" | sort -u | head -1)
        if [ -n "$ua" ]; then
            echo "    $ip -> $ua"
        fi
    done
fi
echo ""

echo "=== Non-standard WebSocket close codes seen ==="
grep -oE "close [0-9]{4}" "$LOG_FILE" 2>/dev/null | sort | uniq -c | sort -rn || echo "(none found)"
echo ""

echo "=== Top connecting IPs (possible bots/scanners if reconnecting rapidly) ==="
grep "New WebSocket connection" "$LOG_FILE" 2>/dev/null | \
    sed -E 's/.*from ([0-9.]+) .*/\1/' | sort | uniq -c | sort -rn | head -10 || echo "(none found)"
echo ""

echo "=== Current /health snapshot ==="
curl -s https://relay.paulstephenborile.com/health | (command -v jq >/dev/null && jq . || cat) || echo "(health check failed)"
echo ""

echo "Full log saved at: $LOG_FILE"
echo "Review it directly for anything the summary above doesn't explain: less '$LOG_FILE'"
