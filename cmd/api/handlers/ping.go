package handlers

import "github.com/gin-gonic/gin"

// PingHandler returns a simple "pong" message to confirm the API is online.
func PingHandler(c *gin.Context) {
	c.JSON(200, gin.H{"message": "pong"})
}
