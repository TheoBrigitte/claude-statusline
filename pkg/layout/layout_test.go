package layout

import (
	"strings"
	"testing"

	"github.com/keyamasabaya/claude-statusline/pkg/style"
)

func TestNewPart(t *testing.T) {
	t.Run("without style", func(t *testing.T) {
		p := NewPart("hello", nil)
		if p.Text != "hello" {
			t.Errorf("text = %q, want %q", p.Text, "hello")
		}
		if p.Length() != 5 {
			t.Errorf("length = %d, want 5", p.Length())
		}
	})

	t.Run("with style has same logical length", func(t *testing.T) {
		p := NewPart("hello", style.Parse("red"))
		if p.Length() != 5 {
			t.Errorf("length = %d, want 5 (should track raw text length, not ANSI)", p.Length())
		}
		if !strings.Contains(p.Text, "hello") {
			t.Errorf("text should contain 'hello', got %q", p.Text)
		}
	})

	t.Run("empty string", func(t *testing.T) {
		p := NewPart("", nil)
		if p.Length() != 0 {
			t.Errorf("length = %d, want 0", p.Length())
		}
	})
}

func TestPartAppend(t *testing.T) {
	t.Run("with separator", func(t *testing.T) {
		p := NewPart("a", nil)
		p.Append("b", " | ", nil)
		if p.Text != "a | b" {
			t.Errorf("text = %q, want %q", p.Text, "a | b")
		}
		if p.Length() != 5 {
			t.Errorf("length = %d, want 5", p.Length())
		}
	})

	t.Run("with style preserves logical length", func(t *testing.T) {
		p := NewPart("a", nil)
		p.Append("b", " ", style.Parse("cyan"))
		if p.Length() != 3 {
			t.Errorf("length = %d, want 3", p.Length())
		}
	})
}

func TestPartPrepend(t *testing.T) {
	p := NewPart("b", nil)
	p.Prepend("a", " | ", nil)
	if p.Text != "a | b" {
		t.Errorf("text = %q, want %q", p.Text, "a | b")
	}
	if p.Length() != 5 {
		t.Errorf("length = %d, want 5", p.Length())
	}
}

func TestPartAppendPart(t *testing.T) {
	p1 := NewPart("hello", nil)
	p2 := NewPart("world", nil)
	p1.AppendPart(p2, " ")
	if p1.Text != "hello world" {
		t.Errorf("text = %q, want %q", p1.Text, "hello world")
	}
	if p1.Length() != 11 {
		t.Errorf("length = %d, want 11", p1.Length())
	}
}

func TestPartPrependPart(t *testing.T) {
	p1 := NewPart("world", nil)
	p2 := NewPart("hello", nil)
	p1.PrependPart(p2, " ")
	if p1.Text != "hello world" {
		t.Errorf("text = %q, want %q", p1.Text, "hello world")
	}
	if p1.Length() != 11 {
		t.Errorf("length = %d, want 11", p1.Length())
	}
}

func TestLines(t *testing.T) {
	tests := []struct {
		name      string
		termWidth int
		parts     []*Part
		wantLines int
	}{
		{
			name:      "all fit on one line",
			termWidth: 100,
			parts:     []*Part{NewPart("A", nil), NewPart("B", nil), NewPart("C", nil)},
			wantLines: 1,
		},
		{
			name:      "wrap to multiple lines",
			termWidth: 10,
			parts:     []*Part{NewPart("AAAAAAA", nil), NewPart("BBBBBBB", nil)},
			wantLines: 2,
		},
		{
			name:      "empty parts skipped",
			termWidth: 100,
			parts:     []*Part{NewPart("A", nil), {Text: "", Len: 0}, NewPart("B", nil)},
			wantLines: 1,
		},
		{
			name:      "no parts",
			termWidth: 100,
			parts:     []*Part{},
			wantLines: 0,
		},
		{
			name:      "single part",
			termWidth: 5,
			parts:     []*Part{NewPart("A", nil)},
			wantLines: 1,
		},
		{
			name:      "each part on own line when narrow",
			termWidth: 5,
			parts:     []*Part{NewPart("AAA", nil), NewPart("BBB", nil), NewPart("CCC", nil)},
			wantLines: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := Lines(tt.termWidth, " | ", tt.parts)
			if len(lines) != tt.wantLines {
				t.Errorf("got %d lines, want %d: %v", len(lines), tt.wantLines, lines)
			}
		})
	}
}

func TestLinesSeparator(t *testing.T) {
	lines := Lines(100, " | ", []*Part{NewPart("A", nil), NewPart("B", nil)})
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if !strings.Contains(lines[0], " | ") {
		t.Errorf("expected separator in line: %q", lines[0])
	}
}

func TestLinesWrappedOmitsSeparator(t *testing.T) {
	lines := Lines(5, " | ", []*Part{NewPart("AAA", nil), NewPart("BBB", nil)})
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if strings.Contains(lines[0], " | ") || strings.Contains(lines[1], " | ") {
		t.Errorf("wrapped lines should not contain separator: %v", lines)
	}
}

// Benchmarks

func BenchmarkLines(b *testing.B) {
	parts := []*Part{
		NewPart("[Opus 4.6]", style.Parse("cyan")),
		NewPart("########----", style.Parse("green")),
		NewPart("(27k/1M tokens)", nil),
		NewPart("7%", nil),
		NewPart("$0.25", style.Parse("yellow")),
		NewPart("⏱️ 4m 5s", nil),
	}
	b.ResetTimer()
	for b.Loop() {
		Lines(120, " | ", parts)
	}
}

// TestLinesExactFitDoesNotWrap covers the boundary: content measuring exactly
// the available width fits, and used to be pushed onto a second line.
func TestLinesExactFitDoesNotWrap(t *testing.T) {
	// "AAA" + " | " + "BBB" is exactly 9 cells.
	parts := []*Part{NewPart("AAA", nil), NewPart("BBB", nil)}

	if lines := Lines(9, " | ", parts); len(lines) != 1 {
		t.Errorf("got %d lines for a 9 cell payload in 9 columns, want 1: %v", len(lines), lines)
	}
	if lines := Lines(8, " | ", parts); len(lines) != 2 {
		t.Errorf("got %d lines for a 9 cell payload in 8 columns, want 2: %v", len(lines), lines)
	}
}

// TestPartLengthCountsCells checks Part measures what the terminal shows, not
// how many bytes the text takes.
func TestPartLengthCountsCells(t *testing.T) {
	tests := []struct {
		name string
		text string
		want int
	}{
		{"nerd font glyph", "\U000F029A 5h", 4},
		{"block characters", "████", 4},
		{"emoji", "🟢 ok", 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewPart(tt.text, style.Parse("cyan")).Length(); got != tt.want {
				t.Errorf("Length() = %d, want %d (len is %d bytes)", got, tt.want, len(tt.text))
			}
		})
	}
}

// TestLinesWrapsOnCellsNotBytes is the layout-level guard for the same
// regression: a payload of block characters fits, even though its byte length
// is three times the available width.
func TestLinesWrapsOnCellsNotBytes(t *testing.T) {
	parts := []*Part{NewPart("██████████", nil), NewPart("██████████", nil)}

	lines := Lines(23, " | ", parts)
	if len(lines) != 1 {
		t.Errorf("got %d lines, want 1: %v", len(lines), lines)
	}
}
