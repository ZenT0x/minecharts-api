package api

import "github.com/gin-gonic/gin"

// SetupRoutes registers the API routes.
func SetupRoutes(router *gin.Engine) {
	router.GET("/ping", PingHandler)
	router.GET("/pods", ListPodsHandler)
	router.POST("/pods", CreateMinecraftPodHandler)
	router.DELETE("/pods/:podName", DeleteMinecraftPodHandler)
	router.POST("/pods/:podName/stop", StopMinecraftPodHandler)
}
