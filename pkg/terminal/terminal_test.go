package terminal

import "testing"

func BenchmarkWidth(b *testing.B) {
	for b.Loop() {
		Width()
	}
}
