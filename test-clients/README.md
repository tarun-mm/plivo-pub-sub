# Test Clients for PubSub System

This folder contains test clients for the WebSocket PubSub system.

## Go Test Client

### Prerequisites
The Go client uses the same dependencies as the main project (already in `go.mod`).

### Running the Go Client

From the project root:
```bash
go run test-clients/main.go
```

Or with custom options:
```bash
go run test-clients/main.go -url ws://localhost:8080 -client my-client-id
```

### Building the Go Client

```bash
go build -o test-clients/pubsub-client test-clients/main.go
./test-clients/pubsub-client
```

## Node.js Test Client

### Prerequisites
```bash
cd test-clients
npm install
```

### Running the Node.js Client

```bash
cd test-clients
node test-client.js
```

Or with custom server URL:
```bash
node test-client.js ws://localhost:8080
```

## Available Commands (Both Clients)

Once connected, you can use these commands:

- `sub <topic> [last_n]` - Subscribe to a topic (optionally get last N messages)
- `unsub <topic>` - Unsubscribe from a topic
- `pub <topic> <data>` - Publish a message to a topic
- `ping` - Test connection with a ping
- `help` - Show available commands
- `quit` - Exit the client

## Example Usage

1. **Start the server** (from project root):
   ```bash
   go run cmd/server/main.go
   ```

2. **Create a topic** (using REST API):
   ```bash
   curl -X POST http://localhost:8080/topics/orders
   ```

3. **Run test client** (in a new terminal):
   ```bash
   go run test-clients/main.go
   ```

4. **Subscribe to topic**:
   ```
   > sub orders
   ```

5. **Publish messages** (open another client in a different terminal):
   ```bash
   go run test-clients/main.go
   ```

   Then:
   ```
   > pub orders {"order_id": "123", "amount": 99.5}
   > pub orders hello-world
   ```

6. **Check the first client** - you should see the published messages!

## Testing Multiple Clients

Open multiple terminals and run the client in each to test the pub/sub functionality:

**Terminal 1:**
```bash
go run test-clients/main.go
> sub orders
```

**Terminal 2:**
```bash
go run test-clients/main.go
> sub orders
```

**Terminal 3:**
```bash
go run test-clients/main.go
> pub orders {"test": "message"}
```

Both Terminal 1 and Terminal 2 should receive the message!
