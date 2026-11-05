package database

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/awlx/bmwtools/pkg/data"
)

func testManager(t *testing.T) *Manager {
	t.Helper()
	m, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.Close() })
	return m
}

// testSessions returns n sessions, one per day from day `from`.
func testSessions(from, n int) []data.Session {
	base := time.Date(2025, 1, 1, 8, 0, 0, 0, time.UTC)
	out := make([]data.Session, 0, n)
	for i := from; i < from+n; i++ {
		start := base.Add(time.Duration(i) * 24 * time.Hour)
		out = append(out, data.Session{
			ID:             fmt.Sprintf("%d", start.Unix()),
			StartTime:      start,
			EndTime:        start.Add(time.Hour),
			SocStart:       20,
			SocEnd:         80,
			EnergyFromGrid: 50 + float64(i%7),
			EnergyAddedHvb: 48,
			Mileage:        float64(1000 + 100*i),
		})
	}
	return out
}

func count(t *testing.T, m *Manager, q string, args ...any) int {
	t.Helper()
	var n int
	if err := m.db.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestContentKeyThenVINArchiveJoinsSameVehicle(t *testing.T) {
	m := testManager(t)
	first := testSessions(0, 30)
	if _, err := m.StoreSessionsForVehicle(first, VehicleKeyFor("export.json", first), "BMW i4"); err != nil {
		t.Fatal(err)
	}
	// A later, larger archive of the same car, now with a VIN in the name.
	larger := testSessions(0, 50)
	vinKey := VehicleKeyFor("WBA12345678901234_cardata.json", larger)
	if !vinKey.FromVIN {
		t.Fatal("VIN not found in filename")
	}
	if _, err := m.StoreSessionsForVehicle(larger, vinKey, "BMW i4"); err != nil {
		t.Fatal(err)
	}
	if n := count(t, m, "SELECT COUNT(*) FROM vehicles"); n != 1 {
		t.Fatalf("vehicles = %d, want 1", n)
	}
	if n := count(t, m, "SELECT COUNT(*) FROM sessions"); n != 50 {
		t.Fatalf("sessions = %d, want 50", n)
	}
	if n := count(t, m, "SELECT COUNT(*) FROM vehicle_aliases WHERE alias_hash = ?", vinKey.Hash); n != 1 {
		t.Fatalf("alias rows = %d", n)
	}
	if n := count(t, m, "SELECT COUNT(*) FROM vehicles WHERE key_kind = 'vin'"); n != 1 {
		t.Fatal("vehicle kind not upgraded to vin")
	}
	// The next upload with the VIN resolves directly and stores nothing new.
	stored, err := m.StoreSessionsForVehicle(larger, vinKey, "BMW i4")
	if err != nil || stored {
		t.Fatalf("second store = %v, %v", stored, err)
	}
	if n := count(t, m, "SELECT COUNT(*) FROM sessions"); n != 50 {
		t.Fatalf("sessions after re-upload = %d", n)
	}
}

func TestLegacyUnprefixedSessionsJoinVINUpload(t *testing.T) {
	m := testManager(t)
	res, err := m.db.Exec("INSERT INTO vehicles (fin_hash, model, created_at) VALUES ('legacyhash000000000', 'BMW i4', ?)", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	vid, _ := res.LastInsertId()
	for _, s := range testSessions(0, 25) {
		if _, err := m.db.Exec(`INSERT INTO sessions (id, vehicle_id, start_time, end_time, soc_start, soc_end, energy_from_grid, energy_added_hvb)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, s.ID, vid, s.StartTime.UTC(), s.EndTime.UTC(), s.SocStart, s.SocEnd, s.EnergyFromGrid, s.EnergyAddedHvb); err != nil {
			t.Fatal(err)
		}
	}
	up := testSessions(0, 40)
	if _, err := m.StoreSessionsForVehicle(up, VehicleKeyFor("WBA12345678901234.json", up), "BMW i4"); err != nil {
		t.Fatal(err)
	}
	if n := count(t, m, "SELECT COUNT(*) FROM vehicles"); n != 1 {
		t.Fatalf("vehicles = %d, want 1", n)
	}
	if n := count(t, m, "SELECT COUNT(*) FROM sessions WHERE vehicle_id = ?", vid); n != 40 {
		t.Fatalf("sessions = %d, want 40 (no fingerprint duplicates)", n)
	}
}

func TestTwoVINVehiclesAreNeverJoined(t *testing.T) {
	m := testManager(t)
	s := testSessions(0, 30)
	if _, err := m.StoreSessionsForVehicle(s, VehicleKeyFor("WBA12345678901234.json", s), "BMW i4"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.StoreSessionsForVehicle(s, VehicleKeyFor("WBA99999999999999.json", s), "BMW i4"); err != nil {
		t.Fatal(err)
	}
	if n := count(t, m, "SELECT COUNT(*) FROM vehicles"); n != 2 {
		t.Fatalf("vehicles = %d, want 2", n)
	}
}

func TestSmallOverlapDoesNotJoin(t *testing.T) {
	m := testManager(t)
	a := testSessions(0, 100)
	if _, err := m.StoreSessionsForVehicle(a, VehicleKeyFor("a.json", a), "BMW i4"); err != nil {
		t.Fatal(err)
	}
	// 5 of 100 sessions overlap: below both thresholds.
	b := testSessions(95, 100)
	if _, err := m.StoreSessionsForVehicle(b, VehicleKeyFor("b.json", b), "BMW i4"); err != nil {
		t.Fatal(err)
	}
	if n := count(t, m, "SELECT COUNT(*) FROM vehicles"); n != 2 {
		t.Fatalf("vehicles = %d, want 2", n)
	}
}

func TestRefineModel(t *testing.T) {
	cases := []struct {
		cur, req string
		cap      float64
		hasCap   bool
		want     string
		changed  bool
	}{
		{"Unknown BMW EV", "BMW i4", 0, false, "BMW i4", true},
		{"BMW i4", "BMW i4 80.7 kWh", 78, true, "BMW i4 80.7 kWh", true},
		{"BMW i4", "BMW i4 80.7 kWh", 0, false, "BMW i4 80.7 kWh", true},
		{"BMW i4", "BMW i4 80.7 kWh", 65, true, "BMW i4", false}, // fits eDrive35 better
		{"BMW i4 80.7 kWh", "BMW i4", 78, true, "BMW i4 80.7 kWh", false},
		{"BMW i4", "BMW iX", 40, true, "BMW i4", false}, // different family, no fit
		{"BMW i4", "BMW i3", 40, true, "BMW i3", true},  // different family, fits i3 120 Ah
		{"BMW i4", "BMW iX", 0, false, "BMW i4", false}, // no capacity, keep
		{"BMW i4", "BMW i4", 78, true, "BMW i4", false},
	}
	for _, c := range cases {
		got, changed := refineModel(c.cur, c.req, c.cap, c.hasCap)
		if got != c.want || changed != c.changed {
			t.Errorf("refineModel(%q, %q, %.0f, %v) = %q, %v; want %q, %v", c.cur, c.req, c.cap, c.hasCap, got, changed, c.want, c.changed)
		}
	}
}

func TestVINSaltFromEnv(t *testing.T) {
	t.Setenv(VINSaltEnv, "")
	def := hashFIN("WBA12345678901234")
	t.Setenv(VINSaltEnv, defaultVINSalt)
	if hashFIN("WBA12345678901234") != def {
		t.Fatal("default salt changed existing hashes")
	}
	t.Setenv(VINSaltEnv, "other")
	if hashFIN("WBA12345678901234") == def {
		t.Fatal("salt from env not used")
	}
}
