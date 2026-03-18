// Package terminal provides terminal dimension utilities.
package terminal

import (
	"os"

	"golang.org/x/term"
)

const defaultWidth = 80

// Width returns the current terminal width, defaulting to defaultWidth if unavailable.
// Uses stderr's fd which is connected to the terminal even when stdin is piped.
func Width() int {
	f, err := os.Open("/dev/tty")
	if err != nil {
		return defaultWidth
	}
	defer f.Close() //nolint:errcheck

	w, _, err := term.GetSize(int(f.Fd()))
	if err != nil {
		return defaultWidth
	}
	return w
}
