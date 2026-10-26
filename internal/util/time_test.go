package util

import (
	"testing"
	"time"
)

// TestRelTime_SmallWindows covers sub-hour and future cases.
// These are insensitive to wall-clock time.
func TestRelTime_SmallWindows(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name string
		in   time.Time
		want string
	}{
		{"zero returns empty", time.Time{}, ""},
		{"future collapses to just now", now.Add(1 * time.Hour), "just now"},
		{"30 seconds ago is just now", now.Add(-30 * time.Second), "just now"},
		{"5 minutes ago", now.Add(-5 * time.Minute), "5m ago"},
		{"59 minutes ago", now.Add(-59 * time.Minute), "59m ago"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RelTime(tc.in)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestRelTime_Yesterday constructs noon yesterday — unambiguously yesterday
// regardless of current clock time.
func TestRelTime_Yesterday(t *testing.T) {
	now := time.Now()
	y, m, d := now.Date()
	yesterdayNoon := time.Date(y, m, d-1, 12, 0, 0, 0, now.Location())
	got := RelTime(yesterdayNoon)
	if got != "yesterday" {
		t.Errorf("yesterday noon: got %q, want %q", got, "yesterday")
	}
}

// TestRelTime_WeekPlus — events older than 7 days render as "Jan 2".
func TestRelTime_WeekPlus(t *testing.T) {
	now := time.Now()
	tt := now.AddDate(0, 0, -30)
	got := RelTime(tt)
	expected := tt.Format("Jan 2")
	if got != expected {
		t.Errorf("30 days ago: got %q, want %q", got, expected)
	}
}
func TestFmtTime_Zero(t *testing.T) {
	if got := FmtTime(time.Time{}); got != "" {
		t.Fatalf("zero time: want empty, got %q", got)
	}
}

func TestFmtTime_Format(t *testing.T) {
	ts := time.Date(2026, time.October, 25, 16, 49, 0, 0, time.UTC)
	got := FmtTime(ts)
	want := "Oct 25, 2026 4:49 PM"
	if got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

func TestViewHeat_Zero(t *testing.T) {
	if got := ViewHeat(time.Time{}); got != "" {
		t.Fatalf("zero time: want empty, got %q", got)
	}
}

func TestViewHeat_Buckets(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name string
		t    time.Time
		want string
	}{
		{"hot_30min", now.Add(-30 * time.Minute), "hot"},
		{"warm_5h", now.Add(-5 * time.Hour), "warm"},
		{"cool_3d", now.Add(-3 * 24 * time.Hour), "cool"},
		{"cold_30d", now.Add(-30 * 24 * time.Hour), "cold"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ViewHeat(c.t); got != c.want {
				t.Fatalf("want %q, got %q", c.want, got)
			}
		})
	}
}
