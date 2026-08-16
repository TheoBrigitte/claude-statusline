// Format and display a status line for the latest Claude API call,
// showing model, context usage, cost, duration, and API status.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/keyamasabaya/claude-statusline/pkg/config"
	"github.com/keyamasabaya/claude-statusline/pkg/format"
	"github.com/keyamasabaya/claude-statusline/pkg/layout"
	"github.com/keyamasabaya/claude-statusline/pkg/model"
	"github.com/keyamasabaya/claude-statusline/pkg/status"
	"github.com/keyamasabaya/claude-statusline/pkg/style"
	"github.com/keyamasabaya/claude-statusline/pkg/terminal"
	"github.com/keyamasabaya/claude-statusline/pkg/width"
)

var (
	configPath string
	logFile    string
	debug      bool
)

func main() {
	flag.BoolVar(&debug, "debug", false, "Enable debug mode with verbose logging to stderr")
	flag.StringVar(&configPath, "config", "", "Path to config file (optional)")
	flag.StringVar(&logFile, "log-file", "", "Path to a .jsonl file to append raw status updates to")
	flag.Parse()

	if err := run(configPath, logFile); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// styleCache avoids re-parsing the same style string across modules.
var styleCache = make(map[string]*style.Style)

func cachedParse(s string) *style.Style {
	if s == "" {
		return nil
	}
	if st, ok := styleCache[s]; ok {
		return st
	}
	st := style.Parse(s)
	styleCache[s] = st
	return st
}

func run(configPath, logFile string) error {
	termWidth, err := terminal.Width()
	if err != nil {
		termWidth = terminal.DefaultWidth
		if debug {
			fmt.Fprintf(os.Stderr, "warning: failed to get terminal width, defaulting to %d: %v\n", termWidth, err) //nolint:errcheck
		}
	}
	return runWith(configPath, logFile, os.Stdin, os.Stdout, termWidth)
}

// appendLog appends a raw JSON blob as a single line to the given file.
func appendLog(path string, raw []byte) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gosec // G302: log file, world-readable is fine
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck

	// Compact to ensure single-line output.
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		return err
	}
	compact.WriteByte('\n')
	_, err = f.Write(compact.Bytes())
	return err
}

// runWith is the testable core: loads config, decodes JSON, renders output.
func runWith(configPath, logFile string, r io.Reader, w io.Writer, termWidth int) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Buffer stdin so we can both log and decode it.
	raw, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("reading stdin: %w", err)
	}

	if logFile != "" {
		if err := appendLog(logFile, raw); err != nil {
			fmt.Fprintf(os.Stderr, "warning: log-file write failed: %v\n", err)
		}
	}

	var in model.Input
	if err := json.Unmarshal(raw, &in); err != nil {
		return fmt.Errorf("parsing JSON from stdin: %w", err)
	}

	modules := renderModules(cfg, in, termWidth)

	for _, lineTemplate := range cfg.Lines {
		segments := strings.Split(lineTemplate, cfg.Separator)
		var parts []*layout.Part
		for _, seg := range segments {
			rendered := renderSegment(strings.TrimSpace(seg), modules)
			if rendered == "" {
				continue
			}
			parts = append(parts, &layout.Part{
				Text: rendered,
				Len:  width.Cells(rendered),
			})
		}
		for _, line := range layout.Lines(termWidth-cfg.Padding, cfg.Separator, parts) {
			fmt.Fprintln(w, line) //nolint:errcheck // best-effort stdout write
		}
	}
	return nil
}

