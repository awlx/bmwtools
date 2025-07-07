package data

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"time"
)

// Manager handles all data operations
type Manager struct {
	sessions []Session
}

// NewManager creates a new data manager
func NewManager() *Manager {
	return &Manager{
		sessions: []Session{},
	}
}

// SetSessions sets the sessions data to the provided sessions
func (m *Manager) SetSessions(sessions []Session) {
	m.sessions = sessions
}

// Session represents a charging session
type Session struct {
	ID                   string    `json:"id"`
	StartTime            time.Time `json:"start_time"`
	EndTime              time.Time `json:"end_time"`
	SocStart             float64   `json:"soc_start"`
	SocEnd               float64   `json:"soc_end"`
	EnergyFromGrid       float64   `json:"energy_from_grid"`
	EnergyAddedHvb       float64   `json:"energy_added_hvb"`
	Cost                 float64   `json:"cost"`
	Efficiency           float64   `json:"efficiency"`
	Location             string    `json:"location"`
	Latitude             float64   `json:"latitude"`
	Longitude            float64   `json:"longitude"`
	AvgPower             float64   `json:"avg_power"`
	GridPowerStart       []float64 `json:"grid_power_start"`
	Mileage              float64   `json:"mileage"`
	SessionTimeMinutes   float64   `json:"session_time_minutes"`
	Provider             string    `json:"provider"`
	UsingEstimatedEnergy bool      `json:"using_estimated_energy"`
}

// RawSession is the raw BMW CarData format
type RawSession struct {
	StartTime                      int64   `json:"startTime"`
	EndTime                        int64   `json:"endTime"`
	DisplayedStartSoc              float64 `json:"displayedStartSoc"`
	DisplayedSoc                   float64 `json:"displayedSoc"`
	EnergyConsumedFromPowerGridKwh float64 `json:"energyConsumedFromPowerGridKwh"`
	EnergyIncreaseHvbKwh           float64 `json:"energyIncreaseHvbKwh,omitempty"`
	ChargingCostInformation        struct {
		CalculatedChargingCost float64 `json:"calculatedChargingCost"`
	} `json:"chargingCostInformation,omitempty"`
	ChargingLocation struct {
		FormattedAddress    string  `json:"formattedAddress"`
		MapMatchedLatitude  float64 `json:"mapMatchedLatitude"`
		MapMatchedLongitude float64 `json:"mapMatchedLongitude"`
	} `json:"chargingLocation"`
	ChargingBlocks []struct {
		AveragePowerGridKw float64 `json:"averagePowerGridKw"`
	} `json:"chargingBlocks"`
	Mileage             int `json:"mileage"`
	PublicChargingPoint struct {
		PotentialChargingPointMatches []struct {
			ProviderName string `json:"providerName"`
		} `json:"potentialChargingPointMatches"`
	} `json:"publicChargingPoint"`
}

// LoadJSONFromFile loads BMW CarData JSON from a file
func (m *Manager) LoadJSONFromFile(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	return m.LoadJSON(file)
}

// LoadJSON loads BMW CarData JSON from a reader
func (m *Manager) LoadJSON(r io.Reader) error {
	var rawSessions []RawSession

	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&rawSessions); err != nil {
		return err
	}

	return m.ProcessRawSessions(rawSessions)
}

