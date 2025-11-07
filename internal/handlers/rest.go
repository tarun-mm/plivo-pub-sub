package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tarunm/pubsub-system/internal/models"
	"github.com/tarunm/pubsub-system/internal/pubsub"
)

// RESTHandler handles REST API endpoints
type RESTHandler struct {
	engine *pubsub.PubSubEngine
}

// NewRESTHandler creates a new REST handler
func NewRESTHandler(engine *pubsub.PubSubEngine) *RESTHandler {
	return &RESTHandler{engine: engine}
}

// CreateTopic handles POST /topics
func (h *RESTHandler) CreateTopic(c *gin.Context) {
	var req models.CreateTopicRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	// Validate topic name
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "topic name cannot be empty"})
		return
	}

	// Create topic
	err := h.engine.CreateTopic(req.Name)
	if err == pubsub.ErrTopicExists {
		c.JSON(http.StatusConflict, gin.H{"error": "topic already exists"})
		return
	} else if err != nil {
		log.Printf("[ERROR] Failed to create topic: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusCreated, models.CreateTopicResponse{
		Status: "created",
		Topic:  req.Name,
	})
}

// DeleteTopic handles DELETE /topics/:name
func (h *RESTHandler) DeleteTopic(c *gin.Context) {
	name := c.Param("name")

	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "topic name is required"})
		return
	}

	// Delete topic
	err := h.engine.DeleteTopic(name)
	if err == pubsub.ErrTopicNotFound {
		c.JSON(http.StatusNotFound, gin.H{"error": "topic not found"})
		return
	} else if err != nil {
		log.Printf("[ERROR] Failed to delete topic: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, models.DeleteTopicResponse{
		Status: "deleted",
		Topic:  name,
	})
}

// ListTopics handles GET /topics
func (h *RESTHandler) ListTopics(c *gin.Context) {
	topics := h.engine.ListTopics()

	c.JSON(http.StatusOK, models.ListTopicsResponse{
		Topics: topics,
	})
}

// GetHealth handles GET /health
func (h *RESTHandler) GetHealth(c *gin.Context) {
	health := h.engine.GetHealth()
	c.JSON(http.StatusOK, health)
}

// GetStats handles GET /stats
func (h *RESTHandler) GetStats(c *gin.Context) {
	stats := h.engine.GetStats()
	c.JSON(http.StatusOK, stats)
}
