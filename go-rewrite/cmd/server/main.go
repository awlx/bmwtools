package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/awlx/bmwtools/pkg/api"
	"github.com/awlx/bmwtools/pkg/data"
	"github.com/awlx/bmwtools/pkg/database"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
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

	// Initialize the database manager
	// Create data directory if it doesn't exist
	dataDir := filepath.Join(".", "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	dbPath := filepath.Join(dataDir, "bmwtools.db")
	dbManager, err := database.New(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer dbManager.Close()

	// Create the API handler
	apiHandler := api.NewHandler(dataManager, dbManager)

	// Set up routes
	// API routes
	r.POST("/api/upload", apiHandler.UploadJSON)
	r.GET("/api/demo", apiHandler.LoadDemoData)
	r.GET("/api/sessions", apiHandler.GetSessions)
	r.GET("/api/stats", apiHandler.GetStats)
	r.GET("/api/session/:id", apiHandler.GetSession)
	r.GET("/api/map", apiHandler.GetMapData)
	r.GET("/api/grouped-providers", apiHandler.GetGroupedProviders)
	r.GET("/api/version", apiHandler.GetVersion)
	r.GET("/api/anonymous-stats", apiHandler.GetAnonymousStats)
	r.GET("/api/battery-health", apiHandler.GetBatteryHealth)

	// Create a custom static file handler with cache control headers
	staticHandler := func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		c.Next()
	}

	// Static file serving for the frontend with cache control
	r.Group("/static").Use(staticHandler).StaticFS("", http.Dir("./static"))
	r.StaticFile("/", "./static/index.html")
	r.StaticFile("/stats", "./static/stats.html")
	r.StaticFile("/battery", "./static/battery.html")

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
