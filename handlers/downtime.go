package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"go-downtime-api/models"
)

// CreateDowntimeRequest is the body for POST /downtime.
// Times are pointers so we can tell "missing" (nil) apart from a real value.
// They must be RFC 3339 strings, e.g. "2026-11-04T08:30:00+05:30".
type CreateDowntimeRequest struct {
	MachineID uint       `json:"machine_id"`
	Reason    string     `json:"reason"`
	StartTime *time.Time `json:"start_time"`
	EndTime   *time.Time `json:"end_time"`
}

// CreateDowntime handles POST /downtime.
func (h *Handler) CreateDowntime(c echo.Context) error {
	var req CreateDowntimeRequest
	if err := c.Bind(&req); err != nil {
		return errorJSON(c, http.StatusBadRequest,
			"invalid JSON body (times must be RFC 3339, e.g. 2026-11-04T08:30:00Z)")
	}

	// 1. Cheap checks on the body first: 400 Bad Request.
	if err := req.Validate(); err != nil {
		return errorJSON(c, http.StatusBadRequest, err.Error())
	}

	// 2. Then check the machine exists: 404 Not Found.
	var machine models.Machine
	if err := h.DB.First(&machine, req.MachineID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorJSON(c, http.StatusNotFound, "machine not found")
		}
		return internalError(c, err)
	}

	// 3. The server works out minutes; the client never sends them.
	entry := models.DowntimeLog{
		MachineID: machine.ID,
		Reason:    req.Reason,
		StartTime: *req.StartTime,
		EndTime:   *req.EndTime,
		Minutes:   models.MinutesBetween(*req.StartTime, *req.EndTime),
	}
	if err := h.DB.Create(&entry).Error; err != nil {
		return internalError(c, err)
	}

	return c.JSON(http.StatusCreated, entry)
}

// ListDowntime handles GET /downtime with optional filters:
//
//	?machine_id=3       only logs for that machine
//	?date=2026-11-04    only logs that started on that day (server's local time zone)
func (h *Handler) ListDowntime(c echo.Context) error {
	// Check every query parameter before touching the database.
	var machineID uint
	if raw := c.QueryParam("machine_id"); raw != "" {
		id, err := parseMachineID(raw)
		if err != nil {
			return errorJSON(c, http.StatusBadRequest, err.Error())
		}
		machineID = id
	}

	var from, to time.Time
	if raw := c.QueryParam("date"); raw != "" {
		f, t, err := parseDay(raw)
		if err != nil {
			return errorJSON(c, http.StatusBadRequest, err.Error())
		}
		from, to = f, t
	}

	query := h.DB.Model(&models.DowntimeLog{}).Order("start_time DESC")
	if machineID != 0 {
		query = query.Where("machine_id = ?", machineID)
	}
	if !from.IsZero() {
		// A half-open range [from, to) can use the start_time index.
		query = query.Where("start_time >= ? AND start_time < ?", from, to)
	}

	logs := []models.DowntimeLog{}
	if err := query.Find(&logs).Error; err != nil {
		return internalError(c, err)
	}
	return c.JSON(http.StatusOK, logs)
}
