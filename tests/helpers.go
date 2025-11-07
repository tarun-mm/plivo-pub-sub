package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/tarunm/pubsub-system/config"
	"github.com/tarunm/pubsub-system/internal/auth"
	"github.com/tarunm/pubsub-system/internal/handlers"
	"github.com/tarunm/pubsub-system/internal/models"
	"github.com/tarunm/pubsub-system/internal/pubsub"
)

// TestServer wraps the HTTP server for testing
type TestServer struct {
	URL    string
	WSURL  string
	server *http.Server
	engine *pubsub.PubSubEngine
}

// SetupTestServer creates and starts a test server on a random port
func SetupTestServer(t *testing.T) (*TestServer, func()) {
	t.Helper()

	// Find available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to find available port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	// Create test configuration
	cfg := &config.Config{
		Port:            fmt.Sprintf("%d", port),
		GinMode:         "release",
		RingBufferSize:  100,
		SubscriberQueue: 100,
		PingPeriod:      30 * time.Second,
		PongWait:        60 * time.Second,
		WriteWait:       10 * time.Second,
		ReadTimeout:     15 * time.Second,
		WriteTimeout:    15 * time.Second,
		IdleTimeout:     0, // No idle timeout for WebSocket tests
		ShutdownTimeout: 5 * time.Second,
	}

	// Initialize engine and handlers (no auth for backward compatibility)
	engine := pubsub.NewPubSubEngine(cfg)
	validator := auth.NewAPIKeyValidator([]string{}, false)
	wsHandler := handlers.NewWebSocketHandler(engine, cfg, validator)
	restHandler := handlers.NewRESTHandler(engine)

	// Setup router
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	// Routes
	router.GET("/ws", wsHandler.HandleWebSocket)
	router.POST("/topics", restHandler.CreateTopic)
	router.DELETE("/topics/:name", restHandler.DeleteTopic)
	router.GET("/topics", restHandler.ListTopics)
	router.GET("/health", restHandler.GetHealth)
	router.GET("/stats", restHandler.GetStats)

	// Create server
	srv := &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", port),
		Handler: router,
	}

	// Start server
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Logf("Server error: %v", err)
		}
	}()

	// Wait for server to be ready
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	wsURL := fmt.Sprintf("ws://127.0.0.1:%d", port)

	retries := 10
	for i := 0; i < retries; i++ {
		resp, err := http.Get(baseURL + "/health")
		if err == nil {
			resp.Body.Close()
			break
		}
		if i == retries-1 {
			t.Fatalf("Server failed to start: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}

	testServer := &TestServer{
		URL:    baseURL,
		WSURL:  wsURL,
		server: srv,
		engine: engine,
	}

	// Cleanup function
	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		engine.Shutdown()
		srv.Shutdown(ctx)
	}

	return testServer, cleanup
}

// SetupTestServerWithAuth creates and starts a test server with authentication support
func SetupTestServerWithAuth(t *testing.T, authEnabled bool, apiKeys []string) (*TestServer, func()) {
	t.Helper()

	// Find available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to find available port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	// Create test configuration
	cfg := &config.Config{
		Port:            fmt.Sprintf("%d", port),
		GinMode:         "release",
		RingBufferSize:  100,
		SubscriberQueue: 100,
		PingPeriod:      30 * time.Second,
		PongWait:        60 * time.Second,
		WriteWait:       10 * time.Second,
		ReadTimeout:     15 * time.Second,
		WriteTimeout:    15 * time.Second,
		IdleTimeout:     0,
		ShutdownTimeout: 5 * time.Second,
		AuthEnabled:     authEnabled,
		APIKeys:         apiKeys,
	}

	// Initialize authentication
	validator := auth.NewAPIKeyValidator(apiKeys, authEnabled)

	// Initialize engine and handlers
	engine := pubsub.NewPubSubEngine(cfg)
	wsHandler := handlers.NewWebSocketHandler(engine, cfg, validator)
	restHandler := handlers.NewRESTHandler(engine)

	// Setup router
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	// Create auth middleware
	authMiddleware := auth.AuthMiddleware(validator)

	// Unprotected endpoints
	router.GET("/health", restHandler.GetHealth)

	// WebSocket endpoint (has built-in auth)
	router.GET("/ws", wsHandler.HandleWebSocket)

	// Protected REST API endpoints
	protected := router.Group("/")
	protected.Use(authMiddleware)
	{
		protected.POST("/topics", restHandler.CreateTopic)
		protected.DELETE("/topics/:name", restHandler.DeleteTopic)
		protected.GET("/topics", restHandler.ListTopics)
		protected.GET("/stats", restHandler.GetStats)
	}

	// Create server
	srv := &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", port),
		Handler: router,
	}

	// Start server
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Logf("Server error: %v", err)
		}
	}()

	// Wait for server to be ready
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	wsURL := fmt.Sprintf("ws://127.0.0.1:%d", port)

	retries := 10
	for i := 0; i < retries; i++ {
		resp, err := http.Get(baseURL + "/health")
		if err == nil {
			resp.Body.Close()
			break
		}
		if i == retries-1 {
			t.Fatalf("Server failed to start: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}

	testServer := &TestServer{
		URL:    baseURL,
		WSURL:  wsURL,
		server: srv,
		engine: engine,
	}

	// Cleanup function
	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		engine.Shutdown()
		srv.Shutdown(ctx)
	}

	return testServer, cleanup
}

// REST API Helper Functions

// CreateTopic creates a topic via REST API
func CreateTopic(t *testing.T, serverURL, topicName string) *http.Response {
	t.Helper()

	body := map[string]string{"name": topicName}
	jsonBody, _ := json.Marshal(body)

	resp, err := http.Post(
		serverURL+"/topics",
		"application/json",
		bytes.NewBuffer(jsonBody),
	)
	if err != nil {
		t.Fatalf("Failed to create topic: %v", err)
	}

	return resp
}

// DeleteTopic deletes a topic via REST API
func DeleteTopic(t *testing.T, serverURL, topicName string) *http.Response {
	t.Helper()

	req, _ := http.NewRequest("DELETE", serverURL+"/topics/"+topicName, nil)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to delete topic: %v", err)
	}

	return resp
}

