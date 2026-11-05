package database

import (
	"encoding/json"
	"math"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

// car returns n points for one vehicle, starting at startKm and losing
// lossPer10k kWh every 10 000 km, with one point every 1000 km and 10 days.
func car(id int64, model string, start, startKm, lossPer10k float64, n int) []BatteryHealthPoint {
	var pts []BatteryHealthPoint
	for i := 0; i < n; i++ {
		km := startKm + float64(i)*1000
		pts = append(pts, BatteryHealthPoint{
			VehicleID: id,
			Model:     model,
			Date:      t0.AddDate(0, 0, 10*i),
			Capacity:  start - lossPer10k*(km-startKm)/10000,
			SocChange: 50,
			Mileage:   km,
		})
	}
	return pts
}

func approx(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s = %.3f, want %.3f ± %.3f", name, got, want, tol)
	}
}

func TestComputeBaselineUsesEarliestMileageWindow(t *testing.T) {
	pts := []BatteryHealthPoint{
		{Mileage: 50000, Capacity: 70, Date: t0},
		{Mileage: 1000, Capacity: 80, Date: t0.AddDate(0, 1, 0)},
		{Mileage: 1500, Capacity: 82, Date: t0.AddDate(0, 2, 0)},
		{Mileage: 2000, Capacity: 81, Date: t0.AddDate(0, 3, 0)},
		{Mileage: 40000, Capacity: 72, Date: t0.AddDate(0, 4, 0)},
		{Mileage: 45000, Capacity: 71, Date: t0.AddDate(0, 5, 0)},
	}
	// Window holds 3 points, so the first 5 by mileage are used: 80, 81, 82, 72, 71.
	b, ok := computeBaseline(pts)
	if !ok {
		t.Fatal("expected baseline")
	}
	approx(t, "baseline", b, 80, 1e-9)

	// A dense window: all points within 3000 km are used, even more than 5.
	dense := car(1, "m", 80, 100, 0, 4) // 100..3100 km → 4 points, all 80
	dense = append(dense, car(1, "m", 60, 3200, 0, 6)...)
	b, _ = computeBaseline(dense)
	approx(t, "dense baseline", b, 80, 1e-9)
}

func TestComputeBaselineSparseAndNoMileage(t *testing.T) {
	if _, ok := computeBaseline(car(1, "m", 80, 0, 0, 2)); ok {
		t.Error("2 points must not give a baseline")
	}
	pts := []BatteryHealthPoint{
		{Capacity: 60, Date: t0.AddDate(1, 0, 0)},
		{Capacity: 80, Date: t0},
		{Capacity: 79, Date: t0.AddDate(0, 1, 0)},
		{Capacity: 81, Date: t0.AddDate(0, 2, 0)},
	}
	b, ok := computeBaseline(pts)
	if !ok {
		t.Fatal("expected baseline without mileage")
	}
	// First 5 by date: all four → median(60,79,80,81) = 79.5
	approx(t, "baseline", b, 79.5, 1e-9)
}

func TestClusterBaselines(t *testing.T) {
	got := clusterBaselines([]float64{81, 64, 79.5, 44, 66, 83, 77})
	want := []int{2, 1, 2, 0, 1, 2, 2}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("clusters = %v, want %v", got, want)
		}
	}
	single := clusterBaselines([]float64{78.5, 80.3, 82.5, 83.2})
	for _, c := range single {
		if c != 0 {
			t.Fatalf("expected one cluster, got %v", single)
		}
	}
}

