package main

import (
	"log"

	"minecharts/cmd/api"
	"minecharts/cmd/config"
	"minecharts/cmd/database"
	_ "minecharts/cmd/docs" // Import swagger docs
	"minecharts/cmd/kubernetes"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title           Minecharts API
// @version         0.1
// @description     API for managing Minecraft servers in Kubernetes
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.minecharts.io/support
// @contact.email  support@minecharts.io

// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

// @securityDefinitions.apikey APIKeyAuth
// @in header
// @name X-API-Key
// @description API Key for authentication.

func main() {
	// Initialize the global Kubernetes clientset from the kubernetes package.
	if err := kubernetes.Init(); err != nil {
		log.Fatalf("Failed to initialize Kubernetes client: %v", err)
	}

	// Initialize the database
	if err := database.InitDB(config.DatabaseType, config.DatabaseConnectionString); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.GetDB().Close()

	// Create a new Gin router.
	router := gin.Default()

	// Setup API routes.
	api.SetupRoutes(router)

	// Setup Swagger documentation
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Run the API server on port 8080.
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}
}
