package models

import "time"

// Machine is a piece of equipment on a production line.
// GORM maps it to the "machines" table.
type Machine struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"type:varchar(100);not null;uniqueIndex" json:"name"`
	Line      string    `gorm:"type:varchar(50);not null;index" json:"line"`
	CreatedAt time.Time `json:"created_at"`

	// Has-many relation: not a column, just lets you Preload a machine's logs.
	DowntimeLogs []DowntimeLog `json:"downtime_logs,omitempty"`
}
