package database

import (
	"database/sql"
	"fmt"
	"log"
	"path/filepath"
	"regexp"
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

	// Open with pragmas that prevent the intermittent "database is locked"
	// errors: WAL lets readers run alongside a writer, and a busy timeout makes
	// callers wait for the lock instead of failing immediately.
	dsn := dbPath + "?_busy_timeout=5000&_journal_mode=WAL&_foreign_keys=on&_synchronous=NORMAL"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}

	// SQLite allows only a single writer. Serialising access through one
	// connection eliminates lock contention from concurrent HTTP handlers.
	db.SetMaxOpenConns(1)

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
			mileage REAL DEFAULT 0,
			FOREIGN KEY (vehicle_id) REFERENCES vehicles(id)
		);
	`)

	return err
}

// StoreSessions stores charging sessions in the database.
//
// All sessions in a single upload belong to one vehicle, which is identified by
// the VIN embedded in the export filename. Using a per-upload vehicle identity
// (instead of the old per-session identity) means re-uploads from the same car
// land on the same vehicle row and duplicate sessions are skipped, while
// genuinely new sessions are appended.
func (m *Manager) StoreSessions(sessions []data.Session, filename string, userSpecifiedModel string) (bool, error) {
	// Generate a hash of the content to detect duplicate uploads.
	contentHash := hashSessions(sessions)

	// Derive a stable, privacy-preserving identity for the vehicle this upload
	// belongs to. The VIN in the filename gives us a single id per car; if it is
	// missing we fall back to the file content hash so the upload still maps to
	// exactly one vehicle rather than one-per-session.
	vehicleKey := vehicleKeyFromFilename(filename)
	if vehicleKey == "" {
		vehicleKey = "content:" + contentHash
	}
	finHash := hashFIN(vehicleKey)
	finPrefix := finHash[:16]

	// Build the vehicle-scoped session ids so two different cars can share the
	// same charge start timestamp without colliding.
	storedID := func(s data.Session) string { return finPrefix + "_" + s.ID }

	// Start a transaction. The named return error drives the deferred rollback,
	// so every early-exit path below assigns to `err` (never a shadowed copy).
	tx, err := m.db.Begin()
	if err != nil {
		return false, err
	}
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()

	// Find which of these sessions already exist (scoped to this vehicle).
	existingSessions := make(map[string]bool)
	if len(sessions) > 0 {
		ids := make([]interface{}, len(sessions))
		placeholders := make([]string, len(sessions))
		for i, s := range sessions {
			ids[i] = storedID(s)
			placeholders[i] = "?"
		}

		query := fmt.Sprintf("SELECT id FROM sessions WHERE id IN (%s)", strings.Join(placeholders, ","))
		rows, qErr := tx.Query(query, ids...)
		if qErr != nil {
			err = fmt.Errorf("error checking for existing sessions: %w", qErr)
			return false, err
		}
		for rows.Next() {
			var id string
			if scanErr := rows.Scan(&id); scanErr != nil {
				rows.Close()
				err = scanErr
				return false, err
			}
			existingSessions[id] = true
		}
		if rowsErr := rows.Err(); rowsErr != nil {
			rows.Close()
			err = rowsErr
			return false, err
		}
		rows.Close()
	}

	duplicateSessionCount := len(existingSessions)
	newSessionCount := len(sessions) - duplicateSessionCount

	// Nothing new in this upload: treat it as a duplicate and don't record it.
	if newSessionCount <= 0 {
		return false, nil
	}

	// Record the upload. A duplicate content hash is not fatal here because the
	// per-session check above already determined there is new data to store.
	if _, upErr := tx.Exec(
		"INSERT INTO uploads (content_hash, uploaded_at, session_count) VALUES (?, ?, ?)",
		contentHash, time.Now(), newSessionCount,
	); upErr != nil && !strings.Contains(upErr.Error(), "UNIQUE constraint failed") {
		err = upErr
		return false, err
	}

	// Resolve (or create) the single vehicle for this upload.
	var vehicleID int64
	vErr := tx.QueryRow("SELECT id FROM vehicles WHERE fin_hash = ?", finHash).Scan(&vehicleID)
	switch {
	case vErr == sql.ErrNoRows:
		model := userSpecifiedModel
		if model == "" {
			model = "Unknown BMW EV"
		}
		res, insErr := tx.Exec(
			"INSERT INTO vehicles (fin_hash, model, created_at) VALUES (?, ?, ?)",
			finHash, model, time.Now(),
		)
		if insErr != nil {
			err = insErr
			return false, err
		}
		vehicleID, _ = res.LastInsertId()
	case vErr != nil:
		err = vErr
		return false, err
	default:
		// Vehicle already known: fill in the model if it was previously unknown.
		if userSpecifiedModel != "" {
			if _, upErr := tx.Exec(
				"UPDATE vehicles SET model = ? WHERE id = ? AND (model IS NULL OR model = '' OR model = 'Unknown BMW EV')",
				userSpecifiedModel, vehicleID,
			); upErr != nil {
				err = upErr
				return false, err
			}
		}
	}

	// Process each new session.
	for _, session := range sessions {
		sid := storedID(session)
		if existingSessions[sid] {
			continue
		}

		status := "failed"
		if session.SocEnd > session.SocStart {
			status = "successful"
		}

		locationHash := ""
		if session.Location != "" {
			h := sha256.New()
			h.Write([]byte(session.Location))
			locationHash = hex.EncodeToString(h.Sum(nil))
		}

		if _, insErr := tx.Exec(
			`INSERT INTO sessions 
			(id, vehicle_id, start_time, end_time, soc_start, soc_end, 
			energy_from_grid, energy_added_hvb, cost, efficiency, provider, 
			avg_power, session_time_minutes, status, location_hash) 
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			sid, vehicleID, session.StartTime, session.EndTime,
			session.SocStart, session.SocEnd, session.EnergyFromGrid,
			session.EnergyAddedHvb, session.Cost, session.Efficiency,
			session.Provider, session.AvgPower, session.SessionTimeMinutes,
			status, locationHash,
		); insErr != nil {
			err = insErr
			return false, err
		}

		// Battery-health (SoH) data point. BMW exports almost never include the
		// measured energyIncreaseHvbKwh, so the HVB energy is usually estimated
		// from grid energy. We still use those points (otherwise there would be no
		// SoH curve at all) but record whether the figure was raw or estimated via
		// is_raw_data. We require a sizeable charge for a reliable extrapolation.
		socChange := session.SocEnd - session.SocStart
		if session.EnergyAddedHvb >= 30 && socChange >= 20 {
			estimatedCapacity := (session.EnergyAddedHvb * 100) / socChange

			var existingBHCount int
			if scanErr := tx.QueryRow(`SELECT COUNT(*) FROM battery_health 
				WHERE vehicle_id = ? 
				AND date = ? 
				AND ABS(estimated_capacity - ?) < 0.01 
				AND ABS(soc_change - ?) < 0.01`,
				vehicleID, session.StartTime, estimatedCapacity, socChange,
			).Scan(&existingBHCount); scanErr != nil {
				err = scanErr
				return false, err
			}

			if existingBHCount == 0 {
				if _, insErr := tx.Exec(
					`INSERT INTO battery_health 
					(vehicle_id, date, estimated_capacity, soc_change, is_raw_data, mileage) 
					VALUES (?, ?, ?, ?, ?, ?)`,
					vehicleID, session.StartTime, estimatedCapacity,
					socChange, !session.UsingEstimatedEnergy, session.Mileage,
				); insErr != nil {
					err = insErr
					return false, err
				}
			}
		}
	}

	if err = tx.Commit(); err != nil {
		return false, err
	}
	committed = true

	log.Printf("Stored %d new sessions, skipped %d duplicate sessions", newSessionCount, duplicateSessionCount)

	return true, nil
}

