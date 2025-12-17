#!/bin/bash

# Fixed Relay Performance Monitor
# Monitors relay health during load testing

set -e

MONITOR_INTERVAL=5  # seconds
OUTPUT_FILE="./load_test_results/relay_metrics.csv"
RELAY_URL="http://localhost:7000"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log() {
    echo -e "${GREEN}[$(date '+%H:%M:%S')]${NC} $1"
}

# Initialize CSV file with proper header
init_csv() {
    mkdir -p ./load_test_results
    echo "timestamp,cpu_percent,memory_mb,relay_process_count,tcp_connections_open" > "$OUTPUT_FILE"
    log "Monitoring initialized: $OUTPUT_FILE"
}

# Get system metrics
get_system_metrics() {
    local timestamp=$(date -Iseconds)
    
    # CPU usage (simplified)
    local cpu_usage=$(top -bn1 | grep "Cpu(s)" | awk '{print $2}' | sed 's/%us,//' | sed 's/,.*//' || echo "0")
    
    # Memory usage by relay process (simplified)
    local memory_mb=0
    local relay_pid=$(pgrep -f "relay" | head -1)
    if [[ -n "$relay_pid" ]]; then
        memory_mb=$(ps -p "$relay_pid" -o rss= | awk '{print $1/1024}' 2>/dev/null || echo "0")
    fi
    
    # Relay process count (exclude grep process)
    local process_count=$(pgrep -f "relay --addr" | wc -l | tr -d ' ')
    
    # TCP connections to port 7000
    local tcp_connections=$(netstat -an 2>/dev/null | grep ":7000" | grep "ESTABLISHED" | wc -l || echo "0")
    
    echo "$timestamp,$cpu_usage,$memory_mb,$process_count,$tcp_connections"
}

# Monitor relay function
monitor_relay() {
    log "Starting relay performance monitoring..."
    log "Monitoring interval: ${MONITOR_INTERVAL}s"
    log "Output file: $OUTPUT_FILE"
    log "Press Ctrl+C to stop monitoring"
    echo ""
    
    # Display header
    printf "%-20s %-10s %-10s %-15s %-15s\n" "TIMESTAMP" "CPU%" "MEMORY(MB)" "PROCESSES" "TCP_CONNS"
    echo "--------------------------------------------------------------------------------"
    
    while true; do
        local metrics_line=$(get_system_metrics)
        echo "$metrics_line" >> "$OUTPUT_FILE"
        
        # Parse for display
        IFS=',' read -r timestamp cpu memory processes tcp_conns <<< "$metrics_line"
        
        # Format timestamp for display
        local display_time=$(date -d "$timestamp" '+%H:%M:%S' 2>/dev/null || echo "${timestamp:11:8}")
        
        # Display current metrics
        printf "%-20s %-10s %-10s %-15s %-15s\n" "$display_time" "$cpu" "$memory" "$processes" "$tcp_conns"
        
        sleep $MONITOR_INTERVAL
    done
}

# Handle interruption
handle_interrupt() {
    log ""
    log "Monitoring stopped. Data saved to: $OUTPUT_FILE"
    log "To analyze data: cat $OUTPUT_FILE | column -t -s','"
    exit 0
}

trap handle_interrupt INT TERM

# Check if relay is running
check_relay() {
    if ! pgrep -f "relay" > /dev/null; then
        echo "❌ No relay process found. Please start the relay first:"
        echo "   ./relay"
        exit 1
    fi
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -i|--interval)
            MONITOR_INTERVAL="$2"
            shift 2
            ;;
        -o|--output)
            OUTPUT_FILE="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [options]"
            echo "  -i, --interval SEC    Monitoring interval in seconds (default: 5)"
            echo "  -o, --output FILE     Output CSV file (default: ./load_test_results/relay_metrics.csv)"
            echo "  -h, --help           Show this help"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Main execution
main() {
    check_relay
    init_csv
    monitor_relay
}

main "$@"