package api

import (
	"fmt"
	"log"
	"net/http"

	"trading-bot/internal/manager"

	"github.com/gin-gonic/gin"
)

// Server holds the dependencies for the API server.
type Server struct {
	router   *gin.Engine
	manager  *manager.Manager
	apiKey   string
	httpPort string
}

// NewServer creates a new API server instance.
func NewServer(manager *manager.Manager, apiKey, httpPort string) *Server {
	router := gin.Default()
	server := &Server{
		router:   router,
		manager:  manager,
		apiKey:   apiKey,
		httpPort: httpPort,
	}

	server.setupMiddleware()
	server.setupRoutes()

	return server
}

// Start runs the HTTP server on a specific address.
func (s *Server) Start() {
	log.Printf("Starting API server on port %s", s.httpPort)
	if err := s.router.Run(fmt.Sprintf(":%s", s.httpPort)); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Failed to start API server: %v", err)
	}
}

func (s *Server) setupMiddleware() {
	s.router.Use(CORSMiddleware())
}

func (s *Server) setupRoutes() {
	api := s.router.Group("/")
	api.Use(APIKeyAuthMiddleware(s.apiKey))
	{
		api.GET("/status", s.statusHandler)
		api.POST("/start", s.startHandler)
		api.POST("/stop", s.stopHandler)
	}
}