// GetAvailableModels returns a list of available BMW models in the database
func (m *Manager) GetAvailableModels() ([]string, error) {
	// First check if the table exists
	var tableExists int
	err := m.db.QueryRow(`SELECT count(name) FROM sqlite_master WHERE type='table' AND name='vehicles'`).Scan(&tableExists)
	if err != nil {
		return nil, fmt.Errorf("error checking if vehicles table exists: %w", err)
	}

	if tableExists == 0 {
		// Table doesn't exist, return empty result instead of error
		return []string{}, nil
	}

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
	// First check if the table exists
	var tableExists int
	err := m.db.QueryRow(`SELECT count(name) FROM sqlite_master WHERE type='table' AND name='battery_health'`).Scan(&tableExists)
	if err != nil {
		return nil, fmt.Errorf("error checking if battery_health table exists: %w", err)
	}

	if tableExists == 0 {
		// Table doesn't exist, return empty result instead of error
		return []map[string]interface{}{}, nil
	}

	query := `
		SELECT 
			bh.date, 
			bh.estimated_capacity, 
			bh.soc_change,
			bh.is_raw_data,
			bh.mileage,
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
		var capacity, socChange, mileage float64
		var isRawData bool
		var model string

		if err := rows.Scan(&date, &capacity, &socChange, &isRawData, &mileage, &model); err != nil {
			return nil, err
		}

		result = append(result, map[string]interface{}{
			"date":               date,
			"estimated_capacity": capacity,
			"mileage":            mileage,
			"soc_change":         socChange,
			"is_raw_data":        isRawData,
			"model":              model,
		})
	}

	return result, rows.Err()
}

// GetMonthlyBatteryHealthTrend returns the monthly aggregated battery health trend
func (m *Manager) GetMonthlyBatteryHealthTrend(modelFilter string) ([]map[string]interface{}, error) {
	// First check if the table exists
	var tableExists int
	err := m.db.QueryRow(`SELECT count(name) FROM sqlite_master WHERE type='table' AND name='battery_health'`).Scan(&tableExists)
	if err != nil {
		return nil, fmt.Errorf("error checking if battery_health table exists: %w", err)
	}

	if tableExists == 0 {
		// Table doesn't exist, return empty result instead of error
		return []map[string]interface{}{}, nil
	}

	// Capacity is averaged weighted by SoC change so larger, more reliable charge
	// sessions dominate the estimate. This matches the weighting used by the
	// in-memory per-file calculation (data.CalculateEstimatedBatteryCapacity).
	query := `
		SELECT 
			strftime('%Y-%m', bh.date) as month,
			sum(bh.estimated_capacity * bh.soc_change) / sum(bh.soc_change) as avg_capacity,
			sum(bh.soc_change) as total_soc_change,
			count(*) as data_points,
			avg(bh.mileage) as avg_mileage,
			v.model
		FROM battery_health bh
		JOIN vehicles v ON bh.vehicle_id = v.id
	`

	args := []interface{}{}
	if modelFilter != "" {
		query += " WHERE v.model = ?"
		args = append(args, modelFilter)
	}

	// Order chronologically so the trend line is plotted in time order.
	query += " GROUP BY strftime('%Y-%m', bh.date), v.model ORDER BY month"

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []map[string]interface{}
	for rows.Next() {
		var month string
		var avgCapacity, totalSocChange, avgMileage float64
		var dataPoints int
		var model string

		if err := rows.Scan(&month, &avgCapacity, &totalSocChange, &dataPoints, &avgMileage, &model); err != nil {
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
			"mileage":            avgMileage,
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

// vinPattern matches a 17-character BMW VIN/FIN. VINs never contain I, O or Q.
var vinPattern = regexp.MustCompile(`[A-HJ-NPR-Z0-9]{17}`)

// vehicleKeyFromFilename extracts the VIN/FIN embedded in a BMW CarData export
// filename (e.g. "BMW-CarData-Ladehistorie_<FIN>_01-03-2025.json").
// The raw VIN is never stored; callers hash it via hashFIN for privacy. Returns
// an empty string when no VIN can be found so the caller can fall back to a
// content-based key.
func vehicleKeyFromFilename(filename string) string {
	if filename == "" {
		return ""
	}
	// Strip directory and extension, then upper-case for a stable match.
	base := strings.ToUpper(filepath.Base(filename))
	if idx := strings.LastIndex(base, "."); idx >= 0 {
		base = base[:idx]
	}
	return vinPattern.FindString(base)
}

// hashFIN creates a strong one-way hash with salt for privacy
func hashFIN(vehicleIdentifier string) string {
	// Add a salt to make it even more secure against brute-force attacks
	// Using a fixed salt that's specific to this application
	salt := "BMWToolsAnonymousFleetStats2025"

	h := sha256.New()
	h.Write([]byte(salt + vehicleIdentifier))
	return hex.EncodeToString(h.Sum(nil))
}
