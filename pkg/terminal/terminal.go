// Package terminal provides terminal dimension utilities.
package terminal

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"golang.org/x/term"
)

const DefaultWidth = 80

// Width returns the current terminal width in columns.
//
// It queries /dev/tty, which stays connected to the terminal even when stdin
// is piped — as it always is for a status line. When that yields no usable
// size, because /dev/tty is absent or reports none, it falls back to the
// COLUMNS environment variable.
func Width() (int, error) {
	if w, ok := widthFromTTY(); ok {
		return w, nil
	}
	return widthFromEnv()
}

// widthFromTTY reports the size of the controlling terminal, and whether it
// could be determined at all.
func widthFromTTY() (int, bool) {
	f, err := os.Open("/dev/tty")
	if err != nil {
		return 0, false
	}
	defer f.Close() //nolint:errcheck

	w, _, err := term.GetSize(int(f.Fd())) //nolint:gosec
	if err != nil || w <= 0 {
		return 0, false
	}
	return w, true
}

// widthFromEnv reads the width from the COLUMNS environment variable.
func widthFromEnv() (int, error) {
	columns, exists := os.LookupEnv("COLUMNS")
	if !exists {
		return -1, errors.New("no size reported by /dev/tty and COLUMNS is unset")
	}

	w, err := strconv.Atoi(columns)
	if err != nil {
		return -1, err
	}
	if w <= 0 {
		return -1, fmt.Errorf("COLUMNS is %d, want a positive width", w)
	}

	return w, nil
}
