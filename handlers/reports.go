package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/sync/errgroup"
)

// ReasonMinutes is one entry in top_reasons.
type ReasonMinutes struct {
	Reason  string `json:"reason"`
	Minutes int64  `json:"minutes"`
}

// MachineMinutes is the worst_machine entry.
type MachineMinutes struct {
	ID      uint   `json:"id"`
	Name    string `json:"name"`
	Line    string `json:"line"`
	Minutes int64  `json:"minutes"`
}

// LineMinutes is one entry in downtime_by_line.
type LineMinutes struct {
	Line    string `json:"line"`
	Minutes int64  `json:"minutes"`
}

// DailyReport is the response body for GET /reports/daily.
type DailyReport struct {
	Date                 string          `json:"date"`
	Mode                 string          `json:"mode"`
	TotalDowntimeMinutes int64           `json:"total_downtime_minutes"`
	TopReasons           []ReasonMinutes `json:"top_reasons"`
	WorstMachine         *MachineMinutes `json:"worst_machine"` // null when the day has no downtime
	DowntimeByLine       []LineMinutes   `json:"downtime_by_line"`
	TookMs               float64         `json:"took_ms"`
}

const (
	modeConcurrent = "concurrent"
	modeSequential = "sequential"
)

// DailyReport handles GET /reports/daily?date=YYYY-MM-DD[&mode=sequential].
// A log counts toward a day if it started on that day (server's local time zone).
func (h *Handler) DailyReport(c echo.Context) error {
	rawDate := c.QueryParam("date")
	if rawDate == "" {
		return errorJSON(c, http.StatusBadRequest, "date is required (YYYY-MM-DD)")
	}
	from, to, err := parseDay(rawDate)
	if err != nil {
		return errorJSON(c, http.StatusBadRequest, err.Error())
	}

	mode := c.QueryParam("mode")
	if mode == "" {
		mode = modeConcurrent
	}
	if mode != modeConcurrent && mode != modeSequential {
		return errorJSON(c, http.StatusBadRequest, "mode must be 'concurrent' or 'sequential'")
	}

	// The request's context is cancelled if the client disconnects,
	// which also stops any database queries that are still running.
	ctx := c.Request().Context()

	start := time.Now()
	var report *DailyReport
	if mode == modeSequential {
		report, err = h.reportSequential(ctx, from, to)
	} else {
		report, err = h.reportConcurrent(ctx, from, to)
	}
	took := time.Since(start)

	if err != nil {
		return internalError(c, err)
	}

	log.Printf("daily report date=%s mode=%s took=%s", rawDate, mode, took)

	report.Date = rawDate
	report.Mode = mode
	report.TookMs = float64(took.Microseconds()) / 1000
	return c.JSON(http.StatusOK, report)
}

// reportSequential runs the four queries one after another.
// Total time is roughly the sum of the four query times.
func (h *Handler) reportSequential(ctx context.Context, from, to time.Time) (*DailyReport, error) {
	total, err := h.queryTotalMinutes(ctx, from, to)
	if err != nil {
		return nil, err
	}
	top, err := h.queryTopReasons(ctx, from, to)
	if err != nil {
		return nil, err
	}
	worst, err := h.queryWorstMachine(ctx, from, to)
	if err != nil {
		return nil, err
	}
	byLine, err := h.queryDowntimeByLine(ctx, from, to)
	if err != nil {
		return nil, err
	}

	return &DailyReport{
		TotalDowntimeMinutes: total,
		TopReasons:           top,
		WorstMachine:         worst,
		DowntimeByLine:       byLine,
	}, nil
}

// reportConcurrent runs the four queries at the same time, each in its own
// goroutine. Total time is roughly the time of the slowest query.
//
// How it stays race-free:
//   - Each goroutine writes to its OWN variable (total, top, worst, byLine).
//     No two goroutines ever touch the same variable, so no mutex is needed.
//   - We only read those variables after g.Wait() returns. Wait guarantees
//     every goroutine has finished, and Go guarantees their writes are visible.
//   - *gorm.DB is safe to share: each query starts a new chain from h.DB,
//     and the underlying connection pool hands each query its own connection.
func (h *Handler) reportConcurrent(ctx context.Context, from, to time.Time) (*DailyReport, error) {
	var (
		total  int64
		top    []ReasonMinutes
		worst  *MachineMinutes
		byLine []LineMinutes
	)

	// If any goroutine returns an error, errgroup cancels gctx so the
	// other queries stop early, and g.Wait() returns that first error.
	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		var err error
		total, err = h.queryTotalMinutes(gctx, from, to)
		return err
	})
	g.Go(func() error {
		var err error
		top, err = h.queryTopReasons(gctx, from, to)
		return err
	})
	g.Go(func() error {
		var err error
		worst, err = h.queryWorstMachine(gctx, from, to)
		return err
	})
	g.Go(func() error {
		var err error
		byLine, err = h.queryDowntimeByLine(gctx, from, to)
		return err
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return &DailyReport{
		TotalDowntimeMinutes: total,
		TopReasons:           top,
		WorstMachine:         worst,
		DowntimeByLine:       byLine,
	}, nil
}

// ---- The four queries. Each one is independent of the others. ----

const inDay = "d.start_time >= ? AND d.start_time < ?"

func (h *Handler) queryTotalMinutes(ctx context.Context, from, to time.Time) (int64, error) {
	var total int64
	err := h.DB.WithContext(ctx).
		Table("downtime_logs AS d").
		Select("COALESCE(SUM(d.minutes), 0)").
		Where(inDay, from, to).
		Scan(&total).Error
	if err != nil {
		return 0, fmt.Errorf("total downtime: %w", err)
	}
	return total, nil
}

func (h *Handler) queryTopReasons(ctx context.Context, from, to time.Time) ([]ReasonMinutes, error) {
	rows := []ReasonMinutes{}
	err := h.DB.WithContext(ctx).
		Table("downtime_logs AS d").
		Select("d.reason, SUM(d.minutes) AS minutes").
		Where(inDay, from, to).
		Group("d.reason").
		Order("minutes DESC, d.reason").
		Limit(3).
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("top reasons: %w", err)
	}
	return rows, nil
}

func (h *Handler) queryWorstMachine(ctx context.Context, from, to time.Time) (*MachineMinutes, error) {
	var rows []MachineMinutes
	err := h.DB.WithContext(ctx).
		Table("downtime_logs AS d").
		Select("m.id, m.name, m.line, SUM(d.minutes) AS minutes").
		Joins("JOIN machines AS m ON m.id = d.machine_id").
		Where(inDay, from, to).
		Group("m.id, m.name, m.line").
		Order("minutes DESC, m.id").
		Limit(1).
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("worst machine: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil // no downtime that day
	}
	return &rows[0], nil
}

func (h *Handler) queryDowntimeByLine(ctx context.Context, from, to time.Time) ([]LineMinutes, error) {
	rows := []LineMinutes{}
	err := h.DB.WithContext(ctx).
		Table("downtime_logs AS d").
		Select("m.line, SUM(d.minutes) AS minutes").
		Joins("JOIN machines AS m ON m.id = d.machine_id").
		Where(inDay, from, to).
		Group("m.line").
		Order("minutes DESC, m.line").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("downtime by line: %w", err)
	}
	return rows, nil
}