func TestAnalyzeSplitsPackVariantsAndLateJoinerStays(t *testing.T) {
	var pts []BatteryHealthPoint
	pts = append(pts, car(1, "BMW i4", 81, 500, 1, 20)...)
	pts = append(pts, car(2, "BMW i4", 82, 300, 1, 20)...)
	pts = append(pts, car(3, "BMW i4", 66, 800, 1, 20)...)
	pts = append(pts, car(4, "BMW i4", 65, 400, 1, 20)...)
	// Joins late with an already degraded pack: must stay in the big cohort.
	pts = append(pts, car(5, "BMW i4", 77, 60000, 1, 20)...)
	// One point only: sparse, assigned to the nearest cohort.
	pts = append(pts, car(6, "BMW i4", 80, 20000, 0, 1)...)
	// One point far from every cohort: unassigned.
	pts = append(pts, car(7, "BMW i4", 30, 20000, 0, 1)...)
	pts = append(pts, car(8, "BMW iX1", 64, 100, 1, 10)...)

	res := AnalyzeBatteryCohorts(pts)
	if len(res.Cohorts) != 3 {
		t.Fatalf("cohorts = %+v, want 3", res.Cohorts)
	}
	small, big, ix1 := res.Cohorts[0], res.Cohorts[1], res.Cohorts[2]
	if small.Model != "BMW i4" || small.VehicleCount != 2 || small.MedianBaseline > 67 {
		t.Errorf("small cohort = %+v", small)
	}
	if big.VehicleCount != 3 || big.SparseCount != 1 || big.MedianBaseline < 80 {
		t.Errorf("big cohort = %+v", big)
	}
	if ix1.Model != "BMW iX1" || ix1.VehicleCount != 1 {
		t.Errorf("iX1 cohort = %+v", ix1)
	}

	unassigned := 0
	for _, p := range res.Points {
		if p.Cohort == unassignedCohortID {
			unassigned++
			if p.PctOfBaseline != nil {
				t.Error("unassigned point must not have pct")
			}
		}
		if p.Mileage == 60000 {
			if p.Cohort != big.ID {
				t.Errorf("late joiner in cohort %d, want %d", p.Cohort, big.ID)
			}
			approx(t, "late joiner first pct", *p.PctOfBaseline, 100, 0.5)
			approx(t, "late joiner km since first", *p.KmSinceFirst, 0, 1e-9)
		}
		if p.Mileage == 20000 && p.Capacity == 80 && p.PctOfBaseline != nil {
			t.Error("sparse car must not have pct")
		}
	}
	if unassigned != 1 {
		t.Errorf("unassigned points = %d, want 1", unassigned)
	}
}

func TestAnalyzeRelativeValues(t *testing.T) {
	pts := car(1, "m", 80, 10000, 2, 21) // loses 2 kWh per 10 000 km, 20 000 km total
	res := AnalyzeBatteryCohorts(pts)
	last := res.Points[len(res.Points)-1]
	// Baseline = median of 10000..13000 km window = 4 points (79.4..80) and min 5 points → median of first 5.
	b := 80 - 2*0.2 // 5 points at 0,1,2,3,4 thousand km → median at 2000 km
	approx(t, "pct", *last.PctOfBaseline, (80-4)/b*100, 1e-9)
	approx(t, "km since first", *last.KmSinceFirst, 20000, 1e-9)
	approx(t, "months since first", *last.MonthsSinceFirst, 200/daysPerMonth, 1e-9)
}

