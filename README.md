# WebSocket Pub/Sub System

A high-performance, in-memory publish-subscribe system with WebSocket support and REST API management.

**Built for Plivo Technical Assignment** - A scalable, thread-safe pub/sub system handling real-time message delivery.

## Features

- **WebSocket pub/sub** - Real-time bidirectional communication (`/ws`)
- **REST API** - Topic management (create, delete, list, health, stats)
- **Thread-safe** - Concurrent operations with RWMutex
- **Message history** - Ring buffer with replay support (`last_n`)
- **Backpressure handling** - Slow consumer detection with drop-oldest policy
- **Graceful shutdown** - Clean connection closure
- **X-API-Key authentication** - Optional API key-based auth
- **Docker support** - Containerized deployment

## Quick Start

### Run Locally

```bash
# Build and run
go build -o pubsub-server ./cmd/server
./pubsub-server

# Server starts on http://localhost:8080
# WebSocket: ws://localhost:8080/ws
```

### Run with Docker

```bash
# Using docker-compose
docker-compose up --build

# Or directly
docker build -t pubsub-system .
docker run -p 8080:8080 pubsub-system
```

### Basic Usage

```bash
# 1. Create a topic
curl -X POST http://localhost:8080/topics \
  -H "Content-Type: application/json" \
  -d '{"name": "orders"}'

# 2. Check health
curl http://localhost:8080/health

# 3. Connect WebSocket and subscribe
# See API_DOCUMENTATION.md for WebSocket protocol details
```

For complete API documentation, see **[API_DOCUMENTATION.md](API_DOCUMENTATION.md)**.

## Design Decisions & Assumptions

### Concurrency Safety

**Approach:** Go's `sync.RWMutex` for topic and subscriber registries.

**Rationale:**
- Read-heavy workload (many subscribers, fewer topic changes)
- RWMutex allows multiple concurrent readers
- Exclusive write lock for modifications
- Verified race-free with `go run -race`

### Backpressure Policy

**Approach:** Buffered channels with **drop-oldest policy**.

**Implementation:**
1. Each subscriber has a 100-message buffered channel
2. On overflow: Drop oldest message from queue
3. Attempt to enqueue new message
4. If still full: Send `SLOW_CONSUMER` error and disconnect

**Rationale:**
- Prevents fast publishers from blocking on slow consumers
- Protects server memory from unbounded growth
- Drop-oldest ensures newest data is prioritized
- Alternative considered: Drop-newest (rejected - stale data less valuable)

### Message History (last_n)

**Approach:** Fixed-size ring buffer per topic (100 messages).

**Implementation:**
- Circular buffer with O(1) write, O(n) read
- Thread-safe with RWMutex
- Overwrites oldest when full

**Rationale:**
- Efficient memory usage (bounded)
- Fast writes (no reallocation)
- Supports replay for late-joining subscribers

**Trade-off:** No persistence across restarts (in-memory only).

### No Persistence

**Assumption:** As per requirements, no external databases or brokers.

**Implications:**
- All state is in-memory
- Messages lost on server restart
- Single-instance only (no horizontal scaling)
- Suitable for real-time streaming use cases

### Authentication

**Implementation:** Optional X-API-Key authentication.

**Approach:**
- REST API: `X-API-Key` header validation via Gin middleware
- WebSocket: Initial `auth` message required before operations
- Feature flag: `AUTH_ENABLED` (default: false)

**Rationale:**
- Industry standard for high-scale WebSocket systems (Slack, Discord, Pusher)
- Initial message approach prevents API keys in URLs/logs
- Backward compatible: disabled by default

## Authentication (Optional)

Authentication is **disabled by default**. To enable:

```bash
export AUTH_ENABLED=true
export API_KEYS=key1,key2,key3
./pubsub-server
```

### REST API

Add `X-API-Key` header to requests:

```bash
curl -X POST http://localhost:8080/topics \
  -H "Content-Type: application/json" \
  -H "X-API-Key: key1" \
  -d '{"name": "orders"}'
```

### WebSocket

Send `auth` message as **first message** after connecting:

```json
{
  "type": "auth",
  "api_key": "key1",
  "request_id": "optional-uuid"
}
```

**Unprotected:** `/health` endpoint always accessible without auth.

## Configuration

Key environment variables (see `.env.example` for all options):

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP server port |
| `RING_BUFFER_SIZE` | `100` | Messages stored per topic |
| `SUBSCRIBER_QUEUE_SIZE` | `100` | Buffer per subscriber (backpressure threshold) |
| `AUTH_ENABLED` | `false` | Enable X-API-Key authentication |
| `API_KEYS` | (empty) | Comma-separated valid API keys |

Example:

```bash
export PORT=9000
export RING_BUFFER_SIZE=200
export AUTH_ENABLED=true
export API_KEYS=dev-key,prod-key
./pubsub-server
```

## Testing

```bash
# Run tests
go test -v ./tests

# Run with race detector
go test -race ./tests

# Run authentication tests
go test -v ./tests -run TestAuth
```

### Manual Testing

```bash
# Terminal 1: Start server
./pubsub-server

# Terminal 2: Create topic
curl -X POST http://localhost:8080/topics \
  -H "Content-Type: application/json" \
  -d '{"name": "test"}'

# Terminal 3: Connect test client
go run test-clients/main.go
> sub test 0
> pub test hello
```

## Project Structure

```
├── cmd/server/          # Server entry point
├── internal/
│   ├── auth/           # Authentication (validator, middleware)
│   ├── handlers/       # WebSocket & REST handlers
│   ├── models/         # Message types
│   └── pubsub/         # Core pub/sub engine
├── config/             # Configuration
├── tests/              # Test suite
├── test-clients/       # Interactive test client
├── Dockerfile
├── docker-compose.yml
└── API_DOCUMENTATION.md  # Complete API reference
```

## Documentation

- **[API_DOCUMENTATION.md](API_DOCUMENTATION.md)** - Complete API reference (WebSocket & REST)
- **[CONFIGURATION.md](CONFIGURATION.md)** - Detailed configuration guide
- **[PUBSUB_IMPLEMENTATION_GUIDE.md](PUBSUB_IMPLEMENTATION_GUIDE.md)** - Implementation details

## License

MIT License

## Author

**Tarun M** - Assignment for Plivo Software Engineering Position

## Acknowledgments

- Go WebSocket library: [gorilla/websocket](https://github.com/gorilla/websocket)
- HTTP router: [gin-gonic/gin](https://github.com/gin-gonic/gin)
- UUID generation: [google/uuid](https://github.com/google/uuid)