// renderModules renders every module into a map keyed by $token name.
func renderModules(cfg config.Config, in model.Input, termWidth int) map[string]string {
	currentUsage := model.ParseCurrentUsage(in.ContextWindow.CurrentUsage)
	contextPct := 0
	if in.ContextWindow.UsedPercentage != nil {
		contextPct = int(*in.ContextWindow.UsedPercentage)
	}

	m := map[string]string{
		"$model":          "",
		"$context_bar":    "",
		"$context_tokens": "",
		"$context_pct":    "",
		"$cost":           "",
		"$duration":       "",
		"$status":         "",
		"$rate_5h":        "",
		"$rate_7d":        "",
	}

	// Model
	if shouldRenderModule(cfg.Model, termWidth) {
		if in.Model.DisplayName != "" {
			raw := applyFormat(cfg.Model.Format, in.Model.DisplayName, cfg.Model.Symbol)
			s := cachedParse(cfg.Model.Style)
			m["$model"] = s.Sprint(raw)
		}
	}

	// Context bar
	if shouldRenderModule(cfg.ContextBar.ModuleConfig, termWidth) {
		barWidth := cfg.ContextBar.Width
		if barWidth == 0 {
			barWidth = max(termWidth/4, 10)
		}
		if barWidth > 0 {
			// The percentage comes from the session JSON, so clamp it: an
			// out-of-range value would ask strings.Repeat for a negative
			// count and take the whole status line down with a panic.
			filled := min(max(contextPct*barWidth/100, 0), barWidth)
			empty := barWidth - filled
			fc, ec := cfg.ContextBar.FillChar, cfg.ContextBar.EmptyChar
			if fc == "" {
				fc = "#"
			}
			if ec == "" {
				ec = "-"
			}
			raw := cfg.ContextBar.Symbol + strings.Repeat(fc, filled) + strings.Repeat(ec, empty)
			s := resolveThresholdStyle(cfg.ContextBar.ThresholdConfig, float64(contextPct))
			m["$context_bar"] = s.Sprint(raw)
		}
	}

	// Context tokens
	if shouldRenderModule(cfg.ContextTokens, termWidth) {
		value := format.SI(currentUsage) + "/" + format.SI(in.ContextWindow.ContextWindowSize) + " tokens"
		raw := applyFormat(cfg.ContextTokens.Format, value, cfg.ContextTokens.Symbol)
		s := cachedParse(cfg.ContextTokens.Style)
		m["$context_tokens"] = s.Sprint(raw)
	}

	// Context percentage
	if shouldRenderModule(cfg.ContextPct, termWidth) {
		value := fmt.Sprintf("%d", contextPct)
		raw := applyFormat(cfg.ContextPct.Format, value, cfg.ContextPct.Symbol)
		s := cachedParse(cfg.ContextPct.Style)
		m["$context_pct"] = s.Sprint(raw)
	}

	// Cost
	if shouldRenderModule(cfg.Cost.ModuleConfig, termWidth) {
		value := format.Cost(in.Cost.TotalCostUSD)
		raw := applyFormat(cfg.Cost.Format, value, cfg.Cost.Symbol)
		s := resolveThresholdStyle(cfg.Cost, in.Cost.TotalCostUSD)
		m["$cost"] = s.Sprint(raw)
	}

	// Duration
	if shouldRenderModule(cfg.Duration, termWidth) {
		value := format.Duration(in.Cost.TotalDurationMS)
		raw := applyFormat(cfg.Duration.Format, value, cfg.Duration.Symbol)
		s := cachedParse(cfg.Duration.Style)
		m["$duration"] = s.Sprint(raw)
	}

	// Status
	if shouldRenderModule(cfg.Status, termWidth) {
		value := status.Get()
		raw := applyFormat(cfg.Status.Format, value, cfg.Status.Symbol)
		s := cachedParse(cfg.Status.Style)
		m["$status"] = s.Sprint(raw)
	}

	// Rate limit 5h
	if shouldRenderModule(cfg.RateLimit5h.ModuleConfig, termWidth) {
		pct := in.RateLimits.FiveHour.UsedPercentage
		value := fmt.Sprintf("%.2f", pct)
		reset := format.TimeUntil(in.RateLimits.FiveHour.ResetsAt)
		raw := applyRateLimitFormat(cfg.RateLimit5h.Format, value, cfg.RateLimit5h.Symbol, reset)
		s := resolveThresholdStyle(cfg.RateLimit5h, float64(pct))
		m["$rate_5h"] = s.Sprint(raw)
	}

	// Rate limit 7d
	if shouldRenderModule(cfg.RateLimit7d.ModuleConfig, termWidth) {
		pct := in.RateLimits.SevenDay.UsedPercentage
		value := fmt.Sprintf("%.2f", pct)
		reset := format.TimeUntil(in.RateLimits.SevenDay.ResetsAt)
		raw := applyRateLimitFormat(cfg.RateLimit7d.Format, value, cfg.RateLimit7d.Symbol, reset)
		s := resolveThresholdStyle(cfg.RateLimit7d, float64(pct))
		m["$rate_7d"] = s.Sprint(raw)
	}

	return m
}

