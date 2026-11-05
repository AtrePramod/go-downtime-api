package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

func ptr(t time.Time) *time.Time { return &t }

func TestCreateDowntimeRequestValidate(t *testing.T) {
	start := time.Date(2026, 11, 4, 8, 0, 0, 0, time.UTC)
	end := start.Add(45 * time.Minute)

	// valid returns a request that passes; each case changes one thing.
	valid := func() CreateDowntimeRequest {
		return CreateDowntimeRequest{MachineID: 1, Reason: "Breakdown", StartTime: ptr(start), EndTime: ptr(end)}
	}

	tests := []struct {
		name    string
		change  func(r *CreateDowntimeRequest)
		wantErr string // "" means no error expected
	}{
		{"valid request", func(r *CreateDowntimeRequest) {}, ""},
		{"missing machine_id", func(r *CreateDowntimeRequest) { r.MachineID = 0 }, "machine_id is required"},
		{"missing start_time", func(r *CreateDowntimeRequest) { r.StartTime = nil }, "start_time and end_time are required"},
		{"missing end_time", func(r *CreateDowntimeRequest) { r.EndTime = nil }, "start_time and end_time are required"},
		{"end before start", func(r *CreateDowntimeRequest) { r.EndTime = ptr(start.Add(-time.Minute)) }, "end_time must be after start_time"},
		{"end equals start", func(r *CreateDowntimeRequest) { r.EndTime = ptr(start) }, "end_time must be after start_time"},
		{"unknown reason", func(r *CreateDowntimeRequest) { r.Reason = "Coffee break" }, "reason must be one of"},
		{"empty reason", func(r *CreateDowntimeRequest) { r.Reason = "" }, "reason must be one of"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := valid()
			tc.change(&req)
			err := req.Validate()

			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got %q", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestCreateMachineRequestValidate(t *testing.T) {
	tests := []struct {
		name, inName, inLine, wantErr string
	}{
		{"valid", "CNC-03", "Line A", ""},
		{"missing name", "", "Line A", "name is required"},
		{"spaces-only name", "   ", "Line A", "name is required"},
		{"missing line", "CNC-03", "", "line is required"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := CreateMachineRequest{Name: tc.inName, Line: tc.inLine}
			err := req.Validate()
			if tc.wantErr == "" && err != nil {
				t.Fatalf("expected no error, got %q", err)
			}
			if tc.wantErr != "" && (err == nil || err.Error() != tc.wantErr) {
				t.Fatalf("expected %q, got %v", tc.wantErr, err)
			}
		})
	}

	// Validate should also trim the spaces it allows.
	req := CreateMachineRequest{Name: "  CNC-03 ", Line: " Line A"}
	if err := req.Validate(); err != nil {
		t.Fatal(err)
	}
	if req.Name != "CNC-03" || req.Line != "Line A" {
		t.Errorf("fields not trimmed: %q, %q", req.Name, req.Line)
	}
}

func TestParseDay(t *testing.T) {
	from, to, err := parseDay("2026-11-04")
	if err != nil {
		t.Fatal(err)
	}
	if from.Format(time.DateOnly) != "2026-11-04" || to.Sub(from) != 24*time.Hour {
		t.Errorf("got range %s to %s", from, to)
	}

	for _, bad := range []string{"", "2026-13-01", "2026-02-30", "04-11-2026", "2026/11/04", "yesterday"} {
		if _, _, err := parseDay(bad); err == nil {
			t.Errorf("parseDay(%q) should fail", bad)
		}
	}
}

func TestParseMachineID(t *testing.T) {
	if id, err := parseMachineID("3"); err != nil || id != 3 {
		t.Errorf("parseMachineID(\"3\") = %d, %v", id, err)
	}
	for _, bad := range []string{"0", "-1", "abc", "1.5", ""} {
		if _, err := parseMachineID(bad); err == nil {
			t.Errorf("parseMachineID(%q) should fail", bad)
		}
	}
}

// TestHandlersReturn400 sends real HTTP requests through Echo to check the
// status code and the {"error": "..."} body. These cases fail validation
// before any database call, so the Handler can have a nil DB.
func TestHandlersReturn400(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = HTTPErrorHandler
	h := New(nil)
	e.POST("/machines", h.CreateMachine)
	e.POST("/downtime", h.CreateDowntime)
	e.GET("/downtime", h.ListDowntime)
	e.GET("/reports/daily", h.DailyReport)

	tests := []struct {
		name, method, path, body, wantErr string
	}{
		{"machine missing line", "POST", "/machines", `{"name":"X"}`, "line is required"},
		{"machine bad JSON", "POST", "/machines", `{bad`, "invalid JSON body"},
		{"downtime end before start", "POST", "/downtime",
			`{"machine_id":1,"reason":"Breakdown","start_time":"2026-11-04T09:00:00Z","end_time":"2026-11-04T08:00:00Z"}`,
			"end_time must be after start_time"},
		{"downtime bad reason", "POST", "/downtime",
			`{"machine_id":1,"reason":"Lunch","start_time":"2026-11-04T08:00:00Z","end_time":"2026-11-04T09:00:00Z"}`,
			"reason must be one of"},
		{"downtime bad time format", "POST", "/downtime",
			`{"machine_id":1,"reason":"Breakdown","start_time":"04/11/2026","end_time":"2026-11-04T09:00:00Z"}`,
			"invalid JSON body"},
		{"list bad date", "GET", "/downtime?date=2026-13-40", "", "date must be in YYYY-MM-DD format"},
		{"list bad machine_id", "GET", "/downtime?machine_id=abc", "", "machine_id must be a positive integer"},
		{"report missing date", "GET", "/reports/daily", "", "date is required"},
		{"report bad date", "GET", "/reports/daily?date=2026-02-30", "", "date must be in YYYY-MM-DD format"},
		{"report bad mode", "GET", "/reports/daily?date=2026-11-04&mode=fast", "", "mode must be"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.body != "" {
				req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			}
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body: %s)", rec.Code, rec.Body)
			}
			var body ErrorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("body is not {\"error\": ...} JSON: %s", rec.Body)
			}
			if !strings.Contains(body.Error, tc.wantErr) {
				t.Errorf("error = %q, want it to contain %q", body.Error, tc.wantErr)
			}
		})
	}
}
