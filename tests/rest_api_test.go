package tests

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// TestHealthEndpoint tests the /health endpoint
func TestHealthEndpoint(t *testing.T) {
	server, cleanup := SetupTestServer(t)
	defer cleanup()

	health := GetHealth(t, server.URL)

	// Verify health response
	if health.Topics != 0 {
		t.Errorf("Expected 0 topics, got %d", health.Topics)
	}
	if health.Subscribers != 0 {
		t.Errorf("Expected 0 subscribers, got %d", health.Subscribers)
	}
	if health.UptimeSec < 0 {
		t.Errorf("Invalid uptime: %d", health.UptimeSec)
	}
}

// TestCreateTopic tests topic creation via POST /topics
func TestCreateTopic(t *testing.T) {
	server, cleanup := SetupTestServer(t)
	defer cleanup()

	// Create a topic
	resp := CreateTopic(t, server.URL, "orders")
	defer resp.Body.Close()

	// Verify status code
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", resp.StatusCode)
	}

	// Verify response body
	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)

	if result["status"] != "created" {
		t.Errorf("Expected status 'created', got '%s'", result["status"])
	}
	if result["topic"] != "orders" {
		t.Errorf("Expected topic 'orders', got '%s'", result["topic"])
	}
}

// TestCreateDuplicateTopic tests creating a topic that already exists
func TestCreateDuplicateTopic(t *testing.T) {
	server, cleanup := SetupTestServer(t)
	defer cleanup()

	// Create topic first time
	resp1 := CreateTopic(t, server.URL, "orders")
	resp1.Body.Close()

	// Try to create same topic again
	resp2 := CreateTopic(t, server.URL, "orders")
	defer resp2.Body.Close()

	// Verify conflict status
	if resp2.StatusCode != http.StatusConflict {
		t.Errorf("Expected status 409 Conflict, got %d", resp2.StatusCode)
	}

	body, _ := io.ReadAll(resp2.Body)
	if len(body) == 0 {
		t.Error("Expected error message in response body")
	}
}

// TestListTopics tests listing topics via GET /topics
func TestListTopics(t *testing.T) {
	server, cleanup := SetupTestServer(t)
	defer cleanup()

	// Initially should be empty
	topics := ListTopics(t, server.URL)
	if len(topics) != 0 {
		t.Errorf("Expected 0 topics, got %d", len(topics))
	}

	// Create some topics
	CreateTopic(t, server.URL, "orders")
	CreateTopic(t, server.URL, "notifications")
	CreateTopic(t, server.URL, "payments")

	// List topics
	topics = ListTopics(t, server.URL)
	if len(topics) != 3 {
		t.Errorf("Expected 3 topics, got %d", len(topics))
	}

	// Verify topic names
	topicNames := make(map[string]bool)
	for _, topic := range topics {
		topicNames[topic.Name] = true
		if topic.Subscribers != 0 {
			t.Errorf("Expected 0 subscribers for topic %s, got %d", topic.Name, topic.Subscribers)
		}
	}

	expectedTopics := []string{"orders", "notifications", "payments"}
	for _, name := range expectedTopics {
		if !topicNames[name] {
			t.Errorf("Expected topic '%s' not found in list", name)
		}
	}
}

// TestDeleteTopic tests topic deletion via DELETE /topics/:name
func TestDeleteTopic(t *testing.T) {
	server, cleanup := SetupTestServer(t)
	defer cleanup()

	// Create a topic
	CreateTopic(t, server.URL, "orders")

	// Verify it exists
	topics := ListTopics(t, server.URL)
	if len(topics) != 1 {
		t.Fatalf("Expected 1 topic, got %d", len(topics))
	}

	// Delete the topic
	resp := DeleteTopic(t, server.URL, "orders")
	defer resp.Body.Close()

	// Verify status code
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Verify response body
	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)

	if result["status"] != "deleted" {
		t.Errorf("Expected status 'deleted', got '%s'", result["status"])
	}
	if result["topic"] != "orders" {
		t.Errorf("Expected topic 'orders', got '%s'", result["topic"])
	}

	// Verify topic is gone
	topics = ListTopics(t, server.URL)
	if len(topics) != 0 {
		t.Errorf("Expected 0 topics after deletion, got %d", len(topics))
	}
}

