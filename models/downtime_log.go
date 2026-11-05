package models

import (
	"slices"
	"time"
)

// Reasons a machine can be down. Go has no enums, so these are string
// constants, and the database enforces the list with a CHECK constraint.
const (
	ReasonBreakdown    = "Breakdown"
	ReasonNoMaterial   = "No material"
	ReasonChangeover   = "Changeover"
	ReasonQualityIssue = "Quality issue"
	ReasonPowerCut     = "Power cut"
)

// AllReasons lists every valid downtime reason.
var AllReasons = []string{
	ReasonBreakdown,
	ReasonNoMaterial,
	ReasonChangeover,
	ReasonQualityIssue,
	ReasonPowerCut,
}

// IsValidReason reports whether r is one of AllReasons (exact, case-sensitive match).
func IsValidReason(r string) bool {
	return slices.Contains(AllReasons, r)
}

// MinutesBetween returns the whole minutes from start to end, dropping any
// leftover seconds (8:00:00 to 8:45:59 is 45). Time zones don't matter:
// it compares the actual instants. Returns 0 if end is not after start.
func MinutesBetween(start, end time.Time) int {
	if !end.After(start) {
		return 0
	}
	return int(end.Sub(start) / time.Minute)
}

// DowntimeLog records one period when a machine was not running.
// GORM maps it to the "downtime_logs" table.
type DowntimeLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	MachineID uint      `gorm:"not null;index" json:"machine_id"`
	Reason    string    `gorm:"type:varchar(32);not null;check:chk_downtime_logs_reason,reason IN ('Breakdown','No material','Changeover','Quality issue','Power cut')" json:"reason"`
	StartTime time.Time `gorm:"not null;index" json:"start_time"`
	EndTime   time.Time `gorm:"not null" json:"end_time"`
	Minutes   int       `gorm:"not null" json:"minutes"`
	CreatedAt time.Time `json:"created_at"`

	// Belongs-to relation: not a column. Lets you Preload("Machine") a log's machine.
	// Together with Machine.DowntimeLogs, AutoMigrate adds the foreign key
	// machine_id -> machines.id, so deleting a machine that still has logs fails.
	Machine *Machine `json:"machine,omitempty"`
}
