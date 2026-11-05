package models

import (
	"testing"
	"time"
)

// mustTime parses an RFC 3339 string and stops the test if it's invalid.
func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("bad test time %q: %v", s, err)
	}
	return v
}

func TestMinutesBetween(t *testing.T) {
	// A "table-driven" test: each row is one case, and the loop below runs them all.
	tests := []struct {
		name  string
		start string
		end   string
		want  int
	}{
		{"exact minutes", "2026-11-04T08:00:00Z", "2026-11-04T08:45:00Z", 45},
		{"leftover seconds are dropped", "2026-11-04T08:00:00Z", "2026-11-04T08:45:59Z", 45},
		{"under one minute", "2026-11-04T08:00:00Z", "2026-11-04T08:00:30Z", 0},
		{"crosses midnight", "2026-11-04T23:30:00Z", "2026-11-05T00:15:00Z", 45},
		{"several hours", "2026-11-04T06:00:00Z", "2026-11-04T09:30:00Z", 210},
		// 08:00 in India (+05:30) is 02:30 UTC, so this is a 30-minute stop.
		{"different time zones", "2026-11-04T08:00:00+05:30", "2026-11-04T03:00:00Z", 30},
		{"end equals start", "2026-11-04T08:00:00Z", "2026-11-04T08:00:00Z", 0},
		{"end before start", "2026-11-04T09:00:00Z", "2026-11-04T08:00:00Z", 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := MinutesBetween(mustTime(t, tc.start), mustTime(t, tc.end))
			if got != tc.want {
				t.Errorf("MinutesBetween(%s, %s) = %d, want %d", tc.start, tc.end, got, tc.want)
			}
		})
	}
}

func TestIsValidReason(t *testing.T) {
	for _, r := range AllReasons {
		if !IsValidReason(r) {
			t.Errorf("IsValidReason(%q) = false, want true", r)
		}
	}

	for _, r := range []string{"", "breakdown", "Coffee break", " Breakdown", "Power Cut"} {
		if IsValidReason(r) {
			t.Errorf("IsValidReason(%q) = true, want false", r)
		}
	}
}
