package database

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	vehiclecatalog "github.com/awlx/bmwtools/pkg/vehicles"
)

// Cohort analysis splits the cars of one model into pack variants. A model
// name such as "BMW i4" covers several battery sizes, so a plain fleet average
// mixes them and hides real degradation. Each car gets a baseline capacity from
// its earliest data, cars are clustered by that baseline, and every point is
// also expressed relative to its own car's baseline.
const (
	// baselineWindowKm is the mileage window (from a car's first mileage) used
	// to compute its baseline.
	baselineWindowKm = 3000.0
	// baselineMinWindowPoints is the minimum number of earliest points used for
	// the baseline when the mileage window holds fewer points.
	baselineMinWindowPoints = 5
	// minPointsForBaseline is the minimum number of points a car needs before
	// its baseline is trusted for clustering and the relative (%) view.
	minPointsForBaseline = 3
	// cohortGapRatio splits sorted baselines where the relative gap between
	// neighbours is at least this ratio. Within one pack variant neighbouring
	// baselines differ by a few percent at most.
	cohortGapRatio = 0.07

	trendKmBin = 5000.0
	// trendMonthBin is 6 months: one bin holds both warm and cold months, so
	// the seasonal change of charging losses has less effect on the % trend.
	trendMonthBin = 6.0
	// minVehiclesPerBin is the minimum number of cars for a bin in the output.
	minVehiclesPerBin = 2
	// minVehiclesForTrend is the minimum number of cars for a reliable bin.
	// Only reliable bins form the trend line; the page shows others faded.
	minVehiclesForTrend = 5
	// Per-car slopes use only cars with enough points and distance.
	slopeMinPoints = 10
	// minSlopeVehicles is the minimum number of qualifying cars for a slope.
	minSlopeVehicles = 5
	// minAnalysisSocChange is the minimum SoC change (%) of a point used in the
	// analysis. Shorter charges have a large error and read too high (approx.
	// +4.5 % at 30-39 % SoC, under ±1.3 % from 40 % SoC). Stored data is kept.
	minAnalysisSocChange = 40.0
	slopeMinSpanKm       = 20000.0
	// slopeMinSpanMonths is the time span a car needs for the per-year slope,
	// so that a full year of seasons is in the fit.
	slopeMinSpanMonths = 12.0
	daysPerMonth       = 30.4375
	unassignedCohortID = -1
)

// BatteryHealthPoint is one stored capacity estimate. VehicleID is internal and
// must never leave the server.
type BatteryHealthPoint struct {
	VehicleID int64
	Model     string
	Date      time.Time
	Capacity  float64
	SocChange float64
	Mileage   float64
	IsRaw     bool
}

// BatteryCohort describes one pack-variant cohort of a model.
//
// Source is "selected" when owners chose this pack group at upload, "matched"
// when only family-level cars were matched to the pack by their baseline, and
// "auto" when no catalog pack is close (cohort found by gap clustering only).
type BatteryCohort struct {
	ID             int     `json:"id"`
	Model          string  `json:"model"`
	Label          string  `json:"label"`
	Pack           string  `json:"pack,omitempty"`
	NetKWh         float64 `json:"net_kwh,omitempty"`
	Source         string  `json:"source"`
	MedianBaseline float64 `json:"median_baseline"`
	VehicleCount   int     `json:"vehicle_count"`
	SparseCount    int     `json:"sparse_vehicle_count"`
	// Median per-car linear slope in % of start capacity. Nil when fewer
	// than minSlopeVehicles cars qualify; the counts are always set.
	SlopePer10kKm *float64 `json:"slope_pct_per_10k_km,omitempty"`
	SlopeKmCars   int      `json:"slope_km_vehicles"`
	SlopePerYear  *float64 `json:"slope_pct_per_year,omitempty"`
	SlopeYearCars int      `json:"slope_year_vehicles"`
}

// CohortPoint is a capacity estimate annotated with its cohort. It has no
// vehicle identifier.
type CohortPoint struct {
	Cohort           int       `json:"cohort"`
	Model            string    `json:"model"`
	Date             time.Time `json:"date"`
	Mileage          float64   `json:"mileage"`
	Capacity         float64   `json:"estimated_capacity"`
	SocChange        float64   `json:"soc_change"`
	IsRaw            bool      `json:"is_raw_data"`
	PctOfBaseline    *float64  `json:"pct_of_baseline,omitempty"`
	KmSinceFirst     *float64  `json:"km_since_first,omitempty"`
	MonthsSinceFirst *float64  `json:"months_since_first,omitempty"`
}