// ProcessRawSessions processes raw BMW CarData sessions
func (m *Manager) ProcessRawSessions(rawSessions []RawSession) error {
	sessions := make([]Session, 0, len(rawSessions))
	// We'll track this through the IsUsingEstimatedValues() method now

	for _, raw := range rawSessions {
		startTime := time.Unix(raw.StartTime, 0)
		endTime := time.Unix(raw.EndTime, 0)
		socStart := raw.DisplayedStartSoc
		socEnd := raw.DisplayedSoc
		energyFromGrid := raw.EnergyConsumedFromPowerGridKwh
		cost := raw.ChargingCostInformation.CalculatedChargingCost

		// Check if energyIncreaseHvbKwh exists in the data
		energyIncreaseHvb := raw.EnergyIncreaseHvbKwh
		if energyIncreaseHvb == 0 {
			// Determine if it's DC or AC charging based on average power
			// Calculate average power first
			var sumPower float64
			for _, block := range raw.ChargingBlocks {
				sumPower += block.AveragePowerGridKw
			}
			avgPower := sumPower / math.Max(float64(len(raw.ChargingBlocks)), 1)

			// Use different efficiency estimates based on charging type
			if avgPower >= 12 { // DC charging (typically >= 12kW)
				energyIncreaseHvb = energyFromGrid * 0.98 // 98% efficiency for DC
			} else { // AC charging
				energyIncreaseHvb = energyFromGrid * 0.92 // 92% efficiency for AC
			}
			// This session will be marked as using estimated values
		}

		efficiency := 0.0
		if energyFromGrid > 0 {
			efficiency = energyIncreaseHvb / energyFromGrid
		}

		location := raw.ChargingLocation.FormattedAddress
		if location == "" {
			location = "Unknown Location"
		}

		latitude := raw.ChargingLocation.MapMatchedLatitude
		longitude := raw.ChargingLocation.MapMatchedLongitude

		var sumPower float64
		gridPowerStart := make([]float64, len(raw.ChargingBlocks))
		for i, block := range raw.ChargingBlocks {
			sumPower += block.AveragePowerGridKw
			gridPowerStart[i] = block.AveragePowerGridKw
		}
		avgPower := sumPower / math.Max(float64(len(raw.ChargingBlocks)), 1)

		mileage := float64(raw.Mileage)
		sessionTimeMinutes := endTime.Sub(startTime).Minutes()

		provider := "Unknown"
		if len(raw.PublicChargingPoint.PotentialChargingPointMatches) > 0 {
			provider = raw.PublicChargingPoint.PotentialChargingPointMatches[0].ProviderName
		}

		session := Session{
			ID:                   fmt.Sprintf("%d", raw.StartTime),
			StartTime:            startTime,
			EndTime:              endTime,
			SocStart:             socStart,
			SocEnd:               socEnd,
			EnergyFromGrid:       energyFromGrid,
			EnergyAddedHvb:       energyIncreaseHvb,
			Cost:                 cost,
			Efficiency:           efficiency,
			Location:             location,
			Latitude:             latitude,
			Longitude:            longitude,
			AvgPower:             avgPower,
			GridPowerStart:       gridPowerStart,
			Mileage:              mileage,
			SessionTimeMinutes:   sessionTimeMinutes,
			Provider:             provider,
			UsingEstimatedEnergy: energyIncreaseHvb != raw.EnergyIncreaseHvbKwh,
		}

		sessions = append(sessions, session)

		// Note: Each session already tracks if it's using estimated energy values
		// via the UsingEstimatedEnergy field, which is used by IsUsingEstimatedValues()
	}

	m.sessions = sessions
	return nil
}

// GetSessions returns all sessions
func (m *Manager) GetSessions() []Session {
	return m.sessions
}

// GetSessionByID returns a session by its ID
func (m *Manager) GetSessionByID(id string) (Session, bool) {
	for _, session := range m.sessions {
		if session.ID == id {
			return session, true
		}
	}
	return Session{}, false
}

// GetSessionsByDateRange returns sessions within a given date range
func (m *Manager) GetSessionsByDateRange(startDate, endDate time.Time) []Session {
	if startDate.IsZero() || endDate.IsZero() {
		return m.sessions
	}

	var filtered []Session
	for _, session := range m.sessions {
		if (session.StartTime.Equal(startDate) || session.StartTime.After(startDate)) &&
			(session.StartTime.Equal(endDate) || session.StartTime.Before(endDate)) {
			filtered = append(filtered, session)
		}
	}

	return filtered
}