// TestDeleteNonExistentTopic tests deleting a topic that doesn't exist
func TestDeleteNonExistentTopic(t *testing.T) {
	server, cleanup := SetupTestServer(t)
	defer cleanup()

	// Try to delete non-existent topic
	resp := DeleteTopic(t, server.URL, "nonexistent")
	defer resp.Body.Close()

	// Verify 404 status
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.StatusCode)
	}
}

// TestStatsEndpoint tests the /stats endpoint
func TestStatsEndpoint(t *testing.T) {
	server, cleanup := SetupTestServer(t)
	defer cleanup()

	// Initially empty
	stats := GetStats(t, server.URL)
	if len(stats.Topics) != 0 {
		t.Errorf("Expected 0 topics in stats, got %d", len(stats.Topics))
	}

	// Create topics
	CreateTopic(t, server.URL, "orders")
	CreateTopic(t, server.URL, "notifications")

	// Get stats again
	stats = GetStats(t, server.URL)
	if len(stats.Topics) != 2 {
		t.Errorf("Expected 2 topics in stats, got %d", len(stats.Topics))
	}

	// Verify each topic has 0 messages initially
	for topicName, topicStats := range stats.Topics {
		if topicStats.Messages != 0 {
			t.Errorf("Expected 0 messages for topic %s, got %d", topicName, topicStats.Messages)
		}
		if topicStats.Subscribers != 0 {
			t.Errorf("Expected 0 subscribers for topic %s, got %d", topicName, topicStats.Subscribers)
		}
	}
}

// TestCreateTopicInvalidRequest tests POST /topics with invalid request
func TestCreateTopicInvalidRequest(t *testing.T) {
	server, cleanup := SetupTestServer(t)
	defer cleanup()

	// Send empty body
	resp, err := http.Post(server.URL+"/topics", "application/json", nil)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Should return 400 Bad Request
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

// TestHealthAfterActivity tests health endpoint reflects activity
func TestHealthAfterActivity(t *testing.T) {
	server, cleanup := SetupTestServer(t)
	defer cleanup()

	// Create topics
	CreateTopic(t, server.URL, "orders")
	CreateTopic(t, server.URL, "payments")

	// Get health
	health := GetHealth(t, server.URL)

	// Verify topics count
	if health.Topics != 2 {
		t.Errorf("Expected 2 topics in health, got %d", health.Topics)
	}

	// Uptime should be non-negative (can be 0 if test runs very fast)
	if health.UptimeSec < 0 {
		t.Errorf("Expected non-negative uptime, got %d", health.UptimeSec)
	}
}

// TestMultipleTopicsWorkflow tests a complete workflow with multiple topics
func TestMultipleTopicsWorkflow(t *testing.T) {
	server, cleanup := SetupTestServer(t)
	defer cleanup()

	// Create multiple topics
	topicNames := []string{"orders", "payments", "notifications", "events"}
	for _, name := range topicNames {
		resp := CreateTopic(t, server.URL, name)
		resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Errorf("Failed to create topic %s: status %d", name, resp.StatusCode)
		}
	}

	// Verify all topics exist
	topics := ListTopics(t, server.URL)
	if len(topics) != len(topicNames) {
		t.Errorf("Expected %d topics, got %d", len(topicNames), len(topics))
	}

	// Delete some topics
	DeleteTopic(t, server.URL, "payments")
	DeleteTopic(t, server.URL, "events")

	// Verify remaining topics
	topics = ListTopics(t, server.URL)
	if len(topics) != 2 {
		t.Errorf("Expected 2 topics after deletions, got %d", len(topics))
	}

	// Verify correct topics remain
	remainingNames := make(map[string]bool)
	for _, topic := range topics {
		remainingNames[topic.Name] = true
	}

	if !remainingNames["orders"] || !remainingNames["notifications"] {
		t.Error("Wrong topics remaining after deletions")
	}
}
