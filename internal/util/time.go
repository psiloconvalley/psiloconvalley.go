// internal/util/time.go
package util

import (
	"fmt"
	"time"
)

// RelTime renders a past time as a short, operator-friendly string.
// Returned strings are English; callers can wrap or map them to other
// locales as needed.
//
//	< 60 seconds   -> "just now"
//	< 60 minutes   -> "Nm ago"
//	< 24 hours     -> "Nh ago"
//	same calendar day as yesterday -> "yesterday"
//	< 7 days       -> "Nd ago"
//	otherwise      -> "Jan 2"
//
// Future timestamps (clock skew) collapse to "just now".
// Zero-value times return the empty string so templates can render "—"
// via their own conditional logic.

func RelTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	now := time.Now()
	diff := now.Sub(t)
	if diff < 0 {
		return "just now"
	}
	if diff < time.Minute {
		return "just now"
	}
	if diff < time.Hour {
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	}

	// Calendar-day comparison for "yesterday" happens BEFORE the <24h branch,
	// because an event at 11pm yesterday is "yesterday" even if it was <24h ago.
	y, m, d := now.Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	yesterday := today.AddDate(0, 0, -1)
	ty, tm, td := t.Date()
	tDate := time.Date(ty, tm, td, 0, 0, 0, 0, t.Location())

	if tDate.Equal(today) {
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	}
	if tDate.Equal(yesterday) {
		return "yesterday"
	}
	if diff < 7*24*time.Hour {
		return fmt.Sprintf("%dd ago", int(diff.Hours()/24))
	}
	return t.Format("Jan 2")
}
// FmtTime renders a timestamp in operator-friendly absolute form:
// "Oct 25, 2026 4:49 AM". Used for hover tooltips alongside RelTime.
// Zero time renders as empty string.
func FmtTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("Jan 2, 2006 3:04 PM")
}

// ViewHeat buckets a view timestamp into one of four heat levels
// used for CSS color coding in list views:
//   "hot"  = < 1h   (bright green)
//   "warm" = < 24h  (green)
//   "cool" = < 7d   (amber)
//   "cold" = >= 7d  (gray)
// Zero time returns "" (caller should branch on never-opened separately).
func ViewHeat(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Hour:
		return "hot"
	case d < 24*time.Hour:
		return "warm"
	case d < 7*24*time.Hour:
		return "cool"
	default:
		return "cold"
	}
}
