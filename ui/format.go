package ui

import (
	"fmt"
	"time"
)

// relativeDate describes a time the way a file list does.
func relativeDate(t time.Time) string {
	now := time.Now()
	switch d := now.Sub(t); {
	case d < time.Minute:
		return "Just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case sameDay(now, t):
		return "Today, " + t.Format("15:04")
	case sameDay(now.AddDate(0, 0, -1), t):
		return "Yesterday, " + t.Format("15:04")
	case now.Year() == t.Year():
		return t.Format("Jan 2")
	}
	return t.Format("Jan 2, 2006")
}

func sameDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}

// fileSize formats a byte count.
func fileSize(n int) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%d KB", n>>10)
	}
	return fmt.Sprintf("%d B", n)
}
