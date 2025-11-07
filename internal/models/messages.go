package models

import "time"

// Message represents a published message
type Message struct {
	ID        string      `json:"id"`
	Payload   interface{} `json:"payload"`
	Timestamp time.Time   `json:"-"`
}

// ClientMessage represents messages from client to server
type ClientMessage struct {
	Type      string   `json:"type"` // subscribe, unsubscribe, publish, ping
	Topic     string   `json:"topic,omitempty"`
	Message   *Message `json:"message,omitempty"`
	ClientID  string   `json:"client_id,omitempty"`
	LastN     int      `json:"last_n,omitempty"`
	RequestID string   `json:"request_id,omitempty"`
}

// ServerMessage represents messages from server to client
type ServerMessage struct {
	Type      string     `json:"type"` // ack, event, error, pong, info
	RequestID string     `json:"request_id,omitempty"`
	Topic     string     `json:"topic,omitempty"`
	Message   *Message   `json:"message,omitempty"`
	Error     *ErrorInfo `json:"error,omitempty"`
	Status    string     `json:"status,omitempty"`
	Msg       string     `json:"msg,omitempty"`
	Timestamp string     `json:"ts"`
}

// ErrorInfo represents error details
type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// TopicInfo represents topic metadata
type TopicInfo struct {
	Name        string `json:"name"`
	Subscribers int    `json:"subscribers"`
}

// TopicStats represents topic statistics
type TopicStats struct {
	Messages    int64 `json:"messages"`
	Subscribers int   `json:"subscribers"`
}

// StatsResponse represents the /stats endpoint response
type StatsResponse struct {
	Topics map[string]TopicStats `json:"topics"`
}

// HealthResponse represents the /health endpoint response
type HealthResponse struct {
	UptimeSec   int `json:"uptime_sec"`
	Topics      int `json:"topics"`
	Subscribers int `json:"subscribers"`
}

// CreateTopicRequest represents the request body for creating a topic
type CreateTopicRequest struct {
	Name string `json:"name" binding:"required"`
}

// CreateTopicResponse represents the response for creating a topic
type CreateTopicResponse struct {
	Status string `json:"status"`
	Topic  string `json:"topic"`
}

// DeleteTopicResponse represents the response for deleting a topic
type DeleteTopicResponse struct {
	Status string `json:"status"`
	Topic  string `json:"topic"`
}

// ListTopicsResponse represents the response for listing topics
type ListTopicsResponse struct {
	Topics []TopicInfo `json:"topics"`
}