// CohortTrendPoint is one bin of a per-cohort trend line. Metric is "pct" or
// "kwh", Axis is "km" or "time". For metric "kwh" with axis "time", Month holds
// the calendar month and X is unset.
type CohortTrendPoint struct {
	Cohort   int     `json:"cohort"`
	Metric   string  `json:"metric"`
	Axis     string  `json:"axis"`
	X        float64 `json:"x"`
	Month    string  `json:"month,omitempty"`
	Value    float64 `json:"value"`
	P25      float64 `json:"p25"`
	P75      float64 `json:"p75"`
	Vehicles int     `json:"vehicles"`
	Points   int     `json:"points"`
	// Reliable is true when the bin has at least minVehiclesForTrend cars.
	Reliable bool `json:"reliable"`
}

// BatteryCohortResult is the complete cohort analysis.
type BatteryCohortResult struct {
	Cohorts []BatteryCohort    `json:"cohorts"`
	Points  []CohortPoint      `json:"points"`
	Trends  []CohortTrendPoint `json:"trends"`
	// Excluded counts cars per model whose battery does not fit the model.
	Excluded []ExcludedVehicles `json:"excluded"`
}

// ExcludedVehicles is the number of cars of a model left out of the analysis.
type ExcludedVehicles struct {
	Model    string `json:"model"`
	Vehicles int    `json:"vehicles"`
	Reason   string `json:"reason"`
}

type vehicleStats struct {
	id          int64
	model       string
	family      string
	pack        *vehiclecatalog.PackGroup
	points      []BatteryHealthPoint
	baseline    float64
	hasBaseline bool
	firstKm     float64
	hasKm       bool
	firstDate   time.Time
	cohort      int
	excluded    bool
	window      baselineWindow
}

// baselineWindow marks the points used for a car's baseline. These points are
// near 100 % by construction, so the % trend leaves them out.
type baselineWindow struct {
	byKm    bool
	lastKm  float64
	lastDay time.Time
}

func (w baselineWindow) contains(p BatteryHealthPoint) bool {
	if w.byKm && p.Mileage > 0 {
		return p.Mileage <= w.lastKm
	}
	return !p.Date.After(w.lastDay)
}

// packPlausibilityRatio is the maximum relative distance between a car's start
// capacity and the nearest known pack of its model. Estimates include charging
// losses and degradation, but a larger distance means a wrong model selection.
const packPlausibilityRatio = 0.15

