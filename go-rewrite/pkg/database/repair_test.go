package database

import (
	"fmt"
	"testing"
	"time"
)

func insertVehicle(t *testing.T, m *Manager, hash, kind string) int64 {
	t.Helper()
	var k any
	if kind != "" {
		k = kind
	}
	res, err := m.db.Exec("INSERT INTO vehicles (fin_hash, model, created_at, key_kind) VALUES (?, 'BMW i4', ?, ?)", hash, time.Now(), k)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return id
}

// insertDays stores one session per day in [from, to). prefix "" gives
// legacy unprefixed ids. socStart makes fingerprints differ between cars.
func insertDays(t *testing.T, m *Manager, vid int64, prefix string, from, to int, socStart float64) {
	t.Helper()
	base := time.Date(2025, 1, 1, 8, 0, 0, 0, time.UTC)
	for d := from; d < to; d++ {
		start := base.Add(time.Duration(d) * 24 * time.Hour)
		id := fmt.Sprintf("%d", start.Unix())
		if prefix != "" {
			id = prefix + "_" + id
		}
		if _, err := m.db.Exec(`INSERT INTO sessions (id, vehicle_id, start_time, end_time, soc_start, soc_end, energy_from_grid, energy_added_hvb, session_time_minutes)
			VALUES (?, ?, ?, ?, ?, 80, 40, 38, 60)`, id, vid, start, start.Add(time.Hour), socStart); err != nil {
			t.Fatal(err)
		}
	}
}

func insertPoint(t *testing.T, m *Manager, vid int64, day int, km float64) {
	t.Helper()
	date := time.Date(2025, 1, 1, 8, 0, 0, 0, time.UTC).Add(time.Duration(day) * 24 * time.Hour)
	if _, err := m.db.Exec(`INSERT INTO battery_health (vehicle_id, date, estimated_capacity, soc_change, is_raw_data, mileage)
		VALUES (?, ?, 78, 60, 0, ?)`, vid, date, km); err != nil {
		t.Fatal(err)
	}
}

