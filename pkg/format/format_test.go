package format

import (
	"fmt"
	"testing"
)

func TestSI(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1k"},
		{1500, "1k"},
		{20000, "20k"},
		{200000, "200k"},
		{1000000, "1M"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := SI(tt.n); got != tt.want {
				t.Errorf("SI(%d) = %q, want %q", tt.n, got, tt.want)
			}
		})
	}
}

func TestDuration(t *testing.T) {
	tests := []struct {
		ms   int
		want string
	}{
		{0, "0m 0s"},
		{5000, "0m 5s"},
		{60000, "1m 0s"},
		{90000, "1m 30s"},
		{245000, "4m 5s"},
		{3_600_000, "60m 0s"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%dms", tt.ms), func(t *testing.T) {
			if got := Duration(tt.ms); got != tt.want {
				t.Errorf("Duration(%d) = %q, want %q", tt.ms, got, tt.want)
			}
		})
	}
}

func TestCost(t *testing.T) {
	tests := []struct {
		cost float64
		want string
	}{
		{0, "$0.00"},
		{0.001, "$0.00"},
		{0.005, "$0.01"},
		{0.08734, "$0.09"},
		{1.5, "$1.50"},
		{12.345, "$12.35"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := Cost(tt.cost); got != tt.want {
				t.Errorf("Cost(%f) = %q, want %q", tt.cost, got, tt.want)
			}
		})
	}
}

// Benchmarks

func BenchmarkCost(b *testing.B) {
	for b.Loop() {
		Cost(0.247)
	}
}

func BenchmarkDuration(b *testing.B) {
	for b.Loop() {
		Duration(245000)
	}
}

func BenchmarkSI(b *testing.B) {
	for b.Loop() {
		SI(27400)
	}
}