// plausibleForFamily reports whether a start capacity fits any known pack of
// the family. Families without known packs accept every value.
func plausibleForFamily(family string, baseline float64) bool {
	packs := vehiclecatalog.GroupsOf(family)
	if len(packs) == 0 {
		return true
	}
	for _, g := range packs {
		if g.NetKWh > 0 && math.Abs(baseline-g.NetKWh)/g.NetKWh <= packPlausibilityRatio {
			return true
		}
	}
	return false
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	s := append([]float64(nil), values...)
	sort.Float64s(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

// quantile returns the q-quantile (0..1) with linear interpolation.
func quantile(values []float64, q float64) float64 {
	if len(values) == 0 {
		return 0
	}
	s := append([]float64(nil), values...)
	sort.Float64s(s)
	pos := q * float64(len(s)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	return s[lo] + (s[hi]-s[lo])*(pos-float64(lo))
}

// computeBaseline returns the median capacity of a car's earliest points. Points
// with a known mileage are ordered by mileage and the first baselineWindowKm
// are used (at least baselineMinWindowPoints points). Without mileage the
// earliest points by date are used. ok is false for cars with too few points.
func computeBaseline(points []BatteryHealthPoint) (baseline float64, ok bool) {
	baseline, _, ok = computeBaselineWindow(points)
	return baseline, ok
}

// computeBaselineWindow is computeBaseline that also returns the window of
// points that form the baseline.
func computeBaselineWindow(points []BatteryHealthPoint) (baseline float64, window baselineWindow, ok bool) {
	if len(points) < minPointsForBaseline {
		return 0, window, false
	}
	var withKm []BatteryHealthPoint
	for _, p := range points {
		if p.Mileage > 0 {
			withKm = append(withKm, p)
		}
	}
	var ordered []BatteryHealthPoint
	if len(withKm) >= minPointsForBaseline {
		ordered = withKm
		sort.SliceStable(ordered, func(i, j int) bool {
			if ordered[i].Mileage != ordered[j].Mileage {
				return ordered[i].Mileage < ordered[j].Mileage
			}
			return ordered[i].Date.Before(ordered[j].Date)
		})
	} else {
		ordered = append([]BatteryHealthPoint(nil), points...)
		sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Date.Before(ordered[j].Date) })
	}

	window.byKm = ordered[0].Mileage > 0
	var caps []float64
	for i, p := range ordered {
		inWindow := window.byKm && p.Mileage-ordered[0].Mileage <= baselineWindowKm
		if i < baselineMinWindowPoints || inWindow {
			caps = append(caps, p.Capacity)
			if p.Mileage > window.lastKm {
				window.lastKm = p.Mileage
			}
			if p.Date.After(window.lastDay) {
				window.lastDay = p.Date
			}
		}
	}
	return median(caps), window, true
}

// clusterBaselines groups sorted baselines, splitting where the relative gap to
// the previous value is at least cohortGapRatio. It returns, for each input
// index, the cluster index (clusters ordered by ascending baseline).
func clusterBaselines(baselines []float64) []int {
	idx := make([]int, len(baselines))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return baselines[idx[a]] < baselines[idx[b]] })

	out := make([]int, len(baselines))
	cluster := 0
	for k, i := range idx {
		if k > 0 {
			prev := baselines[idx[k-1]]
			if prev > 0 && (baselines[i]-prev)/prev >= cohortGapRatio {
				cluster++
			}
		}
		out[i] = cluster
	}
	return out
}

// odometerResetKm is the mileage drop that marks a different car under the
// same vehicle identity (e.g. a replaced car uploaded with the same hash).
const odometerResetKm = 1000.0

// splitOdometerResets splits a vehicle into separate series at each point
// where the odometer goes back by more than odometerResetKm. An odometer does
// not go back, so each series is one physical car.
func splitOdometerResets(v *vehicleStats) []*vehicleStats {
	pts := append([]BatteryHealthPoint(nil), v.points...)
	sort.SliceStable(pts, func(i, j int) bool { return pts[i].Date.Before(pts[j].Date) })

	var out []*vehicleStats
	cur := &vehicleStats{id: v.id, model: v.model, cohort: unassignedCohortID}
	lastKm := 0.0
	for _, p := range pts {
		if p.Mileage > 0 {
			if lastKm > 0 && lastKm-p.Mileage > odometerResetKm {
				out = append(out, cur)
				cur = &vehicleStats{id: v.id, model: v.model, cohort: unassignedCohortID}
			}
			lastKm = p.Mileage
		}
		cur.points = append(cur.points, p)
	}
	return append(out, cur)
}

