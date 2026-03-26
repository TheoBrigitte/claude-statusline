package main

import (
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/TheoBrigitte/claude-statusline/pkg/config"
	"github.com/TheoBrigitte/claude-statusline/pkg/layout"
	"github.com/TheoBrigitte/claude-statusline/pkg/model"
)

const testInputJSON = `{
	"session_id": "abc123",
	"model": {"id": "claude-opus-4-6[1m]", "display_name": "Opus 4.6 (1M context)"},
	"cost": {"total_cost_usd": 0.247, "total_duration_ms": 245000},
	"context_window": {
		"context_window_size": 1000000,
		"current_usage": {"input_tokens": 21000, "output_tokens": 6400},
		"used_percentage": 7,
		"remaining_percentage": 93
	}
}`

func testInput(b *testing.B) model.Input {
	b.Helper()
	var in model.Input
	if err := json.Unmarshal([]byte(testInputJSON), &in); err != nil {
		b.Fatal(err)
	}
	return in
}

func testConfig() config.Config {
	cfg := config.Default()
	cfg.Status.Disabled = true // avoid network/file I/O in benchmarks
	return cfg
}

// BenchmarkRunWith benchmarks the full main pipeline: config loading,
// JSON decoding from a reader, rendering all modules, and writing output.
func BenchmarkRunWith(b *testing.B) {
	for b.Loop() {
		r := strings.NewReader(testInputJSON)
		if err := runWith("", "", r, io.Discard, 120); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRenderModules(b *testing.B) {
	cfg := testConfig()
	in := testInput(b)
	b.ResetTimer()
	for b.Loop() {
		renderModules(cfg, in, 120)
	}
}

func BenchmarkApplyFormat(b *testing.B) {
	for b.Loop() {
		applyFormat("{symbol}{value}", "Opus 4.6", "🤖 ")
	}
}

func BenchmarkRenderSegment(b *testing.B) {
	cfg := testConfig()
	in := testInput(b)
	modules := renderModules(cfg, in, 120)
	seg := "$model $context_bar $context_tokens $context_pct"
	b.ResetTimer()
	for b.Loop() {
		renderSegment(seg, modules)
	}
}

func BenchmarkDisplayLen(b *testing.B) {
	cfg := testConfig()
	in := testInput(b)
	modules := renderModules(cfg, in, 120)
	seg := "$model $context_bar $context_tokens $context_pct"
	rendered := renderSegment(seg, modules)
	b.ResetTimer()
	for b.Loop() {
		displayLen(rendered, modules, seg)
	}
}

func BenchmarkEndToEnd(b *testing.B) {
	cfg := testConfig()
	in := testInput(b)
	termWidth := 120 - cfg.Padding
	b.ResetTimer()
	for b.Loop() {
		modules := renderModules(cfg, in, termWidth)
		for _, lineTemplate := range cfg.Lines {
			segments := strings.Split(lineTemplate, cfg.Separator)
			var parts []*layout.Part
			for _, seg := range segments {
				seg = strings.TrimSpace(seg)
				rendered := renderSegment(seg, modules)
				if rendered == "" {
					continue
				}
				parts = append(parts, &layout.Part{
					Text: rendered,
					Len:  displayLen(rendered, modules, seg),
				})
			}
			layout.Lines(termWidth, cfg.Separator, parts)
		}
	}
}
