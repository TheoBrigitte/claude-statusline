// Package terminal provides terminal dimension utilities.
package terminal

import (
	"os"
	"strconv"

	"golang.org/x/term"
)

const DefaultWidth = 80

// Width returns the current terminal width, defaulting to defaultWidth if unavailable.
// Uses stderr's fd which is connected to the terminal even when stdin is piped.
func Width() (int, error) {
	f, err := os.Open("/dev/tty")
	if err != nil {
		columns, exists := os.LookupEnv("COLUMNS")
		if !exists {
			return -1, err
		}

		w, err := strconv.Atoi(columns)
		if err != nil {
			return -1, err
		}

		return w, nil
	}
	defer f.Close() //nolint:errcheck

	w, _, err := term.GetSize(int(f.Fd())) //nolint:gosec
	if err != nil {
		return -1, err
	}

	return w, nil
}
