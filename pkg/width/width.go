// Package width measures how many terminal cells a string occupies once
// printed.
//
// The status line is laid out against a terminal width expressed in columns,
// so every length that feeds the layout has to be a cell count. Go's len()
// returns bytes, which is only the same number for pure ASCII: a Nerd Font
// glyph such as "󰊚" is 4 bytes but 1 cell, "█" is 3 bytes but 1 cell, and
// "🟢" is 4 bytes but 2 cells. Measuring in bytes therefore over-estimates
// any styled or icon-bearing segment and wraps the line earlier than needed.
package width

import (
	"strings"

	"github.com/rivo/uniseg"
)

const escape = '\x1b'

// Cells returns the number of terminal cells s occupies when printed.
//
// It differs from len(s) in two ways that both matter for layout:
//
//   - ANSI escape sequences are skipped. They are sent to the terminal but
//     render nothing, so a styled string measures the same as its plain text.
//   - Text is measured per grapheme cluster in cells rather than in bytes.
//     Combining marks and variation selectors fold into their base cluster,
//     emoji count as 2 cells, and East Asian wide characters count as 2.
func Cells(s string) int {
	if s == "" {
		return 0
	}

	// Fast path: printable ASCII is one cell per byte, and most segments of a
	// status line — numbers, separators, the default bar characters — are
	// nothing else. Walking the bytes once here is cheaper than a full
	// grapheme cluster pass.
	printableASCII, hasEscape := true, false
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] > 0x7e {
			printableASCII = false
			if s[i] == escape {
				hasEscape = true
				break
			}
		}
	}
	if printableASCII {
		return len(s)
	}
	if !hasEscape {
		return uniseg.StringWidth(s)
	}

	total := 0
	for s != "" {
		i := strings.IndexByte(s, escape)
		if i < 0 {
			total += uniseg.StringWidth(s)
			break
		}
		total += uniseg.StringWidth(s[:i])

		n := escapeLen(s[i:])
		if n == 0 {
			// Truncated sequence running to the end of the string: whatever
			// follows the ESC is part of it and renders nothing.
			break
		}
		s = s[i+n:]
	}
	return total
}

// escapeLen returns the byte length of the ANSI escape sequence at the start
// of s, which must begin with ESC. It returns 0 when the sequence is
// truncated, i.e. no terminator appears before the end of s.
func escapeLen(s string) int {
	if len(s) < 2 {
		return 0
	}

	switch s[1] {
	case '[':
		// CSI: ESC [ parameters, ended by a byte in the range @ to ~.
		// This covers the SGR colour codes emitted by pkg/style.
		for i := 2; i < len(s); i++ {
			if s[i] >= '@' && s[i] <= '~' {
				return i + 1
			}
		}
		return 0
	case ']':
		// OSC: ESC ] payload, ended by BEL or by ST (ESC \).
		for i := 2; i < len(s); i++ {
			if s[i] == '\a' {
				return i + 1
			}
			if s[i] == escape && i+1 < len(s) && s[i+1] == '\\' {
				return i + 2
			}
		}
		return 0
	default:
		// nF-class escape: ESC, zero or more intermediate bytes in the range
		// 0x20 to 0x2F, then one final byte — e.g. ESC ( B to select a
		// character set.
		if s[1] >= 0x20 && s[1] <= 0x2f {
			for i := 2; i < len(s); i++ {
				if s[i] < 0x20 || s[i] > 0x2f {
					return i + 1
				}
			}
			return 0
		}
		// Fe-class two-byte escape, e.g. ESC \ (ST) or ESC M.
		return 2
	}
}