func shouldRenderModule(cfg config.ModuleConfig, termWidth int) bool {
	return !cfg.Disabled && (cfg.MinTermWidth == 0 || termWidth >= cfg.MinTermWidth) && (cfg.MaxTermWidth == 0 || termWidth <= cfg.MaxTermWidth)
}

// applyFormat applies a format string. Supports {value} and {symbol}
// placeholders, each substituted at every occurrence.
// If format is empty, returns symbol + value.
func applyFormat(format, value, symbol string) string {
	if format == "" {
		return symbol + value
	}
	// {value} last: it carries the most dynamic text, so substituting it at
	// the end means its content is never rescanned for another placeholder.
	s := strings.ReplaceAll(format, "{symbol}", symbol)
	return strings.ReplaceAll(s, "{value}", value)
}

const resetToken = "{reset}"

// applyRateLimitFormat extends applyFormat with a {reset} placeholder for countdown display.
// If reset is empty (timestamp is zero or in the past), any text between the last
// non-space before {reset} and {reset} itself is removed to avoid dangling prefixes like "~".
func applyRateLimitFormat(f, value, symbol, reset string) string {
	s := applyFormat(f, value, symbol)
	if reset != "" {
		return strings.TrimSpace(strings.ReplaceAll(s, resetToken, reset))
	}

	// Remove every {reset} and the non-space characters preceding it (e.g.
	// "~"), which only make sense when a reset value is present.
	for {
		idx := strings.Index(s, resetToken)
		if idx < 0 {
			break
		}
		start := idx
		for start > 0 && s[start-1] != ' ' {
			start--
		}
		s = s[:start] + s[idx+len(resetToken):]
	}
	return strings.TrimSpace(s)
}

// resolveThresholdStyle picks the appropriate style based on threshold config.
func resolveThresholdStyle(cfg config.ThresholdConfig, value float64) *style.Style {
	if cfg.CriticalThreshold > 0 && value >= cfg.CriticalThreshold {
		if s := cachedParse(cfg.CriticalStyle); s != nil {
			return s
		}
	}
	if cfg.WarnThreshold > 0 && value >= cfg.WarnThreshold {
		if s := cachedParse(cfg.WarnStyle); s != nil {
			return s
		}
	}
	return cachedParse(cfg.Style)
}

// renderSegment replaces all $module tokens in a segment template with their
// rendered values. Returns empty string if all tokens resolved to empty.
func renderSegment(seg string, modules map[string]string) string {
	result := seg
	hasContent, hasGap := false, false
	for token, rendered := range modules {
		if strings.Contains(result, token) {
			if rendered == "" {
				hasGap = true
			} else {
				hasContent = true
			}
			result = strings.ReplaceAll(result, token, rendered)
		}
	}
	if !hasContent {
		return ""
	}
	if hasGap {
		result = collapseSpaces(result)
	}
	return strings.TrimSpace(result)
}

// collapseSpaces squeezes runs of spaces down to one. A module hidden by
// min_term_width renders as an empty string and would otherwise leave the
// spaces that separated it from its neighbours as a visible gap.
func collapseSpaces(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := false
	for i := 0; i < len(s); i++ {
		// Byte-wise is safe: 0x20 never occurs inside a UTF-8 sequence.
		if s[i] == ' ' && prevSpace {
			continue
		}
		prevSpace = s[i] == ' '
		b.WriteByte(s[i])
	}
	return b.String()
}
