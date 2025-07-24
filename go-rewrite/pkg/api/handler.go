package api

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/awlx/bmwtools/pkg/data"
	"github.com/awlx/bmwtools/pkg/database"
	"github.com/gin-gonic/gin"
)

// AppVersion is the current version of the application
const AppVersion = "1.0.1"

// Handler handles API requests
type Handler struct {
	dataManager *data.Manager
	dbManager   *database.Manager
}

// NewHandler creates a new API handler
func NewHandler(dataManager *data.Manager, dbManager *database.Manager) *Handler {
	return &Handler{
		dataManager: dataManager,
		dbManager:   dbManager,
	}
}

// GetVersion returns the current application version
func (h *Handler) GetVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"version": AppVersion,
	})
}

// GetAnonymousStats returns anonymous statistics about charging sessions
func (h *Handler) GetAnonymousStats(c *gin.Context) {
	// Check if we have a database manager
	if h.dbManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database not initialized"})
		return
	}

	// Get provider stats from database
	providerStats, err := h.dbManager.GetProviderStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error fetching provider stats: %v", err)})
		return
	}

	// Get SOC statistics from database
	socStats, err := h.dbManager.GetSOCStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error fetching SOC stats: %v", err)})
		return
	}

	// Calculate global statistics
	var totalSessions, totalSuccessful, totalFailed int
	var totalEnergyAdded float64

	for _, provider := range providerStats {
		totalSessions += provider["total_sessions"].(int)
		totalSuccessful += provider["successful_sessions"].(int)
		totalFailed += provider["failed_sessions"].(int)
		totalEnergyAdded += provider["total_energy_added"].(float64)
	}

	// Calculate global success rate
	globalSuccessRate := 0.0
	if totalSessions > 0 {
		globalSuccessRate = float64(totalSuccessful) / float64(totalSessions) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"providers": providerStats,
		"global_stats": map[string]interface{}{
			"total_sessions":      totalSessions,
			"successful_sessions": totalSuccessful,
			"failed_sessions":     totalFailed,
			"success_rate":        globalSuccessRate,
			"total_energy_added":  totalEnergyAdded,
		},
		"soc_stats": socStats,
	})
}

// UploadJSON handles JSON file uploads
func (h *Handler) UploadJSON(c *gin.Context) {
	// Get the file from the request
	file, fileHeader, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}
	defer file.Close()

	// Process the file with the data manager
	err = h.dataManager.LoadJSON(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Error processing JSON: %v", err)})
		return
	}

	// Get the processed sessions
	sessions := h.dataManager.GetSessions()

	// Check for consent to store in fleet statistics database
	consentParam := c.PostForm("consent")
	hasConsent := consentParam == "true" || consentParam == "1" || consentParam == "yes"

	// Get the BMW model if provided
	model := c.PostForm("model")

	// Store sessions in the database if consent was given
	isNew := false
	var storedCount int
	if h.dbManager != nil && hasConsent {
		// Validate model parameter is provided when consent is given
		if model == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "BMW model selection is required when sharing data"})
			return
		}

		var err error
		isNew, err = h.dbManager.StoreSessions(sessions, fileHeader.Filename, model)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error storing sessions in database: %v", err)})
			return
		}

		// Query the database to get an accurate count of stored sessions
		// This ensures that the count shown in the UI matches what's actually in the database
		var countErr error
		storedCount, countErr = h.dbManager.GetSessionCount()
		if countErr != nil {
			log.Printf("Warning: Unable to get session count: %v", countErr)
			storedCount = len(sessions) // Fallback to local count if query fails
		}

		// If this was a duplicate upload, notify the user
		if !isNew {
			c.JSON(http.StatusOK, gin.H{
				"message":      "This file has already been processed previously",
				"count":        len(sessions),
				"new":          false,
				"stored":       hasConsent,
				"stored_count": storedCount,
			})
			return
		}
	} // Return success with appropriate message based on consent
	message := "File uploaded and processed successfully"
	if !hasConsent {
		message = "File uploaded and processed (not stored in fleet statistics)"
		storedCount = 0
	}
	c.JSON(http.StatusOK, gin.H{
		"message":      message,
		"count":        len(sessions),
		"new":          true,
		"stored":       hasConsent,
		"stored_count": storedCount,
	})
}

// LoadDemoData loads demo data from a file
func (h *Handler) LoadDemoData(c *gin.Context) {
	// Open the demo file
	file, err := os.Open("FINAL_DEMO_CHARGING_DATA_SMOOTH_CURVES.JSON")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not load demo data"})
		return
	}
	defer file.Close()

	// Process the file
	err = h.dataManager.LoadJSON(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error processing demo data: %v", err)})
		return
	}

	// Get the processed sessions
	sessions := h.dataManager.GetSessions()

	// Demo data is intentionally NOT stored in the database
	// This is because demo data is just for testing/preview and shouldn't
	// pollute the actual fleet statistics

	// Return success
	c.JSON(http.StatusOK, gin.H{
		"message": "Demo data loaded successfully (not stored in database)",
		"count":   len(sessions),
		"demo":    true,
	})
}

