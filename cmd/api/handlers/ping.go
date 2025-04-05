package handlers

import (
	"minecharts/cmd/logging"

	"github.com/gin-gonic/gin"
)

// PingHandler returns a simple "pong" message to confirm the API is online.
//
// @Summary      Ping API
// @Description  Health check endpoint that returns a pong message
// @Tags         system
// @Produce      json
// @Success      200  {object}  map[string]string  "Pong response"
// @Router       /ping [get]
func PingHandler(c *gin.Context) {
	logging.API.WithFields("remote_ip", c.ClientIP()).Info("Ping request received")
	c.JSON(200, gin.H{"message": "pong"})
}
