package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/tarunm/pubsub-system/internal/models"
)

// TestRESTAuth_Disabled tests REST API when authentication is disabled
func TestRESTAuth_Disabled(t *testing.T) {
	server, cleanup := SetupTestServerWithAuth(t, false, nil)
	defer cleanup()

	// Create topic without API key - should succeed
	resp := CreateTopic(t, server.URL, "test-topic")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", resp.StatusCode)
	}

	// Access stats without API key - should succeed
	resp = makeGetRequest(t, server.URL+"/stats", "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for stats, got %d", resp.StatusCode)
	}
}

// TestRESTAuth_Enabled_NoKey tests REST API when auth is enabled but no key provided
func TestRESTAuth_Enabled_NoKey(t *testing.T) {
	apiKeys := []string{"test-key-123", "dev-key-456"}
	server, cleanup := SetupTestServerWithAuth(t, true, apiKeys)
	defer cleanup()

	// Try to create topic without API key - should fail
	resp := CreateTopic(t, server.URL, "test-topic")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", resp.StatusCode)
	}

	// Check error response
	var errorResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&errorResp)

	if errorResp["error"] == nil {
		t.Error("Expected error in response")
	}

	errorInfo := errorResp["error"].(map[string]interface{})
	if errorInfo["code"] != "MISSING_API_KEY" {
		t.Errorf("Expected error code MISSING_API_KEY, got %s", errorInfo["code"])
	}
}

// TestRESTAuth_Enabled_InvalidKey tests REST API with invalid API key
func TestRESTAuth_Enabled_InvalidKey(t *testing.T) {
	apiKeys := []string{"test-key-123", "dev-key-456"}
	server, cleanup := SetupTestServerWithAuth(t, true, apiKeys)
	defer cleanup()

	// Try to create topic with invalid API key - should fail
	resp := CreateTopicWithAuth(t, server.URL, "test-topic", "wrong-key")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", resp.StatusCode)
	}

	// Check error response
	var errorResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&errorResp)

	errorInfo := errorResp["error"].(map[string]interface{})
	if errorInfo["code"] != "INVALID_API_KEY" {
		t.Errorf("Expected error code INVALID_API_KEY, got %s", errorInfo["code"])
	}
}

// TestRESTAuth_Enabled_ValidKey tests REST API with valid API key
func TestRESTAuth_Enabled_ValidKey(t *testing.T) {
	apiKeys := []string{"test-key-123", "dev-key-456"}
	server, cleanup := SetupTestServerWithAuth(t, true, apiKeys)
	defer cleanup()

	// Create topic with valid API key - should succeed
	resp := CreateTopicWithAuth(t, server.URL, "test-topic", "test-key-123")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", resp.StatusCode)
	}

	// Test with second valid key
	resp = CreateTopicWithAuth(t, server.URL, "another-topic", "dev-key-456")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status 201 with second key, got %d", resp.StatusCode)
	}

	// List topics with valid key
	topics := ListTopicsWithAuth(t, server.URL, "test-key-123")
	if len(topics) != 2 {
		t.Errorf("Expected 2 topics, got %d", len(topics))
	}
}

// TestRESTAuth_HealthUnprotected tests that health endpoint is always accessible
func TestRESTAuth_HealthUnprotected(t *testing.T) {
	apiKeys := []string{"test-key-123"}
	server, cleanup := SetupTestServerWithAuth(t, true, apiKeys)
	defer cleanup()

	// Health endpoint should work without auth
	resp, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("Failed to get health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for health, got %d", resp.StatusCode)
	}

	var health models.HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Errorf("Failed to decode health response: %v", err)
	}
}

