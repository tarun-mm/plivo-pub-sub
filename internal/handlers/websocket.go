package handlers

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/tarunm/pubsub-system/internal/auth"
	"github.com/tarunm/pubsub-system/internal/models"
	"github.com/tarunm/pubsub-system/internal/pubsub"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for demo purposes
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// WebSocketConfig interface for handler configuration
type WebSocketConfig interface {
	GetSubscriberQueue() int
	GetPingPeriod() time.Duration
	GetPongWait() time.Duration
	GetWriteWait() time.Duration
}

// WebSocketHandler handles WebSocket connections
type WebSocketHandler struct {
	engine    *pubsub.PubSubEngine
	config    WebSocketConfig
	validator *auth.APIKeyValidator
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(engine *pubsub.PubSubEngine, config WebSocketConfig, validator *auth.APIKeyValidator) *WebSocketHandler {
	return &WebSocketHandler{
		engine:    engine,
		config:    config,
		validator: validator,
	}
}

// HandleWebSocket handles WebSocket upgrade and connection
func (h *WebSocketHandler) HandleWebSocket(c *gin.Context) {
	// Check if engine is shutting down
	if h.engine.IsShuttingDown() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "server is shutting down"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[ERROR] WebSocket upgrade error: %v", err)
		return
	}

	// Generate or get client ID
	clientID := c.Query("client_id")
	if clientID == "" {
		clientID = generateClientID()
	}

	// Create and register subscriber with configuration
	subscriber := pubsub.NewSubscriberWithConfig(
		clientID,
		conn,
		h.config.GetSubscriberQueue(),
		h.config.GetPingPeriod(),
		h.config.GetPongWait(),
		h.config.GetWriteWait(),
	)
	h.engine.RegisterClient(subscriber)

	log.Printf("[INFO] WebSocket client connected: %s from %s", clientID, c.ClientIP())

	// Start write pump in goroutine
	go subscriber.WritePump()

	// Start read pump (blocking)
	h.readPump(subscriber)

	// Cleanup on disconnect
	h.engine.UnregisterClient(clientID)
	log.Printf("[INFO] WebSocket client disconnected: %s", clientID)
}

// readPump reads messages from the WebSocket connection
func (h *WebSocketHandler) readPump(sub *pubsub.Subscriber) {
	defer sub.Conn.Close()

	sub.Conn.SetReadDeadline(time.Now().Add(h.config.GetPongWait()))
	sub.Conn.SetPongHandler(func(string) error {
		sub.Conn.SetReadDeadline(time.Now().Add(h.config.GetPongWait()))
		return nil
	})

	// If auth is enabled, wait for auth message with timeout
	if h.validator.IsEnabled() {
		authTimeout := time.NewTimer(10 * time.Second)
		defer authTimeout.Stop()

		authChan := make(chan bool, 1)

		go func() {
			var msg models.ClientMessage
			err := sub.Conn.ReadJSON(&msg)
			if err != nil {
				authChan <- false
				return
			}

			if msg.Type != "auth" {
				h.sendError(sub, msg.RequestID, auth.ErrCodeUnauthorized, "Authentication required. First message must be of type 'auth'")
				authChan <- false
				return
			}

			if !h.validator.ValidateKey(msg.APIKey) {
				h.sendError(sub, msg.RequestID, auth.ErrCodeInvalidAPIKey, auth.ErrMsgInvalidAPIKey)
				authChan <- false
				return
			}

			// Send success ack
			sub.SendMessage(models.ServerMessage{
				Type:      "ack",
				RequestID: msg.RequestID,
				Status:    "authenticated",
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			})
			log.Printf("[INFO] Client %s authenticated successfully", sub.ClientID)
			authChan <- true
		}()

		select {
		case success := <-authChan:
			if !success {
				log.Printf("[WARN] Client %s authentication failed", sub.ClientID)
				return
			}
			// Authentication successful, proceed to main loop
		case <-authTimeout.C:
			h.sendError(sub, "", auth.ErrCodeUnauthorized, "Authentication timeout")
			log.Printf("[WARN] Client %s authentication timeout", sub.ClientID)
			return
		}
	}

	// Main message loop (only reachable if authenticated)
	for {
		var msg models.ClientMessage
		err := sub.Conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[ERROR] WebSocket error for client %s: %v", sub.ClientID, err)
			}
			break
		}

		// Reset read deadline on any message received (proves connection is alive)
		sub.Conn.SetReadDeadline(time.Now().Add(h.config.GetPongWait()))

		// Reject auth messages after initial authentication
		if msg.Type == "auth" {
			h.sendError(sub, msg.RequestID, "BAD_REQUEST", "Already authenticated")
			continue
		}

		log.Printf("[DEBUG] Received message from client %s: type=%s, topic=%s", sub.ClientID, msg.Type, msg.Topic)
		h.handleMessage(sub, msg)
	}
}

