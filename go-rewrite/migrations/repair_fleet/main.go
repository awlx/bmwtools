// Command repair_fleet is a one-off repair of the fleet database. See
// database.RepairFleet for the steps.
//
// The default is a dry run: the tool repairs a temporary copy and prints the
// result. With -apply it first writes a backup next to the database, then
// repairs the database itself.
//
// Usage:
//
//	go run ./migrations/repair_fleet -db ./data/bmwtools.db          # dry run
//	go run ./migrations/repair_fleet -db ./data/bmwtools.db -apply   # write
package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/awlx/bmwtools/pkg/database"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	dbPath := flag.String("db", "./data/bmwtools.db", "path to the SQLite database")
	apply := flag.Bool("apply", false, "repair the database (default: dry run on a temporary copy)")
	headline := flag.String("headline-model", "BMW i4", "model family for the battery headline before and after")
	flag.Parse()

	if _, err := os.Stat(*dbPath); err != nil {
		log.Fatalf("database: %v", err)
	}

	target := *dbPath
	if *apply {
		backup := fmt.Sprintf("%s.repair_bak_%s", *dbPath, time.Now().Format("20060102_150405"))
		if err := copyDB(*dbPath, backup); err != nil {
			log.Fatalf("backup: %v", err)
		}
		fmt.Printf("Backup written to %s\n", backup)
	} else {
		dir, err := os.MkdirTemp("", "repair_fleet")
		if err != nil {
			log.Fatal(err)
		}
		defer os.RemoveAll(dir)
		target = filepath.Join(dir, "dryrun.db")
		if err := copyDB(*dbPath, target); err != nil {
			log.Fatalf("copy: %v", err)
		}
	}

	m, err := database.New(target)
	if err != nil {
		log.Fatalf("open: %v", err)
	}
	defer m.Close()

	before := headlines(m, *headline)
	rep, err := m.RepairFleet()
	if err != nil {
		log.Fatalf("repair failed, database unchanged: %v", err)
	}
	after := headlines(m, *headline)

	out, _ := json.MarshalIndent(rep, "", "  ")
	fmt.Println(string(out))
	fmt.Printf("\nBattery headline (%s), before:\n%s\nafter:\n%s\n", *headline, before, after)
	if *apply {
		fmt.Println("\nRepair applied.")
	} else {
		fmt.Println("\nDRY RUN: the database is unchanged. Use -apply to write.")
	}
}

// copyDB writes a consistent copy of a SQLite database, WAL included.
func copyDB(src, dst string) error {
	db, err := sql.Open("sqlite3", src+"?_busy_timeout=5000")
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec("VACUUM INTO ?", dst)
	return err
}

func headlines(m *database.Manager, model string) string {
	res, err := m.GetBatteryCohorts(model)
	if err != nil {
		return "error: " + err.Error()
	}
	s := ""
	for _, c := range res.Cohorts {
		line := fmt.Sprintf("  %s: %d cars", c.Label, c.VehicleCount)
		if c.SlopePer10kKm != nil {
			line += fmt.Sprintf(", %.2f %% per 10,000 km (%d cars)", *c.SlopePer10kKm, c.SlopeKmCars)
		}
		if c.SlopePerYear != nil {
			line += fmt.Sprintf(", %.2f %% per year (%d cars)", *c.SlopePerYear, c.SlopeYearCars)
		}
		s += line + "\n"
	}
	for _, e := range res.Excluded {
		s += fmt.Sprintf("  excluded: %d (%s)\n", e.Vehicles, e.Reason)
	}
	return s
}
