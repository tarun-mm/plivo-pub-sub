package pubsub

import (
	"sync"
	"time"

	"github.com/tarunm/pubsub-system/internal/models"
)

const (
	// DefaultBufferSize is the ring buffer capacity for message history
	DefaultBufferSize = 100
)

// Topic represents a pub/sub topic with subscribers and message history
type Topic struct {
	Name          string
	Subscribers   map[string]*Subscriber
	MessageBuffer *RingBuffer
	MessageCount  int64
	CreatedAt     time.Time
	mu            sync.RWMutex
}

// NewTopic creates a new topic with the given name and default buffer size
func NewTopic(name string) *Topic {
	return NewTopicWithBufferSize(name, DefaultBufferSize)
}

// NewTopicWithBufferSize creates a new topic with a custom buffer size
func NewTopicWithBufferSize(name string, bufferSize int) *Topic {
	return &Topic{
		Name:          name,
		Subscribers:   make(map[string]*Subscriber),
		MessageBuffer: NewRingBuffer(bufferSize),
		MessageCount:  0,
		CreatedAt:     time.Now(),
	}
}

// AddSubscriber adds a subscriber to the topic
func (t *Topic) AddSubscriber(sub *Subscriber) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Subscribers[sub.ClientID] = sub
}

// RemoveSubscriber removes a subscriber from the topic
func (t *Topic) RemoveSubscriber(clientID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.Subscribers, clientID)
}

// GetSubscriber returns a specific subscriber by client ID
func (t *Topic) GetSubscriber(clientID string) (*Subscriber, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	sub, exists := t.Subscribers[clientID]
	return sub, exists
}

// GetSubscribers returns a slice of all subscribers
func (t *Topic) GetSubscribers() []*Subscriber {
	t.mu.RLock()
	defer t.mu.RUnlock()

	subs := make([]*Subscriber, 0, len(t.Subscribers))
	for _, sub := range t.Subscribers {
		subs = append(subs, sub)
	}
	return subs
}

// GetSubscriberCount returns the number of subscribers
func (t *Topic) GetSubscriberCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.Subscribers)
}

// PublishMessage publishes a message to all subscribers and stores it in history
func (t *Topic) PublishMessage(msg models.Message) {
	// Store message in buffer and increment count
	t.mu.Lock()
	t.MessageBuffer.Add(msg)
	t.MessageCount++
	t.mu.Unlock()

	// Fan-out to all subscribers
	subscribers := t.GetSubscribers()

	serverMsg := models.ServerMessage{
		Type:      "event",
		Topic:     t.Name,
		Message:   &msg,
		Timestamp: msg.Timestamp.UTC().Format(time.RFC3339),
	}

	for _, sub := range subscribers {
		// Skip closed subscribers
		if !sub.IsClosed() {
			sub.SendMessage(serverMsg)
		}
	}
}

// GetLastN retrieves the last n messages from the topic's history
func (t *Topic) GetLastN(n int) []models.Message {
	return t.MessageBuffer.GetLast(n)
}

// GetMessageCount returns the total number of messages published to this topic
func (t *Topic) GetMessageCount() int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.MessageCount
}
