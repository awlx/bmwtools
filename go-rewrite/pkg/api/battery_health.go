package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetBatteryHealth returns battery health data for the entire fleet or filtered by model
func (h *Handler) GetBatteryHealth(c *gin.Context) {
	// Get optional model filter
	model := c.Query("model")

	// Check if database is initialized
	if h.dbManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database manager is not initialized"})
		return
	}

	// Return empty data if battery_health table doesn't exist or has no data
	// This prevents 500 errors when there's no battery health data yet
	rawData := []map[string]interface{}{}
	trendData := []map[string]interface{}{}
	availableModels := []string{}

	// Get the raw data points - with error handling
	rawData, err := h.dbManager.GetFleetBatteryHealth(model)
	if err != nil {
		// Log the error but don't fail
		fmt.Printf("Error getting battery health data: %v\n", err)
		// Return empty data instead of error
		c.JSON(http.StatusOK, gin.H{
			"battery_health_data": []map[string]interface{}{},
			"available_models":    []string{},
			"model_filter":        model,
			"error":               fmt.Sprintf("Failed to get battery health data: %v", err),
		})
		return
	}

	// Get the monthly trend data (aggregated) - with error handling
	trendData, err = h.dbManager.GetMonthlyBatteryHealthTrend(model)
	if err != nil {
		// Log the error but don't fail
		fmt.Printf("Error getting battery health trend: %v\n", err)
		// We can continue with just the raw data
		trendData = []map[string]interface{}{}
	}

	// Get available models for filtering
	availableModels, err = h.dbManager.GetAvailableModels()
	if err != nil {
		// Non-critical error, we can continue
		fmt.Printf("Error getting available models: %v\n", err)
		availableModels = []string{}
	}

	// Merge raw data and trend data
	allData := append(rawData, trendData...)

	// Return the data
	c.JSON(http.StatusOK, gin.H{
		"battery_health_data": allData,
		"available_models":    availableModels,
		"model_filter":        model,
	})
}
