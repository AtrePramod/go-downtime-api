package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
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
	if req.MachineID == 0 {
		return errorJSON(c, http.StatusBadRequest, "machine_id is required")
	}
	if req.StartTime == nil || req.EndTime == nil {
		return errorJSON(c, http.StatusBadRequest, "start_time and end_time are required")
	}
	if !req.EndTime.After(*req.StartTime) {
		return errorJSON(c, http.StatusBadRequest, "end_time must be after start_time")
	}
	if !models.IsValidReason(req.Reason) {
		return errorJSON(c, http.StatusBadRequest,
			"reason must be one of: "+strings.Join(models.AllReasons, ", "))
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
		Minutes:   int(req.EndTime.Sub(*req.StartTime).Minutes()),
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
	query := h.DB.Model(&models.DowntimeLog{}).Order("start_time DESC")

	if raw := c.QueryParam("machine_id"); raw != "" {
		id, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || id == 0 {
			return errorJSON(c, http.StatusBadRequest, "machine_id must be a positive integer")
		}
		query = query.Where("machine_id = ?", id)
	}

	if raw := c.QueryParam("date"); raw != "" {
		day, err := time.ParseInLocation(time.DateOnly, raw, time.Local)
		if err != nil {
			return errorJSON(c, http.StatusBadRequest, "date must be in YYYY-MM-DD format")
		}
		// A half-open range [day, next day) can use the start_time index.
		query = query.Where("start_time >= ? AND start_time < ?", day, day.AddDate(0, 0, 1))
	}

	logs := []models.DowntimeLog{}
	if err := query.Find(&logs).Error; err != nil {
		return internalError(c, err)
	}
	return c.JSON(http.StatusOK, logs)
}