// CalculateEstimatedBatteryCapacity calculates estimated battery capacity (SoH)
func (m *Manager) CalculateEstimatedBatteryCapacity() []map[string]interface{} {
	estimatedBatteryCapacity := make([]map[string]interface{}, 0)

	// First, collect all individual capacity calculations
	type capacityDataPoint struct {
		date           time.Time
		timestamp      float64 // Unix timestamp in days since epoch
		capacity       float64
		socChange      float64
		energyAddedHvb float64
		yearMonth      string // Year-Month format for bucketing
	}

	var dataPoints []capacityDataPoint
	monthlyBuckets := make(map[string][]capacityDataPoint)

	// First pass: collect all valid data points
	for _, session := range m.sessions {
		if session.EnergyAddedHvb >= 30 {
			socChange := session.SocEnd - session.SocStart
			if socChange >= 20 { // Significant change for more reliable calculations
				estimatedCapacity := (session.EnergyAddedHvb * 100) / socChange
				// Convert time to days since epoch for linear regression
				daysSinceEpoch := float64(session.StartTime.Unix()) / (60 * 60 * 24)
				// Format year-month for bucketing (e.g., "2023-01")
				yearMonth := session.StartTime.Format("2006-01")

				dp := capacityDataPoint{
					date:           session.StartTime,
					timestamp:      daysSinceEpoch,
					capacity:       estimatedCapacity,
					socChange:      socChange,
					energyAddedHvb: session.EnergyAddedHvb,
					yearMonth:      yearMonth,
				}

				dataPoints = append(dataPoints, dp)
				monthlyBuckets[yearMonth] = append(monthlyBuckets[yearMonth], dp)
			}
		}
	}

	// Calculate monthly averages first
	type monthlyAverage struct {
		yearMonth    string
		date         time.Time // middle of the month as representative date
		avgCapacity  float64
		sumSocChange float64
		count        int
		avgTimestamp float64
	}

	var monthlyAverages []monthlyAverage

	// Calculate average for each month
	for yearMonth, points := range monthlyBuckets {
		var sumCapacity, sumTimestamp, sumSocChange float64
		for _, dp := range points {
			// Weight by SoC change for more accurate averages
			weight := dp.socChange
			sumCapacity += dp.capacity * weight
			sumTimestamp += dp.timestamp * weight
			sumSocChange += weight
		}

		// Calculate weighted average for the month
		avgCapacity := sumCapacity / sumSocChange
		avgTimestamp := sumTimestamp / sumSocChange

		// Find a representative date for the middle of the month
		representativeDate := points[0].date // Default to first date
		if len(points) > 1 {
			// Try to find a point near the middle of the collection
			representativeDate = points[len(points)/2].date
		}

		monthlyAverages = append(monthlyAverages, monthlyAverage{
			yearMonth:    yearMonth,
			date:         representativeDate,
			avgCapacity:  avgCapacity,
			sumSocChange: sumSocChange,
			count:        len(points),
			avgTimestamp: avgTimestamp,
		})
	}

	// If we have less than 2 months, we can't calculate a meaningful trend
	if len(monthlyAverages) < 2 {
		// Just return the raw points if we have any
		for _, dp := range dataPoints {
			estimatedBatteryCapacity = append(estimatedBatteryCapacity, map[string]interface{}{
				"date":                       dp.date,
				"estimated_battery_capacity": dp.capacity,
				"raw_capacity":               dp.capacity,
				"soc_change":                 dp.socChange,
				"trend":                      dp.capacity, // Same as raw since no trend can be calculated
			})
		}
		return estimatedBatteryCapacity
	}

	// Calculate linear regression using monthly averages
	// y = mx + b where y is capacity, x is time
	var sumX, sumY, sumXY, sumXX float64
	var totalWeight float64

	// Apply weights based on total SoC change for the month
	for _, ma := range monthlyAverages {
		weight := ma.sumSocChange / 100.0 // Normalize weight
		totalWeight += weight
		sumX += ma.avgTimestamp * weight
		sumY += ma.avgCapacity * weight
		sumXY += ma.avgTimestamp * ma.avgCapacity * weight
		sumXX += ma.avgTimestamp * ma.avgTimestamp * weight
	}

	// Normalize by total weight
	sumX /= totalWeight
	sumY /= totalWeight
	sumXY /= totalWeight
	sumXX /= totalWeight

	// Calculate slope and intercept
	var slope, intercept float64
	if sumXX-sumX*sumX != 0 {
		slope = (sumXY - sumX*sumY) / (sumXX - sumX*sumX)
		intercept = sumY - slope*sumX
	} else {
		// If division by zero would occur, use average
		slope = 0
		intercept = sumY
	}

	// First, include all raw data points for context
	for _, dp := range dataPoints {
		// Calculate trend value based on linear equation
		trendCapacity := slope*dp.timestamp + intercept

		estimatedBatteryCapacity = append(estimatedBatteryCapacity, map[string]interface{}{
			"date":                       dp.date,
			"estimated_battery_capacity": dp.capacity, // Use the raw capacity for individual points
			"raw_capacity":               dp.capacity, // Same as estimated_battery_capacity for individual points
			"soc_change":                 dp.socChange,
			"trend":                      trendCapacity, // Trend line value based on regression
		})
	}

	// Then, add monthly averages for the trend line
	for _, ma := range monthlyAverages {
		// Calculate trend value based on linear equation
		trendCapacity := slope*ma.avgTimestamp + intercept

		estimatedBatteryCapacity = append(estimatedBatteryCapacity, map[string]interface{}{
			"date":                       ma.date,
			"estimated_battery_capacity": trendCapacity,   // Use the linear regression value
			"raw_capacity":               ma.avgCapacity,  // Show the monthly average
			"soc_change":                 ma.sumSocChange, // Total SoC change for the month
			"trend":                      trendCapacity,   // Same as estimated_battery_capacity
			"is_monthly_average":         true,
			"month":                      ma.yearMonth,
			"count":                      ma.count,
		})
	}

	// Sort by date to ensure chronological order
	sortByDate(estimatedBatteryCapacity)

	return estimatedBatteryCapacity
}

