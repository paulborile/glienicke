# Rate Limiting & Connection Management Design

## 🎯 Overview

This document traces all changes needed for implementing production-ready rate limiting and connection management for the glienicke Nostr relay.

## 📋 Phase 1 Requirements

Based on issue `glienicke-ecs`, we need to implement:

### 1. Rate Limiting
- Per-IP rate limits for EVENT, REQ, COUNT messages
- Global rate limits across all clients
- Configurable limits via config file
- Rate limiter state management

### 2. Connection Management
- Max connections per IP
- Connection pool management
- Connection timeout handling
- Active connection tracking

### 3. Event Validation
- Event size limits
- Frequency controls per IP
- Content length validation
- Duplicate detection improvements

## 🏗️ Architecture Design

### Rate Limiter Structure
```go
type RateLimiter struct {
    // Global limiters
    eventLimiter   *rate.Limiter    // Global EVENT rate limit
    reqLimiter      *rate.Limiter     // Global REQ rate limit
    countLimiter    *rate.Limiter     // Global COUNT rate limit
    
    // Per-IP limiters
    ipLimiters      map[string]*IPLimiter  // Per-IP rate limiting
    ipLimitersMu    sync.RWMutex              // Thread-safe access
    
    // Configuration
    config          *RateLimitConfig
}

type IPLimiter struct {
    eventLimiter   *rate.Limiter
    reqLimiter      *rate.Limiter
    countLimiter    *rate.Limiter
    lastAccess       time.Time
}

type RateLimitConfig struct {
    // Global limits
    GlobalEventLimit    string  // e.g., "1000/s"
    GlobalReqLimit      string  // e.g., "10/s"
    GlobalCountLimit   string  // e.g., "5/s"
    
    // Per-IP limits
    IPEventLimit      string  // e.g., "1/minute"
    IPReqLimit        string  // e.g., "10/s"
    IPCountLimit      string  // e.g., "1/s"
    
    // Connection limits
    MaxConnectionsPerIP    int           // e.g., 10
    MaxGlobalConnections   int           // e.g., 1000
    
    // Event limits
    MaxEventSize          int  // e.g., 10000 bytes
    MaxEventsPerMinute    int  // e.g., 60 per IP
}
```

### Connection Manager Structure
```go
type ConnectionManager struct {
    activeConnections   map[string]*ConnectionInfo  // IP -> ConnectionInfo
    connectionsMu       sync.RWMutex
    globalConnections   int
    maxConnectionsPerIP int
    maxGlobalConnections int
}

type ConnectionInfo struct {
    Count       int
    LastAccess  time.Time
    FirstSeen   time.Time
    Connections []*Client
}
```

## 🔧 Implementation Plan

### Step 1: Configuration System
1. Add config file support (YAML/JSON)
2. Environment variable override support
3. Backward compatibility with CLI flags
4. Configuration validation

### Step 2: Rate Limiting Implementation
1. Add rate limiter structs to `pkg/relay/relay.go`
2. Implement rate limiting in `HandleEvent()`
3. Implement rate limiting in `HandleReq()`
4. Implement rate limiting in `HandleCount()`
5. Add rate limiter initialization in `New()`

### Step 3: Connection Management
1. Track connections in `ServeHTTP()`
2. Implement connection limits
3. Add connection cleanup
4. Connection timeout handling

### Step 4: Event Validation
1. Event size validation
2. Content length checks
3. Frequency validation per IP
4. Enhanced duplicate detection

## 📁 File Structure Changes

### New Files
```
pkg/config/
├── config.go          # Configuration loading
├── ratelimit.go      # Rate limiting logic
└── connection.go      # Connection management

pkg/ratelimit/
├── limiter.go         # Rate limiter implementation
└── types.go          # Rate limiter types

config/
├── relay.yaml          # Default configuration
└── relay.example.yaml  # Example configuration
```

### Modified Files
```
pkg/relay/relay.go      # Add rate limiting and connection management
cmd/relay/main.go        # Add config loading
go.mod                   # Add rate limiting dependencies
```

## ⚙️ Configuration Examples

### Default Config (config/relay.yaml)
```yaml
# Relay Configuration
server:
  addr: ":7000"
  database: "relay.db"

# Rate Limiting
rate_limits:
  # Global rate limits
  global:
    event: "1000/s"
    req: "10/s"
    count: "5/s"
  
  # Per-IP rate limits
  ip:
    event: "1/minute"
    req: "10/s"
    count: "1/s"
    max_connections: 10

# Event Limits
event_limits:
  max_size: 10000          # bytes
  max_per_minute: 60      # per IP
  max_content_length: 1000  # characters

# Connection Limits
connection_limits:
  max_per_ip: 10
  max_global: 1000
  timeout: "5m"           # connection timeout
```

