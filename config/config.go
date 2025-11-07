package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds application configuration
type Config struct {
	// Server Configuration
	Port    string
	GinMode string

	// PubSub Configuration
	RingBufferSize  int // Number of messages to store per topic for replay
	SubscriberQueue int // Buffer size for each subscriber's message queue

	// WebSocket Configuration
	PingPeriod   time.Duration // How often to send heartbeat pings
	PongWait     time.Duration // Max time to wait for pong response
	WriteWait    time.Duration // Max time to write a message
	ReadTimeout  time.Duration // HTTP read timeout
	WriteTimeout time.Duration // HTTP write timeout
	IdleTimeout  time.Duration // HTTP idle timeout

	// Shutdown Configuration
	ShutdownTimeout time.Duration // Max time to wait for graceful shutdown

	// Authentication Configuration
	AuthEnabled bool     // Enable/disable API key authentication
	APIKeys     []string // Valid API keys for authentication
}

// LoadConfig loads configuration from environment variables with defaults
// All timing values can be overridden via environment variables
func LoadConfig() *Config {
	return &Config{
		// Server
		Port:    getEnv("PORT", "8080"),
		GinMode: getEnv("GIN_MODE", "release"),

		// PubSub
		RingBufferSize:  getEnvInt("RING_BUFFER_SIZE", 100),
		SubscriberQueue: getEnvInt("SUBSCRIBER_QUEUE_SIZE", 100),

		// WebSocket Timeouts
		PingPeriod: getEnvDuration("PING_PERIOD_SEC", 30) * time.Second,
		PongWait:   getEnvDuration("PONG_WAIT_SEC", 60) * time.Second,
		WriteWait:  getEnvDuration("WRITE_WAIT_SEC", 10) * time.Second,

		// HTTP Timeouts
		ReadTimeout:  getEnvDuration("READ_TIMEOUT_SEC", 15) * time.Second,
		WriteTimeout: getEnvDuration("WRITE_TIMEOUT_SEC", 15) * time.Second,
		IdleTimeout:  getEnvDuration("IDLE_TIMEOUT_SEC", 0) * time.Second, // 0 = no timeout for WebSocket connections

		// Shutdown
		ShutdownTimeout: getEnvDuration("SHUTDOWN_TIMEOUT_SEC", 10) * time.Second,

		// Authentication
		AuthEnabled: getEnvBool("AUTH_ENABLED", false),
		APIKeys:     getEnvSlice("API_KEYS", []string{}),
	}
}

// getEnv retrieves string environment variable or returns default
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt retrieves integer environment variable or returns default
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvDuration retrieves duration (in seconds) environment variable or returns default
func getEnvDuration(key string, defaultSeconds int) time.Duration {
	return time.Duration(getEnvInt(key, defaultSeconds))
}

// GetRingBufferSize returns the ring buffer size configuration
func (c *Config) GetRingBufferSize() int {
	return c.RingBufferSize
}

// GetSubscriberQueue returns the subscriber queue size
func (c *Config) GetSubscriberQueue() int {
	return c.SubscriberQueue
}

// GetPingPeriod returns the ping period duration
func (c *Config) GetPingPeriod() time.Duration {
	return c.PingPeriod
}

// GetPongWait returns the pong wait duration
func (c *Config) GetPongWait() time.Duration {
	return c.PongWait
}

// GetWriteWait returns the write wait duration
func (c *Config) GetWriteWait() time.Duration {
	return c.WriteWait
}

// getEnvBool retrieves boolean environment variable or returns default
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

// getEnvSlice retrieves comma-separated string environment variable and returns as slice
func getEnvSlice(key string, defaultValue []string) []string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
