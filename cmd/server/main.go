package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/tarunm/pubsub-system/config"
	"github.com/tarunm/pubsub-system/internal/auth"
	"github.com/tarunm/pubsub-system/internal/handlers"
	"github.com/tarunm/pubsub-system/internal/pubsub"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("[INFO] Starting PubSub server...")

	// Load configuration
	cfg := config.LoadConfig()
	log.Printf("[INFO] Configuration loaded: Port=%s, RingBuffer=%d, SubscriberQueue=%d",
		cfg.Port, cfg.RingBufferSize, cfg.SubscriberQueue)

	// Initialize authentication
	validator := auth.NewAPIKeyValidator(cfg.APIKeys, cfg.AuthEnabled)
	if cfg.AuthEnabled {
		log.Printf("[INFO] Authentication enabled with %d API key(s)", len(cfg.APIKeys))
	} else {
		log.Println("[INFO] Authentication disabled")
	}

	// Initialize pub/sub engine with configuration
	engine := pubsub.NewPubSubEngine(cfg)

	// Initialize handlers
	wsHandler := handlers.NewWebSocketHandler(engine, cfg, validator)
	restHandler := handlers.NewRESTHandler(engine)

	// Setup Gin router
	gin.SetMode(cfg.GinMode)
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// Create auth middleware
	authMiddleware := auth.AuthMiddleware(validator)

	// Unprotected endpoints
	router.GET("/health", restHandler.GetHealth)
	router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service": "PubSub System",
			"version": "1.0.0",
			"endpoints": gin.H{
				"websocket": "/ws",
				"topics":    "/topics",
				"health":    "/health",
				"stats":     "/stats",
			},
		})
	})

	// WebSocket endpoint (has built-in auth via initial message)
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

	// HTTP server configuration with timeouts from config
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	// Start server in goroutine
	go func() {
		log.Printf("[INFO] Server listening on port %s", cfg.Port)
		log.Printf("[INFO] WebSocket endpoint: ws://localhost:%s/ws", cfg.Port)
		log.Printf("[INFO] REST API endpoint: http://localhost:%s", cfg.Port)

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[FATAL] Server error: %v", err)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[INFO] Shutting down server...")

	// Create shutdown context with timeout from config
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	// Shutdown pub/sub engine first (closes all connections)
	engine.Shutdown()

	// Shutdown HTTP server
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("[ERROR] Server forced to shutdown: %v", err)
	}

	log.Println("[INFO] Server shutdown complete")
}