// AnalyzeBatteryCohorts builds pack-variant cohorts per model, annotates every
// point and computes per-cohort trend lines.
func AnalyzeBatteryCohorts(points []BatteryHealthPoint) BatteryCohortResult {
	result := BatteryCohortResult{
		Cohorts:  []BatteryCohort{},
		Points:   []CohortPoint{},
		Trends:   []CohortTrendPoint{},
		Excluded: []ExcludedVehicles{},
	}

	vehicles := map[int64]*vehicleStats{}
	var order []int64
	for _, p := range points {
		if p.SocChange < minAnalysisSocChange {
			continue
		}
		v, found := vehicles[p.VehicleID]
		if !found {
			v = &vehicleStats{id: p.VehicleID, model: p.Model, cohort: unassignedCohortID}
			vehicles[p.VehicleID] = v
			order = append(order, p.VehicleID)
		}
		v.points = append(v.points, p)
	}
	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })

	var series []*vehicleStats
	for _, id := range order {
		series = append(series, splitOdometerResets(vehicles[id])...)
	}

	byFamily := map[string][]*vehicleStats{}
	excluded := map[string]int{}
	var families []string
	for _, v := range series {
		v.family = vehiclecatalog.FamilyOf(v.model)
		if g, ok := vehiclecatalog.Group(v.model); ok {
			v.pack = &g
		}
		v.baseline, v.window, v.hasBaseline = computeBaselineWindow(v.points)
		for _, p := range v.points {
			if p.Mileage > 0 && (!v.hasKm || p.Mileage < v.firstKm) {
				v.firstKm, v.hasKm = p.Mileage, true
			}
			if v.firstDate.IsZero() || p.Date.Before(v.firstDate) {
				v.firstDate = p.Date
			}
		}
		if v.hasBaseline && !plausibleForFamily(v.family, v.baseline) {
			v.excluded = true
			excluded[v.family]++
			continue
		}
		if _, seen := byFamily[v.family]; !seen {
			families = append(families, v.family)
		}
		byFamily[v.family] = append(byFamily[v.family], v)
	}
	sort.Strings(families)

	for family, n := range excluded {
		result.Excluded = append(result.Excluded, ExcludedVehicles{Model: family, Vehicles: n, Reason: "battery size does not match model"})
	}
	sort.Slice(result.Excluded, func(i, j int) bool { return result.Excluded[i].Model < result.Excluded[j].Model })

	nextID := 0
	for _, family := range families {
		cohorts := buildFamilyCohorts(family, byFamily[family], nextID)
		result.Cohorts = append(result.Cohorts, cohorts...)
		nextID += len(cohorts)
	}

	for _, v := range series {
		if v.excluded {
			continue
		}
		sort.SliceStable(v.points, func(i, j int) bool { return v.points[i].Date.Before(v.points[j].Date) })
		for _, p := range v.points {
			cp := CohortPoint{
				Cohort:    v.cohort,
				Model:     v.family,
				Date:      p.Date,
				Mileage:   p.Mileage,
				Capacity:  p.Capacity,
				SocChange: p.SocChange,
				IsRaw:     p.IsRaw,
			}
			if v.hasBaseline && v.cohort != unassignedCohortID && v.baseline > 0 {
				pct := p.Capacity / v.baseline * 100
				months := p.Date.Sub(v.firstDate).Hours() / 24 / daysPerMonth
				cp.PctOfBaseline = &pct
				cp.MonthsSinceFirst = &months
				if v.hasKm && p.Mileage > 0 {
					km := p.Mileage - v.firstKm
					cp.KmSinceFirst = &km
				}
			}
			result.Points = append(result.Points, cp)
		}
	}
	sort.SliceStable(result.Points, func(i, j int) bool { return result.Points[i].Date.Before(result.Points[j].Date) })

	result.Trends = buildCohortTrends(series)
	slopes := cohortSlopes(series)
	for i := range result.Cohorts {
		c := &result.Cohorts[i]
		sl, ok := slopes[c.ID]
		if !ok {
			continue
		}
		c.SlopeKmCars, c.SlopeYearCars = len(sl.perKm), len(sl.perYear)
		if len(sl.perKm) >= minSlopeVehicles {
			m := median(sl.perKm)
			c.SlopePer10kKm = &m
		}
		if len(sl.perYear) >= minSlopeVehicles {
			m := median(sl.perYear)
			c.SlopePerYear = &m
		}
	}
	return result
}

type cohortBuild struct {
	pack      *vehiclecatalog.PackGroup
	baselines []float64
	sparse    int
	members   []*vehicleStats
	selected  bool
	matched   bool
}

// ref is the capacity used to compare cars against this cohort: the median
// measured baseline, or the catalog net capacity when nothing is measured yet.
func (b *cohortBuild) ref() float64 {
	if len(b.baselines) > 0 {
		return median(b.baselines)
	}
	if b.pack != nil {
		return b.pack.NetKWh
	}
	return 0
}

func (b *cohortBuild) add(v *vehicleStats) {
	b.members = append(b.members, v)
	if v.hasBaseline {
		b.baselines = append(b.baselines, v.baseline)
	} else {
		b.sparse++
	}
}

func sparseCapacity(v *vehicleStats) float64 {
	caps := make([]float64, 0, len(v.points))
	for _, p := range v.points {
		caps = append(caps, p.Capacity)
	}
	return median(caps)
}