// GetSessions returns all sessions or filtered by date range
func (h *Handler) GetSessions(c *gin.Context) {
	var sessions []data.Session

	// Get date range parameters
	startDateStr := c.Query("startDate")
	endDateStr := c.Query("endDate")

	if startDateStr != "" && endDateStr != "" {
		startDate, err1 := time.Parse("2006-01-02", startDateStr)
		endDate, err2 := time.Parse("2006-01-02", endDateStr)

		if err1 == nil && err2 == nil {
			// Add a day to endDate to make it inclusive
			endDate = endDate.AddDate(0, 0, 1)
			sessions = h.dataManager.GetSessionsByDateRange(startDate, endDate)
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format"})
			return
		}
	} else {
		sessions = h.dataManager.GetSessions()
	}

	// Convert to a frontend-friendly format
	result := make([]map[string]interface{}, 0, len(sessions))
	for i, s := range sessions {
		result = append(result, map[string]interface{}{
			"id":                     s.ID,
			"index":                  i,
			"start_time":             s.StartTime.Format(time.RFC3339),
			"end_time":               s.EndTime.Format(time.RFC3339),
			"soc_start":              s.SocStart,
			"soc_end":                s.SocEnd,
			"energy_from_grid":       s.EnergyFromGrid,
			"energy_added_hvb":       s.EnergyAddedHvb,
			"cost":                   s.Cost,
			"efficiency":             s.Efficiency,
			"location":               s.Location,
			"latitude":               s.Latitude,
			"longitude":              s.Longitude,
			"avg_power":              s.AvgPower,
			"grid_power_start":       s.GridPowerStart,
			"mileage":                s.Mileage,
			"session_time_minutes":   s.SessionTimeMinutes,
			"provider":               s.Provider,
			"using_estimated_energy": s.UsingEstimatedEnergy,
			"label":                  fmt.Sprintf("%s - %s", s.StartTime.Format("2006-01-02 15:04"), s.Location),
		})
	}

	c.JSON(http.StatusOK, result)
}

// GetSession returns a single session by ID
func (h *Handler) GetSession(c *gin.Context) {
	id := c.Param("id")

	session, found := h.dataManager.GetSessionByID(id)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	c.JSON(http.StatusOK, session)
}

// GetStats returns various statistics
func (h *Handler) GetStats(c *gin.Context) {
	// Get date range parameters (same as in GetSessions)
	startDateStr := c.Query("startDate")
	endDateStr := c.Query("endDate")

	var sessions []data.Session
	var startDate, endDate time.Time
	var dateFilterActive bool

	if startDateStr != "" && endDateStr != "" {
		var err1, err2 error
		startDate, err1 = time.Parse("2006-01-02", startDateStr)
		endDate, err2 = time.Parse("2006-01-02", endDateStr)

		if err1 == nil && err2 == nil {
			// Add a day to endDate to make it inclusive
			endDate = endDate.AddDate(0, 0, 1)
			sessions = h.dataManager.GetSessionsByDateRange(startDate, endDate)
			dateFilterActive = true
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format"})
			return
		}
	} else {
		sessions = h.dataManager.GetSessions()
	}

	// Calculate statistics using filtered sessions if filter is active
	var overallEfficiency, powerConsumption, powerConsumptionWithoutGridLosses float64
	var socStats map[string]interface{}
	var sessionStats map[string]interface{}
	var estimatedCapacity []map[string]interface{}

	if dateFilterActive {
		// Create a temporary data manager with only the filtered sessions
		tempManager := data.NewManager()
		tempManager.SetSessions(sessions)

		// Calculate stats using the filtered data
		overallEfficiency, powerConsumption, powerConsumptionWithoutGridLosses = tempManager.CalculateOverallStats()
		socStats = tempManager.CalculateSOCStatistics()
		sessionStats = tempManager.GetSessionStats()
		estimatedCapacity = tempManager.CalculateEstimatedBatteryCapacity()
	} else {
		// Use all data
		overallEfficiency, powerConsumption, powerConsumptionWithoutGridLosses = h.dataManager.CalculateOverallStats()
		socStats = h.dataManager.CalculateSOCStatistics()
		sessionStats = h.dataManager.GetSessionStats()
		estimatedCapacity = h.dataManager.CalculateEstimatedBatteryCapacity()
	}

	// Combine everything into one response
	stats := map[string]interface{}{
		"overall_efficiency":               overallEfficiency,
		"power_consumption_per_100km":      powerConsumption,
		"power_consumption_without_losses": powerConsumptionWithoutGridLosses,
		"soc_stats":                        socStats,
		"session_stats":                    sessionStats,
		"estimated_capacity":               estimatedCapacity,
		"using_estimated_values":           h.dataManager.IsUsingEstimatedValues(),
		"date_filter_active":               dateFilterActive,
	}

	c.JSON(http.StatusOK, stats)
}

