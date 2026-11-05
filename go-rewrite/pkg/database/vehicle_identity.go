package database

import (
	"database/sql"
	"fmt"
	"log"
	"math"
	"os"
	"strings"
	"time"

	"github.com/awlx/bmwtools/pkg/data"
	vehiclecatalog "github.com/awlx/bmwtools/pkg/vehicles"
)

// Vehicle key kinds stored in vehicles.key_kind. NULL means unknown (rows
// created before the column existed).
const (
	keyKindVIN     = "vin"     // hash of the VIN in the upload filename
	keyKindContent = "content" // hash of the file content (no VIN in the name)
	keyKindLegacy  = "legacy"  // rebuilt by the regroup migration
	keyKindSplit   = "split"   // split from a vehicle that held two cars
)

// Fingerprint match thresholds: an upload without a known vehicle hash joins
// an existing vehicle when at least this share of its sessions, or at least
// this number of sessions, are already stored for that vehicle.
const (
	fingerprintMatchShare = 0.30
	fingerprintMatchMin   = 20
)

// defaultVINSalt is the salt used before it became configurable. Keep it as
// the default, otherwise existing vehicle hashes no longer match.
const defaultVINSalt = "BMWToolsAnonymousFleetStats2025"

// VINSaltEnv is the environment variable that sets the salt of vehicle hashes.
const VINSaltEnv = "BMWTOOLS_VIN_SALT"

func vinSalt() string {
	if s := os.Getenv(VINSaltEnv); s != "" {
		return s
	}
	return defaultVINSalt
}

// VehicleKey identifies the car of an upload by a salted one-way hash.
type VehicleKey struct {
	Hash    string
	FromVIN bool
}

func (k VehicleKey) kind() string {
	if k.FromVIN {
		return keyKindVIN
	}
	return keyKindContent
}

// sessionFingerprint identifies one charging session independent of its
// stored id: start time, SoC start and end, and grid energy (0.1 kWh).
func sessionFingerprint(start time.Time, socStart, socEnd, energy float64) string {
	return fmt.Sprintf("%d|%.1f|%.1f|%.1f", start.Unix(), socStart, socEnd, energy)
}

func fingerprintOf(s data.Session) string {
	return sessionFingerprint(s.StartTime, s.SocStart, s.SocEnd, s.EnergyFromGrid)
}

type queryer interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
}

