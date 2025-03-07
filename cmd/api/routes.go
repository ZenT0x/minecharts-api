package api

import "github.com/gin-gonic/gin"

// SetupRoutes registers the API routes.
func SetupRoutes(router *gin.Engine) {
	router.POST("/pods/:podName/start", StartMinecraftPodHandler)
	router.POST("/pods/:podName/stop", StopMinecraftPodHandler)
	router.DELETE("/pods/:podName", DeleteMinecraftPodHandler)
	router.POST("/pods/:podName/exec", ExecCommandHandler)
}
