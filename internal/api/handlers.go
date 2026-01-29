package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/glefebvre/stalkeer/internal/database"
)

func (s *Server) healthCheck(c *gin.Context) {
	if err := database.HealthCheck(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unhealthy",
			"error":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
	})
}

func (s *Server) listItems(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"processed_lines": []interface{}{},
	})
}

func (s *Server) getItem(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{
		"id": id,
	})
}

func (s *Server) listFilters(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"movies":  []interface{}{},
		"tvshows": []interface{}{},
	})
}

func (s *Server) createFilter(c *gin.Context) {
	c.JSON(http.StatusCreated, gin.H{
		"message": "filter created",
	})
}

func (s *Server) listLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"logs": []interface{}{},
	})
}
