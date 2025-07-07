package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/awlx/bmwtools/pkg/api"
	"github.com/awlx/bmwtools/pkg/data"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	// Set up the Gin router
	r := gin.Default()

	// Configure CORS
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization"}
	r.Use(cors.New(config))

	// Initialize the data manager
	dataManager := data.NewManager()

	// Create the API handler
	apiHandler := api.NewHandler(dataManager)

	// Set up routes
	// API routes
	r.POST("/api/upload", apiHandler.UploadJSON)
	r.GET("/api/demo", apiHandler.LoadDemoData)
	r.GET("/api/sessions", apiHandler.GetSessions)
	r.GET("/api/stats", apiHandler.GetStats)
	r.GET("/api/session/:id", apiHandler.GetSession)
	r.GET("/api/map", apiHandler.GetMapData)
	r.GET("/api/grouped-providers", apiHandler.GetGroupedProviders)

	// Static file serving for the frontend
	r.StaticFS("/static", http.Dir("./static"))
	r.StaticFile("/", "./static/index.html")

	// Add a catch-all route for SPA
	r.NoRoute(func(c *gin.Context) {
		c.File("./static/index.html")
	})

	// Determine the port
	port := os.Getenv("PORT")
	if port == "" {
		port = "8050" // Default port to match the Python version
	}

	// Start the server
	log.Printf("Server starting on port %s...", port)
	if err := r.Run(fmt.Sprintf(":%s", port)); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
