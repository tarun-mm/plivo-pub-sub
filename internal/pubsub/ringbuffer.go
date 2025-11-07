package pubsub

import (
	"sync"

	"github.com/tarunm/pubsub-system/internal/models"
)

// RingBuffer is a thread-safe circular buffer for storing message history
type RingBuffer struct {
	messages []models.Message
	capacity int
	index    int
	size     int
	mu       sync.RWMutex
}

// NewRingBuffer creates a new ring buffer with the specified capacity
func NewRingBuffer(capacity int) *RingBuffer {
	return &RingBuffer{
		messages: make([]models.Message, capacity),
		capacity: capacity,
		index:    0,
		size:     0,
	}
}

// Add adds a message to the ring buffer
func (rb *RingBuffer) Add(msg models.Message) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.messages[rb.index] = msg
	rb.index = (rb.index + 1) % rb.capacity

	if rb.size < rb.capacity {
		rb.size++
	}
}

// GetLast retrieves the last n messages from the ring buffer
// Returns messages in chronological order (oldest to newest)
func (rb *RingBuffer) GetLast(n int) []models.Message {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if n <= 0 || rb.size == 0 {
		return []models.Message{}
	}

	if n > rb.size {
		n = rb.size
	}

	result := make([]models.Message, n)
	start := (rb.index - n + rb.capacity) % rb.capacity

	for i := 0; i < n; i++ {
		idx := (start + i) % rb.capacity
		result[i] = rb.messages[idx]
	}

	return result
}

// Size returns the current number of messages in the buffer
func (rb *RingBuffer) Size() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.size
}

// Capacity returns the maximum capacity of the buffer
func (rb *RingBuffer) Capacity() int {
	return rb.capacity
}
