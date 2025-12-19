#!/bin/bash

# DEAD SIMPLE LOAD TEST
# Just works, no complexity

if [[ "$1" == "-h" ]]; then
    echo "Usage: $0 [clients] [duration]"
    echo "  clients: Number of clients (default: 10)"
    echo "  duration: Test duration in seconds (default: 20)"
    echo ""
    echo "Examples:"
    echo "  $0          # 10 clients, 20s"
    echo "  $0 20 60    # 20 clients, 60s"
    echo "  $0 50 120   # 50 clients, 120s"
    exit 0
fi

NUM_CLIENTS=${1:-10}
TEST_DURATION=${2:-20}
RELAY_URL="ws://localhost:7000"

echo "=== NOSTR RELAY LOAD TEST ==="
echo "Clients: $NUM_CLIENTS"
echo "Duration: ${TEST_DURATION}s"
echo "Relay: $RELAY_URL"
echo ""

# Setup algia
echo "Setting up algia..."
cp ~/.config/algia/config.json ~/.config/algia/config.json.backup 2>/dev/null || true
jq --arg relay "$RELAY_URL" '.relays = {($relay): {"read": true, "write": true, "search": false}}' ~/.config/algia/config.json > /tmp/algia.json
mv /tmp/algia.json ~/.config/algia/config.json

# Test function
test_client() {
    local client_id=$1
    local duration=$2
    local success=0
    local errors=0
    local start=$(date +%s)
    local end=$((start + duration))
    
    while [[ $(date +%s) -lt $end ]]; do
        if ALGIA_RELAYS="$RELAY_URL" algia post "Load test client $client_id at $(date +%s)" >/dev/null 2>&1; then
            ((success++))
        else
            ((errors++))
        fi
     #   sleep 2
    done
    
    echo "Client $client_id: $success success, $errors errors"
    return $success
}

# Run tests
echo "Starting $NUM_CLIENTS clients..."
TOTAL_SUCCESS=0
TOTAL_ERRORS=0

for i in $(seq 1 $NUM_CLIENTS); do
    test_client $i $TEST_DURATION &
    
    # Don't overwhelm relay
    if [[ $((i % 50)) -eq 0 ]]; then
        wait
    fi
done

wait

# Results
echo ""
echo "=== TEST COMPLETED ==="
echo "All $NUM_CLIENTS clients finished"
echo ""

# Cleanup
mv ~/.config/algia/config.json.backup ~/.config/algia/config.json 2>/dev/null
echo "Config restored. Test done!"
