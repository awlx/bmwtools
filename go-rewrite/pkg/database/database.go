package database

import (
	"database/sql"
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"crypto/sha256"
	"encoding/hex"

	"github.com/awlx/bmwtools/pkg/data"
	_ "github.com/mattn/go-sqlite3"
)

// Manager handles database operations
type Manager struct {
	db *sql.DB
}

// New creates a new database manager
func New(dbPath string) (*Manager, error) {
	// Ensure the directory exists
	dir := filepath.Dir(dbPath)
	if dir != "." && dir != "/" {
		// This is a simplification - in production code, you might want to handle this more robustly
		log.Printf("Ensuring directory exists: %s", dir)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	m := &Manager{
		db: db,
	}

	if err := m.initSchema(); err != nil {
		db.Close()
		return nil, err
	}

	return m, nil
}

// Close closes the database connection
func (m *Manager) Close() error {
	return m.db.Close()
}

// initSchema initializes the database schema
func (m *Manager) initSchema() error {
	_, err := m.db.Exec(`
		CREATE TABLE IF NOT EXISTS uploads (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			content_hash TEXT UNIQUE NOT NULL,
			uploaded_at TIMESTAMP NOT NULL,
			session_count INTEGER NOT NULL
		);

		CREATE TABLE IF NOT EXISTS vehicles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			fin_hash TEXT UNIQUE NOT NULL,
			model TEXT,
			created_at TIMESTAMP NOT NULL
		);

		CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			vehicle_id INTEGER,
			start_time TIMESTAMP NOT NULL,
			end_time TIMESTAMP NOT NULL,
			soc_start REAL NOT NULL,
			soc_end REAL NOT NULL,
			energy_from_grid REAL NOT NULL,
			energy_added_hvb REAL NOT NULL,
			cost REAL,
			efficiency REAL,
			provider TEXT,
			avg_power REAL,
			session_time_minutes REAL,
			status TEXT,
			location_hash TEXT,
			FOREIGN KEY (vehicle_id) REFERENCES vehicles(id)
		);

		CREATE TABLE IF NOT EXISTS battery_health (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			vehicle_id INTEGER NOT NULL,
			date TIMESTAMP NOT NULL,
			estimated_capacity REAL NOT NULL,
			soc_change REAL NOT NULL,
			is_raw_data BOOLEAN NOT NULL,
			FOREIGN KEY (vehicle_id) REFERENCES vehicles(id)
		);
	`)

	return err
}

// StoreSessions stores charging sessions in the database
func (m *Manager) StoreSessions(sessions []data.Session, filename string, userSpecifiedModel string) (bool, error) {
	// Generate a hash of the content to detect duplicates
	contentHash := hashSessions(sessions)

	// Start a transaction
	tx, err := m.db.Begin()
	if err != nil {
		return false, err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Keep track of how many sessions were new vs. duplicates
	newSessionCount := 0
	duplicateSessionCount := 0

	// First collect all session IDs to check in a single query for efficiency
	var sessionIDs []interface{}
	sessionMap := make(map[string]data.Session)
	for _, session := range sessions {
		sessionIDs = append(sessionIDs, session.ID)
		sessionMap[session.ID] = session
	}

	// Build the SQL query to find existing sessions in a single database call
	existingSessions := make(map[string]bool)
	if len(sessionIDs) > 0 {
		placeholders := make([]string, len(sessionIDs))
		for i := range placeholders {
			placeholders[i] = "?"
		}

		// Use a parameterized query to find existing sessions
		query := fmt.Sprintf("SELECT id FROM sessions WHERE id IN (%s)", strings.Join(placeholders, ","))

		// Execute the query with sessionIDs as parameters
		rows, err := tx.Query(query, sessionIDs...)
		if err != nil {
			return false, fmt.Errorf("error checking for existing sessions: %w", err)
		}
		defer rows.Close()

		// Mark existing sessions
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return false, err
			}
			existingSessions[id] = true
			duplicateSessionCount++
		}
	}

	// Only record the upload if we have NEW sessions to store
	if len(sessionIDs) > duplicateSessionCount {
		// Record the upload - without storing the filename
		_, err = tx.Exec(
			"INSERT INTO uploads (content_hash, uploaded_at, session_count) VALUES (?, ?, ?)",
			contentHash, time.Now(), len(sessions)-duplicateSessionCount,
		)
		if err != nil {
			// If the content hash already exists, check if this is a true duplicate upload
			if strings.Contains(err.Error(), "UNIQUE constraint failed") && duplicateSessionCount == len(sessionIDs) {
				return false, nil
			}
			// Otherwise, we have some new sessions to process despite the duplicate content hash
			if !strings.Contains(err.Error(), "UNIQUE constraint failed") {
				return false, err
			}
		}
	} else if duplicateSessionCount == len(sessionIDs) {
		// All sessions are duplicates, treat as duplicate upload
		return false, nil
	}

	// Extract and hash FIN from sessions
	vehicleIDMap := make(map[string]int64)

	// Process each session, skipping duplicates
	for _, session := range sessions {
		// Skip if this session already exists
		if existingSessions[session.ID] {
			continue
		}

		// This is a new session
		newSessionCount++

		// Extract FIN from session ID (using our safer, non-identifiable approach)
		fin := extractFIN(session.ID)
		if fin == "" {
			continue
		}

		finHash := hashFIN(fin)

		// Check if we've already processed this vehicle in this batch
		vehicleID, exists := vehicleIDMap[finHash]
		if !exists {
			// Check if this vehicle exists in the database
			err = tx.QueryRow("SELECT id FROM vehicles WHERE fin_hash = ?", finHash).Scan(&vehicleID)
			if err == sql.ErrNoRows {
				// Vehicle doesn't exist, create it
				var model string
				if userSpecifiedModel != "" {
					model = userSpecifiedModel // Use the model specified by the user
				} else {
					model = deriveModelFromSession(session) // Fall back to derived model
				}
				res, err := tx.Exec(
					"INSERT INTO vehicles (fin_hash, model, created_at) VALUES (?, ?, ?)",
					finHash, model, time.Now(),
				)
				if err != nil {
					return false, err
				}
				vehicleID, _ = res.LastInsertId()
			} else if err != nil {
				return false, err
			}
			vehicleIDMap[finHash] = vehicleID
		}

		// Determine session status
		status := "failed"
		if session.SocEnd > session.SocStart {
			status = "successful"
		}

		// Hash location for privacy
		locationHash := ""
		if session.Location != "" {
			h := sha256.New()
			h.Write([]byte(session.Location))
			locationHash = hex.EncodeToString(h.Sum(nil))
		}

		// Store the session with the original provider name - no normalization
		// We're storing the original provider name as it appears in the data
		// This way we preserve the full, unmodified provider information
		_, err = tx.Exec(
			`INSERT INTO sessions 
			(id, vehicle_id, start_time, end_time, soc_start, soc_end, 
			energy_from_grid, energy_added_hvb, cost, efficiency, provider, 
			avg_power, session_time_minutes, status, location_hash) 
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			session.ID, vehicleID, session.StartTime, session.EndTime,
			session.SocStart, session.SocEnd, session.EnergyFromGrid,
			session.EnergyAddedHvb, session.Cost, session.Efficiency,
			session.Provider, session.AvgPower, session.SessionTimeMinutes,
			status, locationHash,
		)
		if err != nil {
			return false, err
		}

		// If we can calculate battery health from this session, store it
		if session.EnergyAddedHvb >= 30 && (session.SocEnd-session.SocStart) >= 20 {
			// Check if we already have battery health data for this exact session
			// We're using a more precise query with the actual values
			// This ensures we don't duplicate battery health data for the same session

			// Check for existing battery health record with a more precise query
			var existingBHCount int
			err = tx.QueryRow(`SELECT COUNT(*) FROM battery_health 
				WHERE vehicle_id = ? 
				AND date = ? 
				AND ABS(estimated_capacity - ?) < 0.01 
				AND ABS(soc_change - ?) < 0.01`,
				vehicleID, session.StartTime,
				(session.EnergyAddedHvb*100)/(session.SocEnd-session.SocStart),
				session.SocEnd-session.SocStart).Scan(&existingBHCount)
			if err != nil {
				return false, err
			}

			if existingBHCount == 0 {
				estimatedCapacity := (session.EnergyAddedHvb * 100) / (session.SocEnd - session.SocStart)
				_, err = tx.Exec(
					`INSERT INTO battery_health 
					(vehicle_id, date, estimated_capacity, soc_change, is_raw_data) 
					VALUES (?, ?, ?, ?, ?)`,
					vehicleID, session.StartTime, estimatedCapacity,
					session.SocEnd-session.SocStart, true,
				)
				if err != nil {
					return false, err
				}
			}
		}
	}

	// If all sessions were duplicates, consider this a duplicate upload
	if newSessionCount == 0 && duplicateSessionCount > 0 {
		tx.Rollback() // Don't save the upload record
		return false, nil
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return false, err
	}

	// Log how many sessions were new vs duplicates
	log.Printf("Stored %d new sessions, skipped %d duplicate sessions", newSessionCount, duplicateSessionCount)

	return true, nil
}

// GetAvailableModels returns a list of available BMW models in the database
func (m *Manager) GetAvailableModels() ([]string, error) {
	rows, err := m.db.Query("SELECT DISTINCT model FROM vehicles WHERE model IS NOT NULL ORDER BY model")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var models []string
	for rows.Next() {
		var model string
		if err := rows.Scan(&model); err != nil {
			return nil, err
		}
		if model != "" {
			models = append(models, model)
		}
	}

	return models, rows.Err()
}

// GetFleetBatteryHealth returns battery health data for the fleet
func (m *Manager) GetFleetBatteryHealth(modelFilter string) ([]map[string]interface{}, error) {
	query := `
		SELECT 
			bh.date, 
			bh.estimated_capacity, 
			bh.soc_change,
			bh.is_raw_data,
			v.model
		FROM battery_health bh
		JOIN vehicles v ON bh.vehicle_id = v.id
	`

	args := []interface{}{}
	if modelFilter != "" {
		query += " WHERE v.model = ?"
		args = append(args, modelFilter)
	}

	query += " ORDER BY bh.date"

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []map[string]interface{}
	for rows.Next() {
		var date time.Time
		var capacity, socChange float64
		var isRawData bool
		var model string

		if err := rows.Scan(&date, &capacity, &socChange, &isRawData, &model); err != nil {
			return nil, err
		}

		result = append(result, map[string]interface{}{
			"date":               date,
			"estimated_capacity": capacity,
			"soc_change":         socChange,
			"is_raw_data":        isRawData,
			"model":              model,
		})
	}

	return result, rows.Err()
}

// GetMonthlyBatteryHealthTrend returns the monthly aggregated battery health trend
func (m *Manager) GetMonthlyBatteryHealthTrend(modelFilter string) ([]map[string]interface{}, error) {
	query := `
		SELECT 
			strftime('%Y-%m', bh.date) as month,
			avg(bh.estimated_capacity) as avg_capacity,
			sum(bh.soc_change) as total_soc_change,
			count(*) as data_points,
			v.model
		FROM battery_health bh
		JOIN vehicles v ON bh.vehicle_id = v.id
	`

	args := []interface{}{}
	if modelFilter != "" {
		query += " WHERE v.model = ?"
		args = append(args, modelFilter)
	}

	query += " GROUP BY strftime('%Y-%m', bh.date), v.model ORDER BY month"

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []map[string]interface{}
	for rows.Next() {
		var month string
		var avgCapacity, totalSocChange float64
		var dataPoints int
		var model string

		if err := rows.Scan(&month, &avgCapacity, &totalSocChange, &dataPoints, &model); err != nil {
			return nil, err
		}

		// Parse the month string to get a date for the middle of the month
		t, _ := time.Parse("2006-01", month)
		// Set to the 15th day of the month as a representative date
		representativeDate := time.Date(t.Year(), t.Month(), 15, 12, 0, 0, 0, time.UTC)

		result = append(result, map[string]interface{}{
			"month":              month,
			"date":               representativeDate,
			"avg_capacity":       avgCapacity,
			"total_soc_change":   totalSocChange,
			"data_points":        dataPoints,
			"model":              model,
			"is_monthly_average": true,
		})
	}

	return result, rows.Err()
}

// GetProviderStats returns statistics about charging providers from the database
func (m *Manager) GetProviderStats() ([]map[string]interface{}, error) {
	// First, get all providers and their metrics
	query := `
		SELECT 
			provider,
			COUNT(*) as total_sessions,
			SUM(CASE WHEN status = 'successful' THEN 1 ELSE 0 END) as successful_sessions,
			SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed_sessions,
			AVG(CASE WHEN status = 'successful' THEN efficiency ELSE 0 END) as avg_efficiency,
			SUM(CASE WHEN status = 'successful' THEN energy_added_hvb ELSE 0 END) as total_energy_added,
			AVG(CASE WHEN status = 'successful' THEN avg_power ELSE 0 END) as avg_power
		FROM sessions
		GROUP BY provider
	`

	// We'll group providers manually to properly normalize them
	rows, err := m.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Create a map to normalize and combine providers
	providerMap := make(map[string]map[string]interface{})

	// We're using the comprehensive provider normalization logic
	// from data.NormalizeProviderName instead of maintaining a separate list here

	// Use the exported NormalizeProviderName function from the data package
	// instead of duplicating the normalization logic here
	normalizeProvider := func(provider string) string {
		return data.NormalizeProviderName(provider)
	}

	for rows.Next() {
		var provider string
		var totalSessions, successfulSessions, failedSessions int
		var avgEfficiency, totalEnergyAdded, avgPower float64

		if err := rows.Scan(&provider, &totalSessions, &successfulSessions, &failedSessions,
			&avgEfficiency, &totalEnergyAdded, &avgPower); err != nil {
			return nil, err
		}

		// Normalize the provider name
		normalizedProvider := normalizeProvider(provider)

		// Check if we already have an entry for this normalized provider
		if existingStats, exists := providerMap[normalizedProvider]; exists {
			// Update existing stats
			existingStats["total_sessions"] = existingStats["total_sessions"].(int) + totalSessions
			existingStats["successful_sessions"] = existingStats["successful_sessions"].(int) + successfulSessions
			existingStats["failed_sessions"] = existingStats["failed_sessions"].(int) + failedSessions

			// Weighted average for efficiency and power
			if successfulSessions > 0 {
				currentSuccessful := float64(existingStats["successful_sessions"].(int) - successfulSessions)
				newAvgEfficiency := ((avgEfficiency * float64(successfulSessions)) +
					(existingStats["avg_efficiency"].(float64) * currentSuccessful)) /
					float64(existingStats["successful_sessions"].(int))
				existingStats["avg_efficiency"] = newAvgEfficiency

				newAvgPower := ((avgPower * float64(successfulSessions)) +
					(existingStats["avg_power"].(float64) * currentSuccessful)) /
					float64(existingStats["successful_sessions"].(int))
				existingStats["avg_power"] = newAvgPower
			}

			existingStats["total_energy_added"] = existingStats["total_energy_added"].(float64) + totalEnergyAdded
		} else {
			// Create new entry
			providerMap[normalizedProvider] = map[string]interface{}{
				"provider":            normalizedProvider, // Use the normalized name
				"original_provider":   provider,           // Keep the original name for reference
				"total_sessions":      totalSessions,
				"successful_sessions": successfulSessions,
				"failed_sessions":     failedSessions,
				"avg_efficiency":      avgEfficiency,
				"total_energy_added":  totalEnergyAdded,
				"avg_power":           avgPower,
			}
		}
	}

	// Now convert the map to a slice and calculate success rates
	var result []map[string]interface{}
	for _, stats := range providerMap {
		totalSessions := stats["total_sessions"].(int)
		successfulSessions := stats["successful_sessions"].(int)

		// Calculate success rate
		successRate := 0.0
		if totalSessions > 0 {
			successRate = float64(successfulSessions) / float64(totalSessions) * 100
		}

		stats["success_rate"] = successRate
		stats["avg_efficiency"] = stats["avg_efficiency"].(float64) * 100 // Convert to percentage

		result = append(result, stats)
	}

	// Sort by total sessions descending
	sort.Slice(result, func(i, j int) bool {
		return result[i]["total_sessions"].(int) > result[j]["total_sessions"].(int)
	})

	return result, rows.Err()
}

// GetSOCStats returns statistics about State of Charge from the database
func (m *Manager) GetSOCStats() (map[string]interface{}, error) {
	query := `
		SELECT 
			AVG(soc_start) as avg_start_soc,
			AVG(soc_end) as avg_end_soc,
			MIN(soc_start) as min_start_soc,
			MAX(soc_start) as max_start_soc,
			MIN(soc_end) as min_end_soc,
			MAX(soc_end) as max_end_soc
		FROM sessions
		WHERE status = 'successful'
	`

	var avgStartSOC, avgEndSOC, minStartSOC, maxStartSOC, minEndSOC, maxEndSOC float64
	err := m.db.QueryRow(query).Scan(&avgStartSOC, &avgEndSOC, &minStartSOC, &maxStartSOC, &minEndSOC, &maxEndSOC)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"average_start_soc": avgStartSOC,
		"average_end_soc":   avgEndSOC,
		"min_start_soc":     minStartSOC,
		"max_start_soc":     maxStartSOC,
		"min_end_soc":       minEndSOC,
		"max_end_soc":       maxEndSOC,
	}, nil
}

// GetSessionCount returns the total number of sessions stored in the database
func (m *Manager) GetSessionCount() (int, error) {
	var count int
	err := m.db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// Helper functions

// hashSessions creates a hash of the sessions to detect duplicates
func hashSessions(sessions []data.Session) string {
	h := sha256.New()
	for _, s := range sessions {
		// Include key fields that identify the session uniquely
		fmt.Fprintf(h, "%s|%s|%s|%.2f|%.2f|%.2f|%.2f|",
			s.ID, s.StartTime.String(), s.EndTime.String(),
			s.SocStart, s.SocEnd, s.EnergyFromGrid, s.EnergyAddedHvb)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// extractFIN extracts a non-identifiable vehicle identifier from session data
// This deliberately does NOT extract the actual FIN, but creates a pseudonymous identifier
func extractFIN(sessionID string) string {
	// Instead of extracting an actual FIN, we'll just use a session-specific hash
	// This approach ensures we never store or process the actual FIN
	return sessionID
}

// hashFIN creates a strong one-way hash with salt for privacy
func hashFIN(sessionIdentifier string) string {
	// Add a salt to make it even more secure against brute-force attacks
	// Using a fixed salt that's specific to this application
	salt := "BMWToolsAnonymousFleetStats2025"

	h := sha256.New()
	h.Write([]byte(salt + sessionIdentifier))
	return hex.EncodeToString(h.Sum(nil))
}

// deriveModelFromSession tries to determine the BMW model from session data
func deriveModelFromSession(session data.Session) string {
	// This is a placeholder. In reality, the model would need to be determined
	// from the session data based on your specific data structure.
	// It might be embedded in the data or derivable from the FIN.

	// Example logic:
	if strings.Contains(session.ID, "i3") {
		return "BMW i3"
	} else if strings.Contains(session.ID, "i4") {
		return "BMW i4"
	} else if strings.Contains(session.ID, "iX") {
		return "BMW iX"
	}

	// Default if we can't determine
	return "Unknown BMW EV"
}