func TestTrendsMedianPerVehicleAndMinVehicles(t *testing.T) {
	var pts []BatteryHealthPoint
	// Car 1 uploads many points, car 2 and 3 few. Per-vehicle averaging means
	// car 1 counts once per bin.
	for i := 0; i < 50; i++ {
		pts = append(pts, BatteryHealthPoint{VehicleID: 1, Model: "m", Date: t0, Mileage: 1000 + float64(i), Capacity: 70, SocChange: 50})
	}
	pts = append(pts,
		BatteryHealthPoint{VehicleID: 2, Model: "m", Date: t0, Mileage: 1200, Capacity: 72, SocChange: 50},
		BatteryHealthPoint{VehicleID: 2, Model: "m", Date: t0, Mileage: 1300, Capacity: 72, SocChange: 50},
		BatteryHealthPoint{VehicleID: 2, Model: "m", Date: t0, Mileage: 1400, Capacity: 72, SocChange: 50},
		BatteryHealthPoint{VehicleID: 3, Model: "m", Date: t0, Mileage: 1200, Capacity: 74, SocChange: 50},
		BatteryHealthPoint{VehicleID: 3, Model: "m", Date: t0, Mileage: 1300, Capacity: 74, SocChange: 50},
		BatteryHealthPoint{VehicleID: 3, Model: "m", Date: t0, Mileage: 1400, Capacity: 74, SocChange: 50},
		// Only car 3 reaches this bin: hidden (fewer than 2 cars).
		BatteryHealthPoint{VehicleID: 3, Model: "m", Date: t0, Mileage: 30000, Capacity: 73, SocChange: 50},
	)
	res := AnalyzeBatteryCohorts(pts)
	var kmTrend []CohortTrendPoint
	for _, tr := range res.Trends {
		if tr.Metric == "kwh" && tr.Axis == "km" {
			kmTrend = append(kmTrend, tr)
		}
	}
	if len(kmTrend) != 1 {
		t.Fatalf("kwh/km trend = %+v, want 1 bin", kmTrend)
	}
	approx(t, "median", kmTrend[0].Value, 72, 1e-9)
	if kmTrend[0].Vehicles != 3 || kmTrend[0].Points != 56 {
		t.Errorf("bin = %+v", kmTrend[0])
	}
	approx(t, "bin centre", kmTrend[0].X, 2500, 1e-9)

	found := map[string]bool{}
	for _, tr := range res.Trends {
		found[tr.Metric+"/"+tr.Axis] = true
		if tr.Metric == "kwh" && tr.Axis == "time" && tr.Month != "2024-01" {
			t.Errorf("month = %q", tr.Month)
		}
	}
	for _, k := range []string{"kwh/km", "kwh/time", "pct/km", "pct/time"} {
		if !found[k] {
			t.Errorf("missing trend %s", k)
		}
	}
}

func TestCohortJSONHasNoVehicleID(t *testing.T) {
	res := AnalyzeBatteryCohorts(append(car(424242, "m", 80, 100, 1, 5), car(737373, "m", 81, 100, 1, 5)...))
	b, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, bad := range []string{"424242", "737373", "vehicle_id", "VehicleID"} {
		if strings.Contains(s, bad) {
			t.Errorf("JSON leaks %q", bad)
		}
	}
}

func TestGetBatteryCohortsFromDatabase(t *testing.T) {
	m, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	empty, err := m.GetBatteryCohorts("BMW i4")
	if err != nil || len(empty.Points) != 0 || empty.Cohorts == nil {
		t.Fatalf("empty = %+v, %v", empty, err)
	}

	for i, model := range []string{"BMW i4", "BMW i4", "BMW iX"} {
		res, err := m.db.Exec("INSERT INTO vehicles (fin_hash, model, created_at) VALUES (?, ?, ?)", "h"+string(rune('a'+i)), model, t0)
		if err != nil {
			t.Fatal(err)
		}
		vid, _ := res.LastInsertId()
		for _, p := range car(vid, model, 80+float64(i), 500, 1, 5) {
			if _, err := m.db.Exec(`INSERT INTO battery_health (vehicle_id, date, estimated_capacity, soc_change, is_raw_data, mileage) VALUES (?, ?, ?, ?, ?, ?)`,
				vid, p.Date, p.Capacity, p.SocChange, false, p.Mileage); err != nil {
				t.Fatal(err)
			}
		}
	}
	res, err := m.GetBatteryCohorts("BMW i4")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Cohorts) != 1 || res.Cohorts[0].VehicleCount != 2 || len(res.Points) != 10 {
		t.Fatalf("result = %+v", res.Cohorts)
	}
}

