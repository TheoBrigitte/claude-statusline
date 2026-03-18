// Package layout handles assembling and wrapping status line parts.
package layout

import "github.com/TheoBrigitte/claude-statusline/pkg/style"

// Part represents a segment of the status line, tracking both the
// rendered text (with ANSI colors) and the logical display width.
type Part struct {
	Text string
	Len  int
}

// NewPart creates a Part with the given text, optionally styled.
func NewPart(text string, s *style.Style) *Part {
	p := &Part{}
	p.Append(text, "", s)
	return p
}

// Append adds text to the end of the part.
func (p *Part) Append(text, separator string, s *style.Style) {
	p.Text += separator + s.Sprint(text)
	p.Len += len(text + separator)
}

// Prepend adds text to the beginning of the part.
func (p *Part) Prepend(text, separator string, s *style.Style) {
	p.Text = s.Sprint(text) + separator + p.Text
	p.Len += len(text + separator)
}

// AppendPart appends another part's content with a separator.
func (p *Part) AppendPart(other *Part, separator string) {
	p.Text += separator + other.Text
	p.Len += other.Len + len(separator)
}

// PrependPart prepends another part's content with a separator.
func (p *Part) PrependPart(other *Part, separator string) {
	p.Text = other.Text + separator + p.Text
	p.Len += other.Len + len(separator)
}

// Length returns the logical display width (excluding ANSI escape codes).
func (p *Part) Length() int {
	return p.Len
}

// Lines groups parts into lines joined by " | ", wrapping when a line
// would exceed termWidth.
func Lines(termPaddedWidth int, separator string, parts []*Part) []string {
	var lines []string
	var lineWidth int
	for _, p := range parts {
		if p.Text == "" {
			continue
		}
		candidateWidth := lineWidth + len(separator) + p.Length()
		if len(lines) == 0 || candidateWidth >= termPaddedWidth {
			lines = append(lines, p.Text)
			lineWidth = p.Length()
		} else {
			lines[len(lines)-1] += separator + p.Text
			lineWidth = candidateWidth
		}
	}
	return lines
}
