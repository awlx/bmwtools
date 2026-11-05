package database

import (
	"database/sql"
	"fmt"
	"sort"
	"time"
)

// RepairCounts are table counts used to report a repair.
type RepairCounts struct {
	Vehicles          int `json:"vehicles"`
	Sessions          int `json:"sessions"`
	BatteryHealth     int `json:"battery_health"`
	DuplicateSessions int `json:"duplicate_sessions"` // extra rows with a fingerprint seen before
	EpochSessions     int `json:"epoch_sessions"`
	EpochEndTimes     int `json:"epoch_end_times"`
}

// RepairReport describes what RepairFleet changed.
type RepairReport struct {
	Before               RepairCounts `json:"before"`
	After                RepairCounts `json:"after"`
	LegacyMarked         int          `json:"legacy_marked"`
	EpochVehiclesDeleted int          `json:"epoch_vehicles_deleted"`
	EndTimesFixed        int          `json:"end_times_fixed"`
	EndTimesCleared      int          `json:"end_times_cleared"`
	Splits               []string     `json:"splits"`
	Merges               []string     `json:"merges"`
	SkippedMerges        []string     `json:"skipped_merges"`
	SessionsDeduped      int          `json:"sessions_deduped"`
	BatteryHealthDeduped int          `json:"battery_health_deduped"`
	ModelChanges         []string     `json:"model_changes"`
}

// minSplitPoints is the minimum number of battery-health points after an
// odometer drop. Fewer points are treated as a data error, not a second car.
const minSplitPoints = 3

// epochLimit is the end of 1970. Times before it come from a zero timestamp.
var epochLimit = time.Date(1971, 1, 1, 0, 0, 0, 0, time.UTC).Unix()

// RepairFleet repairs the fleet database in one transaction:
//
//  1. mark vehicles from the regroup migration (ids without hash prefix) as legacy
//  2. delete sessions and points with a zero start time, and vehicles left empty
//  3. set 1970 end times from the charging duration, or clear them
//  4. split vehicles whose odometer goes back by more than odometerResetKm
//  5. merge vehicles that hold the same sessions (fingerprint match)
//  6. remove duplicate sessions and battery-health points per vehicle
//
// Two vehicles with VIN hashes are never merged.
func (m *Manager) RepairFleet() (RepairReport, error) {
	var rep RepairReport
	var err error
	if rep.Before, err = repairCounts(m.db); err != nil {
		return rep, err
	}
	tx, err := m.db.Begin()
	if err != nil {
		return rep, err
	}
	defer tx.Rollback()

	steps := []func(*sql.Tx, *RepairReport) error{
		markLegacyVehicles,
		deleteEpochRecords,
		fixEpochEndTimes,
		splitResetVehicles,
		mergeDuplicateVehicles,
		dedupeWithinVehicles,
	}
	for _, step := range steps {
		if err := step(tx, &rep); err != nil {
			return rep, err
		}
	}
	if err := tx.Commit(); err != nil {
		return rep, err
	}
	rep.After, err = repairCounts(m.db)
	return rep, err
}

func repairCounts(q queryer) (RepairCounts, error) {
	var c RepairCounts
	for _, x := range []struct {
		dst   *int
		query string
	}{
		{&c.Vehicles, "SELECT COUNT(*) FROM vehicles"},
		{&c.Sessions, "SELECT COUNT(*) FROM sessions"},
		{&c.BatteryHealth, "SELECT COUNT(*) FROM battery_health"},
		{&c.EpochSessions, fmt.Sprintf("SELECT COUNT(*) FROM sessions WHERE CAST(strftime('%%s', start_time) AS INTEGER) < %d", epochLimit)},
		{&c.EpochEndTimes, "SELECT COUNT(*) FROM sessions WHERE end_time LIKE '1970-%'"},
	} {
		if err := q.QueryRow(x.query).Scan(x.dst); err != nil {
			return c, err
		}
	}
	sessions, err := loadRepairSessions(q)
	if err != nil {
		return c, err
	}
	seen := map[string]bool{}
	for _, s := range sessions {
		if seen[s.fp] {
			c.DuplicateSessions++
		}
		seen[s.fp] = true
	}
	return c, nil
}

type repairSession struct {
	rowid     int64
	vehicleID int64
	start     int64
	fp        string
}