// ListTopics lists all topics via REST API
func ListTopics(t *testing.T, serverURL string) []models.TopicInfo {
	t.Helper()

	resp, err := http.Get(serverURL + "/topics")
	if err != nil {
		t.Fatalf("Failed to list topics: %v", err)
	}
	defer resp.Body.Close()

	var result models.ListTopicsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode topics: %v", err)
	}

	return result.Topics
}

// GetHealth gets health status via REST API
func GetHealth(t *testing.T, serverURL string) models.HealthResponse {
	t.Helper()

	resp, err := http.Get(serverURL + "/health")
	if err != nil {
		t.Fatalf("Failed to get health: %v", err)
	}
	defer resp.Body.Close()

	var health models.HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("Failed to decode health: %v", err)
	}

	return health
}

// GetStats gets statistics via REST API
func GetStats(t *testing.T, serverURL string) models.StatsResponse {
	t.Helper()

	resp, err := http.Get(serverURL + "/stats")
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}
	defer resp.Body.Close()

	var stats models.StatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		t.Fatalf("Failed to decode stats: %v", err)
	}

	return stats
}

// WebSocket Helper Functions

// ConnectWebSocket connects a WebSocket client
func ConnectWebSocket(t *testing.T, wsURL, clientID string) *websocket.Conn {
	t.Helper()

	url := fmt.Sprintf("%s/ws?client_id=%s", wsURL, clientID)
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("Failed to connect WebSocket: %v", err)
	}

	return conn
}

// SendMessage sends a message to the WebSocket
func SendMessage(t *testing.T, conn *websocket.Conn, msg models.ClientMessage) {
	t.Helper()

	if err := conn.WriteJSON(msg); err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}
}

// ReceiveMessage receives a message from the WebSocket with timeout
func ReceiveMessage(t *testing.T, conn *websocket.Conn, timeout time.Duration) models.ServerMessage {
	t.Helper()

	conn.SetReadDeadline(time.Now().Add(timeout))

	var msg models.ServerMessage
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("Failed to receive message: %v", err)
	}

	return msg
}

// ReceiveMessageNoFail receives a message but doesn't fail on timeout
func ReceiveMessageNoFail(conn *websocket.Conn, timeout time.Duration) (models.ServerMessage, error) {
	conn.SetReadDeadline(time.Now().Add(timeout))

	var msg models.ServerMessage
	err := conn.ReadJSON(&msg)
	return msg, err
}

// Subscribe subscribes to a topic via WebSocket
func Subscribe(t *testing.T, conn *websocket.Conn, topic string, lastN int, requestID string) {
	t.Helper()

	msg := models.ClientMessage{
		Type:      "subscribe",
		Topic:     topic,
		ClientID:  "test-client",
		LastN:     lastN,
		RequestID: requestID,
	}

	SendMessage(t, conn, msg)
}

// Unsubscribe unsubscribes from a topic via WebSocket
func Unsubscribe(t *testing.T, conn *websocket.Conn, topic string, requestID string) {
	t.Helper()

	msg := models.ClientMessage{
		Type:      "unsubscribe",
		Topic:     topic,
		ClientID:  "test-client",
		RequestID: requestID,
	}

	SendMessage(t, conn, msg)
}

// Publish publishes a message to a topic via WebSocket
func Publish(t *testing.T, conn *websocket.Conn, topic string, messageID string, payload interface{}, requestID string) {
	t.Helper()

	msg := models.ClientMessage{
		Type:  "publish",
		Topic: topic,
		Message: &models.Message{
			ID:      messageID,
			Payload: payload,
		},
		RequestID: requestID,
	}

	SendMessage(t, conn, msg)
}

// SendPing sends a ping message via WebSocket
func SendPing(t *testing.T, conn *websocket.Conn, requestID string) {
	t.Helper()

	msg := models.ClientMessage{
		Type:      "ping",
		RequestID: requestID,
	}

	SendMessage(t, conn, msg)
}

// WaitForAck waits for an acknowledgment message
func WaitForAck(t *testing.T, conn *websocket.Conn, expectedRequestID string, timeout time.Duration) models.ServerMessage {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		msg := ReceiveMessage(t, conn, time.Until(deadline))
		if msg.Type == "ack" && msg.RequestID == expectedRequestID {
			return msg
		}
	}

	t.Fatalf("Did not receive ack for request %s within timeout", expectedRequestID)
	return models.ServerMessage{}
}

// WaitForEvent waits for an event message
func WaitForEvent(t *testing.T, conn *websocket.Conn, timeout time.Duration) models.ServerMessage {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		msg := ReceiveMessage(t, conn, time.Until(deadline))
		if msg.Type == "event" {
			return msg
		}
	}

	t.Fatalf("Did not receive event within timeout")
	return models.ServerMessage{}
}
