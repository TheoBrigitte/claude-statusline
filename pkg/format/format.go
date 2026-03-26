// Package format provides formatting functions for cost, duration, and SI units.
package format

import (
	"strconv"
	"time"
)

// Cost formats a USD amount as "$X.XX".
func Cost(usd float64) string {
	return "$" + strconv.FormatFloat(usd, 'f', 2, 64)
}

// Duration formats milliseconds as "Xm Ys".
func Duration(ms int) string {
	mins := ms / 60000
	secs := (ms % 60000) / 1000
	return strconv.Itoa(mins) + "m " + strconv.Itoa(secs) + "s"
}

// TimeUntil formats a Unix timestamp as a compact relative countdown (e.g. "2h30m", "3d5h").
// Returns an empty string if the timestamp is zero or already in the past.
func TimeUntil(unixTS int64) string {
	if unixTS == 0 {
		return ""
	}
	d := time.Until(time.Unix(unixTS, 0))
	if d <= 0 {
		return ""
	}
	d = d.Truncate(time.Minute)
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	switch {
	case days > 0:
		return strconv.Itoa(days) + "d" + strconv.Itoa(hours) + "h"
	case hours > 0:
		return strconv.Itoa(hours) + "h" + strconv.Itoa(mins) + "m"
	default:
		return strconv.Itoa(mins) + "m"
	}
}

// SI formats a number with SI suffixes (e.g. 1500 -> "1k", 1000000 -> "1M").
func SI(n int) string {
	switch {
	case n >= 1_000_000_000:
		return strconv.Itoa(n/1_000_000_000) + "G"
	case n >= 1_000_000:
		return strconv.Itoa(n/1_000_000) + "M"
	case n >= 1_000:
		return strconv.Itoa(n/1_000) + "k"
	default:
		return strconv.Itoa(n)
	}
}
