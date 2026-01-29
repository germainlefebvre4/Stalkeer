package api

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// requestIDMiddleware adds a unique request ID to each request
func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// errorHandlerMiddleware handles panics and errors
func errorHandlerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				c.JSON(500, ErrorResponse{
					Error:   "internal server error",
					Message: "an unexpected error occurred",
				})
				c.Abort()
			}
		}()
		c.Next()
	}
}
