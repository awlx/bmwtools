// Command regroup_vehicles is a ONE-OFF migration that repairs the historical
// fleet-statistics database created before vehicle identity was fixed.
//
// Background: the old code derived a vehicle's identity from each session's
// start timestamp, so every charging session became its own "vehicle" (e.g.
// 6650 sessions => 6650 vehicles). The real VIN was never stored, so it cannot
// be recovered directly. This tool reconstructs plausible vehicles heuristically:
//
//  1. Sessions are inserted in upload order (rowid). BMW exports list sessions
//     newest-first, so within one upload the start timestamp decreases. An
//     UPWARD jump in timestamp therefore marks the start of a new upload block.
//  2. Each block is one upload of one car's history. A single car uploads several
//     times over the months, producing multiple blocks. Blocks are merged into a
//     single vehicle when they share enough charging locations (Jaccard overlap)
//     and have a compatible model.
//
// The tool defaults to a DRY RUN. Pass -apply to write changes; a timestamped
// backup of the database is created first.
//
// Usage:
//
//	go run ./migrations/regroup_vehicles -db ./data/bmwtools.db                 # dry run
//	go run ./migrations/regroup_vehicles -db ./data/bmwtools.db -threshold 0.3  # tune
//	go run ./migrations/regroup_vehicles -db ./data/bmwtools.db -apply          # write
package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// hashFIN mirrors database.hashFIN so migrated vehicles use the same id scheme.
func hashFIN(vehicleIdentifier string) string {
	salt := "BMWToolsAnonymousFleetStats2025"
	h := sha256.New()
	h.Write([]byte(salt + vehicleIdentifier))
	return hex.EncodeToString(h.Sum(nil))
}

type sessionRow struct {
	rowid        int64
	ts           int64
	oldVehicleID int64
	locHash      string
	model        string
	block        int
}

type block struct {
	index         int
	locs          map[string]struct{}
	modelCounts   map[string]int
	oldVehicleIDs []int64
	sessionCount  int
	minTs, maxTs  int64
}

type group struct {
	locs          map[string]struct{}
	model         string
	oldVehicleIDs []int64
	blockIndexes  []int
	sessionCount  int
	minTs, maxTs  int64
}

func main() {
	dbPath := flag.String("db", "./data/bmwtools.db", "path to the SQLite database")
	threshold := flag.Float64("threshold", 0.60, "minimum location overlap coefficient (intersection / smaller set) to merge an upload block into an existing vehicle")
	minInter := flag.Int("min-shared-locations", 3, "minimum number of shared charging locations required to merge two blocks")
	maxGapDays := flag.Int("max-gap-days", 120, "maximum gap (days) between block time ranges for them to be considered the same vehicle")
	apply := flag.Bool("apply", false, "actually write changes (default is a dry run)")
	flag.Parse()

	db, err := sql.Open("sqlite3", *dbPath+"?_busy_timeout=5000")
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	rows := loadSessions(db)
	if len(rows) == 0 {
		log.Fatal("no sessions found; nothing to do")
	}

	blocks := detectBlocks(rows)
	groups := mergeBlocks(blocks, *threshold, *minInter, time.Duration(*maxGapDays)*24*time.Hour)

	printSummary(rows, blocks, groups, *threshold)

	if !*apply {
		fmt.Println("\nDRY RUN — no changes written. Re-run with -apply to migrate.")
		return
	}

	backup := backupDB(*dbPath)
	fmt.Printf("\nBackup written to %s\n", backup)

	if err := writeGroups(db, rows, groups); err != nil {
		log.Fatalf("migration failed (database unchanged, restore from %s if needed): %v", backup, err)
	}
	fmt.Printf("\nMigration complete: %d sessions regrouped into %d vehicles.\n", len(rows), len(groups))
}

func loadSessions(db *sql.DB) []sessionRow {
	q := `
		SELECT s.rowid, CAST(s.id AS INTEGER) AS ts, s.vehicle_id,
		       COALESCE(s.location_hash, ''), COALESCE(v.model, '')
		FROM sessions s
		LEFT JOIN vehicles v ON s.vehicle_id = v.id
		ORDER BY s.rowid`
	r, err := db.Query(q)
	if err != nil {
		log.Fatalf("load sessions: %v", err)
	}
	defer r.Close()

	var out []sessionRow
	for r.Next() {
		var s sessionRow
		if err := r.Scan(&s.rowid, &s.ts, &s.oldVehicleID, &s.locHash, &s.model); err != nil {
			log.Fatalf("scan session: %v", err)
		}
		out = append(out, s)
	}
	if err := r.Err(); err != nil {
		log.Fatalf("iterate sessions: %v", err)
	}
	return out
}

