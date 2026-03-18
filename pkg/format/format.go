// Package format provides formatting functions for cost, duration, and SI units.
package format

import (
	"strconv"
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