// buildFamilyCohorts groups the cars of one family:
//  1. cars with a selected pack group form that group's cohort;
//  2. family-only (legacy) cars with enough data are gap-clustered, and each
//     cluster joins the nearest catalog pack within cohortGapRatio, or forms an
//     auto cohort;
//  3. legacy cars with too little data join the nearest cohort within
//     cohortGapRatio, or stay unassigned.
func buildFamilyCohorts(family string, members []*vehicleStats, firstID int) []BatteryCohort {
	packs := vehiclecatalog.GroupsOf(family)
	byPack := map[string]*cohortBuild{}
	var builds []*cohortBuild
	packBuild := func(g vehiclecatalog.PackGroup) *cohortBuild {
		if b, ok := byPack[g.Value]; ok {
			return b
		}
		gc := g
		b := &cohortBuild{pack: &gc}
		byPack[g.Value] = b
		builds = append(builds, b)
		return b
	}

	var legacyTrusted, legacySparse []*vehicleStats
	for _, v := range members {
		switch {
		case v.pack != nil:
			b := packBuild(*v.pack)
			b.selected = true
			b.add(v)
		case v.hasBaseline:
			legacyTrusted = append(legacyTrusted, v)
		default:
			legacySparse = append(legacySparse, v)
		}
	}

	if len(legacyTrusted) > 0 {
		baselines := make([]float64, len(legacyTrusted))
		for i, v := range legacyTrusted {
			baselines[i] = v.baseline
		}
		clusters := clusterBaselines(baselines)
		grouped := map[int][]*vehicleStats{}
		var clusterIDs []int
		for i, c := range clusters {
			if _, ok := grouped[c]; !ok {
				clusterIDs = append(clusterIDs, c)
			}
			grouped[c] = append(grouped[c], legacyTrusted[i])
		}
		sort.Ints(clusterIDs)
		for _, c := range clusterIDs {
			var bs []float64
			for _, v := range grouped[c] {
				bs = append(bs, v.baseline)
			}
			m := median(bs)

			var target *cohortBuild
			bestDist := cohortGapRatio
			for _, g := range packs {
				ref := g.NetKWh
				if b, ok := byPack[g.Value]; ok && len(b.baselines) > 0 {
					ref = b.ref()
				}
				if d := math.Abs(m-ref) / ref; d < bestDist {
					bestDist = d
					target = packBuild(g)
				}
			}
			if target == nil {
				target = &cohortBuild{}
				builds = append(builds, target)
			} else {
				target.matched = true
			}
			for _, v := range grouped[c] {
				target.add(v)
			}
		}
	}

	for _, v := range legacySparse {
		m := sparseCapacity(v)
		var target *cohortBuild
		bestDist := cohortGapRatio
		for _, b := range builds {
			if ref := b.ref(); ref > 0 {
				if d := math.Abs(m-ref) / ref; d < bestDist {
					bestDist = d
					target = b
				}
			}
		}
		if target != nil {
			target.add(v)
		}
	}

	sort.SliceStable(builds, func(i, j int) bool { return builds[i].ref() < builds[j].ref() })
	cohorts := make([]BatteryCohort, 0, len(builds))
	for i, b := range builds {
		id := firstID + i
		for _, v := range b.members {
			v.cohort = id
		}
		c := BatteryCohort{
			ID:             id,
			Model:          family,
			MedianBaseline: math.Round(median(b.baselines)*10) / 10,
			VehicleCount:   len(b.baselines),
			SparseCount:    b.sparse,
		}
		switch {
		case b.pack != nil:
			c.Pack = b.pack.Value
			c.NetKWh = b.pack.NetKWh
			c.Label = fmt.Sprintf("%s (%.1f kWh net)", strings.Join(b.pack.Trims, " / "), b.pack.NetKWh)
			c.Source = "matched"
			if b.selected {
				c.Source = "selected"
			}
		default:
			c.Label = fmt.Sprintf("~%.0f kWh pack (auto)", b.ref())
			c.Source = "auto"
		}
		cohorts = append(cohorts, c)
	}
	return cohorts
}

type trendKey struct {
	cohort int
	metric string
	axis   string
	bin    string
}

type trendBin struct {
	x        float64
	month    string
	vehicles map[int64]*[2]float64 // weighted sum, weight
	points   int
}