### Environment Variables
```bash
# Override config file
GLIENICKE_RATE_LIMITS_GLOBAL_EVENT="2000/s"
GLIENICKE_RATE_LIMITS_IP_EVENT="2/minute"
GLIENICKE_CONNECTION_LIMITS_MAX_PER_IP="20"
```

## 🔄 Rate Limiting Logic

### Before Event Processing
```go
func (r *Relay) checkRateLimit(clientIP string, messageType string) error {
    // Check per-IP limits
    ipLimiter := r.getIPLimiter(clientIP)
    
    switch messageType {
    case "EVENT":
        if !ipLimiter.eventLimiter.Allow() {
            return fmt.Errorf("rate limited: too many events")
        }
        if !r.rateLimiter.eventLimiter.Allow() {
            return fmt.Errorf("rate limited: global event limit reached")
        }
    case "REQ":
        if !ipLimiter.reqLimiter.Allow() {
            return fmt.Errorf("rate limited: too many requests")
        }
        if !r.rateLimiter.reqLimiter.Allow() {
            return fmt.Errorf("rate limited: global request limit reached")
        }
    // ... similar for COUNT
    }
    
    return nil
}
```

### Connection Management
```go
func (r *Relay) canAcceptConnection(clientIP string) bool {
    r.connectionsMu.RLock()
    defer r.connectionsMu.RUnlock()
    
    connInfo := r.activeConnections[clientIP]
    
    // Check connection count per IP
    if connInfo != nil && connInfo.Count >= r.config.ConnectionLimits.MaxPerIP {
        return false
    }
    
    // Check global connections
    if r.globalConnections >= r.config.ConnectionLimits.MaxGlobal {
        return false
    }
    
    return true
}
```

## 📊 Metrics and Monitoring

### Rate Limiting Metrics
```go
type RateLimitMetrics struct {
    RateLimitedEvents    int64
    RateLimitedRequests int64
    ConnectionDrops    int64
    CurrentConnections  int64
    ActiveIPs         int64
}

func (r *Relay) exportMetrics() RateLimitMetrics {
    return RateLimitMetrics{
        RateLimitedEvents:   r.metrics.rateLimitedEvents.Load(),
        RateLimitedRequests: r.metrics.rateLimitedRequests.Load(),
        ConnectionDrops:     r.metrics.connectionDrops.Load(),
        CurrentConnections:  r.metrics.currentConnections.Load(),
        ActiveIPs:          r.metrics.activeIPs.Load(),
    }
}
```

## 🧪 Testing Strategy

### Unit Tests
- Rate limiter token bucket logic
- Configuration loading and validation
- Connection management edge cases
- Event validation scenarios

### Integration Tests
- Rate limiting under load
- Connection rejection scenarios
- Configuration hot-reloading
- Performance impact assessment

### Load Testing
- Test with current load test script
- Verify rate limiting works with 100+ clients
- Monitor memory usage of limiters
- Test configuration changes

## 🚀 Rollout Plan

### Phase 1: Foundation (Week 1)
1. Implement configuration system
2. Basic rate limiting for EVENT messages
3. Connection counting and limits

### Phase 2: Full Implementation (Week 2)
1. Rate limiting for REQ/COUNT messages
2. Event validation and size limits
3. Metrics and monitoring integration

### Phase 3: Production Ready (Week 3)
1. Performance optimization
2. Advanced features (adaptive limits)
3. Administrative interfaces
4. Documentation and deployment guides

## 📝 Success Criteria

### Functional Requirements
- ✅ All rate limits enforced per configuration
- ✅ Connection limits working correctly
- ✅ Event validation in place
- ✅ Configuration hot-reloading
- ✅ Metrics collection working

### Performance Requirements
- ✅ <1ms overhead for rate limit checks
- ✅ <10MB memory usage for limiters
- ✅ No impact on existing functionality
- ✅ Graceful degradation under load

### Security Requirements
- ✅ Prevents DoS attacks
- ✅ Rate limits cannot be bypassed
- ✅ Configuration validation prevents misconfiguration
- ✅ No information leakage in rate limit messages

## 📋 Dependencies

### Go Libraries
```go
require (
    github.com/juju/ratelimit  // Rate limiting
    gopkg.in/yaml.v3       // Configuration parsing
    github.com/prometheus/client_golang  // Metrics
)
```

### External Dependencies
- None (self-contained implementation)

## 🔄 Next Steps

1. **Create issues** for subcomponents using `bd create --parent glienicke-ecs`
2. **Start implementation** with configuration system first
3. **Test incrementally** with each feature
4. **Update load testing** to verify rate limiting works
5. **Documentation updates** with new configuration options

This design provides a comprehensive foundation for production-ready rate limiting while maintaining compatibility with existing code.