// storedFingerprints returns, for the stored sessions that start in the time
// range of the given sessions, the vehicle ids per fingerprint.
func storedFingerprints(q queryer, sessions []data.Session) (map[string]map[int64]bool, error) {
	out := map[string]map[int64]bool{}
	if len(sessions) == 0 {
		return out, nil
	}
	lo, hi := sessions[0].StartTime.Unix(), sessions[0].StartTime.Unix()
	for _, s := range sessions {
		u := s.StartTime.Unix()
		if u < lo {
			lo = u
		}
		if u > hi {
			hi = u
		}
	}
	rows, err := q.Query(`SELECT vehicle_id, CAST(strftime('%s', start_time) AS INTEGER),
			soc_start, soc_end, energy_from_grid
		FROM sessions
		WHERE CAST(strftime('%s', start_time) AS INTEGER) BETWEEN ? AND ?`, lo, hi)
	if err != nil {
		return nil, fmt.Errorf("error reading session fingerprints: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var vid sql.NullInt64
		var unix int64
		var a, b, e float64
		if err := rows.Scan(&vid, &unix, &a, &b, &e); err != nil {
			return nil, err
		}
		if !vid.Valid {
			continue
		}
		fp := sessionFingerprint(time.Unix(unix, 0), a, b, e)
		if out[fp] == nil {
			out[fp] = map[int64]bool{}
		}
		out[fp][vid.Int64] = true
	}
	return out, rows.Err()
}

// bestFingerprintMatch returns the vehicle that holds most of the upload's
// sessions, if it passes the match thresholds.
func bestFingerprintMatch(sessions []data.Session, stored map[string]map[int64]bool) (int64, int, bool) {
	counts := map[int64]int{}
	seen := map[string]bool{}
	for _, s := range sessions {
		fp := fingerprintOf(s)
		if seen[fp] {
			continue
		}
		seen[fp] = true
		for vid := range stored[fp] {
			counts[vid]++
		}
	}
	var best int64
	bestN := 0
	for vid, n := range counts {
		if n > bestN || (n == bestN && vid > best) {
			best, bestN = vid, n
		}
	}
	if bestN == 0 {
		return 0, 0, false
	}
	ok := bestN >= fingerprintMatchMin || float64(bestN) >= fingerprintMatchShare*float64(len(seen))
	return best, bestN, ok
}

type vehicleRow struct {
	id    int64
	model string
	kind  string // "" when unknown
}

// lookupVehicle finds a vehicle by its own hash or by an alias hash.
func lookupVehicle(q queryer, hash string) (vehicleRow, bool, error) {
	var v vehicleRow
	var model, kind sql.NullString
	err := q.QueryRow("SELECT id, model, key_kind FROM vehicles WHERE fin_hash = ?", hash).Scan(&v.id, &model, &kind)
	if err == sql.ErrNoRows {
		err = q.QueryRow(`SELECT v.id, v.model, v.key_kind FROM vehicle_aliases a
			JOIN vehicles v ON v.id = a.vehicle_id WHERE a.alias_hash = ?`, hash).Scan(&v.id, &model, &kind)
	}
	if err == sql.ErrNoRows {
		return vehicleRow{}, false, nil
	}
	if err != nil {
		return vehicleRow{}, false, err
	}
	v.model, v.kind = model.String, kind.String
	return v, true, nil
}

func vehicleByID(q queryer, id int64) (vehicleRow, error) {
	v := vehicleRow{id: id}
	var model, kind sql.NullString
	err := q.QueryRow("SELECT model, key_kind FROM vehicles WHERE id = ?", id).Scan(&model, &kind)
	v.model, v.kind = model.String, kind.String
	return v, err
}

// canJoin reports whether an upload with key k may join vehicle v that was
// found by fingerprint. Two different VIN hashes are never merged.
func canJoin(k VehicleKey, v vehicleRow) bool {
	return !(k.FromVIN && v.kind == keyKindVIN)
}

func isUnknownModel(model string) bool {
	return model == "" || model == "Unknown BMW EV"
}

// modelFitsCapacity reports whether a measured capacity fits the packs of a
// model. For a pack group the group must be the nearest pack of its family.
// Families without known packs cannot be checked and do not fit.
func modelFitsCapacity(model string, capacity float64) bool {
	family := vehiclecatalog.FamilyOf(model)
	packs := vehiclecatalog.GroupsOf(family)
	if len(packs) == 0 || capacity <= 0 {
		return false
	}
	g, isGroup := vehiclecatalog.Group(model)
	if !isGroup {
		return plausibleForFamily(family, capacity)
	}
	nearest := packs[0]
	for _, p := range packs[1:] {
		if math.Abs(capacity-p.NetKWh) < math.Abs(capacity-nearest.NetKWh) {
			nearest = p
		}
	}
	return nearest.Value == g.Value && math.Abs(capacity-g.NetKWh)/g.NetKWh <= packPlausibilityRatio
}

// refineModel decides the stored model when a later upload names a model.
// An unknown model is always replaced. A family model may become a pack
// group of the same family. Any other change needs a measured capacity that
// fits the requested model. hasCapacity is false when too few points exist.
func refineModel(current, requested string, capacity float64, hasCapacity bool) (string, bool) {
	if requested == "" || requested == current {
		return current, false
	}
	if isUnknownModel(current) {
		return requested, true
	}
	_, curIsGroup := vehiclecatalog.Group(current)
	_, reqIsGroup := vehiclecatalog.Group(requested)
	sameFamily := vehiclecatalog.FamilyOf(current) == vehiclecatalog.FamilyOf(requested)
	if sameFamily && !curIsGroup && reqIsGroup {
		if !hasCapacity || modelFitsCapacity(requested, capacity) {
			return requested, true
		}
		return current, false
	}
	if sameFamily && !reqIsGroup {
		return current, false // never make a model less specific
	}
	if hasCapacity && modelFitsCapacity(requested, capacity) {
		return requested, true
	}
	return current, false
}

// minModelCheckPoints is the minimum number of battery-health points needed
// to use the measured capacity for a model change.
const minModelCheckPoints = 3

// vehicleCapacity returns the median estimated capacity of a vehicle from
// points with a SoC change of at least minAnalysisSocChange.
func vehicleCapacity(q queryer, vehicleID int64) (float64, bool, error) {
	rows, err := q.Query(`SELECT estimated_capacity FROM battery_health
		WHERE vehicle_id = ? AND soc_change >= ?`, vehicleID, minAnalysisSocChange)
	if err != nil {
		return 0, false, err
	}
	defer rows.Close()
	var vals []float64
	for rows.Next() {
		var c float64
		if err := rows.Scan(&c); err != nil {
			return 0, false, err
		}
		vals = append(vals, c)
	}
	if err := rows.Err(); err != nil {
		return 0, false, err
	}
	if len(vals) < minModelCheckPoints {
		return 0, false, nil
	}
	return median(vals), true, nil
}

// updateModel applies refineModel to a stored vehicle.
func updateModel(tx *sql.Tx, v vehicleRow, requested string) error {
	if requested == "" || requested == v.model {
		return nil
	}
	capacity, ok, err := vehicleCapacity(tx, v.id)
	if err != nil {
		return err
	}
	model, changed := refineModel(v.model, requested, capacity, ok)
	if !changed {
		log.Printf("Vehicle model kept as %q (upload named %q, capacity check failed or not possible)", v.model, requested)
		return nil
	}
	_, err = tx.Exec("UPDATE vehicles SET model = ? WHERE id = ?", model, v.id)
	return err
}

// migrateIdentitySchema adds the key kind column and the alias table to
// databases created before they existed.
func migrateIdentitySchema(db *sql.DB) error {
	rows, err := db.Query("PRAGMA table_info(vehicles)")
	if err != nil {
		return err
	}
	hasKind := false
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			rows.Close()
			return err
		}
		if strings.EqualFold(name, "key_kind") {
			hasKind = true
		}
	}
	rows.Close()
	if !hasKind {
		if _, err := db.Exec("ALTER TABLE vehicles ADD COLUMN key_kind TEXT"); err != nil {
			return err
		}
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS vehicle_aliases (
			alias_hash TEXT PRIMARY KEY,
			vehicle_id INTEGER NOT NULL,
			FOREIGN KEY (vehicle_id) REFERENCES vehicles(id)
		);
		CREATE INDEX IF NOT EXISTS idx_sessions_vehicle ON sessions(vehicle_id);
		CREATE INDEX IF NOT EXISTS idx_battery_health_vehicle ON battery_health(vehicle_id);
	`)
	return err
}
