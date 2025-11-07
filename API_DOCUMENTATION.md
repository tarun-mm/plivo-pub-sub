# API Documentation

## Authentication

The system supports optional X-API-Key authentication for both REST and WebSocket endpoints.

### Configuration

Authentication is **disabled by default**. To enable:

```bash
export AUTH_ENABLED=true
export API_KEYS=key1,key2,key3
```

### REST API Authentication

When authentication is enabled, all REST endpoints (except `/health`) require the `X-API-Key` header:

```http
POST /topics
Content-Type: application/json
X-API-Key: your-api-key-here

{"name": "orders"}
```

**Protected Endpoints:**
- `POST /topics`
- `DELETE /topics/:name`
- `GET /topics`
- `GET /stats`

**Unprotected Endpoints:**
- `GET /health` (always accessible)

### WebSocket Authentication

When authentication is enabled, clients **must send an `auth` message as the first message** after connecting:

```json
{
  "type": "auth",
  "api_key": "your-api-key-here",
  "request_id": "optional-uuid"
}
```

**Response on success:**
```json
{
  "type": "ack",
  "request_id": "optional-uuid",
  "status": "authenticated",
  "ts": "2025-08-25T10:00:00Z"
}
```

**Response on failure:**
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

**Important:**
- Clients have 10 seconds to authenticate after connecting
- Once authenticated, subsequent `auth` messages will be rejected
- All other operations are blocked until authentication succeeds

---

## WebSocket Endpoint

**URL**: `ws://localhost:8080/ws?client_id=<optional-id>`

If `client_id` is not provided, a unique ID will be auto-generated.

### Client → Server Messages

#### 1. Subscribe to Topic

```json
{
  "type": "subscribe",
  "topic": "orders",
  "client_id": "client-1",
  "last_n": 5,
  "request_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

**Fields:**
- `type`: `"subscribe"`
- `topic`: Topic name (required)
- `client_id`: Client identifier (required)
- `last_n`: Number of historical messages to replay (optional, default: 0)
- `request_id`: Correlation ID (optional)

#### 2. Unsubscribe from Topic

```json
{
  "type": "unsubscribe",
  "topic": "orders",
  "client_id": "client-1",
  "request_id": "340e8400-e29b-41d4-a716-446655440098"
}
```

#### 3. Publish Message

```json
{
  "type": "publish",
  "topic": "orders",
  "message": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "payload": {
      "order_id": "ORD-123",
      "amount": 99.5,
      "currency": "USD"
    }
  },
  "request_id": "340e8400-e29b-41d4-a716-446655440098"
}
```

**Fields:**
- `message.id`: Must be a valid UUID
- `message.payload`: Any JSON value

#### 4. Ping

```json
{
  "type": "ping",
  "request_id": "570t8400-e29b-41d4-a716-446655440123"
}
```

#### 5. Authenticate (when AUTH_ENABLED=true)

```json
{
  "type": "auth",
  "api_key": "your-api-key-here",
  "request_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

**Fields:**
- `type`: `"auth"`
- `api_key`: Your API key (required when authentication is enabled)
- `request_id`: Correlation ID (optional)

**Note:** This must be the **first message** sent after connecting when `AUTH_ENABLED=true`.

### Server → Client Messages

#### 1. Acknowledgment (ack)

```json
{
  "type": "ack",
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "topic": "orders",
  "status": "ok",
  "ts": "2025-08-25T10:00:00Z"
}
```

#### 2. Event (published message)

```json
{
  "type": "event",
  "topic": "orders",
  "message": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "payload": {
      "order_id": "ORD-123",
      "amount": 99.5,
      "currency": "USD"
    }
  },
  "ts": "2025-08-25T10:01:00Z"
}
```

#### 3. Error

```json
{
  "type": "error",
  "request_id": "req-67890",
  "error": {
    "code": "TOPIC_NOT_FOUND",
    "message": "Topic 'orders' does not exist"
  },
  "ts": "2025-08-25T10:02:00Z"
}
```

**Error Codes:**
- `BAD_REQUEST` - Invalid message format or missing required fields
- `TOPIC_NOT_FOUND` - Attempting to publish/subscribe to non-existent topic
- `SLOW_CONSUMER` - Subscriber queue overflow (backpressure triggered)
- `UNAUTHORIZED` - Authentication required (first message must be `auth` when AUTH_ENABLED=true)
- `INVALID_API_KEY` - API key is invalid or expired
- `MISSING_API_KEY` - X-API-Key header missing (REST API only)
- `INTERNAL` - Unexpected server error

#### 4. Pong

```json
{
  "type": "pong",
  "request_id": "ping-abc",
  "ts": "2025-08-25T10:03:00Z"
}
```

#### 5. Info (server notifications)

**Heartbeat:**
```json
{
  "type": "info",
  "msg": "ping",
  "ts": "2025-08-25T10:04:00Z"
}
```

**Topic Deleted:**
```json
{
  "type": "info",
  "topic": "orders",
  "msg": "topic_deleted",
  "ts": "2025-08-25T10:05:00Z"
}
```

## REST API Endpoints

**Note:** When `AUTH_ENABLED=true`, all endpoints (except `/health`) require the `X-API-Key` header.

### 1. Create Topic

```http
POST /topics
Content-Type: application/json
X-API-Key: your-api-key-here

{
  "name": "orders"
}
```

**Response (201 Created):**
```json
{
  "status": "created",
  "topic": "orders"
}
```

**Error (409 Conflict):**
```json
{
  "error": "topic already exists"
}
```

### 2. Delete Topic

```http
DELETE /topics/orders
```

**Response (200 OK):**
```json
{
  "status": "deleted",
  "topic": "orders"
}
```

**Note:** All subscribers will receive a `topic_deleted` notification and be unsubscribed.

**Error (404 Not Found):**
```json
{
  "error": "topic not found"
}
```

### 3. List Topics

```http
GET /topics
```

**Response (200 OK):**
```json
{
  "topics": [
    {
      "name": "orders",
      "subscribers": 3
    },
    {
      "name": "notifications",
      "subscribers": 1
    }
  ]
}
```

### 4. Health Check

```http
GET /health
```

**Response (200 OK):**
```json
{
  "uptime_sec": 3600,
  "topics": 2,
  "subscribers": 5
}
```

### 5. Statistics

```http
GET /stats
```

**Response (200 OK):**
```json
{
  "topics": {
    "orders": {
      "messages": 1250,
      "subscribers": 3
    },
    "notifications": {
      "messages": 42,
      "subscribers": 1
    }
  }
}
```