func TestSelectedPackGroupsAndLegacyMatching(t *testing.T) {
	var pts []BatteryHealthPoint
	// Owners selected the pack group.
	pts = append(pts, car(1, "BMW i4 80.7 kWh", 81, 500, 1, 10)...)
	pts = append(pts, car(2, "BMW i4 67.1 kWh", 66, 500, 1, 10)...)
	// Legacy family-only cars: one matches the 80.7 kWh pack, one the 67.1 kWh
	// pack (no group member needed), one fits no pack.
	pts = append(pts, car(3, "BMW i4", 82, 500, 1, 10)...)
	pts = append(pts, car(4, "BMW i4", 44, 500, 1, 10)...)
	// Selected group but too few points: stays in its group as sparse.
	pts = append(pts, car(5, "BMW i4 80.7 kWh", 79, 500, 0, 1)...)

	res := AnalyzeBatteryCohorts(pts)
	if len(res.Cohorts) != 2 {
		t.Fatalf("cohorts = %+v", res.Cohorts)
	}
	// The 44 kWh car fits no i4 battery: it is excluded, not an own cohort.
	if len(res.Excluded) != 1 || res.Excluded[0].Model != "BMW i4" || res.Excluded[0].Vehicles != 1 {
		t.Errorf("excluded = %+v", res.Excluded)
	}
	small, big := res.Cohorts[0], res.Cohorts[1]
	if small.Pack != "BMW i4 67.1 kWh" || small.Source != "selected" || small.VehicleCount != 1 {
		t.Errorf("small = %+v", small)
	}
	if big.Pack != "BMW i4 80.7 kWh" || big.Source != "selected" || big.VehicleCount != 2 || big.SparseCount != 1 {
		t.Errorf("big = %+v", big)
	}
	if big.Label != "eDrive40 / xDrive40 / M50 / M60 (80.7 kWh net)" {
		t.Errorf("label = %q", big.Label)
	}
	for _, c := range res.Cohorts {
		if c.Model != "BMW i4" {
			t.Errorf("cohort model = %q, want family", c.Model)
		}
	}
	for _, p := range res.Points {
		if p.Model != "BMW i4" {
			t.Errorf("point model = %q, want family", p.Model)
		}
	}

	// Only legacy cars: the 66 kWh cluster is matched to the catalog pack.
	res = AnalyzeBatteryCohorts(append(car(3, "BMW i4", 66, 500, 1, 10), car(4, "BMW i4", 65, 500, 1, 10)...))
	if len(res.Cohorts) != 1 || res.Cohorts[0].Source != "matched" || res.Cohorts[0].Pack != "BMW i4 67.1 kWh" {
		t.Errorf("legacy only = %+v", res.Cohorts)
	}
}

func TestFamilyFilterIncludesLegacyAndGroups(t *testing.T) {
	m, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	for i, model := range []string{"BMW i4", "BMW i4 80.7 kWh", "BMW iX1 64.7 kWh", "Mini Cooper SE"} {
		res, err := m.db.Exec("INSERT INTO vehicles (fin_hash, model, created_at) VALUES (?, ?, ?)", "f"+string(rune('a'+i)), model, t0)
		if err != nil {
			t.Fatal(err)
		}
		vid, _ := res.LastInsertId()
		capacity := map[string]float64{"BMW iX1 64.7 kWh": 64, "Mini Cooper SE": 49}[model]
		if capacity == 0 {
			capacity = 80
		}
		for _, p := range car(vid, model, capacity, 500, 1, 4) {
			if _, err := m.db.Exec(`INSERT INTO battery_health (vehicle_id, date, estimated_capacity, soc_change, is_raw_data, mileage) VALUES (?, ?, ?, ?, ?, ?)`,
				vid, p.Date, p.Capacity, p.SocChange, false, p.Mileage); err != nil {
				t.Fatal(err)
			}
		}
	}
	families, err := m.GetAvailableModels()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(families, ",") != "BMW i4,BMW iX1,Mini Cooper" {
		t.Errorf("families = %v", families)
	}
	raw, err := m.GetFleetBatteryHealth("BMW i4")
	if err != nil || len(raw) != 8 {
		t.Errorf("i4 raw = %d, %v", len(raw), err)
	}
	trend, err := m.GetMonthlyBatteryHealthTrend("BMW i4")
	if err != nil || len(trend) != 1 || trend[0]["data_points"] != 8 {
		t.Errorf("i4 monthly = %d, %v", len(trend), err)
	}
	none, err := m.GetFleetBatteryHealth("BMW i7")
	if err != nil || len(none) != 0 {
		t.Errorf("i7 raw = %d, %v", len(none), err)
	}
	coh, err := m.GetBatteryCohorts("Mini Cooper")
	if err != nil || len(coh.Points) != 4 {
		t.Errorf("mini cohorts = %d points, %v", len(coh.Points), err)
	}
}