// detectBlocks splits sessions into upload blocks. Within an upload the BMW
// export is newest-first (decreasing ts), so an increase in ts starts a new block.
func detectBlocks(rows []sessionRow) []block {
	var blocks []block
	cur := -1
	var prevTs int64
	for i := range rows {
		if i == 0 || rows[i].ts > prevTs {
			blocks = append(blocks, block{
				index:       len(blocks),
				locs:        map[string]struct{}{},
				modelCounts: map[string]int{},
				minTs:       rows[i].ts,
				maxTs:       rows[i].ts,
			})
			cur = len(blocks) - 1
		}
		rows[i].block = cur
		b := &blocks[cur]
		b.sessionCount++
		b.oldVehicleIDs = append(b.oldVehicleIDs, rows[i].oldVehicleID)
		if rows[i].locHash != "" {
			b.locs[rows[i].locHash] = struct{}{}
		}
		if rows[i].model != "" {
			b.modelCounts[rows[i].model]++
		}
		if rows[i].ts < b.minTs {
			b.minTs = rows[i].ts
		}
		if rows[i].ts > b.maxTs {
			b.maxTs = rows[i].ts
		}
		prevTs = rows[i].ts
	}
	return blocks
}

func majorityModel(counts map[string]int) string {
	best, bestN := "", 0
	for m, n := range counts {
		if n > bestN {
			best, bestN = m, n
		}
	}
	return best
}

func unknownModel(m string) bool {
	return m == "" || m == "Unknown BMW EV"
}

func modelsCompatible(a, b string) bool {
	if unknownModel(a) || unknownModel(b) {
		return true
	}
	return a == b
}

// intersectionSize counts shared (non-empty) location hashes.
func intersectionSize(a, b map[string]struct{}) int {
	inter := 0
	for k := range a {
		if _, ok := b[k]; ok {
			inter++
		}
	}
	return inter
}

// overlapCoeff is intersection / size of the smaller set. Unlike Jaccard it is
// not penalised when a small incremental upload is compared against a vehicle
// that has accumulated many locations, which is exactly the re-upload case.
func overlapCoeff(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	inter := intersectionSize(a, b)
	min := len(a)
	if len(b) < min {
		min = len(b)
	}
	if min == 0 {
		return 0
	}
	return float64(inter) / float64(min)
}

// mergeBlocks groups upload blocks into vehicles. A block is merged into an
// existing vehicle when their charging locations overlap strongly (overlap
// coefficient), they share at least minInter locations, the models are
// compatible, and their time ranges are continuous (a re-upload extends the
// timeline rather than starting a brand new one).
func mergeBlocks(blocks []block, threshold float64, minInter int, maxGap time.Duration) []group {
	gapSecs := int64(maxGap / time.Second)
	var groups []group
	for bi := range blocks {
		b := blocks[bi]
		bModel := majorityModel(b.modelCounts)

		best, bestSim := -1, 0.0
		for gi := range groups {
			if !modelsCompatible(groups[gi].model, bModel) {
				continue
			}
			// Require continuous timelines: the block must overlap the group or
			// sit within maxGap of it on either side.
			if b.minTs > groups[gi].maxTs+gapSecs || b.maxTs < groups[gi].minTs-gapSecs {
				continue
			}
			if intersectionSize(groups[gi].locs, b.locs) < minInter {
				continue
			}
			if sim := overlapCoeff(groups[gi].locs, b.locs); sim > bestSim {
				bestSim, best = sim, gi
			}
		}

		if best >= 0 && bestSim >= threshold {
			g := &groups[best]
			for k := range b.locs {
				g.locs[k] = struct{}{}
			}
			if unknownModel(g.model) && !unknownModel(bModel) {
				g.model = bModel
			}
			g.oldVehicleIDs = append(g.oldVehicleIDs, b.oldVehicleIDs...)
			g.blockIndexes = append(g.blockIndexes, b.index)
			g.sessionCount += b.sessionCount
			if b.minTs < g.minTs {
				g.minTs = b.minTs
			}
			if b.maxTs > g.maxTs {
				g.maxTs = b.maxTs
			}
			continue
		}

		locs := map[string]struct{}{}
		for k := range b.locs {
			locs[k] = struct{}{}
		}
		groups = append(groups, group{
			locs:          locs,
			model:         bModel,
			oldVehicleIDs: append([]int64(nil), b.oldVehicleIDs...),
			blockIndexes:  []int{b.index},
			sessionCount:  b.sessionCount,
			minTs:         b.minTs,
			maxTs:         b.maxTs,
		})
	}
	return groups
}

