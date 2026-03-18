// Package style parses Starship-compatible style strings into ANSI escape sequences.
//
// Supported syntax:
//
//	"bold"                        – modifier only
//	"red"                         – named foreground color
//	"bold green"                  – modifier + named color
//	"fg:#c792ea"                  – 24-bit hex foreground
//	"bold fg:#ff5370 bg:#1a1a2e"  – modifier + hex fg + hex bg
//	"bright_red"                  – bright/hi-intensity named color
package style

import (
	"strconv"
	"strings"
)

// Style holds a pre-computed ANSI prefix for wrapping text.
type Style struct {
	prefix string // e.g. "\033[1;31m"
}

var namedFg = map[string]int{
	"black": 30, "red": 31, "green": 32, "yellow": 33,
	"blue": 34, "purple": 35, "magenta": 35, "cyan": 36, "white": 37,
	"bright_black": 90, "bright_red": 91, "bright_green": 92, "bright_yellow": 93,
	"bright_blue": 94, "bright_purple": 95, "bright_magenta": 95, "bright_cyan": 96, "bright_white": 97,
}

var namedBg = map[string]int{
	"black": 40, "red": 41, "green": 42, "yellow": 43,
	"blue": 44, "purple": 45, "magenta": 45, "cyan": 46, "white": 47,
	"bright_black": 100, "bright_red": 101, "bright_green": 102, "bright_yellow": 103,
	"bright_blue": 104, "bright_purple": 105, "bright_magenta": 105, "bright_cyan": 106, "bright_white": 107,
}

// Parse parses a style string into a Style. Returns nil for empty strings.
func Parse(s string) *Style {
	if s == "" {
		return nil
	}
	var codes []string
	for p := range strings.FieldsSeq(s) {
		switch p {
		case "bold":
			codes = append(codes, "1")
		case "dimmed", "dim":
			codes = append(codes, "2")
		case "italic":
			codes = append(codes, "3")
		case "underline":
			codes = append(codes, "4")
		default:
			if val, ok := strings.CutPrefix(p, "fg:"); ok {
				if r, g, b, ok := parseHex(val); ok {
					codes = append(codes, "38;2;"+strconv.Itoa(int(r))+";"+strconv.Itoa(int(g))+";"+strconv.Itoa(int(b)))
				}
			} else if val, ok := strings.CutPrefix(p, "bg:"); ok {
				if r, g, b, ok := parseHex(val); ok {
					codes = append(codes, "48;2;"+strconv.Itoa(int(r))+";"+strconv.Itoa(int(g))+";"+strconv.Itoa(int(b)))
				} else if code, ok := namedBg[val]; ok {
					codes = append(codes, strconv.Itoa(code))
				}
			} else if strings.HasPrefix(p, "#") {
				if r, g, b, ok := parseHex(p); ok {
					codes = append(codes, "38;2;"+strconv.Itoa(int(r))+";"+strconv.Itoa(int(g))+";"+strconv.Itoa(int(b)))
				}
			} else if code, ok := namedFg[p]; ok {
				codes = append(codes, strconv.Itoa(code))
			}
		}
	}
	if len(codes) == 0 {
		return nil
	}
	return &Style{prefix: "\033[" + strings.Join(codes, ";") + "m"}
}

// Sprint wraps text in ANSI escape codes. Nil-safe: returns text unchanged.
func (s *Style) Sprint(text string) string {
	if s == nil {
		return text
	}
	return s.prefix + text + "\033[0m"
}

// parseHex parses #RGB or #RRGGBB hex color strings.
func parseHex(s string) (r, g, b byte, ok bool) {
	s = strings.TrimPrefix(s, "#")
	if len(s) == 3 {
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	}
	if len(s) != 6 {
		return 0, 0, 0, false
	}
	n, err := strconv.ParseUint(s, 16, 24)
	if err != nil {
		return 0, 0, 0, false
	}
	return byte(n >> 16), byte(n >> 8), byte(n), true
}
