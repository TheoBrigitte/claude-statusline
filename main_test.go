package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/keyamasabaya/claude-statusline/pkg/config"
	"github.com/keyamasabaya/claude-statusline/pkg/layout"
	"github.com/keyamasabaya/claude-statusline/pkg/model"
	"github.com/keyamasabaya/claude-statusline/pkg/width"
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

func BenchmarkMeasureSegment(b *testing.B) {
	cfg := testConfig()
	in := testInput(b)
	modules := renderModules(cfg, in, 120)
	seg := "$model $context_bar $context_tokens $context_pct"
	rendered := renderSegment(seg, modules)
	b.ResetTimer()
	for b.Loop() {
		width.Cells(rendered)
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
					Len:  width.Cells(rendered),
				})
			}
			layout.Lines(termWidth, cfg.Separator, parts)
		}
	}
}

// Tests

// wideGlyphConfig mirrors the Nerd Font setup documented in the README: block
// characters for the context bar and a glyph symbol on the duration module.
// Every one of those is a single cell on screen but three bytes in memory.
// modelStyle is injected so the same layout can be rendered with and without
// ANSI colours.
func wideGlyphConfig(modelStyle string) string {
	return `
lines = ["$model | $context_bar $context_pct | $cost | $duration"]

[status]
disabled = true

[model]
min_term_width = 0
style = "` + modelStyle + `"

[duration]
symbol = "\uf017 "

[context_bar]
width = 20
fill_char = "\u2588"
empty_char = "\u2591"
min_term_width = 0
`
}

func writeConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "claude-statusline.toml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func render(t *testing.T, configPath string, termWidth int) []string {
	t.Helper()
	var buf bytes.Buffer
	if err := runWith(configPath, "", strings.NewReader(testInputJSON), &buf, termWidth); err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
}

// TestRunWithWrapsOnCellsNotBytes is the regression guard for the layout being
// measured in terminal cells. The rendered line is 67 cells wide, so it fits an
// 80 column terminal minus the default padding of 5; measured in bytes it is
// 109 long and used to be split across two lines.
func TestRunWithWrapsOnCellsNotBytes(t *testing.T) {
	cfgPath := writeConfig(t, wideGlyphConfig(""))

	const termWidth = 80
	lines := render(t, cfgPath, termWidth)

	if len(lines) != 1 {
		t.Errorf("got %d lines at width %d, want 1:\n%s", len(lines), termWidth, strings.Join(lines, "\n"))
	}
	for _, line := range lines {
		if cells := width.Cells(line); cells > termWidth {
			t.Errorf("line is %d cells wide, exceeds terminal width %d: %q", cells, termWidth, line)
		}
	}
}

// TestRunWithStillWrapsWhenTooNarrow checks the fix did not simply disable
// wrapping: a terminal too narrow for the same content must still split it.
func TestRunWithStillWrapsWhenTooNarrow(t *testing.T) {
	cfgPath := writeConfig(t, wideGlyphConfig(""))

	lines := render(t, cfgPath, 30)
	if len(lines) < 2 {
		t.Errorf("got %d lines at width 30, want at least 2:\n%s", len(lines), strings.Join(lines, "\n"))
	}
}

// TestRenderedWidthIsIndependentOfStyling checks that turning colours on does
// not change how much room the status line is believed to need.
func TestRenderedWidthIsIndependentOfStyling(t *testing.T) {
	plain := render(t, writeConfig(t, wideGlyphConfig("")), 80)
	styled := render(t, writeConfig(t, wideGlyphConfig("bold cyan")), 80)

	if len(plain) != len(styled) {
		t.Fatalf("styling changed the line count: %d vs %d", len(plain), len(styled))
	}
	for i := range plain {
		if got, want := width.Cells(styled[i]), width.Cells(plain[i]); got != want {
			t.Errorf("line %d: styled width %d, plain width %d", i, got, want)
		}
	}
}
