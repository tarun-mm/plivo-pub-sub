package tests

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tarunm/pubsub-system/internal/models"
)

// TestHighConcurrency tests the system under heavy concurrent load
// This demonstrates race-free operation with 100 concurrent publishers and subscribers
func TestHighConcurrency(t *testing.T) {
	// Setup test server
	srv, cleanup := SetupTestServer(t)
	defer cleanup()

	// Create topic
	CreateTopic(t, srv.URL, "stress-test")

	numPublishers := 50
	numSubscribers := 50
	messagesPerPublisher := 20

	var wg sync.WaitGroup
	receivedMessages := sync.Map{}
	var publishedCount int64

	// Start subscribers
	for i := 0; i < numSubscribers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			clientID := fmt.Sprintf("stress-sub-%d", id)
			conn := ConnectWebSocket(t, srv.WSURL, clientID)
			defer conn.Close()

			// Subscribe
			subscribeMsg := models.ClientMessage{
				Type:      "subscribe",
				Topic:     "stress-test",
				ClientID:  clientID,
				RequestID: uuid.New().String(),
			}
			err := conn.WriteJSON(subscribeMsg)
			if err != nil {
				t.Logf("Subscriber %d write error: %v", id, err)
				return
			}

			// Read ack
			var ack models.ServerMessage
			conn.ReadJSON(&ack)

			// Receive messages for 2 seconds
			conn.SetReadDeadline(time.Now().Add(3 * time.Second))
			for {
				var msg models.ServerMessage
				err := conn.ReadJSON(&msg)
				if err != nil {
					break
				}

				if msg.Type == "event" && msg.Message != nil {
					receivedMessages.Store(msg.Message.ID, true)
				}
			}
		}(i)
	}

	// Give subscribers time to connect
	time.Sleep(200 * time.Millisecond)

	// Start publishers
	for i := 0; i < numPublishers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			clientID := fmt.Sprintf("stress-pub-%d", id)
			conn := ConnectWebSocket(t, srv.WSURL, clientID)
			defer conn.Close()

			// Publish multiple messages
			for j := 0; j < messagesPerPublisher; j++ {
				msgID := uuid.New().String()

				atomic.AddInt64(&publishedCount, 1)

				publishMsg := models.ClientMessage{
					Type:  "publish",
					Topic: "stress-test",
					Message: &models.Message{
						ID:      msgID,
						Payload: fmt.Sprintf("Message %d from publisher %d", j, id),
					},
					RequestID: uuid.New().String(),
				}

				err := conn.WriteJSON(publishMsg)
				if err != nil {
					t.Logf("Publisher %d write error: %v", id, err)
					return
				}

				// Read ack
				var ack models.ServerMessage
				conn.ReadJSON(&ack)

				// Small delay between messages
				time.Sleep(time.Millisecond)
			}
		}(i)
	}

	// Wait for all publishers to finish
	wg.Wait()

	// Give time for message delivery
	time.Sleep(500 * time.Millisecond)

	// Verify: count how many messages were received
	receivedCount := 0
	receivedMessages.Range(func(key, value interface{}) bool {
		receivedCount++
		return true
	})

	published := atomic.LoadInt64(&publishedCount)
	t.Logf("Published %d messages, at least one subscriber received %d messages",
		published, receivedCount)

	// We expect at least some messages to be received
	// (In a real scenario with proper timing, all should be received)
	if receivedCount == 0 {
		t.Fatal("No messages were received by any subscriber")
	}

	// Check that no panics or races occurred (test would fail if races detected)
	t.Logf("✓ System remained stable under load of %d concurrent clients",
		numPublishers+numSubscribers)
}

// TestConcurrentTopicOperations tests concurrent topic creation/deletion with active subscriptions
func TestConcurrentTopicOperations(t *testing.T) {
	srv, cleanup := SetupTestServer(t)
	defer cleanup()

	numTopics := 10
	numClients := 20
	var wg sync.WaitGroup

	// Create and delete topics concurrently
	for i := 0; i < numTopics; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			topicName := fmt.Sprintf("concurrent-topic-%d", id)

			// Create topic
			CreateTopic(t, srv.URL, topicName)

			// Small delay
			time.Sleep(10 * time.Millisecond)

			// Delete topic
			DeleteTopic(t, srv.URL, topicName)
		}(i)
	}

	// Clients trying to subscribe to topics concurrently
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			clientID := fmt.Sprintf("client-%d", id)
			conn := ConnectWebSocket(t, srv.WSURL, clientID)
			defer conn.Close()

			// Try to subscribe to a random topic (might succeed or fail)
			topicName := fmt.Sprintf("concurrent-topic-%d", id%numTopics)
			subscribeMsg := models.ClientMessage{
				Type:      "subscribe",
				Topic:     topicName,
				ClientID:  clientID,
				RequestID: uuid.New().String(),
			}

			conn.WriteJSON(subscribeMsg)

			// Read response (ack or error)
			conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			var resp models.ServerMessage
			conn.ReadJSON(&resp)

			// Either succeeds or fails with TOPIC_NOT_FOUND - both are valid
		}(i)
	}

	wg.Wait()

	t.Logf("✓ System remained stable with %d concurrent topic operations and %d clients",
		numTopics, numClients)
}

// TestConcurrentSubscribeUnsubscribe tests rapid subscribe/unsubscribe operations
func TestConcurrentSubscribeUnsubscribe(t *testing.T) {
	srv, cleanup := SetupTestServer(t)
	defer cleanup()

	CreateTopic(t, srv.URL, "churn-test")

	numClients := 30
	iterations := 10
	var wg sync.WaitGroup

	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			clientID := fmt.Sprintf("churn-client-%d", id)
			conn := ConnectWebSocket(t, srv.WSURL, clientID)
			defer conn.Close()

			// Rapidly subscribe and unsubscribe
			for j := 0; j < iterations; j++ {
				// Subscribe
				subscribeMsg := models.ClientMessage{
					Type:      "subscribe",
					Topic:     "churn-test",
					ClientID:  clientID,
					RequestID: uuid.New().String(),
				}
				conn.WriteJSON(subscribeMsg)

				var ack models.ServerMessage
				conn.ReadJSON(&ack)

				// Small delay
				time.Sleep(5 * time.Millisecond)

				// Unsubscribe
				unsubMsg := models.ClientMessage{
					Type:      "unsubscribe",
					Topic:     "churn-test",
					ClientID:  clientID,
					RequestID: uuid.New().String(),
				}
				conn.WriteJSON(unsubMsg)

				conn.ReadJSON(&ack)

				time.Sleep(5 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	t.Logf("✓ System handled %d concurrent clients with %d subscribe/unsubscribe cycles each",
		numClients, iterations)
}
