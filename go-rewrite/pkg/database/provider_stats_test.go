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
