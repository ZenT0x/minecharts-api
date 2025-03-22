package api

import (
	"minecharts/cmd/api/handlers"

	"github.com/gin-gonic/gin"
)

// SetupRoutes registers the API routes.
func SetupRoutes(router *gin.Engine) {
	// Ping endpoint for health checks
	router.GET("/ping", handlers.PingHandler)

	// Server management endpoints
	router.POST("/servers", handlers.StartMinecraftServerHandler)
	router.POST("/servers/:serverName/restart", handlers.RestartMinecraftServerHandler)
	router.POST("/servers/:serverName/stop", handlers.StopMinecraftServerHandler)
	router.POST("/servers/:serverName/start", handlers.StartStoppedServerHandler)
	router.POST("/servers/:serverName/delete", handlers.DeleteMinecraftServerHandler)
	router.POST("/servers/:serverName/exec", handlers.ExecCommandHandler)

	// Network exposure endpoint
	router.POST("/servers/:serverName/expose", handlers.ExposeMinecraftServerHandler)
}
