package width

import "testing"

func TestCells(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int
	}{
		{"empty", "", 0},
		{"ascii", "hello", 5},
		{"ascii with punctuation", "[Opus 5]", 8},

		// Nerd Font glyphs live in the private use areas and render in a
		// single cell, but are 3 to 4 bytes long in UTF-8.
		{"nerd font glyph", "\U000F029A", 1},
		{"nerd font rate limit symbol", "\U000F029A 5h: ", 6},

		// Block drawing characters are the documented context bar fill.
		{"block characters", "██░░", 4},

		// Emoji occupy two cells regardless of their byte length, whether or
		// not they carry an explicit variation selector.
		{"emoji", "🟢", 2},
		{"emoji with variation selector", "⏱️", 2},
		{"emoji with trailing text", "⏱️ 4m 5s", 8},
		{"zero width joiner sequence", "👨‍👩‍👧", 2},

		// Combining marks fold into the cluster they decorate.
		{"precomposed accent", "é", 1},
		{"combining accent", "é", 1},

		{"east asian wide", "日本語", 6},

		// ANSI sequences are transmitted but render nothing.
		{"sgr wrapped", "\x1b[36m[Opus 5]\x1b[0m", 8},
		{"truecolor sgr", "\x1b[1;38;2;255;136;0mX\x1b[0m", 1},
		{"sgr only", "\x1b[0m", 0},
		{"osc hyperlink", "\x1b]8;;https://example.com\ahi\x1b]8;;\a", 2},
		{"osc with string terminator", "\x1b]0;title\x1b\\hi", 2},
		{"charset designation", "\x1b(Bxy", 2},
		{"truncated csi", "abc\x1b[3", 3},
		{"lone escape", "abc\x1b", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Cells(tt.in); got != tt.want {
				t.Errorf("Cells(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

// TestCellsIgnoresStyling asserts the property the layout relies on: styling a
// string never changes how much room it needs on screen.
func TestCellsIgnoresStyling(t *testing.T) {
	for _, plain := range []string{"hello", "\U000F029A 5h: 42.50%", "⏱️ 4m 5s", "██░░░░"} {
		styled := "\x1b[1;36m" + plain + "\x1b[0m"
		if got, want := Cells(styled), Cells(plain); got != want {
			t.Errorf("Cells(styled %q) = %d, want %d", plain, got, want)
		}
	}
}

// TestCellsUnderestimatesNothing guards the regression this package exists to
// fix: byte length is an upper bound, never the value to lay out against.
func TestCellsUnderestimatesNothing(t *testing.T) {
	const bar = "\U000F029A ██████░░░░░░░░ ⏱️ 4m 5s"
	if cells, bytes := Cells(bar), len(bar); cells >= bytes {
		t.Errorf("Cells(%q) = %d, len = %d: expected the cell count to be smaller", bar, cells, bytes)
	}
}

// Benchmarks

func BenchmarkCellsASCII(b *testing.B) {
	for b.Loop() {
		Cells("[Opus 5] | ########---- | $0.25")
	}
}

func BenchmarkCellsStyled(b *testing.B) {
	for b.Loop() {
		Cells("\x1b[36m\U000F029A 5h: 42.50% ~2h30m\x1b[0m ⏱️ 4m 5s")
	}
}