// buildCohortTrends bins points per cohort. Each car is first averaged inside a
// bin (weighted by SoC change), then the median across cars is taken, so cars
// with many uploads do not dominate a bin.
func buildCohortTrends(series []*vehicleStats) []CohortTrendPoint {
	bins := map[trendKey]*trendBin{}
	add := func(k trendKey, x float64, month string, vid int64, value, weight float64) {
		b, ok := bins[k]
		if !ok {
			b = &trendBin{x: x, month: month, vehicles: map[int64]*[2]float64{}}
			bins[k] = b
		}
		acc, ok := b.vehicles[vid]
		if !ok {
			acc = &[2]float64{}
			b.vehicles[vid] = acc
		}
		if weight <= 0 {
			weight = 1
		}
		acc[0] += value * weight
		acc[1] += weight
		b.points++
	}

	// The series index is the car key: one identity can hold more than one car.
	for i, v := range series {
		id := int64(i)
		if v.excluded || v.cohort == unassignedCohortID {
			continue
		}
		for _, p := range v.points {
			if p.Mileage > 0 {
				bin := math.Floor(p.Mileage / trendKmBin)
				add(trendKey{v.cohort, "kwh", "km", fmt.Sprint(bin)}, bin*trendKmBin+trendKmBin/2, "", id, p.Capacity, p.SocChange)
			}
			month := p.Date.UTC().Format("2006-01")
			add(trendKey{v.cohort, "kwh", "time", month}, 0, month, id, p.Capacity, p.SocChange)

			if !v.hasBaseline || v.baseline <= 0 || v.window.contains(p) {
				continue
			}
			pct := p.Capacity / v.baseline * 100
			if v.hasKm && p.Mileage > 0 {
				bin := math.Floor((p.Mileage - v.firstKm) / trendKmBin)
				add(trendKey{v.cohort, "pct", "km", fmt.Sprint(bin)}, bin*trendKmBin+trendKmBin/2, "", id, pct, p.SocChange)
			}
			months := p.Date.Sub(v.firstDate).Hours() / 24 / daysPerMonth
			bin := math.Floor(months / trendMonthBin)
			add(trendKey{v.cohort, "pct", "time", fmt.Sprint(bin)}, bin*trendMonthBin+trendMonthBin/2, "", id, pct, p.SocChange)
		}
	}

	// Every % trend starts at 100 % at x = 0: the baseline window itself.
	baselineCars := map[int]int{}
	for _, v := range series {
		if !v.excluded && v.cohort != unassignedCohortID && v.hasBaseline && v.baseline > 0 {
			baselineCars[v.cohort]++
		}
	}
	trends := []CohortTrendPoint{}
	for cohort, n := range baselineCars {
		if n < minVehiclesPerBin {
			continue
		}
		for _, axis := range []string{"km", "time"} {
			trends = append(trends, CohortTrendPoint{
				Cohort: cohort, Metric: "pct", Axis: axis, X: 0,
				Value: 100, P25: 100, P75: 100,
				Vehicles: n, Reliable: n >= minVehiclesForTrend,
			})
		}
	}
	for k, b := range bins {
		if len(b.vehicles) < minVehiclesPerBin {
			continue
		}
		var perVehicle []float64
		for _, acc := range b.vehicles {
			perVehicle = append(perVehicle, acc[0]/acc[1])
		}
		trends = append(trends, CohortTrendPoint{
			Cohort:   k.cohort,
			Metric:   k.metric,
			Axis:     k.axis,
			X:        b.x,
			Month:    b.month,
			Value:    median(perVehicle),
			P25:      quantile(perVehicle, 0.25),
			P75:      quantile(perVehicle, 0.75),
			Vehicles: len(b.vehicles),
			Points:   b.points,
			Reliable: len(b.vehicles) >= minVehiclesForTrend,
		})
	}
	sort.Slice(trends, func(i, j int) bool {
		a, c := trends[i], trends[j]
		if a.Cohort != c.Cohort {
			return a.Cohort < c.Cohort
		}
		if a.Metric != c.Metric {
			return a.Metric < c.Metric
		}
		if a.Axis != c.Axis {
			return a.Axis < c.Axis
		}
		if a.Month != c.Month {
			return a.Month < c.Month
		}
		return a.X < c.X
	})
	return trends
}