// GetMapData returns data for the map
func (h *Handler) GetMapData(c *gin.Context) {
	// Get date range parameters (same as in GetSessions and GetStats)
	startDateStr := c.Query("startDate")
	endDateStr := c.Query("endDate")

	var sessions []data.Session

	if startDateStr != "" && endDateStr != "" {
		startDate, err1 := time.Parse("2006-01-02", startDateStr)
		endDate, err2 := time.Parse("2006-01-02", endDateStr)

		if err1 == nil && err2 == nil {
			// Add a day to endDate to make it inclusive
			endDate = endDate.AddDate(0, 0, 1)
			sessions = h.dataManager.GetSessionsByDateRange(startDate, endDate)
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format"})
			return
		}
	} else {
		sessions = h.dataManager.GetSessions()
	}

	// Group sessions by location to count occurrences
	type locationStats struct {
		latitude      float64
		longitude     float64
		name          string
		provider      string
		sessionCount  int
		failedCount   int
		successCount  int
		totalEnergy   float64
		lastSession   time.Time
		lastSessionID string
	}

	locationMap := make(map[string]*locationStats)

	for _, s := range sessions {
		// Skip if no location data
		if s.Latitude == 0 && s.Longitude == 0 {
			continue
		}

		// Create a key for the location based on coordinates (rounded to avoid floating point issues)
		locationKey := fmt.Sprintf("%.5f:%.5f", s.Latitude, s.Longitude)

		isFailedSession := s.SocEnd == s.SocStart

		if _, exists := locationMap[locationKey]; !exists {
			locationMap[locationKey] = &locationStats{
				latitude:      s.Latitude,
				longitude:     s.Longitude,
				name:          s.Location,
				provider:      s.Provider,
				lastSession:   s.StartTime,
				lastSessionID: s.ID,
			}
		}

		// Update stats for this location
		loc := locationMap[locationKey]
		loc.sessionCount++
		loc.totalEnergy += s.EnergyAddedHvb

		if isFailedSession {
			loc.failedCount++
		} else {
			loc.successCount++
		}

		// Keep track of the most recent session at this location
		if s.StartTime.After(loc.lastSession) {
			loc.lastSession = s.StartTime
			loc.lastSessionID = s.ID
			loc.provider = s.Provider // Use the provider from the most recent session
		}
	}

	// Transform the grouped data into map-friendly format
	locations := make([]map[string]interface{}, 0, len(locationMap))

	for _, loc := range locationMap {
		locations = append(locations, map[string]interface{}{
			"latitude":      loc.latitude,
			"longitude":     loc.longitude,
			"name":          loc.name,
			"provider":      loc.provider,
			"session_count": loc.sessionCount,
			"failed_count":  loc.failedCount,
			"success_count": loc.successCount,
			"total_energy":  loc.totalEnergy,
			"session_id":    loc.lastSessionID,
			"energy_added":  loc.totalEnergy,
		})
	}

	c.JSON(http.StatusOK, locations)
}

// GetGroupedProviders returns statistics grouped by provider with similar names merged
func (h *Handler) GetGroupedProviders(c *gin.Context) {
	// Get date range parameters
	startDateStr := c.Query("startDate")
	endDateStr := c.Query("endDate")

	var tempManager *data.Manager

	if startDateStr != "" && endDateStr != "" {
		startDate, err1 := time.Parse("2006-01-02", startDateStr)
		endDate, err2 := time.Parse("2006-01-02", endDateStr)

		if err1 == nil && err2 == nil {
			// Add a day to endDate to make it inclusive
			endDate = endDate.AddDate(0, 0, 1)
			sessions := h.dataManager.GetSessionsByDateRange(startDate, endDate)

			// Create a temporary manager with filtered sessions
			tempManager = data.NewManager()
			tempManager.SetSessions(sessions)
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format"})
			return
		}
	} else {
		// Use the main manager
		tempManager = h.dataManager
	}

	// Get grouped provider statistics
	groupedProviders := tempManager.GroupProviders()

	// Return the data, ensuring all fields are properly handled
	response := map[string]interface{}{
		"grouped_successful_providers": groupedProviders["grouped_successful_providers"],
		"grouped_failed_providers":     groupedProviders["grouped_failed_providers"],
		"all_providers":                groupedProviders["all_providers"],
		"unknown_providers_debug":      groupedProviders["unknown_providers_debug"],
	}

	c.JSON(http.StatusOK, response)
}

// End of handler functions
