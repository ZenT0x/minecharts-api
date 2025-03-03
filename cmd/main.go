package main

import (
	"log"

	"minecharts/cmd/api"
	"minecharts/cmd/kubernetes"

	"github.com/gin-gonic/gin"
)

func main() {
	// Initialize the global Kubernetes clientset from the kubernetes package.
	if err := kubernetes.Init(); err != nil {
		log.Fatalf("Failed to initialize Kubernetes client: %v", err)
	}

	// Create a new Gin router.
	router := gin.Default()

	// Setup API routes.
	api.SetupRoutes(router)

	// Run the API server on port 8080.
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}
}
