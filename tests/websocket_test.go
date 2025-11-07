package tests

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// TestWebSocketConnection tests basic WebSocket connectivity
func TestWebSocketConnection(t *testing.T) {
	server, cleanup := SetupTestServer(t)
	defer cleanup()

	// Connect WebSocket
	conn := ConnectWebSocket(t, server.WSURL, "test-client-1")
	defer conn.Close()

	// Should be able to send ping
	SendPing(t, conn, "ping-1")

	// Should receive pong
	msg := ReceiveMessage(t, conn, 2*time.Second)
	if msg.Type != "pong" {
		t.Errorf("Expected pong, got %s", msg.Type)
	}
	if msg.RequestID != "ping-1" {
		t.Errorf("Expected request_id 'ping-1', got '%s'", msg.RequestID)
	}
}

// TestSubscribeToTopic tests subscribing to a topic
func TestSubscribeToTopic(t *testing.T) {
	server, cleanup := SetupTestServer(t)
	defer cleanup()

	// Create topic first
	CreateTopic(t, server.URL, "orders")

	// Connect and subscribe
	conn := ConnectWebSocket(t, server.WSURL, "sub-1")
	defer conn.Close()

	Subscribe(t, conn, "orders", 0, "req-1")

	// Wait for ack
	ack := WaitForAck(t, conn, "req-1", 2*time.Second)
	if ack.Status != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", ack.Status)
	}
	if ack.Topic != "orders" {
		t.Errorf("Expected topic 'orders', got '%s'", ack.Topic)
	}
}

// TestSubscribeToNonExistentTopic tests subscribing to a topic that doesn't exist
func TestSubscribeToNonExistentTopic(t *testing.T) {
	server, cleanup := SetupTestServer(t)
	defer cleanup()

	conn := ConnectWebSocket(t, server.WSURL, "sub-1")
	defer conn.Close()

	Subscribe(t, conn, "nonexistent", 0, "req-1")

	// Should receive error
	msg := ReceiveMessage(t, conn, 2*time.Second)
	if msg.Type != "error" {
		t.Errorf("Expected error, got %s", msg.Type)
	}
	if msg.Error == nil {
		t.Fatal("Expected error details")
	}
	if msg.Error.Code != "TOPIC_NOT_FOUND" {
		t.Errorf("Expected error code 'TOPIC_NOT_FOUND', got '%s'", msg.Error.Code)
	}
}

// TestPublishAndReceive tests basic pub/sub flow
func TestPublishAndReceive(t *testing.T) {
	server, cleanup := SetupTestServer(t)
	defer cleanup()

	// Create topic
	CreateTopic(t, server.URL, "orders")

	// Connect subscriber
	sub := ConnectWebSocket(t, server.WSURL, "subscriber-1")
	defer sub.Close()

	Subscribe(t, sub, "orders", 0, "sub-req-1")
	WaitForAck(t, sub, "sub-req-1", 2*time.Second)

	// Connect publisher
	pub := ConnectWebSocket(t, server.WSURL, "publisher-1")
	defer pub.Close()

	// Publish a message
	messageID := uuid.New().String()
	payload := map[string]interface{}{
		"order_id": "ORD-123",
		"amount":   99.5,
	}
	Publish(t, pub, "orders", messageID, payload, "pub-req-1")

	// Publisher should receive ack
	WaitForAck(t, pub, "pub-req-1", 2*time.Second)

	// Subscriber should receive event
	event := WaitForEvent(t, sub, 2*time.Second)

	if event.Topic != "orders" {
		t.Errorf("Expected topic 'orders', got '%s'", event.Topic)
	}
	if event.Message == nil {
		t.Fatal("Expected message in event")
	}
	if event.Message.ID != messageID {
		t.Errorf("Expected message ID '%s', got '%s'", messageID, event.Message.ID)
	}

	// Verify payload
	payloadMap, ok := event.Message.Payload.(map[string]interface{})
	if !ok {
		t.Fatal("Expected payload to be a map")
	}
	if payloadMap["order_id"] != "ORD-123" {
		t.Errorf("Expected order_id 'ORD-123', got '%v'", payloadMap["order_id"])
	}
}

// TestMessageFanOut tests that messages are delivered to all subscribers
func TestMessageFanOut(t *testing.T) {
	server, cleanup := SetupTestServer(t)
	defer cleanup()

	// Create topic
	CreateTopic(t, server.URL, "notifications")

	// Connect multiple subscribers
	numSubscribers := 5
	subscribers := make([]*websocketConn, numSubscribers)
	for i := 0; i < numSubscribers; i++ {
		conn := ConnectWebSocket(t, server.WSURL, fmt.Sprintf("sub-%d", i))
		defer conn.Close()

		Subscribe(t, conn, "notifications", 0, fmt.Sprintf("req-%d", i))
		WaitForAck(t, conn, fmt.Sprintf("req-%d", i), 2*time.Second)

		subscribers[i] = &websocketConn{conn: conn, id: fmt.Sprintf("sub-%d", i)}
	}

	// Connect publisher
	pub := ConnectWebSocket(t, server.WSURL, "publisher")
	defer pub.Close()

	// Publish a message
	messageID := uuid.New().String()
	Publish(t, pub, "notifications", messageID, "test notification", "pub-req")
	WaitForAck(t, pub, "pub-req", 2*time.Second)

	// All subscribers should receive the message
	for _, sub := range subscribers {
		event := WaitForEvent(t, sub.conn, 2*time.Second)
		if event.Message.ID != messageID {
			t.Errorf("Subscriber %s: Expected message ID '%s', got '%s'", sub.id, messageID, event.Message.ID)
		}
	}
}