func TestRepairFleet(t *testing.T) {
	m := testManager(t)

	// Legacy vehicle fully contained in a newer VIN vehicle.
	legacy := insertVehicle(t, m, "legacy-hash-0000000", "")
	insertDays(t, m, legacy, "", 0, 25, 10)
	insertPoint(t, m, legacy, 3, 1000)
	vin := insertVehicle(t, m, "vin-hash-000000000000", keyKindVIN)
	insertDays(t, m, vin, "vinprefix0000000", 0, 40, 10)
	insertPoint(t, m, vin, 3, 1000)

	// Vehicle with only an epoch-0 session.
	epoch := insertVehicle(t, m, "epoch-hash-00000000", "")
	if _, err := m.db.Exec(`INSERT INTO sessions (id, vehicle_id, start_time, end_time, soc_start, soc_end, energy_from_grid, energy_added_hvb)
		VALUES ('e_0', ?, ?, ?, 0, 0, 0, 0)`, epoch, time.Unix(0, 0).UTC(), time.Unix(0, 0).UTC()); err != nil {
		t.Fatal(err)
	}

	// Sessions with a missing (1970) end time.
	start := time.Date(2025, 3, 1, 10, 0, 0, 0, time.UTC)
	if _, err := m.db.Exec(`INSERT INTO sessions (id, vehicle_id, start_time, end_time, soc_start, soc_end, energy_from_grid, energy_added_hvb, session_time_minutes) VALUES
		('vinprefix0000000_a', ?, ?, ?, 5, 50, 30, 28, 30), ('vinprefix0000000_b', ?, ?, ?, 6, 50, 30, 28, 0)`,
		vin, start, time.Unix(0, 0).UTC(), vin, start.Add(time.Hour), time.Unix(0, 0).UTC()); err != nil {
		t.Fatal(err)
	}

	// Legacy vehicle that holds two cars: the odometer drops after day 110.
	two := insertVehicle(t, m, "two-cars-hash-00000", "")
	insertDays(t, m, two, "", 100, 111, 30)
	insertDays(t, m, two, "", 130, 141, 31)
	for d := 100; d < 111; d++ {
		insertPoint(t, m, two, d, 50000+float64(d))
	}
	for d := 130; d < 141; d++ {
		insertPoint(t, m, two, d, 500+float64(d))
	}
	// The second car is also stored on its own (content key).
	second := insertVehicle(t, m, "second-car-hash-000", keyKindContent)
	insertDays(t, m, second, "secondprefix0000", 130, 141, 31)

	rep, err := m.RepairFleet()
	if err != nil {
		t.Fatal(err)
	}
	if rep.Before.DuplicateSessions != 36 || rep.After.DuplicateSessions != 0 {
		t.Errorf("duplicates before/after = %d/%d", rep.Before.DuplicateSessions, rep.After.DuplicateSessions)
	}
	if rep.EpochVehiclesDeleted != 1 || rep.After.EpochSessions != 0 {
		t.Errorf("epoch: deleted %d, left %d", rep.EpochVehiclesDeleted, rep.After.EpochSessions)
	}
	if rep.EndTimesFixed != 1 || rep.EndTimesCleared != 1 {
		t.Errorf("end times fixed/cleared = %d/%d", rep.EndTimesFixed, rep.EndTimesCleared)
	}
	if len(rep.Splits) != 1 {
		t.Errorf("splits = %v", rep.Splits)
	}

	if n := count(t, m, "SELECT COUNT(*) FROM vehicles WHERE id IN (?, ?)", legacy, epoch); n != 0 {
		t.Errorf("legacy or epoch vehicle still present")
	}
	if n := count(t, m, "SELECT COUNT(*) FROM sessions WHERE vehicle_id = ?", vin); n != 42 {
		t.Errorf("vin vehicle sessions = %d, want 42", n)
	}
	if n := count(t, m, "SELECT COUNT(*) FROM battery_health WHERE vehicle_id = ?", vin); n != 1 {
		t.Errorf("vin vehicle points = %d, want 1 (duplicate point removed)", n)
	}
	if n := count(t, m, "SELECT COUNT(*) FROM vehicle_aliases WHERE alias_hash = 'legacy-hash-0000000'"); n != 1 {
		t.Errorf("legacy hash is not an alias")
	}
	// The first car keeps its part; the second part joined the second car.
	if n := count(t, m, "SELECT COUNT(*) FROM sessions WHERE vehicle_id = ?", two); n != 11 {
		t.Errorf("first car sessions = %d, want 11", n)
	}
	if n := count(t, m, "SELECT COUNT(*) FROM battery_health WHERE vehicle_id = ? AND mileage < 1000", two); n != 0 {
		t.Errorf("first car still has low-km points")
	}
	if n := count(t, m, "SELECT COUNT(*) FROM sessions WHERE vehicle_id = ?", second); n != 11 {
		t.Errorf("second car sessions = %d, want 11", n)
	}
	if n := count(t, m, "SELECT COUNT(*) FROM battery_health WHERE vehicle_id = ?", second); n != 11 {
		t.Errorf("second car points = %d, want 11", n)
	}
	var end string
	if err := m.db.QueryRow("SELECT CAST(end_time AS TEXT) FROM sessions WHERE id = 'vinprefix0000000_a'").Scan(&end); err != nil {
		t.Fatal(err)
	}
	if end != "2025-03-01 10:30:00+00:00" {
		t.Errorf("fixed end_time = %q", end)
	}

	// A second run finds nothing to do.
	again, err := m.RepairFleet()
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Merges)+len(again.Splits)+again.EpochVehiclesDeleted+again.EndTimesFixed+again.EndTimesCleared+again.BatteryHealthDeduped != 0 {
		t.Errorf("second run changed data: %+v", again)
	}
	if again.Before != again.After {
		t.Errorf("second run counts changed: %+v → %+v", again.Before, again.After)
	}
}

func TestFindResetNeedsEnoughPoints(t *testing.T) {
	pts := []odoPoint{{1, 50000}, {2, 50100}, {3, 500}, {4, 600}}
	if _, _, ok := findReset(pts); ok {
		t.Fatal("2 points after the drop must not split")
	}
	pts = append(pts, odoPoint{5, 700})
	b, a, ok := findReset(pts)
	if !ok || b.mileage != 50100 || a.mileage != 500 {
		t.Fatalf("reset = %v %v %v", b, a, ok)
	}
	if cut := splitCut([]int64{2, 3, 10, 11}, odoPoint{date: 2}, odoPoint{date: 11}); cut != 10 {
		t.Fatalf("cut = %d, want 10", cut)
	}
}
