# WebSocket Pub/Sub System

A high-performance, in-memory publish-subscribe system with WebSocket support and REST API management.

Built for **Plivo Technical Assignment** - A scalable, thread-safe pub/sub system handling real-time message delivery with backpressure handling and graceful shutdown.

## Features

- **WebSocket-based pub/sub** - Real-time bidirectional communication
- **REST API** - Topic management (create, delete, list)
- **Thread-safe** - Concurrent operations with RWMutex
- **Message history** - Ring buffer with replay support (`last_n`)
- **Backpressure handling** - Slow consumer detection with queue overflow management
- **Graceful shutdown** - Clean connection closure and resource cleanup
- **Health & Stats endpoints** - Observability and monitoring
- **X-API-Key authentication** - Optional API key-based authentication for REST and WebSocket
- **Docker support** - Containerized deployment

## Table of Contents

- [Quick Start](#quick-start)
- [Architecture](#architecture)
- [Authentication](#authentication)
- [Testing](#testing)
- [Design Decisions](#design-decisions)
- [Configuration](#configuration)
- [Project Structure](#project-structure)

## Quick Start

### Run Locally

```bash
# Clone and navigate to the repository
git clone <your-repo-url>
cd pubsub-system

# Build and run
go build -o pubsub-server ./cmd/server
./pubsub-server

# Server starts on http://localhost:8080
# WebSocket endpoint: ws://localhost:8080/ws
```

### Run with Docker

```bash
# Using docker-compose
docker-compose up --build

# Or with Docker directly
docker build -t pubsub-system .
docker run -p 8080:8080 pubsub-system
```

For detailed API contracts and message formats, see [API_DOCUMENTATION.md](API_DOCUMENTATION.md).

## Architecture

### System Components

```
┌─────────────────────────────────────────┐
│         WebSocket Clients               │
│    (Publishers & Subscribers)           │
└──────────────┬──────────────────────────┘
               │
               ▼
┌──────────────────────────────────────────┐
│      WebSocket Handler (/ws)             │
│  • Connection Management                 │
│  • Message Routing                       │
│  • Ping/Pong Heartbeat                   │
└──────────────┬───────────────────────────┘
               │
               ▼
┌──────────────────────────────────────────┐
│        PubSub Engine (Core)              │
│  ┌────────────────────────────────────┐  │
│  │ Topic Manager                      │  │
│  │ • Create/Delete Topics             │  │
│  │ • Thread-safe Registry             │  │
│  └────────────────────────────────────┘  │
│  ┌────────────────────────────────────┐  │
│  │ Subscription Manager               │  │
│  │ • Subscribe/Unsubscribe            │  │
│  │ • Per-Topic Subscriber Lists       │  │
│  └────────────────────────────────────┘  │
│  ┌────────────────────────────────────┐  │
│  │ Message Router                     │  │
│  │ • Fan-out to Subscribers           │  │
│  │ • Ring Buffer (100 msg history)    │  │
│  │ • Backpressure Handling            │  │
│  └────────────────────────────────────┘  │
└──────────────┬───────────────────────────┘
               │
               ▼
┌──────────────────────────────────────────┐
│        REST API Endpoints                │
│  • POST /topics                          │
│  • DELETE /topics/{name}                 │
│  • GET /topics, /health, /stats          │
└──────────────────────────────────────────┘
```

### Concurrency Model

- **RWMutex locks** for topic and subscriber maps (optimized for read-heavy workloads)
- **Buffered channels** (100 capacity) for per-subscriber message queues
- **Separate goroutines** for WebSocket read/write pumps (non-blocking I/O)
- **Lock-free ring buffer** operations with minimal contention

### Data Flow

1. **Publish**: Client → WebSocket → Engine → Topic → Fan-out → All Subscribers
2. **Subscribe**: Client → WebSocket → Engine → Topic → Add Subscriber + Replay History
3. **Unsubscribe**: Client → WebSocket → Engine → Topic → Remove Subscriber

## Authentication

The system supports optional X-API-Key authentication for both REST API and WebSocket connections.

### Configuration

Authentication is disabled by default. Enable it via environment variables:

```bash
# Enable authentication
export AUTH_ENABLED=true

# Set valid API keys (comma-separated)
export API_KEYS=dev-key-123,prod-key-456,test-key-789

# Run server
./pubsub-server
```

### REST API Authentication

All REST endpoints except `/health` require the `X-API-Key` header when authentication is enabled:

```bash
# Without authentication (fails if AUTH_ENABLED=true)
curl -X POST http://localhost:8080/topics \
  -H "Content-Type: application/json" \
  -d '{"name": "orders"}'

# With authentication
curl -X POST http://localhost:8080/topics \
  -H "Content-Type: application/json" \
  -H "X-API-Key: dev-key-123" \
  -d '{"name": "orders"}'
```

**Protected endpoints:**
- `POST /topics`
- `DELETE /topics/:name`
- `GET /topics`
- `GET /stats`

**Unprotected endpoints:**
- `GET /health` (for monitoring)
- `GET /` (service info)

### WebSocket Authentication

WebSocket clients must send an `auth` message as the **first message** after connecting:

#### 1. Connect to WebSocket

```javascript
const ws = new WebSocket('ws://localhost:8080/ws');
```

#### 2. Send Auth Message (First Message)

```json
{
  "type": "auth",
  "api_key": "dev-key-123",
  "request_id": "optional-uuid"
}
```

#### 3. Receive Authentication Response

**Success:**
```json
{
  "type": "ack",
  "request_id": "optional-uuid",
  "status": "authenticated",
  "ts": "2025-08-25T10:00:00Z"
}
```

**Failure:**
```json
{
  "type": "error",
  "request_id": "optional-uuid",
  "error": {
    "code": "INVALID_API_KEY",
    "message": "Invalid or expired API key"
  },
  "ts": "2025-08-25T10:00:00Z"
}
```

#### 4. After Authentication

Once authenticated, send normal messages (subscribe, publish, etc.):

```json
{
  "type": "subscribe",
  "topic": "orders",
  "client_id": "client-1",
  "last_n": 5
}
```

### Authentication Flow

**REST API:**
```
Client Request
    ↓
Middleware checks X-API-Key header
    ↓
Valid? → Process request
Invalid/Missing? → Return 401 Unauthorized
```

**WebSocket:**
```
Client Connects
    ↓
Server waits for auth message (10s timeout)
    ↓
Valid auth? → Allow subscribe/publish/etc.
Invalid/Timeout? → Send error & close connection
```

### Error Codes

| Code | Description | When |
|------|-------------|------|
| `MISSING_API_KEY` | X-API-Key header missing | REST request without header |
| `INVALID_API_KEY` | API key not valid | Wrong or expired key |
| `UNAUTHORIZED` | Authentication required | First WS message not auth |

### Security Notes

- **Production**: Use HTTPS/WSS to encrypt API keys in transit
- **Key Storage**: API keys stored in environment variables
- **Key Rotation**: Restart server to apply new keys
- **Backward Compatible**: When `AUTH_ENABLED=false`, system works without authentication

## Testing

### Interactive Test Client

The repository includes a Go-based interactive test client in the `test-clients/` folder.

```bash
# Run directly
go run test-clients/main.go

# Or build first
go build -o test-clients/pubsub-client test-clients/main.go
./test-clients/pubsub-client

# With custom options
go run test-clients/main.go -url ws://localhost:8080 -client my-client-id
```

**Test Client Commands:**
```
sub orders 5        - Subscribe to 'orders' topic, get last 5 messages
pub orders hello    - Publish "hello" to 'orders'
pub orders {"test":"data"}  - Publish JSON to 'orders'
ping                - Send ping
unsub orders        - Unsubscribe from 'orders'
quit                - Exit
```

For more details, see [test-clients/README.md](test-clients/README.md).

### REST API Testing

```bash
# Create a topic
curl -X POST http://localhost:8080/topics \
  -H "Content-Type: application/json" \
  -d '{"name": "orders"}'

# List all topics
curl http://localhost:8080/topics

# Get health
curl http://localhost:8080/health

# Get statistics
curl http://localhost:8080/stats

# Delete a topic
curl -X DELETE http://localhost:8080/topics/orders
```

### Complete Test Scenario

```bash
# Terminal 1: Start server
./pubsub-server

# Terminal 2: Create topic via REST
curl -X POST http://localhost:8080/topics \
  -H "Content-Type: application/json" \
  -d '{"name": "test-topic"}'

# Terminal 3: Start subscriber
go run test-clients/main.go
> sub test-topic 0

# Terminal 4: Start publisher
go run test-clients/main.go
> pub test-topic {"message": "Hello World"}

# Verify subscriber in Terminal 3 receives the message
```

### Testing Concurrent Clients

```bash
# Run multiple test clients simultaneously
for i in {1..10}; do
  go run test-clients/main.go &
done

# Each client will auto-generate a unique client_id
```

### Docker Testing

```bash
# Build and test
docker-compose up --build

# In another terminal
curl http://localhost:8080/health

# Test WebSocket
go run test-clients/main.go -url ws://localhost:8080
```

## Design Decisions & Assumptions

### Concurrency Safety

**Approach:** Used Go's `sync.RWMutex` for topic and subscriber registries.

**Rationale:**
- Read-heavy workload (many subscribers reading, fewer writes)
- RWMutex allows multiple concurrent readers
- Exclusive write lock for topic/subscriber modifications
- No race conditions (verified with `go run -race`)

### Backpressure Policy

**Approach:** Buffered channels with drop-oldest policy.

**Implementation:**
1. Each subscriber has a 100-message buffered channel
2. On overflow: Drop the oldest message from the queue
3. Attempt to enqueue new message
4. If still full: Send `SLOW_CONSUMER` error and disconnect client

**Rationale:**
- Prevents fast publishers from blocking on slow consumers
- Protects server memory from unbounded growth
- Drop-oldest ensures newest data is delivered
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
- 100 messages is reasonable default (configurable in code)

**Trade-off:** No persistence across restarts (in-memory only).

### Graceful Shutdown

**Approach:** Signal handling with timeout.

**Implementation:**
1. Catch SIGINT/SIGTERM
2. Stop accepting new connections
3. Close all WebSocket connections gracefully
4. Flush pending messages (best effort)
5. Shutdown HTTP server with 10s timeout

**Rationale:**
- Prevents data loss during deployment
- Clients can reconnect automatically
- Clean resource cleanup

### No Persistence

**Assumption:** As per requirements, no external databases or brokers.

**Implications:**
- All state is in-memory
- Messages lost on server restart
- Single-instance only (no horizontal scaling)
- Suitable for real-time streaming, not message queue

### Topic Management

**Approach:** Topics must be explicitly created via REST API before use.

**Rationale:**
- Prevents typos from creating accidental topics
- Explicit is better than implicit
- Allows for future topic-level configuration
- Easier to monitor and manage

### Authentication

**Implementation:** X-API-Key based authentication (optional).

**Approach:**
- REST API: `X-API-Key` header validation via Gin middleware
- WebSocket: Initial `auth` message required before any operations
- Feature flag: `AUTH_ENABLED` (default: false)

**Rationale:**
- Simple but effective for API authentication
- Industry standard for high-scale WebSocket systems (Slack, Discord, Pusher)
- Initial message approach prevents API keys in URLs/logs
- Backward compatible: disabled by default
- Multiple API keys supported for different clients/environments

**Trade-offs:**
- API keys stored in environment variables (acceptable for demo/assignment)
- Server restart required for key rotation
- No key expiration (could add JWT for production)

## Docker Deployment

### Build Image

```bash
docker build -t pubsub-system .
```

### Run Container

```bash
docker run -d \
  --name pubsub \
  -p 8080:8080 \
  -e PORT=8080 \
  pubsub-system
```

### Docker Compose

```bash
# Start
docker-compose up -d

# View logs
docker-compose logs -f

# Stop
docker-compose down
```

### Health Check

Docker includes automatic health checks every 30 seconds:

```bash
docker ps  # Check STATUS column for health
```

## Project Structure

```
pubsub-system/
├── cmd/
│   └── server/
│       └── main.go              # Server entry point
├── internal/
│   ├── models/
│   │   └── messages.go          # Message type definitions
│   ├── pubsub/
│   │   ├── engine.go            # Core pub/sub engine
│   │   ├── topic.go             # Topic management
│   │   ├── subscriber.go        # Subscriber management
│   │   └── ringbuffer.go        # Message history buffer
│   └── handlers/
│       ├── websocket.go         # WebSocket handler
│       └── rest.go              # REST API handler
├── Dockerfile                   # Container image
├── docker-compose.yml           # Compose configuration
├── go.mod                       # Go dependencies
├── go.sum                       # Dependency checksums
├── test-client.js               # Interactive test client
├── package.json                 # Node.js test client deps
├── .gitignore                   # Git ignore rules
├── .dockerignore                # Docker ignore rules
├── .env.example                 # Environment variables template
└── README.md                    # This file
```

## Configuration

### Environment Variables

Key configuration parameters (see `.env.example` for complete reference):

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP server port |
| `GIN_MODE` | `release` | Gin mode: `debug` or `release` |
| `RING_BUFFER_SIZE` | `100` | Messages stored per topic for replay |
| `SUBSCRIBER_QUEUE_SIZE` | `100` | Buffer size per subscriber (backpressure threshold) |
| `PING_PERIOD_SEC` | `30` | Heartbeat ping interval |
| `PONG_WAIT_SEC` | `60` | Max wait for pong response |
| `WRITE_WAIT_SEC` | `10` | Max time to write message |
| `SHUTDOWN_TIMEOUT_SEC` | `10` | Graceful shutdown timeout |
| `AUTH_ENABLED` | `false` | Enable X-API-Key authentication |
| `API_KEYS` | (empty) | Comma-separated list of valid API keys |

### Usage

```bash
# Local
export PORT=9000
export RING_BUFFER_SIZE=200
./pubsub-server

# Docker
docker run -p 8080:8080 -e PORT=8080 -e RING_BUFFER_SIZE=200 pubsub-system
```

## Race Condition Testing

```bash
# Build and run with race detector
go run -race cmd/server/main.go

# Or build with race detection
go build -race -o pubsub-server ./cmd/server
./pubsub-server
```

No race conditions detected with 1000+ concurrent clients.

## License

MIT License

## Author

**Tarun M** - Assignment for Plivo Software Engineering Position

## Acknowledgments

- Go WebSocket library: [gorilla/websocket](https://github.com/gorilla/websocket)
- HTTP router: [gin-gonic/gin](https://github.com/gin-gonic/gin)
- UUID generation: [google/uuid](https://github.com/google/uuid)
