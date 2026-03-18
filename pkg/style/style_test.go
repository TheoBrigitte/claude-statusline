package style

import (
	"strings"
	"testing"
)

func TestParseNil(t *testing.T) {
	if s := Parse(""); s != nil {
		t.Error("empty string should return nil")
	}
	if s := Parse("   "); s != nil {
		t.Error("whitespace-only should return nil")
	}
	if s := Parse("bogus_token"); s != nil {
		t.Errorf("unrecognised token should return nil, got prefix %q", s.prefix)
	}
}

func TestSprintNilSafe(t *testing.T) {
	var s *Style
	if got := s.Sprint("hello"); got != "hello" {
		t.Errorf("nil Sprint = %q, want %q", got, "hello")
	}
}

func TestNamedColors(t *testing.T) {
	tests := []struct {
		input    string
		wantCode string
	}{
		{"red", "\033[31m"},
		{"green", "\033[32m"},
		{"cyan", "\033[36m"},
		{"yellow", "\033[33m"},
		{"bright_red", "\033[91m"},
		{"bright_cyan", "\033[96m"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			s := Parse(tt.input)
			if s == nil {
				t.Fatal("expected non-nil style")
			}
			got := s.Sprint("x")
			if !strings.HasPrefix(got, tt.wantCode) {
				t.Errorf("Sprint = %q, want prefix %q", got, tt.wantCode)
			}
			if !strings.HasSuffix(got, "\033[0m") {
				t.Errorf("Sprint = %q, should end with reset", got)
			}
		})
	}
}

func TestModifiers(t *testing.T) {
	tests := []struct {
		input      string
		wantPrefix string
	}{
		{"bold", "\033[1m"},
		{"italic", "\033[3m"},
		{"underline", "\033[4m"},
		{"dimmed", "\033[2m"},
		{"dim", "\033[2m"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			s := Parse(tt.input)
			if s == nil {
				t.Fatal("expected non-nil style")
			}
			if s.prefix != tt.wantPrefix {
				t.Errorf("prefix = %q, want %q", s.prefix, tt.wantPrefix)
			}
		})
	}
}

func TestHexForeground(t *testing.T) {
	tests := []struct {
		input string
		want  string // expected prefix
	}{
		{"fg:#ff0000", "\033[38;2;255;0;0m"},
		{"#00ff00", "\033[38;2;0;255;0m"},
		{"fg:#abc", "\033[38;2;170;187;204m"}, // shorthand
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			s := Parse(tt.input)
			if s == nil {
				t.Fatal("expected non-nil style")
			}
			if s.prefix != tt.want {
				t.Errorf("prefix = %q, want %q", s.prefix, tt.want)
			}
		})
	}
}

func TestHexBackground(t *testing.T) {
	s := Parse("bg:#1a1a2e")
	if s == nil {
		t.Fatal("expected non-nil style")
	}
	want := "\033[48;2;26;26;46m"
	if s.prefix != want {
		t.Errorf("prefix = %q, want %q", s.prefix, want)
	}
}

func TestNamedBackground(t *testing.T) {
	s := Parse("bg:red")
	if s == nil {
		t.Fatal("expected non-nil style")
	}
	want := "\033[41m"
	if s.prefix != want {
		t.Errorf("prefix = %q, want %q", s.prefix, want)
	}
}

func TestCombined(t *testing.T) {
	s := Parse("bold fg:#ff5370 bg:#1a1a2e")
	if s == nil {
		t.Fatal("expected non-nil style")
	}
	got := s.Sprint("test")
	if !strings.Contains(got, "test") {
		t.Errorf("output should contain text: %q", got)
	}
	if !strings.HasPrefix(got, "\033[") {
		t.Errorf("output should start with ESC: %q", got)
	}
	// Should contain all three codes joined with ;
	if !strings.Contains(s.prefix, "1;") {
		t.Errorf("prefix should contain bold code: %q", s.prefix)
	}
}

func TestParseHex(t *testing.T) {
	tests := []struct {
		input   string
		r, g, b byte
		ok      bool
	}{
		{"#ff0000", 255, 0, 0, true},
		{"#abc", 170, 187, 204, true},
		{"ff5370", 255, 83, 112, true},
		{"zzzzzz", 0, 0, 0, false},
		{"", 0, 0, 0, false},
		{"#12", 0, 0, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			r, g, b, ok := parseHex(tt.input)
			if ok != tt.ok || r != tt.r || g != tt.g || b != tt.b {
				t.Errorf("parseHex(%q) = (%d,%d,%d,%v), want (%d,%d,%d,%v)",
					tt.input, r, g, b, ok, tt.r, tt.g, tt.b, tt.ok)
			}
		})
	}
}

// Benchmarks

func BenchmarkParse(b *testing.B) {
	for b.Loop() {
		Parse("bold fg:#ff5370 bg:#1a1a2e")
	}
}

func BenchmarkParseSimple(b *testing.B) {
	for b.Loop() {
		Parse("cyan")
	}
}

func BenchmarkSprint(b *testing.B) {
	s := Parse("bold fg:#ff5370 bg:#1a1a2e")
	b.ResetTimer()
	for b.Loop() {
		s.Sprint("[Opus 4.6 (1M context)]")
	}
}
