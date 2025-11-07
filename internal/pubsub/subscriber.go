package pubsub

import (
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tarunm/pubsub-system/internal/models"
)

const (
	// SubscriberQueueSize is the buffer size for each subscriber's message queue
	SubscriberQueueSize = 100

	// WriteWait is the time allowed to write a message to the peer
	WriteWait = 10 * time.Second

	// PongWait is the time allowed to read the next pong message from the peer
	PongWait = 60 * time.Second

	// PingPeriod sends pings to peer with this period (must be less than pongWait)
	PingPeriod = 30 * time.Second
)

// Subscriber represents a WebSocket client subscribed to topics
type Subscriber struct {
	ClientID    string
	Conn        *websocket.Conn
	Topics      map[string]bool
	MessageChan chan models.ServerMessage
	mu          sync.Mutex
	closed      bool
	// Configuration
	queueSize  int
	pingPeriod time.Duration
	pongWait   time.Duration
	writeWait  time.Duration
}

// NewSubscriber creates a new subscriber with default configuration
func NewSubscriber(clientID string, conn *websocket.Conn) *Subscriber {
	return NewSubscriberWithConfig(clientID, conn, SubscriberQueueSize, PingPeriod, PongWait, WriteWait)
}

// NewSubscriberWithConfig creates a new subscriber with custom configuration
func NewSubscriberWithConfig(clientID string, conn *websocket.Conn, queueSize int, pingPeriod, pongWait, writeWait time.Duration) *Subscriber {
	return &Subscriber{
		ClientID:    clientID,
		Conn:        conn,
		Topics:      make(map[string]bool),
		MessageChan: make(chan models.ServerMessage, queueSize),
		closed:      false,
		queueSize:   queueSize,
		pingPeriod:  pingPeriod,
		pongWait:    pongWait,
		writeWait:   writeWait,
	}
}

// SendMessage sends a message to the subscriber
// Implements backpressure handling: drops oldest message if queue is full
func (s *Subscriber) SendMessage(msg models.ServerMessage) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	select {
	case s.MessageChan <- msg:
		// Message queued successfully
	default:
		// Queue full - implement backpressure policy
		log.Printf("[WARN] Slow consumer detected: client_id=%s, dropping oldest message", s.ClientID)

		// Drop oldest message and try again
		select {
		case <-s.MessageChan: // Remove oldest
		default:
		}

		// Try to send again, or send SLOW_CONSUMER error
		select {
		case s.MessageChan <- msg:
			// Success after dropping oldest
		default:
			// Still full - send error and mark for disconnect
			errMsg := models.ServerMessage{
				Type: "error",
				Error: &models.ErrorInfo{
					Code:    "SLOW_CONSUMER",
					Message: "Subscriber queue overflow, disconnecting",
				},
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			}
			// Try to send error (non-blocking)
			select {
			case s.MessageChan <- errMsg:
			default:
			}
			// Close the subscriber
			go s.Close()
		}
	}
}

// WritePump sends messages from MessageChan to WebSocket
// Also handles heartbeat/ping messages
func (s *Subscriber) WritePump() {
	ticker := time.NewTicker(s.pingPeriod)
	defer func() {
		ticker.Stop()
		s.Close()
	}()

	for {
		select {
		case message, ok := <-s.MessageChan:
			if !ok {
				// Channel closed
				return
			}

			s.Conn.SetWriteDeadline(time.Now().Add(s.writeWait))
			if err := s.Conn.WriteJSON(message); err != nil {
				log.Printf("[ERROR] Write error for client %s: %v", s.ClientID, err)
				return
			}

		case <-ticker.C:
			// Send heartbeat
			s.Conn.SetWriteDeadline(time.Now().Add(s.writeWait))
			heartbeat := models.ServerMessage{
				Type:      "info",
				Msg:       "ping",
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			}
			if err := s.Conn.WriteJSON(heartbeat); err != nil {
				log.Printf("[ERROR] Heartbeat error for client %s: %v", s.ClientID, err)
				return
			}
		}
	}
}

// AddTopic adds a topic to the subscriber's topic list
func (s *Subscriber) AddTopic(topicName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Topics[topicName] = true
}

// RemoveTopic removes a topic from the subscriber's topic list
func (s *Subscriber) RemoveTopic(topicName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.Topics, topicName)
}

// GetTopics returns a copy of the subscriber's topics
func (s *Subscriber) GetTopics() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	topics := make([]string, 0, len(s.Topics))
	for topic := range s.Topics {
		topics = append(topics, topic)
	}
	return topics
}

// Close closes the subscriber's connection and message channel
func (s *Subscriber) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.mu.Unlock()

	close(s.MessageChan)
	s.Conn.Close()
}

// IsClosed returns whether the subscriber is closed
func (s *Subscriber) IsClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}