// TestTopicIsolation tests that messages don't leak between topics
func TestTopicIsolation(t *testing.T) {
	server, cleanup := SetupTestServer(t)
	defer cleanup()

	// Create two topics
	CreateTopic(t, server.URL, "orders")
	CreateTopic(t, server.URL, "payments")

	// Subscribe to orders
	ordersSub := ConnectWebSocket(t, server.WSURL, "orders-sub")
	defer ordersSub.Close()
	Subscribe(t, ordersSub, "orders", 0, "orders-req")
	WaitForAck(t, ordersSub, "orders-req", 2*time.Second)

	// Subscribe to payments
	paymentsSub := ConnectWebSocket(t, server.WSURL, "payments-sub")
	defer paymentsSub.Close()
	Subscribe(t, paymentsSub, "payments", 0, "payments-req")
	WaitForAck(t, paymentsSub, "payments-req", 2*time.Second)

	// Publish to orders topic
	pub := ConnectWebSocket(t, server.WSURL, "publisher")
	defer pub.Close()

	ordersMessageID := uuid.New().String()
	Publish(t, pub, "orders", ordersMessageID, "order message", "pub-orders")
	WaitForAck(t, pub, "pub-orders", 2*time.Second)

	// Orders subscriber should receive the message
	ordersEvent := WaitForEvent(t, ordersSub, 2*time.Second)
	if ordersEvent.Message.ID != ordersMessageID {
		t.Errorf("Expected message ID '%s', got '%s'", ordersMessageID, ordersEvent.Message.ID)
	}

	// Payments subscriber should NOT receive anything (timeout expected)
	_, err := ReceiveMessageNoFail(paymentsSub, 500*time.Millisecond)
	if err == nil {
		t.Error("Payments subscriber should not have received orders message (topic isolation violated)")
	}
}

// TestUnsubscribe tests unsubscribing from a topic
func TestUnsubscribe(t *testing.T) {
	server, cleanup := SetupTestServer(t)
	defer cleanup()

	// Create topic
	CreateTopic(t, server.URL, "events")

	// Subscribe
	sub := ConnectWebSocket(t, server.WSURL, "sub-1")
	defer sub.Close()

	Subscribe(t, sub, "events", 0, "sub-req")
	WaitForAck(t, sub, "sub-req", 2*time.Second)

	// Publish a message (should be received)
	pub := ConnectWebSocket(t, server.WSURL, "pub-1")
	defer pub.Close()

	msg1ID := uuid.New().String()
	Publish(t, pub, "events", msg1ID, "message 1", "pub-req-1")
	event1 := WaitForEvent(t, sub, 2*time.Second)
	if event1.Message.ID != msg1ID {
		t.Errorf("Expected message ID '%s', got '%s'", msg1ID, event1.Message.ID)
	}

	// Unsubscribe
	Unsubscribe(t, sub, "events", "unsub-req")
	WaitForAck(t, sub, "unsub-req", 2*time.Second)

	// Publish another message (should NOT be received)
	msg2ID := uuid.New().String()
	Publish(t, pub, "events", msg2ID, "message 2", "pub-req-2")
	WaitForAck(t, pub, "pub-req-2", 2*time.Second)

	// Subscriber should not receive message 2
	_, err := ReceiveMessageNoFail(sub, 500*time.Millisecond)
	if err == nil {
		t.Error("Unsubscribed client should not receive messages")
	}
}

// TestLastNMessages tests the last_n message replay feature
func TestLastNMessages(t *testing.T) {
	server, cleanup := SetupTestServer(t)
	defer cleanup()

	// Create topic
	CreateTopic(t, server.URL, "history")

	// Publish some messages before subscriber connects
	pub := ConnectWebSocket(t, server.WSURL, "publisher")
	defer pub.Close()

	numMessages := 5
	messageIDs := make([]string, numMessages)
	for i := 0; i < numMessages; i++ {
		messageIDs[i] = uuid.New().String()
		Publish(t, pub, "history", messageIDs[i], fmt.Sprintf("message-%d", i), fmt.Sprintf("pub-req-%d", i))
		WaitForAck(t, pub, fmt.Sprintf("pub-req-%d", i), 2*time.Second)
	}

	// New subscriber with last_n=3
	sub := ConnectWebSocket(t, server.WSURL, "late-subscriber")
	defer sub.Close()

	Subscribe(t, sub, "history", 3, "sub-req")
	WaitForAck(t, sub, "sub-req", 2*time.Second)

	// Should receive last 3 messages
	receivedIDs := make([]string, 0)
	for i := 0; i < 3; i++ {
		event := WaitForEvent(t, sub, 2*time.Second)
		receivedIDs = append(receivedIDs, event.Message.ID)
	}

	// Verify we got the last 3 messages in order
	expectedIDs := messageIDs[2:] // Last 3 messages
	for i, expected := range expectedIDs {
		if receivedIDs[i] != expected {
			t.Errorf("Expected message ID '%s', got '%s'", expected, receivedIDs[i])
		}
	}
}

