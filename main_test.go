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
		renderModules(&cfg, &in, 120)
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
	modules := renderModules(&cfg, &in, 120)
	seg := "$model $context_bar $context_tokens $context_pct"
	b.ResetTimer()
	for b.Loop() {
		renderSegment(seg, modules)
	}
}

func BenchmarkMeasureSegment(b *testing.B) {
	cfg := testConfig()
	in := testInput(b)
	modules := renderModules(&cfg, &in, 120)
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
		modules := renderModules(&cfg, &in, termWidth)
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

// TestContextBarClampsPercentage guards against a panic: a percentage outside
// 0-100 used to ask strings.Repeat for a negative count, which crashed the
// whole status line and left the prompt with no output at all.
func TestContextBarClampsPercentage(t *testing.T) {
	for _, pct := range []string{"150", "-10", "0", "100", "1e9"} {
		t.Run(pct, func(t *testing.T) {
			in := `{"model":{"display_name":"Opus 5"},` +
				`"context_window":{"context_window_size":200000,"used_percentage":` + pct + `}}`

			var buf bytes.Buffer
			cfgPath := writeConfig(t, wideGlyphConfig(""))
			if err := runWith(cfgPath, "", strings.NewReader(in), &buf, 120); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if buf.Len() == 0 {
				t.Error("rendered nothing")
			}
		})
	}
}

func TestApplyFormat(t *testing.T) {
	tests := []struct {
		name          string
		format        string
		value, symbol string
		want          string
	}{
		{"empty format prepends symbol", "", "42", "$ ", "$ 42"},
		{"single placeholders", "{symbol}{value}", "42", "$ ", "$ 42"},
		{"literal text preserved", "[{value}]", "42", "", "[42]"},
		// Every occurrence is substituted, not just the first.
		{"repeated symbol", "{symbol}{value} {symbol}", "42", "@", "@42 @"},
		{"repeated value", "{value}/{value}", "42", "", "42/42"},
		{"no placeholder", "static", "42", "@", "static"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := applyFormat(tt.format, tt.value, tt.symbol); got != tt.want {
				t.Errorf("applyFormat(%q, %q, %q) = %q, want %q", tt.format, tt.value, tt.symbol, got, tt.want)
			}
		})
	}
}

func TestApplyRateLimitFormat(t *testing.T) {
	tests := []struct {
		name                 string
		format               string
		value, symbol, reset string
		want                 string
	}{
		{"with reset", "{symbol}{value}% ~{reset}", "42.50", "5h: ", "2h30m", "5h: 42.50% ~2h30m"},
		{"without reset drops dangling prefix", "{symbol}{value}% ~{reset}", "42.50", "5h: ", "", "5h: 42.50%"},
		{"repeated reset", "{value} {reset} {reset}", "42", "", "1h", "42 1h 1h"},
		{"repeated reset when empty", "{value} ~{reset} ~{reset}", "42", "", "", "42"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyRateLimitFormat(tt.format, tt.value, tt.symbol, tt.reset)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestHiddenModuleLeavesNoGap covers a module hidden by min_term_width: its
// token renders empty and used to leave the spaces around it behind as a
// visible double space.
func TestHiddenModuleLeavesNoGap(t *testing.T) {
	cfg := `
lines = ["$context_bar $context_tokens $context_pct"]

[status]
disabled = true

[context_bar]
width = 6
min_term_width = 0

[context_tokens]
min_term_width = 999

[context_pct]
min_term_width = 0
`
	lines := render(t, writeConfig(t, cfg), 120)
	for _, line := range lines {
		if strings.Contains(line, "  ") {
			t.Errorf("hidden module left a double space: %q", line)
		}
	}
}

// TestDebugWarningStaysOffStdout checks diagnostics never land in the status
// line itself, which is what Claude Code renders verbatim.
func TestDebugWarningStaysOffStdout(t *testing.T) {
	var buf bytes.Buffer
	cfgPath := writeConfig(t, wideGlyphConfig(""))
	if err := runWith(cfgPath, "", strings.NewReader(testInputJSON), &buf, 120); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "warning:") {
		t.Errorf("stdout carries a warning: %q", buf.String())
	}
}

// richInputJSON exercises the fields the default line ignores.
const richInputJSON = `{
	"cwd": "/home/keya/projects/dejavu",
	"model": {"display_name": "Opus 5"},
	"workspace": {"current_dir": "/home/keya/projects/dejavu"},
	"version": "2.0.14",
	"output_style": {"name": "Explanatory"},
	"cost": {"total_cost_usd": 1.247, "total_duration_ms": 245000, "total_api_duration_ms": 98000,
	         "total_lines_added": 342, "total_lines_removed": 117},
	"context_window": {"context_window_size": 200000, "total_input_tokens": 218000,
	                   "total_output_tokens": 34500, "used_percentage": 13}
}`

// stripANSI removes escape sequences so a test can assert on the text a
// module produces without pinning the colors it is styled with.
func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\x1b' {
			b.WriteByte(s[i])
			continue
		}
		for i++; i < len(s); i++ {
			if s[i] >= '@' && s[i] <= '~' && s[i] != '[' {
				break
			}
		}
	}
	return b.String()
}