func loadRepairSessions(q queryer) ([]repairSession, error) {
	rows, err := q.Query(`SELECT rowid, vehicle_id, CAST(strftime('%s', start_time) AS INTEGER),
		soc_start, soc_end, energy_from_grid FROM sessions WHERE vehicle_id IS NOT NULL ORDER BY rowid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []repairSession
	for rows.Next() {
		var s repairSession
		var a, b, e float64
		if err := rows.Scan(&s.rowid, &s.vehicleID, &s.start, &a, &b, &e); err != nil {
			return nil, err
		}
		s.fp = sessionFingerprint(time.Unix(s.start, 0), a, b, e)
		out = append(out, s)
	}
	return out, rows.Err()
}

func markLegacyVehicles(tx *sql.Tx, rep *RepairReport) error {
	res, err := tx.Exec(`UPDATE vehicles SET key_kind = ? WHERE key_kind IS NULL
		AND id IN (SELECT DISTINCT vehicle_id FROM sessions WHERE instr(id, '_') = 0)`, keyKindLegacy)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	rep.LegacyMarked = int(n)
	return nil
}

func deleteEpochRecords(tx *sql.Tx, rep *RepairReport) error {
	rows, err := tx.Query(fmt.Sprintf(`SELECT DISTINCT vehicle_id FROM sessions
		WHERE CAST(strftime('%%s', start_time) AS INTEGER) < %d`, epochLimit))
	if err != nil {
		return err
	}
	var touched []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		touched = append(touched, id)
	}
	rows.Close()
	if _, err := tx.Exec(fmt.Sprintf("DELETE FROM sessions WHERE CAST(strftime('%%s', start_time) AS INTEGER) < %d", epochLimit)); err != nil {
		return err
	}
	if _, err := tx.Exec(fmt.Sprintf("DELETE FROM battery_health WHERE CAST(strftime('%%s', date) AS INTEGER) < %d", epochLimit)); err != nil {
		return err
	}
	for _, id := range touched {
		var n int
		if err := tx.QueryRow(`SELECT (SELECT COUNT(*) FROM sessions WHERE vehicle_id = ?) +
			(SELECT COUNT(*) FROM battery_health WHERE vehicle_id = ?)`, id, id).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			if err := deleteVehicle(tx, id); err != nil {
				return err
			}
			rep.EpochVehiclesDeleted++
		}
	}
	return nil
}

func deleteVehicle(tx *sql.Tx, id int64) error {
	if _, err := tx.Exec("DELETE FROM vehicle_aliases WHERE vehicle_id = ?", id); err != nil {
		return err
	}
	_, err := tx.Exec("DELETE FROM vehicles WHERE id = ?", id)
	return err
}

// fixEpochEndTimes sets a 1970 end time to start + charging duration. Without
// a duration the end time is cleared. Database stats do not use end_time;
// duration and power come from session_time_minutes and avg_power.
func fixEpochEndTimes(tx *sql.Tx, rep *RepairReport) error {
	res, err := tx.Exec(`UPDATE sessions
		SET end_time = datetime(start_time, '+' || CAST(ROUND(session_time_minutes * 60) AS INTEGER) || ' seconds') || '+00:00'
		WHERE end_time LIKE '1970-%' AND session_time_minutes > 0`)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	rep.EndTimesFixed = int(n)
	res, err = tx.Exec(`UPDATE sessions SET end_time = '' WHERE end_time LIKE '1970-%'`)
	if err != nil {
		return err
	}
	n, _ = res.RowsAffected()
	rep.EndTimesCleared = int(n)
	return nil
}

type odoPoint struct {
	date    int64
	mileage float64
}

// findReset returns the last point before and the first point after the
// first odometer drop of more than odometerResetKm, if the part after the
// drop has at least minSplitPoints points.
func findReset(pts []odoPoint) (before, after odoPoint, ok bool) {
	for i := 1; i < len(pts); i++ {
		if pts[i-1].mileage-pts[i].mileage > odometerResetKm {
			if len(pts)-i < minSplitPoints {
				return odoPoint{}, odoPoint{}, false
			}
			return pts[i-1], pts[i], true
		}
	}
	return odoPoint{}, odoPoint{}, false
}

// splitCut returns the split time between two cars: the largest gap between
// session starts from the last point of the first car to the first point of
// the second car.
func splitCut(starts []int64, before, after odoPoint) int64 {
	var in []int64
	for _, s := range starts {
		if s >= before.date && s <= after.date {
			in = append(in, s)
		}
	}
	sort.Slice(in, func(i, j int) bool { return in[i] < in[j] })
	cut, gap := after.date, int64(-1)
	for i := 1; i < len(in); i++ {
		if d := in[i] - in[i-1]; d > gap {
			gap, cut = d, in[i]
		}
	}
	return cut
}

func splitResetVehicles(tx *sql.Tx, rep *RepairReport) error {
	rows, err := tx.Query("SELECT id, COALESCE(model, ''), COALESCE(key_kind, '') FROM vehicles")
	if err != nil {
		return err
	}
	var queue []vehicleRow
	for rows.Next() {
		var v vehicleRow
		if err := rows.Scan(&v.id, &v.model, &v.kind); err != nil {
			rows.Close()
			return err
		}
		queue = append(queue, v)
	}
	rows.Close()

	for len(queue) > 0 {
		v := queue[0]
		queue = queue[1:]
		if v.kind == keyKindVIN {
			continue // one VIN is one car; a drop there is a data error
		}
		pts, err := odometerPoints(tx, v.id)
		if err != nil {
			return err
		}
		before, after, ok := findReset(pts)
		if !ok {
			continue
		}
		starts, err := sessionStarts(tx, v.id)
		if err != nil {
			return err
		}
		cut := splitCut(starts, before, after)
		res, err := tx.Exec("INSERT INTO vehicles (fin_hash, model, created_at, key_kind) VALUES (?, ?, ?, ?)",
			hashFIN(fmt.Sprintf("split:%d:%d", v.id, cut)), v.model, time.Now(), keyKindSplit)
		if err != nil {
			return err
		}
		newID, _ := res.LastInsertId()
		r1, err := tx.Exec("UPDATE sessions SET vehicle_id = ? WHERE vehicle_id = ? AND CAST(strftime('%s', start_time) AS INTEGER) >= ?", newID, v.id, cut)
		if err != nil {
			return err
		}
		r2, err := tx.Exec("UPDATE battery_health SET vehicle_id = ? WHERE vehicle_id = ? AND CAST(strftime('%s', date) AS INTEGER) >= ?", newID, v.id, cut)
		if err != nil {
			return err
		}
		ns, _ := r1.RowsAffected()
		nb, _ := r2.RowsAffected()
		rep.Splits = append(rep.Splits, fmt.Sprintf("vehicle %d split at %s: %d sessions and %d points moved to new vehicle %d (odometer %.0f → %.0f km)",
			v.id, time.Unix(cut, 0).UTC().Format("2006-01-02"), ns, nb, newID, before.mileage, after.mileage))
		queue = append(queue, vehicleRow{id: newID, model: v.model, kind: keyKindSplit})
	}
	return nil
}

func odometerPoints(q queryer, vehicleID int64) ([]odoPoint, error) {
	rows, err := q.Query(`SELECT CAST(strftime('%s', date) AS INTEGER), mileage FROM battery_health
		WHERE vehicle_id = ? AND mileage > 0 ORDER BY date, id`, vehicleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []odoPoint
	for rows.Next() {
		var p odoPoint
		if err := rows.Scan(&p.date, &p.mileage); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func sessionStarts(q queryer, vehicleID int64) ([]int64, error) {
	rows, err := q.Query("SELECT CAST(strftime('%s', start_time) AS INTEGER) FROM sessions WHERE vehicle_id = ?", vehicleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var s int64
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// kindRank orders vehicles for the merge survivor: an identified car first,
// rebuilt legacy vehicles last.
func kindRank(kind string) int {
	switch kind {
	case keyKindVIN:
		return 0
	case "":
		return 1
	case keyKindContent:
		return 2
	case keyKindSplit:
		return 3
	default:
		return 4
	}
}

func mergeDuplicateVehicles(tx *sql.Tx, rep *RepairReport) error {
	sessions, err := loadRepairSessions(tx)
	if err != nil {
		return err
	}
	byFP := map[string]map[int64]bool{}
	perVehicle := map[int64]map[string]bool{}
	for _, s := range sessions {
		if byFP[s.fp] == nil {
			byFP[s.fp] = map[int64]bool{}
		}
		byFP[s.fp][s.vehicleID] = true
		if perVehicle[s.vehicleID] == nil {
			perVehicle[s.vehicleID] = map[string]bool{}
		}
		perVehicle[s.vehicleID][s.fp] = true
	}
	type pair struct{ a, b int64 }
	shared := map[pair]int{}
	for _, vs := range byFP {
		if len(vs) < 2 {
			continue
		}
		ids := make([]int64, 0, len(vs))
		for id := range vs {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		for i := range ids {
			for j := i + 1; j < len(ids); j++ {
				shared[pair{ids[i], ids[j]}]++
			}
		}
	}

	vehicles := map[int64]vehicleRow{}
	rows, err := tx.Query("SELECT id, COALESCE(model, ''), COALESCE(key_kind, '') FROM vehicles")
	if err != nil {
		return err
	}
	for rows.Next() {
		var v vehicleRow
		if err := rows.Scan(&v.id, &v.model, &v.kind); err != nil {
			rows.Close()
			return err
		}
		vehicles[v.id] = v
	}
	rows.Close()

	parent := map[int64]int64{}
	var find func(int64) int64
	find = func(x int64) int64 {
		if p, ok := parent[x]; ok && p != x {
			parent[x] = find(p)
			return parent[x]
		}
		return x
	}
	pairs := make([]pair, 0, len(shared))
	for p := range shared {
		pairs = append(pairs, p)
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].a != pairs[j].a {
			return pairs[i].a < pairs[j].a
		}
		return pairs[i].b < pairs[j].b
	})
	for _, p := range pairs {
		n := shared[p]
		smaller := len(perVehicle[p.a])
		if len(perVehicle[p.b]) < smaller {
			smaller = len(perVehicle[p.b])
		}
		if n < fingerprintMatchMin && float64(n) < fingerprintMatchShare*float64(smaller) {
			continue
		}
		if vehicles[p.a].kind == keyKindVIN && vehicles[p.b].kind == keyKindVIN {
			rep.SkippedMerges = append(rep.SkippedMerges, fmt.Sprintf("vehicles %d and %d share %d sessions but both have VIN hashes", p.a, p.b, n))
			continue
		}
		if ra, rb := find(p.a), find(p.b); ra != rb {
			parent[ra] = rb
		}
	}

	groups := map[int64][]int64{}
	for id := range parent {
		groups[find(id)] = append(groups[find(id)], id)
	}
	roots := make([]int64, 0, len(groups))
	for r, members := range groups {
		if !contains(members, r) {
			groups[r] = append(groups[r], r)
		}
		roots = append(roots, r)
	}
	sort.Slice(roots, func(i, j int) bool { return roots[i] < roots[j] })

	for _, r := range roots {
		members := groups[r]
		vinCount := 0
		for _, id := range members {
			if vehicles[id].kind == keyKindVIN {
				vinCount++
			}
		}
		if vinCount > 1 {
			rep.SkippedMerges = append(rep.SkippedMerges, fmt.Sprintf("group %v holds more than one VIN hash", members))
			continue
		}
		sort.Slice(members, func(i, j int) bool {
			a, b := vehicles[members[i]], vehicles[members[j]]
			if kindRank(a.kind) != kindRank(b.kind) {
				return kindRank(a.kind) < kindRank(b.kind)
			}
			if len(perVehicle[a.id]) != len(perVehicle[b.id]) {
				return len(perVehicle[a.id]) > len(perVehicle[b.id])
			}
			return a.id > b.id
		})
		survivor := vehicles[members[0]]
		for _, id := range members[1:] {
			if err := mergeVehicleInto(tx, vehicles[id], &survivor, perVehicle, rep); err != nil {
				return err
			}
		}
	}
	return nil
}

func contains(ids []int64, id int64) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

func mergeVehicleInto(tx *sql.Tx, from vehicleRow, into *vehicleRow, perVehicle map[int64]map[string]bool, rep *RepairReport) error {
	sessions, err := loadVehicleSessions(tx, from.id)
	if err != nil {
		return err
	}
	dup, moved := 0, 0
	for _, s := range sessions {
		if perVehicle[into.id][s.fp] {
			if _, err := tx.Exec("DELETE FROM sessions WHERE rowid = ?", s.rowid); err != nil {
				return err
			}
			dup++
			continue
		}
		if _, err := tx.Exec("UPDATE sessions SET vehicle_id = ? WHERE rowid = ?", into.id, s.rowid); err != nil {
			return err
		}
		if perVehicle[into.id] == nil {
			perVehicle[into.id] = map[string]bool{}
		}
		perVehicle[into.id][s.fp] = true
		moved++
	}
	// A point of the same charge (same time and SoC change) may have another
	// capacity when an older version estimated it. Keep the survivor's point.
	res, err := tx.Exec(`DELETE FROM battery_health WHERE vehicle_id = ? AND EXISTS (
		SELECT 1 FROM battery_health b WHERE b.vehicle_id = ? AND b.date = battery_health.date
		AND ABS(b.soc_change - battery_health.soc_change) < 0.01)`, from.id, into.id)
	if err != nil {
		return err
	}
	bhDup, _ := res.RowsAffected()
	rep.BatteryHealthDeduped += int(bhDup)
	if _, err := tx.Exec("UPDATE battery_health SET vehicle_id = ? WHERE vehicle_id = ?", into.id, from.id); err != nil {
		return err
	}
	var fromHash string
	if err := tx.QueryRow("SELECT fin_hash FROM vehicles WHERE id = ?", from.id).Scan(&fromHash); err != nil {
		return err
	}
	if _, err := tx.Exec("UPDATE vehicle_aliases SET vehicle_id = ? WHERE vehicle_id = ?", into.id, from.id); err != nil {
		return err
	}
	if err := deleteVehicle(tx, from.id); err != nil {
		return err
	}
	if _, err := tx.Exec("INSERT OR IGNORE INTO vehicle_aliases (alias_hash, vehicle_id) VALUES (?, ?)", fromHash, into.id); err != nil {
		return err
	}
	if from.model != into.model {
		capacity, ok, err := vehicleCapacity(tx, into.id)
		if err != nil {
			return err
		}
		if model, changed := refineModel(into.model, from.model, capacity, ok); changed {
			if _, err := tx.Exec("UPDATE vehicles SET model = ? WHERE id = ?", model, into.id); err != nil {
				return err
			}
			rep.ModelChanges = append(rep.ModelChanges, fmt.Sprintf("vehicle %d: %q → %q", into.id, into.model, model))
			into.model = model
		}
	}
	rep.Merges = append(rep.Merges, fmt.Sprintf("vehicle %d → %d: %d duplicate sessions removed, %d sessions moved", from.id, into.id, dup, moved))
	return nil
}

func loadVehicleSessions(q queryer, vehicleID int64) ([]repairSession, error) {
	rows, err := q.Query(`SELECT rowid, vehicle_id, CAST(strftime('%s', start_time) AS INTEGER),
		soc_start, soc_end, energy_from_grid FROM sessions WHERE vehicle_id = ? ORDER BY rowid`, vehicleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []repairSession
	for rows.Next() {
		var s repairSession
		var a, b, e float64
		if err := rows.Scan(&s.rowid, &s.vehicleID, &s.start, &a, &b, &e); err != nil {
			return nil, err
		}
		s.fp = sessionFingerprint(time.Unix(s.start, 0), a, b, e)
		out = append(out, s)
	}
	return out, rows.Err()
}

func dedupeWithinVehicles(tx *sql.Tx, rep *RepairReport) error {
	sessions, err := loadRepairSessions(tx)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, s := range sessions {
		key := fmt.Sprintf("%d|%s", s.vehicleID, s.fp)
		if seen[key] {
			if _, err := tx.Exec("DELETE FROM sessions WHERE rowid = ?", s.rowid); err != nil {
				return err
			}
			rep.SessionsDeduped++
			continue
		}
		seen[key] = true
	}
	res, err := tx.Exec(`DELETE FROM battery_health WHERE id IN (
		SELECT b.id FROM battery_health a JOIN battery_health b
		ON a.vehicle_id = b.vehicle_id AND a.id < b.id
		AND a.date = b.date
		AND ABS(a.soc_change - b.soc_change) < 0.01)`)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	rep.BatteryHealthDeduped += int(n)
	return nil
}
