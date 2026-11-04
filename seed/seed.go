// Command seed fills the database with sample machines and downtime logs.
//
// Run it from the project root:
//
//	go run ./seed
//
// It empties the machines and downtime_logs tables first, so every run
// starts from the same clean state.
package main

import (
	"log"
	"math/rand/v2"
	"time"

	"github.com/joho/godotenv"

	"go-downtime-api/config"
	"go-downtime-api/models"
)

const logCount = 200

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, using system environment variables")
	}

	config.ConnectDB()
	config.Migrate()

	// Start fresh: remove old rows and reset the ID counters.
	if err := config.DB.Exec("TRUNCATE TABLE downtime_logs, machines RESTART IDENTITY CASCADE").Error; err != nil {
		log.Fatalf("failed to clear tables: %v", err)
	}

	machines := []models.Machine{
		{Name: "CNC-01", Line: "Line A"},
		{Name: "CNC-02", Line: "Line A"},
		{Name: "Press-01", Line: "Line A"},
		{Name: "Packer-01", Line: "Line B"},
		{Name: "Filler-01", Line: "Line B"},
	}
	// Passing a slice inserts all rows and fills in each machine's ID.
	if err := config.DB.Create(&machines).Error; err != nil {
		log.Fatalf("failed to insert machines: %v", err)
	}

	now := time.Now().Truncate(time.Minute)
	window := 7 * 24 * time.Hour

	logs := make([]models.DowntimeLog, 0, logCount)
	for range logCount {
		machine := machines[rand.IntN(len(machines))]

		// Each stop lasts between 5 and 180 minutes.
		duration := time.Duration(5+rand.IntN(176)) * time.Minute

		// Pick a start time somewhere in the last 7 days, and keep the end time in the past.
		start := now.Add(-time.Duration(rand.Int64N(int64(window)))).Truncate(time.Minute)
		if start.Add(duration).After(now) {
			start = now.Add(-duration)
		}
		end := start.Add(duration)

		logs = append(logs, models.DowntimeLog{
			MachineID: machine.ID,
			Reason:    models.AllReasons[rand.IntN(len(models.AllReasons))],
			StartTime: start,
			EndTime:   end,
			Minutes:   int(end.Sub(start).Minutes()),
		})
	}

	// Insert in batches of 100 rows per INSERT statement.
	if err := config.DB.CreateInBatches(&logs, 100).Error; err != nil {
		log.Fatalf("failed to insert downtime logs: %v", err)
	}

	log.Printf("seeded %d machines and %d downtime logs", len(machines), len(logs))
}
