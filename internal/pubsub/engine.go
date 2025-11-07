package pubsub

import (
	"errors"
	"log"
	"sync"
	"time"

	"github.com/tarunm/pubsub-system/internal/models"
)

var (
	// ErrTopicExists is returned when trying to create a topic that already exists
	ErrTopicExists = errors.New("topic already exists")

	// ErrTopicNotFound is returned when trying to access a non-existent topic
	ErrTopicNotFound = errors.New("topic not found")

	// ErrInvalidMessage is returned when a message is invalid
	ErrInvalidMessage = errors.New("invalid message")

	// ErrClientNotFound is returned when a client is not found
	ErrClientNotFound = errors.New("client not found")
)

// PubSubEngine is the core pub/sub engine managing topics and clients
type PubSubEngine struct {
	Topics         map[string]*Topic
	Clients        map[string]*Subscriber
	mu             sync.RWMutex
	shutdown       chan struct{}
	startTime      time.Time
	ringBufferSize int // Configuration for ring buffer size
}

// Config interface for extracting configuration values
type Config interface {
	GetRingBufferSize() int
}

// NewPubSubEngine creates a new pub/sub engine with configuration
func NewPubSubEngine(cfg Config) *PubSubEngine {
	ringBufferSize := cfg.GetRingBufferSize()

	return &PubSubEngine{
		Topics:         make(map[string]*Topic),
		Clients:        make(map[string]*Subscriber),
		shutdown:       make(chan struct{}),
		startTime:      time.Now(),
		ringBufferSize: ringBufferSize,
	}
}

// Topic Management

// CreateTopic creates a new topic
func (e *PubSubEngine) CreateTopic(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.Topics[name]; exists {
		return ErrTopicExists
	}

	e.Topics[name] = NewTopicWithBufferSize(name, e.ringBufferSize)
	log.Printf("[INFO] Topic created: %s (buffer size: %d)", name, e.ringBufferSize)
	return nil
}

// DeleteTopic deletes a topic and notifies all subscribers
func (e *PubSubEngine) DeleteTopic(name string) error {
	e.mu.Lock()
	topic, exists := e.Topics[name]
	if !exists {
		e.mu.Unlock()
		return ErrTopicNotFound
	}

	delete(e.Topics, name)
	e.mu.Unlock()

	log.Printf("[INFO] Topic deleted: %s", name)

	// Notify all subscribers
	subscribers := topic.GetSubscribers()
	notification := models.ServerMessage{
		Type:      "info",
		Topic:     name,
		Msg:       "topic_deleted",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	for _, sub := range subscribers {
		sub.SendMessage(notification)
		sub.RemoveTopic(name)
	}

	return nil
}

// GetTopic retrieves a topic by name
func (e *PubSubEngine) GetTopic(name string) (*Topic, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	topic, exists := e.Topics[name]
	if !exists {
		return nil, ErrTopicNotFound
	}
	return topic, nil
}

// ListTopics returns a list of all topics with their subscriber counts
func (e *PubSubEngine) ListTopics() []models.TopicInfo {
	e.mu.RLock()
	defer e.mu.RUnlock()

	topics := make([]models.TopicInfo, 0, len(e.Topics))
	for _, topic := range e.Topics {
		topics = append(topics, models.TopicInfo{
			Name:        topic.Name,
			Subscribers: topic.GetSubscriberCount(),
		})
	}
	return topics
}

// TopicExists checks if a topic exists
func (e *PubSubEngine) TopicExists(name string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	_, exists := e.Topics[name]
	return exists
}

// Subscription Management

// Subscribe subscribes a client to a topic and returns historical messages if requested
func (e *PubSubEngine) Subscribe(clientID, topicName string, lastN int) ([]models.Message, error) {
	topic, err := e.GetTopic(topicName)
	if err != nil {
		return nil, err
	}

	e.mu.RLock()
	subscriber, exists := e.Clients[clientID]
	e.mu.RUnlock()

	if !exists {
		return nil, ErrClientNotFound
	}

	topic.AddSubscriber(subscriber)
	subscriber.AddTopic(topicName)

	log.Printf("[INFO] Client %s subscribed to topic %s", clientID, topicName)

	// Get historical messages if requested
	var history []models.Message
	if lastN > 0 {
		history = topic.GetLastN(lastN)
		log.Printf("[INFO] Sending %d historical messages to client %s", len(history), clientID)
	}

	return history, nil
}

// Unsubscribe unsubscribes a client from a topic
func (e *PubSubEngine) Unsubscribe(clientID, topicName string) error {
	topic, err := e.GetTopic(topicName)
	if err != nil {
		return err
	}

	topic.RemoveSubscriber(clientID)

	e.mu.RLock()
	subscriber, exists := e.Clients[clientID]
	e.mu.RUnlock()

	if exists {
		subscriber.RemoveTopic(topicName)
	}

	log.Printf("[INFO] Client %s unsubscribed from topic %s", clientID, topicName)
	return nil
}

// Publish publishes a message to a topic
func (e *PubSubEngine) Publish(topicName string, msg models.Message) error {
	topic, err := e.GetTopic(topicName)
	if err != nil {
		return err
	}

	msg.Timestamp = time.Now()
	topic.PublishMessage(msg)

	log.Printf("[INFO] Message published to topic %s: id=%s", topicName, msg.ID)
	return nil
}

// Client Management

// RegisterClient registers a new client
func (e *PubSubEngine) RegisterClient(subscriber *Subscriber) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Clients[subscriber.ClientID] = subscriber
	log.Printf("[INFO] Client registered: %s", subscriber.ClientID)
}