// sortByDate sorts the capacity data by date
func sortByDate(data []map[string]interface{}) {
	// Simple insertion sort since the data is likely nearly sorted already
	for i := 1; i < len(data); i++ {
		j := i
		for j > 0 {
			date1 := data[j-1]["date"].(time.Time)
			date2 := data[j]["date"].(time.Time)

			if date1.After(date2) {
				// Swap
				data[j], data[j-1] = data[j-1], data[j]
				j--
			} else {
				break
			}
		}
	}
}

// CalculateOverallStats calculates overall efficiency and power consumption
func (m *Manager) CalculateOverallStats() (float64, float64, float64) {
	var totalEnergyAdded, totalEnergyFromGrid float64
	minMileage := math.MaxFloat64
	maxMileage := 0.0

	if len(m.sessions) == 0 {
		return 0, 0, 0
	}

	// Find min and max mileage and calculate totals
	for _, s := range m.sessions {
		totalEnergyAdded += s.EnergyAddedHvb
		totalEnergyFromGrid += s.EnergyFromGrid

		if s.Mileage < minMileage {
			minMileage = s.Mileage
		}
		if s.Mileage > maxMileage {
			maxMileage = s.Mileage
		}
	}

	// Calculate overall efficiency and power consumption
	overallEfficiency := 0.0
	if totalEnergyFromGrid > 0 {
		overallEfficiency = totalEnergyAdded / totalEnergyFromGrid
	}

	totalDistance := maxMileage - minMileage
	powerConsumptionPer100km := 0.0
	powerConsumptionPer100kmWithoutGridLosses := 0.0

	if totalDistance > 0 {
		powerConsumptionPer100km = (totalEnergyFromGrid / totalDistance) * 100
		powerConsumptionPer100kmWithoutGridLosses = (totalEnergyAdded / totalDistance) * 100
	}

	return overallEfficiency, powerConsumptionPer100km, powerConsumptionPer100kmWithoutGridLosses
}

// CalculateSOCStatistics calculates statistics about end SOC values
func (m *Manager) CalculateSOCStatistics() map[string]interface{} {
	// Initialize counters
	totalSessions := 0
	failedSessions := 0
	below80Count := 0
	exactly80Count := 0
	above80Count := 0
	exactly100Count := 0
	above90Count := 0

	// Track SOC values for averages
	var totalStartSoc float64
	var totalEndSoc float64
	lowestStartSoc := 100.0 // Start with maximum possible value
	validSessionsCount := 0 // Non-failed sessions

	for _, session := range m.sessions {
		totalSessions++

		if session.SocEnd == session.SocStart {
			failedSessions++
			continue
		}

		// Track SOC for valid sessions only
		totalStartSoc += session.SocStart
		totalEndSoc += session.SocEnd
		validSessionsCount++

		// Track lowest start SoC
		if session.SocStart < lowestStartSoc {
			lowestStartSoc = session.SocStart
		}

		if session.SocEnd < 80 {
			below80Count++
		} else if session.SocEnd == 80 {
			exactly80Count++
		} else if session.SocEnd > 80 {
			above80Count++
		}

		if session.SocEnd > 90 {
			above90Count++
		}

		if session.SocEnd == 100 {
			exactly100Count++
		}
	}

	// Calculate percentages and averages
	averageStartSoc := 0.0
	averageEndSoc := 0.0
	above80Percentage := 0.0
	above90Percentage := 0.0

	if validSessionsCount > 0 {
		averageStartSoc = totalStartSoc / float64(validSessionsCount)
		averageEndSoc = totalEndSoc / float64(validSessionsCount)
		above80Percentage = float64(above80Count) / float64(validSessionsCount) * 100
		above90Percentage = float64(above90Count) / float64(validSessionsCount) * 100
	}

	// Create result map with both counts and percentages
	return map[string]interface{}{
		"total_sessions":      totalSessions,
		"failed_sessions":     failedSessions,
		"below_80_count":      below80Count,
		"exactly_80_count":    exactly80Count,
		"above_80_count":      above80Count,
		"above_90_count":      above90Count,
		"exactly_100_count":   exactly100Count,
		"average_start_soc":   averageStartSoc,
		"average_end_soc":     averageEndSoc,
		"lowest_start_soc":    lowestStartSoc,
		"above_80_percentage": above80Percentage,
		"above_90_percentage": above90Percentage,
	}
}