// TestRESTAuth_AllEndpoints tests all protected endpoints with auth
func TestRESTAuth_AllEndpoints(t *testing.T) {
	apiKeys := []string{"test-key-123"}
	server, cleanup := SetupTestServerWithAuth(t, true, apiKeys)
	defer cleanup()

	tests := []struct {
		name           string
		method         string
		endpoint       string
		requiresAuth   bool
		setupFunc      func() // Optional setup before test
		expectedStatus int
	}{
		{
			name:           "POST /topics requires auth",
			method:         "POST",
			endpoint:       "/topics",
			requiresAuth:   true,
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "GET /topics requires auth",
			method:         "GET",
			endpoint:       "/topics",
			requiresAuth:   true,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "GET /stats requires auth",
			method:         "GET",
			endpoint:       "/stats",
			requiresAuth:   true,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "GET /health does not require auth",
			method:         "GET",
			endpoint:       "/health",
			requiresAuth:   false,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupFunc != nil {
				tt.setupFunc()
			}

			var resp *http.Response

			if tt.method == "POST" && tt.endpoint == "/topics" {
				resp = CreateTopicWithAuth(t, server.URL, "test-topic-"+time.Now().Format("150405"), "test-key-123")
			} else if tt.method == "GET" {
				if tt.requiresAuth {
					resp = makeGetRequest(t, server.URL+tt.endpoint, "test-key-123")
				} else {
					resp = makeGetRequest(t, server.URL+tt.endpoint, "")
				}
			}

			if resp != nil {
				defer resp.Body.Close()
				if resp.StatusCode != tt.expectedStatus {
					t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
				}
			}
		})
	}
}

// TestWebSocketAuth_Disabled tests WebSocket when authentication is disabled
func TestWebSocketAuth_Disabled(t *testing.T) {
	server, cleanup := SetupTestServerWithAuth(t, false, nil)
	defer cleanup()

	// First create a topic via REST
	CreateTopic(t, server.URL, "test-topic")

	// Connect without auth message - should work
	conn := ConnectWebSocket(t, server.WSURL, "client-1")
	defer conn.Close()

	// Should be able to subscribe directly without auth
	Subscribe(t, conn, "test-topic", 0, "req-1")

	// Wait for ack
	msg := WaitForAck(t, conn, "req-1", 2*time.Second)
	if msg.Status != "ok" {
		t.Errorf("Expected status ok, got %s", msg.Status)
	}
}

// TestWebSocketAuth_Enabled_NoAuthMessage tests WebSocket when auth required but not sent
func TestWebSocketAuth_Enabled_NoAuthMessage(t *testing.T) {
	apiKeys := []string{"test-key-123"}
	server, cleanup := SetupTestServerWithAuth(t, true, apiKeys)
	defer cleanup()

	// Create topic with auth
	CreateTopicWithAuth(t, server.URL, "test-topic", "test-key-123")

	// Connect
	conn := ConnectWebSocket(t, server.WSURL, "client-1")
	defer conn.Close()

	// Try to subscribe without sending auth first - should get error or connection close
	Subscribe(t, conn, "test-topic", 0, "req-1")

	// Should receive error or connection close
	msg, err := ReceiveMessageNoFail(conn, 2*time.Second)
	if err != nil {
		// Connection closed, which is acceptable
		return
	}

	if msg.Type != "error" {
		t.Errorf("Expected error message, got %s", msg.Type)
	}

	if msg.Error == nil || msg.Error.Code != "UNAUTHORIZED" {
		t.Error("Expected UNAUTHORIZED error")
	}
}

// TestWebSocketAuth_Enabled_ValidAuth tests WebSocket with valid authentication
func TestWebSocketAuth_Enabled_ValidAuth(t *testing.T) {
	apiKeys := []string{"test-key-123"}
	server, cleanup := SetupTestServerWithAuth(t, true, apiKeys)
	defer cleanup()

	// Create topic with auth
	CreateTopicWithAuth(t, server.URL, "test-topic", "test-key-123")

	// Connect
	conn := ConnectWebSocket(t, server.WSURL, "client-1")
	defer conn.Close()

	// Send auth message first
	authMsg := models.ClientMessage{
		Type:      "auth",
		APIKey:    "test-key-123",
		RequestID: "auth-req-1",
	}
	SendMessage(t, conn, authMsg)

	// Should receive auth ack
	msg := WaitForAck(t, conn, "auth-req-1", 2*time.Second)
	if msg.Status != "authenticated" {
		t.Errorf("Expected status authenticated, got %s", msg.Status)
	}

	// Now should be able to subscribe
	Subscribe(t, conn, "test-topic", 0, "req-1")

	// Wait for subscribe ack
	msg = WaitForAck(t, conn, "req-1", 2*time.Second)
	if msg.Status != "ok" {
		t.Errorf("Expected status ok after auth, got %s", msg.Status)
	}
}

