package api

import "github.com/gin-gonic/gin"

// SetupRoutes registers the API routes.
func SetupRoutes(router *gin.Engine) {
	// Ping endpoint for health checks
	router.GET("/ping", PingHandler)

	// Server management endpoints
	router.POST("/servers", StartMinecraftServerHandler)
	router.POST("/servers/:serverName/restart", RestartMinecraftServerHandler)
	router.POST("/servers/:serverName/stop", StopMinecraftServerHandler)
	router.POST("/servers/:serverName/start", StartStoppedServerHandler)
	router.DELETE("/servers/:serverName", DeleteMinecraftServerHandler)
	router.POST("/servers/:serverName/exec", ExecCommandHandler)
}
