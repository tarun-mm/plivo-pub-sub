# Configuration Cheat Sheet

Quick reference for configuring the PubSub system for different use cases.

## Quick Start

Copy `.env.example` to `.env` and customize:

```bash
cp .env.example .env
# Edit .env with your preferred settings
./pubsub-server
```

## Environment Variables Reference

```bash
# Server
PORT=8080                         # HTTP server port
GIN_MODE=release                  # debug | release

# PubSub
RING_BUFFER_SIZE=100             # Messages per topic for replay
SUBSCRIBER_QUEUE_SIZE=100        # Messages per subscriber buffer

# WebSocket (seconds)
PING_PERIOD_SEC=30               # Heartbeat interval
PONG_WAIT_SEC=60                 # Max wait for pong
WRITE_WAIT_SEC=10                # Max write timeout

# HTTP Timeouts (seconds)
READ_TIMEOUT_SEC=15
WRITE_TIMEOUT_SEC=15
IDLE_TIMEOUT_SEC=0                   # 0 = disabled for WebSocket support

# Shutdown (seconds)
SHUTDOWN_TIMEOUT_SEC=10          # Graceful shutdown timeout

# Authentication (optional)
AUTH_ENABLED=false               # Enable X-API-Key authentication
API_KEYS=                        # Comma-separated API keys (e.g., key1,key2,key3)
```

## Pre-configured Scenarios

### 🏠 Development
```bash
cat > .env << EOF
GIN_MODE=debug
PING_PERIOD_SEC=10
PONG_WAIT_SEC=30
RING_BUFFER_SIZE=50
SUBSCRIBER_QUEUE_SIZE=50
EOF
```

### 🚀 Production (Recommended)
```bash
cat > .env << EOF
GIN_MODE=release
PORT=8080
RING_BUFFER_SIZE=100
SUBSCRIBER_QUEUE_SIZE=100
PING_PERIOD_SEC=30
PONG_WAIT_SEC=60
EOF
```

### 📈 High Traffic
```bash
cat > .env << EOF
GIN_MODE=release
RING_BUFFER_SIZE=500
SUBSCRIBER_QUEUE_SIZE=500
PING_PERIOD_SEC=60
PONG_WAIT_SEC=120
SHUTDOWN_TIMEOUT_SEC=30
EOF
```

### ⚡ Low Latency
```bash
cat > .env << EOF
GIN_MODE=release
SUBSCRIBER_QUEUE_SIZE=50
WRITE_WAIT_SEC=5
PING_PERIOD_SEC=15
PONG_WAIT_SEC=30
EOF
```

### 🧪 Load Testing
```bash
cat > .env << EOF
RING_BUFFER_SIZE=10
SUBSCRIBER_QUEUE_SIZE=50
WRITE_WAIT_SEC=5
SHUTDOWN_TIMEOUT_SEC=30
EOF
```

### 🔐 With Authentication
```bash
cat > .env << EOF
AUTH_ENABLED=true
API_KEYS=dev-key-123,prod-key-456,test-key-789
GIN_MODE=release
PORT=8080
RING_BUFFER_SIZE=100
SUBSCRIBER_QUEUE_SIZE=100
EOF
```

**Usage:**
- REST API: Add `X-API-Key` header to all requests
- WebSocket: Send `{"type":"auth","api_key":"dev-key-123"}` as first message

## Configuration Impact

| Setting | Increase = | Decrease = |
|---------|-----------|------------|
| RING_BUFFER_SIZE | More history, more memory | Less memory, shorter history |
| SUBSCRIBER_QUEUE_SIZE | More tolerance for slow consumers | Faster slow consumer detection |
| PING_PERIOD_SEC | Less overhead, slower detection | Faster detection, more traffic |
| PONG_WAIT_SEC | More tolerance for network lag | Faster disconnect detection |
| WRITE_WAIT_SEC | More tolerance for slow writes | Faster timeout on slow clients |

## Troubleshooting with Configuration

### Issue: Slow consumers getting disconnected
**Solution:** Increase `SUBSCRIBER_QUEUE_SIZE`
```bash
export SUBSCRIBER_QUEUE_SIZE=500
```