// TestPublishWithInvalidMessageID tests publishing with invalid UUID
func TestPublishWithInvalidMessageID(t *testing.T) {
	server, cleanup := SetupTestServer(t)
	defer cleanup()

	// Create topic
	CreateTopic(t, server.URL, "orders")

	// Connect publisher
	pub := ConnectWebSocket(t, server.WSURL, "publisher")
	defer pub.Close()

	// Try to publish with invalid UUID
	Publish(t, pub, "orders", "not-a-uuid", "test", "pub-req")

	// Should receive error
	msg := ReceiveMessage(t, pub, 2*time.Second)
	if msg.Type != "error" {
		t.Errorf("Expected error, got %s", msg.Type)
	}
	if msg.Error == nil {
		t.Fatal("Expected error details")
	}
	if msg.Error.Code != "BAD_REQUEST" {
		t.Errorf("Expected error code 'BAD_REQUEST', got '%s'", msg.Error.Code)
	}
}

// TestStatsWithMessages tests that stats reflect published messages
func TestStatsWithMessages(t *testing.T) {
	server, cleanup := SetupTestServer(t)
	defer cleanup()

	// Create topic
	CreateTopic(t, server.URL, "orders")

	// Publish some messages
	pub := ConnectWebSocket(t, server.WSURL, "publisher")
	defer pub.Close()

	numMessages := 10
	for i := 0; i < numMessages; i++ {
		Publish(t, pub, "orders", uuid.New().String(), fmt.Sprintf("msg-%d", i), fmt.Sprintf("req-%d", i))
		WaitForAck(t, pub, fmt.Sprintf("req-%d", i), 2*time.Second)
	}

	// Check stats
	stats := GetStats(t, server.URL)
	orderStats, exists := stats.Topics["orders"]
	if !exists {
		t.Fatal("Expected 'orders' topic in stats")
	}

	if orderStats.Messages != int64(numMessages) {
		t.Errorf("Expected %d messages, got %d", numMessages, orderStats.Messages)
	}
}

// TestTopicDeletionNotification tests that subscribers are notified when topic is deleted
func TestTopicDeletionNotification(t *testing.T) {
	server, cleanup := SetupTestServer(t)
	defer cleanup()

	// Create topic and subscribe
	CreateTopic(t, server.URL, "temp")

	sub := ConnectWebSocket(t, server.WSURL, "subscriber")
	defer sub.Close()

	Subscribe(t, sub, "temp", 0, "sub-req")
	WaitForAck(t, sub, "sub-req", 2*time.Second)

	// Delete the topic
	DeleteTopic(t, server.URL, "temp")

	// Subscriber should receive topic_deleted info message
	msg := ReceiveMessage(t, sub, 2*time.Second)
	if msg.Type != "info" {
		t.Errorf("Expected info message, got %s", msg.Type)
	}
	if msg.Msg != "topic_deleted" {
		t.Errorf("Expected msg 'topic_deleted', got '%s'", msg.Msg)
	}
	if msg.Topic != "temp" {
		t.Errorf("Expected topic 'temp', got '%s'", msg.Topic)
	}
}

// TestConcurrentPublishers tests multiple publishers publishing to the same topic
func TestConcurrentPublishers(t *testing.T) {
	server, cleanup := SetupTestServer(t)
	defer cleanup()

	// Create topic
	CreateTopic(t, server.URL, "concurrent")

	// Start subscriber
	sub := ConnectWebSocket(t, server.WSURL, "subscriber")
	defer sub.Close()

	Subscribe(t, sub, "concurrent", 0, "sub-req")
	WaitForAck(t, sub, "sub-req", 2*time.Second)

	// Start multiple publishers concurrently
	numPublishers := 10
	var wg sync.WaitGroup

	for i := 0; i < numPublishers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			pub := ConnectWebSocket(t, server.WSURL, fmt.Sprintf("pub-%d", id))
			defer pub.Close()

			msgID := uuid.New().String()
			Publish(t, pub, "concurrent", msgID, fmt.Sprintf("msg from pub-%d", id), fmt.Sprintf("req-%d", id))
			WaitForAck(t, pub, fmt.Sprintf("req-%d", id), 2*time.Second)
		}(i)
	}

	wg.Wait()

	// Subscriber should have received all messages
	stats := GetStats(t, server.URL)
	concurrentStats := stats.Topics["concurrent"]

	if concurrentStats.Messages != int64(numPublishers) {
		t.Errorf("Expected %d messages in stats, got %d", numPublishers, concurrentStats.Messages)
	}
}

// Helper type for tracking subscriber connections
type websocketConn struct {
	conn *websocket.Conn
	id   string
}