// TestWebSocketAuth_Enabled_InvalidAuth tests WebSocket with invalid API key
func TestWebSocketAuth_Enabled_InvalidAuth(t *testing.T) {
	apiKeys := []string{"test-key-123"}
	server, cleanup := SetupTestServerWithAuth(t, true, apiKeys)
	defer cleanup()

	// Connect
	conn := ConnectWebSocket(t, server.WSURL, "client-1")
	defer conn.Close()

	// Send auth message with wrong key
	authMsg := models.ClientMessage{
		Type:      "auth",
		APIKey:    "wrong-key",
		RequestID: "auth-req-1",
	}
	SendMessage(t, conn, authMsg)

	// Should receive error or connection close
	msg, err := ReceiveMessageNoFail(conn, 2*time.Second)
	if err != nil {
		// Connection closed, which is acceptable for invalid auth
		return
	}

	if msg.Type != "error" {
		t.Errorf("Expected error message, got %s", msg.Type)
	}

	if msg.Error == nil || msg.Error.Code != "INVALID_API_KEY" {
		t.Error("Expected INVALID_API_KEY error")
	}
}

// TestWebSocketAuth_Timeout tests authentication timeout
func TestWebSocketAuth_Timeout(t *testing.T) {
	apiKeys := []string{"test-key-123"}
	server, cleanup := SetupTestServerWithAuth(t, true, apiKeys)
	defer cleanup()

	// Connect
	conn := ConnectWebSocket(t, server.WSURL, "client-1")
	defer conn.Close()

	// Don't send auth message, just wait
	// Should receive timeout error after 10 seconds
	conn.SetReadDeadline(time.Now().Add(12 * time.Second))

	var msg models.ServerMessage
	err := conn.ReadJSON(&msg)

	// Should either get timeout error or connection close
	if err == nil {
		if msg.Type != "error" || msg.Error == nil || msg.Error.Code != "UNAUTHORIZED" {
			t.Error("Expected UNAUTHORIZED timeout error")
		}
	}
}

// TestWebSocketAuth_CannotReauthenticate tests that clients cannot send auth twice
func TestWebSocketAuth_CannotReauthenticate(t *testing.T) {
	apiKeys := []string{"test-key-123"}
	server, cleanup := SetupTestServerWithAuth(t, true, apiKeys)
	defer cleanup()

	// Connect and authenticate
	conn := ConnectWebSocket(t, server.WSURL, "client-1")
	defer conn.Close()

	// Send auth message
	authMsg := models.ClientMessage{
		Type:      "auth",
		APIKey:    "test-key-123",
		RequestID: "auth-req-1",
	}
	SendMessage(t, conn, authMsg)

	// Wait for auth ack
	WaitForAck(t, conn, "auth-req-1", 2*time.Second)

	// Try to send auth again - should get error
	authMsg2 := models.ClientMessage{
		Type:      "auth",
		APIKey:    "test-key-123",
		RequestID: "auth-req-2",
	}
	SendMessage(t, conn, authMsg2)

	// Should receive error about already authenticated
	msg := ReceiveMessage(t, conn, 2*time.Second)
	if msg.Type != "error" {
		t.Errorf("Expected error for double auth, got %s", msg.Type)
	}
}

// Helper functions for authenticated requests

func CreateTopicWithAuth(t *testing.T, serverURL, topicName, apiKey string) *http.Response {
	t.Helper()

	body := map[string]string{"name": topicName}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", serverURL+"/topics", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to create topic: %v", err)
	}

	return resp
}

func ListTopicsWithAuth(t *testing.T, serverURL, apiKey string) []models.TopicInfo {
	t.Helper()

	req, _ := http.NewRequest("GET", serverURL+"/topics", nil)
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
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

func makeGetRequest(t *testing.T, url, apiKey string) *http.Response {
	t.Helper()

	req, _ := http.NewRequest("GET", url, nil)
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to make GET request: %v", err)
	}

	return resp
}
