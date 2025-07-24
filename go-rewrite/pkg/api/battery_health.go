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

	// Get the raw data points
	rawData, err := h.dbManager.GetFleetBatteryHealth(model)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get battery health data: %v", err)})
		return
	}

	// Get the monthly trend data (aggregated)
	trendData, err := h.dbManager.GetMonthlyBatteryHealthTrend(model)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get battery health trend: %v", err)})
		return
	}

	// Get available models for filtering
	availableModels, err := h.dbManager.GetAvailableModels()
	if err != nil {
		// Non-critical error, we can continue
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
