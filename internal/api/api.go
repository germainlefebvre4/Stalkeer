package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// Server represents the API server
type Server struct {
	router     *gin.Engine
	httpServer *http.Server
}

// NewServer creates a new API server instance
func NewServer() *Server {
	router := gin.Default()

	// Configure CORS
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"*"} // TODO: Configure from config file
	config.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}
	router.Use(cors.New(config))

	// Add request ID middleware
	router.Use(requestIDMiddleware())

	// Add error handling middleware
	router.Use(errorHandlerMiddleware())

	s := &Server{
		router: router,
	}

	s.setupRoutes()

	return s
}

// Run starts the API server on the specified port
func (s *Server) Run(port int) error {
	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

func (s *Server) setupRoutes() {
	// Health check endpoint
	s.router.GET("/health", s.healthCheck)

	// API v1 routes
	v1 := s.router.Group("/api/v1")
	{
		// Items endpoints
		items := v1.Group("/items")
		{
			items.GET("", s.listItems)
			items.GET("/:id", s.getItem)
			items.PUT("/:id", s.updateItem)
			items.POST("/search", s.searchItems)
		}

		// Movies endpoints
		movies := v1.Group("/movies")
		{
			movies.GET("", s.listMovies)
			movies.GET("/:id", s.getMovie)
		}

		// TV shows endpoints
		tvshows := v1.Group("/tvshows")
		{
			tvshows.GET("", s.listTVShows)
			tvshows.GET("/:id", s.getTVShow)
		}

		// Filter endpoints
		filters := v1.Group("/filters")
		{
			filters.GET("", s.listFilters)
			filters.POST("", s.createFilter)
			filters.PATCH("/:id", s.updateFilter)
			filters.DELETE("/:id", s.deleteFilter)
			filters.DELETE("/runtime", s.clearRuntimeFilters)
		}

		// Dry-run endpoint
		v1.POST("/dryrun", s.executeDryRun)

		// Statistics endpoint
		v1.GET("/stats", s.getStats)
	}
}