// GetSessionStats returns statistics about sessions and providers
func (m *Manager) GetSessionStats() map[string]interface{} {
	totalSessions := len(m.sessions)
	totalFailedSessions := 0
	totalSuccessfulSessions := 0
	failedProviders := make(map[string]int)
	successfulProviders := make(map[string]int)

	for _, session := range m.sessions {
		if session.SocEnd == session.SocStart {
			totalFailedSessions++
			failedProviders[session.Provider]++
		} else {
			totalSuccessfulSessions++
			successfulProviders[session.Provider]++
		}
	}

	// Create a sortable structure for providers
	type ProviderCount struct {
		Provider string
		Count    int
	}

	// Create and populate slices for sorting
	var failedProvidersList []ProviderCount
	for provider, count := range failedProviders {
		if provider != "" && provider != "Unknown" {
			failedProvidersList = append(failedProvidersList, ProviderCount{
				Provider: provider,
				Count:    count,
			})
		}
	}

	var successfulProvidersList []ProviderCount
	for provider, count := range successfulProviders {
		if provider != "" && provider != "Unknown" {
			successfulProvidersList = append(successfulProvidersList, ProviderCount{
				Provider: provider,
				Count:    count,
			})
		}
	}

	// Sort by count in descending order
	sortProviders := func(providers []ProviderCount) {
		sort.Slice(providers, func(i, j int) bool {
			return providers[i].Count > providers[j].Count
		})
	}

	sortProviders(failedProvidersList)
	sortProviders(successfulProvidersList)

	// Convert to map for JSON response
	topFailedProviders := make([]map[string]interface{}, 0, len(failedProvidersList))
	for _, p := range failedProvidersList {
		topFailedProviders = append(topFailedProviders, map[string]interface{}{
			"provider": p.Provider,
			"count":    p.Count,
		})
	}

	topSuccessfulProviders := make([]map[string]interface{}, 0, len(successfulProvidersList))
	for _, p := range successfulProvidersList {
		topSuccessfulProviders = append(topSuccessfulProviders, map[string]interface{}{
			"provider": p.Provider,
			"count":    p.Count,
		})
	}

	// Limit to top 5 providers for each category
	limitedFailedProviders := topFailedProviders
	if len(limitedFailedProviders) > 5 {
		limitedFailedProviders = limitedFailedProviders[:5]
	}

	limitedSuccessfulProviders := topSuccessfulProviders
	if len(limitedSuccessfulProviders) > 5 {
		limitedSuccessfulProviders = limitedSuccessfulProviders[:5]
	}

	return map[string]interface{}{
		"total_sessions":            totalSessions,
		"total_failed_sessions":     totalFailedSessions,
		"total_successful_sessions": totalSuccessfulSessions,
		"top_failed_providers":      limitedFailedProviders,
		"top_successful_providers":  limitedSuccessfulProviders,
	}
}

// Helper functions for finding minimum and maximum of two ints
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// IsUsingEstimatedValues returns true if any session is using estimated energy values
func (m *Manager) IsUsingEstimatedValues() bool {
	for _, s := range m.sessions {
		if s.UsingEstimatedEnergy {
			return true
		}
	}
	return false
}