// UnregisterClient unregisters a client and unsubscribes from all topics
func (e *PubSubEngine) UnregisterClient(clientID string) {
	e.mu.Lock()
	subscriber, exists := e.Clients[clientID]
	if !exists {
		e.mu.Unlock()
		return
	}

	delete(e.Clients, clientID)
	e.mu.Unlock()

	log.Printf("[INFO] Client unregistered: %s", clientID)

	// Get topics before unsubscribing
	topics := subscriber.GetTopics()

	// Unsubscribe from all topics
	for _, topicName := range topics {
		if topic, err := e.GetTopic(topicName); err == nil {
			topic.RemoveSubscriber(clientID)
		}
	}

	subscriber.Close()
}

// GetClient retrieves a client by ID
func (e *PubSubEngine) GetClient(clientID string) (*Subscriber, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	client, exists := e.Clients[clientID]
	if !exists {
		return nil, ErrClientNotFound
	}
	return client, nil
}

// Stats and Health

// GetStats returns statistics for all topics
func (e *PubSubEngine) GetStats() models.StatsResponse {
	e.mu.RLock()
	defer e.mu.RUnlock()

	topics := make(map[string]models.TopicStats)
	for name, topic := range e.Topics {
		topics[name] = models.TopicStats{
			Messages:    topic.GetMessageCount(),
			Subscribers: topic.GetSubscriberCount(),
		}
	}

	return models.StatsResponse{
		Topics: topics,
	}
}

// GetHealth returns health information about the engine
func (e *PubSubEngine) GetHealth() models.HealthResponse {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return models.HealthResponse{
		UptimeSec:   int(time.Since(e.startTime).Seconds()),
		Topics:      len(e.Topics),
		Subscribers: len(e.Clients),
	}
}

// Graceful Shutdown

// Shutdown gracefully shuts down the engine
func (e *PubSubEngine) Shutdown() {
	log.Println("[INFO] Shutting down PubSub engine...")
	close(e.shutdown)

	e.mu.Lock()
	defer e.mu.Unlock()

	// Close all client connections
	for clientID, client := range e.Clients {
		log.Printf("[INFO] Closing client connection: %s", clientID)
		client.Close()
	}

	log.Println("[INFO] PubSub engine shutdown complete")
}

// IsShuttingDown returns whether the engine is shutting down
func (e *PubSubEngine) IsShuttingDown() bool {
	select {
	case <-e.shutdown:
		return true
	default:
		return false
	}
}