func TestOdometerResetSplitsSeries(t *testing.T) {
	// One identity: a 80 kWh car up to 90,000 km, then a 65 kWh car that
	// starts again at 700 km.
	first := car(1, "BMW i4", 80, 10000, 1, 10)
	for i := range first {
		first[i].Mileage = 10000 + float64(i)*8000
	}
	second := car(1, "BMW i4", 65, 700, 1, 10)
	for i := range second {
		second[i].Date = second[i].Date.AddDate(3, 0, 0)
	}
	res := AnalyzeBatteryCohorts(append(first, second...))
	if len(res.Cohorts) != 2 {
		t.Fatalf("cohorts = %+v", res.Cohorts)
	}
	for _, c := range res.Cohorts {
		if c.VehicleCount != 1 {
			t.Errorf("cohort %s has %d cars", c.Label, c.VehicleCount)
		}
	}
	for _, p := range res.Points {
		if p.PctOfBaseline == nil || *p.PctOfBaseline < 95 || *p.PctOfBaseline > 105 {
			t.Fatalf("point %+v not near its own baseline", p)
		}
	}
}

func TestQuantile(t *testing.T) {
	v := []float64{4, 1, 3, 2}
	approx(t, "p25", quantile(v, 0.25), 1.75, 1e-9)
	approx(t, "p50", quantile(v, 0.5), 2.5, 1e-9)
	approx(t, "p75", quantile(v, 0.75), 3.25, 1e-9)
	approx(t, "single", quantile([]float64{7}, 0.75), 7, 1e-9)
}

func TestImplausibleBatteryIsExcluded(t *testing.T) {
	var pts []BatteryHealthPoint
	pts = append(pts, car(1, "BMW i4", 80, 500, 1, 10)...)
	pts = append(pts, car(2, "BMW i4", 79, 500, 1, 10)...)
	// 44 kWh is far from every i4 battery (67.1 and 80.7 kWh).
	pts = append(pts, car(3, "BMW i4", 44, 500, 1, 10)...)
	// 72 kWh is 10 % below 80.7 kWh: plausible (degraded or late joiner).
	pts = append(pts, car(4, "BMW i4", 72, 500, 1, 10)...)
	// Families without known batteries accept every value.
	pts = append(pts, car(5, "Other BMW EV", 20, 500, 1, 10)...)

	res := AnalyzeBatteryCohorts(pts)
	if len(res.Excluded) != 1 || res.Excluded[0].Vehicles != 1 || res.Excluded[0].Reason == "" {
		t.Fatalf("excluded = %+v", res.Excluded)
	}
	cars := 0
	for _, c := range res.Cohorts {
		cars += c.VehicleCount
	}
	if cars != 4 {
		t.Errorf("cars in cohorts = %d, want 4", cars)
	}
	for _, p := range res.Points {
		if p.Capacity < 50 && p.Model == "BMW i4" {
			t.Fatalf("excluded car point in output: %+v", p)
		}
	}
	if plausibleForFamily("BMW i4", 44) || !plausibleForFamily("BMW i4", 70) || !plausibleForFamily("Unknown", 5) {
		t.Error("plausibleForFamily")
	}
}

