package handlers

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"go-downtime-api/models"
)

// The functions in this file check user input. They don't touch the
// database or HTTP, so they're easy to unit-test. Each error's text is
// exactly the message the API sends back in {"error": "..."} with a 400.

// Validate trims spaces from the fields and checks that both are filled in.
func (r *CreateMachineRequest) Validate() error {
	r.Name = strings.TrimSpace(r.Name)
	r.Line = strings.TrimSpace(r.Line)
	if r.Name == "" {
		return errors.New("name is required")
	}
	if r.Line == "" {
		return errors.New("line is required")
	}
	return nil
}

// Validate checks the body of POST /downtime. It does NOT check that the
// machine exists; that needs the database and gives a 404, not a 400.
func (r CreateDowntimeRequest) Validate() error {
	if r.MachineID == 0 {
		return errors.New("machine_id is required")
	}
	if r.StartTime == nil || r.EndTime == nil {
		return errors.New("start_time and end_time are required")
	}
	if !r.EndTime.After(*r.StartTime) {
		return errors.New("end_time must be after start_time")
	}
	if !models.IsValidReason(r.Reason) {
		return errors.New("reason must be one of: " + strings.Join(models.AllReasons, ", "))
	}
	return nil
}

// parseDay turns "YYYY-MM-DD" into the range [start of that day, start of next day)
// in the server's local time zone.
func parseDay(raw string) (from, to time.Time, err error) {
	day, err := time.ParseInLocation(time.DateOnly, raw, time.Local)
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("date must be in YYYY-MM-DD format")
	}
	return day, day.AddDate(0, 0, 1), nil
}

// parseMachineID parses a ?machine_id= value, which must be a positive integer.
func parseMachineID(raw string) (uint, error) {
	id, err := strconv.ParseUint(raw, 10, 0)
	if err != nil || id == 0 {
		return 0, errors.New("machine_id must be a positive integer")
	}
	return uint(id), nil
}