// handleMessage routes messages based on type
func (h *WebSocketHandler) handleMessage(sub *pubsub.Subscriber, msg models.ClientMessage) {
	switch msg.Type {
	case "subscribe":
		h.handleSubscribe(sub, msg)
	case "unsubscribe":
		h.handleUnsubscribe(sub, msg)
	case "publish":
		h.handlePublish(sub, msg)
	case "ping":
		h.handlePing(sub, msg)
	default:
		h.sendError(sub, msg.RequestID, "BAD_REQUEST", "Unknown message type: "+msg.Type)
	}
}

// handleSubscribe handles subscribe requests
func (h *WebSocketHandler) handleSubscribe(sub *pubsub.Subscriber, msg models.ClientMessage) {
	// Validate request
	if msg.Topic == "" {
		h.sendError(sub, msg.RequestID, "BAD_REQUEST", "topic is required")
		return
	}

	// Subscribe to topic
	history, err := h.engine.Subscribe(sub.ClientID, msg.Topic, msg.LastN)
	if err != nil {
		if err == pubsub.ErrTopicNotFound {
			h.sendError(sub, msg.RequestID, "TOPIC_NOT_FOUND", fmt.Sprintf("Topic '%s' does not exist", msg.Topic))
		} else {
			h.sendError(sub, msg.RequestID, "INTERNAL", err.Error())
		}
		return
	}

	// Send acknowledgment
	sub.SendMessage(models.ServerMessage{
		Type:      "ack",
		RequestID: msg.RequestID,
		Topic:     msg.Topic,
		Status:    "ok",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})

	// Send historical messages if requested
	if len(history) > 0 {
		for _, histMsg := range history {
			sub.SendMessage(models.ServerMessage{
				Type:      "event",
				Topic:     msg.Topic,
				Message:   &histMsg,
				Timestamp: histMsg.Timestamp.UTC().Format(time.RFC3339),
			})
		}
		log.Printf("[INFO] Sent %d historical messages to client %s for topic %s", len(history), sub.ClientID, msg.Topic)
	}
}

// handleUnsubscribe handles unsubscribe requests
func (h *WebSocketHandler) handleUnsubscribe(sub *pubsub.Subscriber, msg models.ClientMessage) {
	// Validate request
	if msg.Topic == "" {
		h.sendError(sub, msg.RequestID, "BAD_REQUEST", "topic is required")
		return
	}

	// Unsubscribe from topic
	err := h.engine.Unsubscribe(sub.ClientID, msg.Topic)
	if err != nil {
		if err == pubsub.ErrTopicNotFound {
			h.sendError(sub, msg.RequestID, "TOPIC_NOT_FOUND", fmt.Sprintf("Topic '%s' does not exist", msg.Topic))
		} else {
			h.sendError(sub, msg.RequestID, "INTERNAL", err.Error())
		}
		return
	}

	// Send acknowledgment
	sub.SendMessage(models.ServerMessage{
		Type:      "ack",
		RequestID: msg.RequestID,
		Topic:     msg.Topic,
		Status:    "ok",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// handlePublish handles publish requests
func (h *WebSocketHandler) handlePublish(sub *pubsub.Subscriber, msg models.ClientMessage) {
	// Validate request
	if msg.Topic == "" {
		h.sendError(sub, msg.RequestID, "BAD_REQUEST", "topic is required")
		return
	}

	if msg.Message == nil {
		h.sendError(sub, msg.RequestID, "BAD_REQUEST", "message is required")
		return
	}

	if msg.Message.ID == "" {
		h.sendError(sub, msg.RequestID, "BAD_REQUEST", "message.id is required")
		return
	}

	// Validate message ID is a valid UUID
	if _, err := uuid.Parse(msg.Message.ID); err != nil {
		h.sendError(sub, msg.RequestID, "BAD_REQUEST", "message.id must be a valid UUID")
		return
	}

	// Publish message
	err := h.engine.Publish(msg.Topic, *msg.Message)
	if err != nil {
		if err == pubsub.ErrTopicNotFound {
			h.sendError(sub, msg.RequestID, "TOPIC_NOT_FOUND", fmt.Sprintf("Topic '%s' does not exist", msg.Topic))
		} else {
			h.sendError(sub, msg.RequestID, "INTERNAL", err.Error())
		}
		return
	}

	// Send acknowledgment
	sub.SendMessage(models.ServerMessage{
		Type:      "ack",
		RequestID: msg.RequestID,
		Topic:     msg.Topic,
		Status:    "ok",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// handlePing handles ping requests
func (h *WebSocketHandler) handlePing(sub *pubsub.Subscriber, msg models.ClientMessage) {
	sub.SendMessage(models.ServerMessage{
		Type:      "pong",
		RequestID: msg.RequestID,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// sendError sends an error message to the client
func (h *WebSocketHandler) sendError(sub *pubsub.Subscriber, requestID, code, message string) {
	sub.SendMessage(models.ServerMessage{
		Type:      "error",
		RequestID: requestID,
		Error: &models.ErrorInfo{
			Code:    code,
			Message: message,
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// generateClientID generates a unique client ID
func generateClientID() string {
	return fmt.Sprintf("client-%s", uuid.New().String()[:8])
}