// GetBatteryCohorts loads battery health data (optionally for one model) and
// returns the cohort analysis. Vehicle ids are used internally only.
func (m *Manager) GetBatteryCohorts(modelFilter string) (BatteryCohortResult, error) {
	empty := AnalyzeBatteryCohorts(nil)

	var tableExists int
	if err := m.db.QueryRow(`SELECT count(name) FROM sqlite_master WHERE type='table' AND name='battery_health'`).Scan(&tableExists); err != nil {
		return empty, fmt.Errorf("error checking if battery_health table exists: %w", err)
	}
	if tableExists == 0 {
		return empty, nil
	}

	query := `
		SELECT bh.vehicle_id, v.model, bh.date, bh.estimated_capacity, bh.soc_change, bh.is_raw_data, COALESCE(bh.mileage, 0)
		FROM battery_health bh
		JOIN vehicles v ON bh.vehicle_id = v.id
	`
	clause, args, err := m.familyFilter(modelFilter)
	if err != nil {
		return empty, err
	}
	query += clause
	query += " ORDER BY bh.date"

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return empty, err
	}
	defer rows.Close()

	var points []BatteryHealthPoint
	for rows.Next() {
		var p BatteryHealthPoint
		var model *string
		if err := rows.Scan(&p.VehicleID, &model, &p.Date, &p.Capacity, &p.SocChange, &p.IsRaw, &p.Mileage); err != nil {
			return empty, err
		}
		if model != nil {
			p.Model = *model
		}
		points = append(points, p)
	}
	if err := rows.Err(); err != nil {
		return empty, err
	}
	return AnalyzeBatteryCohorts(points), nil
}

// linearSlope returns the least-squares slope of y over x.
func linearSlope(xs, ys []float64) (float64, bool) {
	n := float64(len(xs))
	if len(xs) < 2 {
		return 0, false
	}
	var sx, sy float64
	for i := range xs {
		sx += xs[i]
		sy += ys[i]
	}
	mx, my := sx/n, sy/n
	var sxy, sxx float64
	for i := range xs {
		sxy += (xs[i] - mx) * (ys[i] - my)
		sxx += (xs[i] - mx) * (xs[i] - mx)
	}
	if sxx == 0 {
		return 0, false
	}
	return sxy / sxx, true
}

type cohortSlope struct {
	perKm, perYear []float64
}

// cohortSlopes fits one line per car (% of baseline over km and over years)
// and collects the slopes per cohort. Only cars with at least slopeMinPoints
// points over slopeMinSpanKm qualify. The per-year slope also needs
// slopeMinSpanMonths of data.
func cohortSlopes(series []*vehicleStats) map[int]*cohortSlope {
	out := map[int]*cohortSlope{}
	for _, v := range series {
		if v.excluded || v.cohort == unassignedCohortID || !v.hasBaseline || v.baseline <= 0 {
			continue
		}
		var km, kmPct, years, yearPct []float64
		minKm, maxKm := math.Inf(1), math.Inf(-1)
		for _, p := range v.points {
			pct := p.Capacity / v.baseline * 100
			if p.Mileage > 0 {
				km = append(km, p.Mileage)
				kmPct = append(kmPct, pct)
				minKm, maxKm = math.Min(minKm, p.Mileage), math.Max(maxKm, p.Mileage)
			}
			years = append(years, p.Date.Sub(v.firstDate).Hours()/24/daysPerMonth/12)
			yearPct = append(yearPct, pct)
		}
		if len(km) < slopeMinPoints || maxKm-minKm < slopeMinSpanKm {
			continue
		}
		c, ok := out[v.cohort]
		if !ok {
			c = &cohortSlope{}
			out[v.cohort] = c
		}
		if slope, ok := linearSlope(km, kmPct); ok {
			c.perKm = append(c.perKm, slope*10000)
		}
		spanYears := 0.0
		for _, y := range years {
			spanYears = math.Max(spanYears, y)
		}
		if spanYears*12 >= slopeMinSpanMonths {
			if slope, ok := linearSlope(years, yearPct); ok {
				c.perYear = append(c.perYear, slope)
			}
		}
	}
	return out
}
