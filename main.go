package main

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func main() {
	// Crée une instance du moteur Gin en mode par défaut
	router := gin.Default()

	// Ajoute un endpoint de test pour vérifier que le serveur fonctionne
	router.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "OK")
	})

	// Démarre le serveur sur le port 8080
	router.Run(":8080")
}