func pctKmTrend(res BatteryCohortResult, cohort int) []CohortTrendPoint {
	var out []CohortTrendPoint
	for _, tr := range res.Trends {
		if tr.Cohort == cohort && tr.Metric == "pct" && tr.Axis == "km" {
			out = append(out, tr)
		}
	}
	return out
}

func TestTrendBinsWithFewCarsAreNotReliable(t *testing.T) {
	var pts []BatteryHealthPoint
	for id := int64(1); id <= 6; id++ {
		pts = append(pts, car(id, "m", 80, 500, 0.8, 20)...)
	}
	// Only 3 cars reach 25,000 km and more.
	for id := int64(7); id <= 9; id++ {
		pts = append(pts, car(id, "m", 80, 500, 0.8, 40)...)
	}
	res := AnalyzeBatteryCohorts(pts)
	seen := 0
	for _, tr := range pctKmTrend(res, res.Cohorts[0].ID) {
		switch {
		case tr.X == 7500:
			seen++
			if tr.Vehicles != 9 || !tr.Reliable {
				t.Errorf("bin 7500 = %+v", tr)
			}
		case tr.X >= 27500:
			seen++
			if tr.Vehicles != 3 || tr.Reliable {
				t.Errorf("tail bin = %+v", tr)
			}
		}
	}
	if seen < 3 {
		t.Errorf("bins checked = %d", seen)
	}
}

func TestPctTrendLeavesOutBaselineWindow(t *testing.T) {
	var pts []BatteryHealthPoint
	for id := int64(1); id <= 5; id++ {
		pts = append(pts, car(id, "m", 80, 500, 2, 15)...)
	}
	res := AnalyzeBatteryCohorts(pts)
	tr := pctKmTrend(res, res.Cohorts[0].ID)
	if len(tr) == 0 || tr[0].X != 0 || tr[0].Value != 100 || tr[0].Vehicles != 5 || !tr[0].Reliable {
		t.Fatalf("anchor = %+v", tr)
	}
	for _, b := range tr[1:] {
		// The first 5 points (0..4000 km) form the baseline: bin 0-5000 km
		// is empty and every later bin is below 100 %.
		if b.X == 2500 {
			t.Errorf("baseline bin in trend: %+v", b)
		}
		if b.Value >= 100 {
			t.Errorf("bin %v = %.2f, want < 100", b.X, b.Value)
		}
	}
	for _, b := range res.Trends {
		if b.Metric == "pct" && b.Axis == "time" && b.X > 0 && b.X != 3 && b.X != 9 {
			t.Errorf("time bin centre %v, want 6-month bins", b.X)
		}
	}
}

func TestCohortSlopePerCar(t *testing.T) {
	var pts []BatteryHealthPoint
	// 0.8 kWh per 10,000 km on an ~80 kWh pack: ≈ -1 % per 10,000 km.
	// 40 points over 39,000 km and 390 days.
	for id := int64(1); id <= 3; id++ {
		pts = append(pts, car(id, "m", 80, 500, 0.8, 40)...)
	}
	// Too short (14,000 km): not in the slope.
	pts = append(pts, car(4, "m", 80, 500, 8, 15)...)
	res := AnalyzeBatteryCohorts(pts)
	c := res.Cohorts[0]
	if c.SlopePer10kKm == nil || c.SlopeKmCars != 3 {
		t.Fatalf("cohort = %+v", c)
	}
	approx(t, "per 10k km", *c.SlopePer10kKm, -0.8/79.84*100, 0.01)
	// 100 km per day: 36,525 km per year.
	if c.SlopePerYear == nil || c.SlopeYearCars != 3 {
		t.Fatalf("per year missing: %+v", c)
	}
	approx(t, "per year", *c.SlopePerYear, -0.8/79.84*100*3.6525, 0.02)

	short := AnalyzeBatteryCohorts(car(5, "m", 80, 500, 0.8, 15))
	if short.Cohorts[0].SlopePer10kKm != nil {
		t.Error("slope from a short car")
	}
}