func renderToken(t *testing.T, token, inputJSON string) string {
	t.Helper()
	cfg := `lines = ["` + token + `"]` + "\n[status]\ndisabled = true\n"
	var buf bytes.Buffer
	if err := runWith(writeConfig(t, cfg), "", strings.NewReader(inputJSON), &buf, 200); err != nil {
		t.Fatal(err)
	}
	return stripANSI(strings.TrimRight(buf.String(), "\n"))
}

// TestModulesOverPreviouslyUnusedData covers the modules added over fields
// Claude Code already sends and the status line used to decode and discard.
func TestModulesOverPreviouslyUnusedData(t *testing.T) {
	tests := []struct {
		token string
		want  string
	}{
		{"$diff", "+342/-117"},
		{"$dir", "dejavu"},
		{"$api_duration", "api 1m 38s"},
		{"$session_tokens", "↑218k ↓34k"},
		{"$version", "v2.0.14"},
		{"$output_style", "Explanatory"},
	}
	for _, tt := range tests {
		t.Run(tt.token, func(t *testing.T) {
			if got := renderToken(t, tt.token, richInputJSON); got != tt.want {
				t.Errorf("%s rendered %q, want %q", tt.token, got, tt.want)
			}
		})
	}
}

// TestModulesHideWithoutData checks a module with nothing to say disappears
// rather than rendering an empty shell like "+0/-0".
func TestModulesHideWithoutData(t *testing.T) {
	const bare = `{"model":{"display_name":"Opus 5"}}`
	for _, token := range []string{"$diff", "$dir", "$session_tokens", "$version", "$output_style"} {
		t.Run(token, func(t *testing.T) {
			if got := renderToken(t, "$model "+token, bare); got != "[Opus 5]" {
				t.Errorf("rendered %q, want just the model", got)
			}
		})
	}
}

// TestUnreferencedModulesAreNotRendered is what keeps the module list cheap to
// grow: a module absent from every line template does no work at all. For
// $status that work is an HTTP request.
func TestUnreferencedModulesAreNotRendered(t *testing.T) {
	cfg := config.Default()
	cfg.Lines = []string{"$model | $cost"}

	var in model.Input
	if err := json.Unmarshal([]byte(richInputJSON), &in); err != nil {
		t.Fatal(err)
	}

	got := renderModules(&cfg, &in, 200)
	for _, token := range []string{"$model", "$cost"} {
		if _, ok := got[token]; !ok {
			t.Errorf("%s is on the line but was not rendered", token)
		}
	}
	for _, token := range []string{"$status", "$context_bar", "$rate_5h", "$diff"} {
		if _, ok := got[token]; ok {
			t.Errorf("%s is not on any line but was rendered", token)
		}
	}
}

// TestModuleTokensAreUnambiguous guards the substitution: tokens are replaced
// by scanning for their literal text, so one token being a prefix of another
// would let the longer one be corrupted by the shorter.
func TestModuleTokensAreUnambiguous(t *testing.T) {
	for i, a := range moduleDefs {
		if !strings.HasPrefix(a.token, "$") || len(a.token) < 2 {
			t.Errorf("token %q is not a $name", a.token)
		}
		for j, b := range moduleDefs {
			if i == j {
				continue
			}
			if a.token == b.token {
				t.Errorf("duplicate token %q", a.token)
			}
			if strings.HasPrefix(b.token, a.token) {
				t.Errorf("token %q is a prefix of %q", a.token, b.token)
			}
		}
	}
}

// TestEveryModuleHasConfig checks each module resolves to a configuration, so
// a new entry cannot be half-wired.
func TestEveryModuleHasConfig(t *testing.T) {
	cfg := config.Default()
	for _, def := range moduleDefs {
		if def.conf == nil || def.value == nil {
			t.Fatalf("%s: conf or value is nil", def.token)
		}
		if _, ok := tokenIndex[def.token]; !ok {
			t.Errorf("%s is missing from tokenIndex", def.token)
		}
		def.conf(&cfg) // must not panic
	}
}

func TestReferencedModules(t *testing.T) {
	used := referencedModules([]string{"$model | $cost", "$dir $unknown_token"})

	want := map[string]bool{"$model": true, "$cost": true, "$dir": true}
	for i, def := range moduleDefs {
		if used[i] != want[def.token] {
			t.Errorf("%s: referenced = %v, want %v", def.token, used[i], want[def.token])
		}
	}
}