func printSummary(rows []sessionRow, blocks []block, groups []group, threshold float64) {
	fmt.Printf("Loaded %d sessions across %d old (per-session) vehicles.\n", len(rows), countOldVehicles(rows))
	fmt.Printf("Detected %d upload blocks; merged into %d vehicles (threshold=%.2f).\n\n", len(blocks), len(groups), threshold)

	sort.Slice(groups, func(i, j int) bool { return groups[i].minTs < groups[j].minTs })
	fmt.Printf("%-4s %-8s %-7s %-12s %-12s %s\n", "veh", "sessions", "blocks", "first", "last", "model")
	for i, g := range groups {
		fmt.Printf("%-4d %-8d %-7d %-12s %-12s %s\n",
			i+1, g.sessionCount, len(g.blockIndexes),
			time.Unix(g.minTs, 0).Format("2006-01-02"),
			time.Unix(g.maxTs, 0).Format("2006-01-02"),
			modelOrUnknown(g.model))
	}
}

func modelOrUnknown(m string) string {
	if unknownModel(m) {
		return "Unknown BMW EV"
	}
	return m
}

func countOldVehicles(rows []sessionRow) int {
	seen := map[int64]struct{}{}
	for _, r := range rows {
		seen[r.oldVehicleID] = struct{}{}
	}
	return len(seen)
}

func backupDB(dbPath string) string {
	backupPath := fmt.Sprintf("%s.regroup_bak_%s", dbPath, time.Now().Format("20060102_150405"))
	in, err := os.Open(dbPath)
	if err != nil {
		log.Fatalf("backup open: %v", err)
	}
	defer in.Close()
	out, err := os.Create(backupPath)
	if err != nil {
		log.Fatalf("backup create: %v", err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		log.Fatalf("backup copy: %v", err)
	}
	return backupPath
}

func writeGroups(db *sql.DB, rows []sessionRow, groups []group) error {
	var maxOldID int64
	if err := db.QueryRow("SELECT COALESCE(MAX(id), 0) FROM vehicles").Scan(&maxOldID); err != nil {
		return fmt.Errorf("max vehicle id: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()

	// Map each old (per-session) vehicle id to the new grouped vehicle id.
	oldToNew := make(map[int64]int64, len(rows))

	for gi := range groups {
		g := groups[gi]
		finHash := hashFIN(fmt.Sprintf("migrated-vehicle-%d", gi))
		res, err := tx.Exec(
			"INSERT INTO vehicles (fin_hash, model, created_at) VALUES (?, ?, ?)",
			finHash, modelOrUnknown(g.model), time.Now(),
		)
		if err != nil {
			return fmt.Errorf("insert vehicle: %w", err)
		}
		newID, err := res.LastInsertId()
		if err != nil {
			return err
		}
		for _, oldID := range g.oldVehicleIDs {
			oldToNew[oldID] = newID
		}
	}

	updSession, err := tx.Prepare("UPDATE sessions SET vehicle_id = ? WHERE vehicle_id = ?")
	if err != nil {
		return err
	}
	defer updSession.Close()
	updBH, err := tx.Prepare("UPDATE battery_health SET vehicle_id = ? WHERE vehicle_id = ?")
	if err != nil {
		return err
	}
	defer updBH.Close()

	for oldID, newID := range oldToNew {
		if _, err := updSession.Exec(newID, oldID); err != nil {
			return fmt.Errorf("reassign sessions for old vehicle %d: %w", oldID, err)
		}
		if _, err := updBH.Exec(newID, oldID); err != nil {
			return fmt.Errorf("reassign battery_health for old vehicle %d: %w", oldID, err)
		}
	}

	// Remove the old per-session vehicle rows (new rows have ids > maxOldID).
	if _, err := tx.Exec("DELETE FROM vehicles WHERE id <= ?", maxOldID); err != nil {
		return fmt.Errorf("delete old vehicles: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}
