package api

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

// Server represents the API server
type Server struct {
	router *gin.Engine
}

// NewServer creates a new API server instance
func NewServer() *Server {
	router := gin.Default()

	s := &Server{
		router: router,
	}

	s.setupRoutes()

	return s
}

// Run starts the API server on the specified port
func (s *Server) Run(port int) error {
	return s.router.Run(fmt.Sprintf(":%d", port))
}

func (s *Server) setupRoutes() {
	// Health check endpoint
	s.router.GET("/health", s.healthCheck)

	// API v1 routes
	v1 := s.router.Group("/api/v1")
	{
		// Processed lines endpoints
		v1.GET("/lines", s.listItems)
		v1.GET("/lines/:id", s.getItem)

		// Movies endpoints
		v1.GET("/movies", s.listFilters)
		v1.POST("/movies", s.createFilter)

		// TV shows endpoints
		v1.GET("/tvshows", s.listLogs)

		// Statistics
		v1.GET("/stats", s.healthCheck)
	}
}
