package database

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/awlx/bmwtools/pkg/data"
)

func TestGetProviderStatsMergesStoredRawNames(t *testing.T) {
	m, err := New(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	var sessions []data.Session
	for i, p := range []string{"HPC IONITY", "HPC/DC IONITY GmbH", "IONITY", "Unknown", "", "AC Charging operator", "HPC/AC EnBW mobility+"} {
		sessions = append(sessions, data.Session{
			ID: fmt.Sprint(i + 1), StartTime: base.Add(time.Duration(i) * time.Hour), EndTime: base.Add(time.Duration(i)*time.Hour + 30*time.Minute),
			SocStart: 20, SocEnd: 80, EnergyFromGrid: 40, EnergyAddedHvb: 38, Efficiency: 0.95, Provider: p,
		})
	}
	if _, err := m.StoreSessions(sessions, "BMW-CarData-Ladehistorie_WBA00000000000001_2025.json", "BMW i4"); err != nil {
		t.Fatal(err)
	}

	stats, err := m.GetProviderStats()
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 2 {
		t.Fatalf("got %d providers %v, want 2", len(stats), stats)
	}
	if stats[0]["provider"] != "IONITY" || stats[0]["total_sessions"] != 3 {
		t.Errorf("got %v, want IONITY with 3 sessions", stats[0])
	}
	if stats[1]["provider"] != "EnBW" {
		t.Errorf("got %v, want EnBW", stats[1])
	}

	total, _, _, _, err := m.GetSessionTotals()
	if err != nil {
		t.Fatal(err)
	}
	if total != len(sessions) {
		t.Errorf("totals: got %d sessions, want %d (unknown operators still count)", total, len(sessions))
	}
}

func TestProviderStatsLeaveOutPlugIns(t *testing.T) {
	m, err := New(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	mk := func(i int, energy, socEnd float64) data.Session {
		return data.Session{
			ID: fmt.Sprint(i), StartTime: base.Add(time.Duration(i) * time.Hour), EndTime: base.Add(time.Duration(i)*time.Hour + time.Minute),
			SocStart: 50, SocEnd: socEnd, EnergyFromGrid: energy, Provider: "IONITY", Failed: socEnd-50 < 1,
		}
	}
	sessions := []data.Session{mk(1, 40, 90), mk(2, 0, 50), mk(3, 0, 50), mk(4, 0.4, 50)}
	if _, err := m.StoreSessions(sessions, "BMW-CarData-Ladehistorie_WBA00000000000001_2025.json", "BMW i4"); err != nil {
		t.Fatal(err)
	}
	stats, err := m.GetProviderStats()
	if err != nil {
		t.Fatal(err)
	}
	// Two plug-ins without energy and SoC gain are left out. The session with
	// energy but no SoC gain is a real failure and stays.
	if len(stats) != 1 || stats[0]["total_sessions"] != 2 || stats[0]["failed_sessions"] != 1 {
		t.Fatalf("stats = %v", stats)
	}

	dm := data.NewManager()
	dm.SetSessions(sessions)
	g := dm.GroupProviders()["all_providers"].([]data.ProviderStats)
	if len(g) != 1 || g[0].Total != 2 || g[0].FailedCount != 1 {
		t.Fatalf("grouped = %+v", g)
	}
}