### Issue: Not enough message history for late joiners
**Solution:** Increase `RING_BUFFER_SIZE`
```bash
export RING_BUFFER_SIZE=500
```

### Issue: Too much memory usage
**Solution:** Decrease buffer sizes
```bash
export RING_BUFFER_SIZE=50
export SUBSCRIBER_QUEUE_SIZE=50
```

### Issue: WebSocket connections timing out
**Solution:** Increase timeout values
```bash
export PONG_WAIT_SEC=120
export WRITE_WAIT_SEC=20
```

### Issue: Slow disconnect detection
**Solution:** Decrease ping/pong intervals
```bash
export PING_PERIOD_SEC=15
export PONG_WAIT_SEC=30
```

## Memory Estimation

Approximate memory usage per connection:

```
Base memory: ~10 MB
+ Per subscriber: ~1 KB
+ Per message in history: ~500 B

Example with 1000 subscribers and 100 messages/topic:
= 10 MB + (1000 × 1 KB) + (100 × 500 B)
= 10 MB + 1 MB + 50 KB
≈ 11 MB

With SUBSCRIBER_QUEUE_SIZE=100:
Additional buffered messages: 1000 × 100 × 500 B = 50 MB
Total ≈ 61 MB
```

## Docker Configuration Examples

### Single container with custom config
```bash
docker run -p 8080:8080 \
  -e RING_BUFFER_SIZE=200 \
  -e SUBSCRIBER_QUEUE_SIZE=200 \
  pubsub-system
```

### Docker Compose with .env file
```yaml
version: '3.8'
services:
  pubsub:
    build: .
    ports:
      - "${PORT:-8080}:8080"
    environment:
      - PORT=${PORT:-8080}
      - RING_BUFFER_SIZE=${RING_BUFFER_SIZE:-100}
      - SUBSCRIBER_QUEUE_SIZE=${SUBSCRIBER_QUEUE_SIZE:-100}
      - PING_PERIOD_SEC=${PING_PERIOD_SEC:-30}
    restart: unless-stopped
```

## Performance Tuning

### For Maximum Throughput
```bash
RING_BUFFER_SIZE=1000
SUBSCRIBER_QUEUE_SIZE=1000
WRITE_WAIT_SEC=15
PING_PERIOD_SEC=60
```

### For Minimum Latency
```bash
RING_BUFFER_SIZE=10
SUBSCRIBER_QUEUE_SIZE=50
WRITE_WAIT_SEC=3
PING_PERIOD_SEC=10
```

### For Maximum Connections
```bash
RING_BUFFER_SIZE=10
SUBSCRIBER_QUEUE_SIZE=50
IDLE_TIMEOUT_SEC=120
```

## Deployment Recommendations

### Development
- Enable debug mode
- Shorter timeouts for faster feedback
- Smaller buffers to test edge cases

### Staging
- Mirror production config
- Test with realistic load
- Monitor slow consumer warnings

### Production
- Use default values initially
- Monitor and adjust based on metrics
- Increase buffers if seeing backpressure warnings
- Document any changes from defaults

## Validation

After changing configuration, verify it's working:

```bash
# Start server and check logs
./pubsub-server

# Should see:
# [INFO] Configuration loaded: Port=8080, RingBuffer=100, SubscriberQueue=100

# Test health endpoint
curl http://localhost:8080/health

# Monitor logs for issues
tail -f logs/pubsub.log | grep -E "(WARN|ERROR)"
```

## Best Practices

1. ✅ Always test configuration changes in staging first
2. ✅ Document why you deviated from defaults
3. ✅ Monitor logs after configuration changes
4. ✅ Use `.env` files, not hardcoded values
5. ✅ Version control your `.env.example`
6. ❌ Don't commit `.env` with secrets
7. ❌ Don't use debug mode in production
8. ❌ Don't set timeouts too low in production

---

**Remember:** The defaults (100/100/30s) are optimized for most use cases. Only adjust if you have specific requirements or see issues in your logs.
