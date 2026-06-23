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

	"github.com/TheoBrigitte/claude-statusline/pkg/config"
	"github.com/TheoBrigitte/claude-statusline/pkg/format"
	"github.com/TheoBrigitte/claude-statusline/pkg/layout"
	"github.com/TheoBrigitte/claude-statusline/pkg/model"
	"github.com/TheoBrigitte/claude-statusline/pkg/status"
	"github.com/TheoBrigitte/claude-statusline/pkg/style"
	"github.com/TheoBrigitte/claude-statusline/pkg/terminal"
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
			fmt.Fprintf(os.Stdout, "warning: failed to get terminal width, defaulting to %d: %v\n", termWidth, err)
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
				Len:  displayLen(rendered, modules, strings.TrimSpace(seg)),
			})
		}
		for _, line := range layout.Lines(termWidth-cfg.Padding, cfg.Separator, parts) {
			fmt.Fprintln(w, line) //nolint:errcheck // best-effort stdout write
		}
	}
	return nil
}

// moduleResult holds both the rendered (styled) and raw (unstyled) text for a module.
type moduleResult struct {
	rendered string
	rawLen   int
}

// renderModules renders every module into a map keyed by $token name.
func renderModules(cfg config.Config, in model.Input, termWidth int) map[string]moduleResult {
	currentUsage := model.ParseCurrentUsage(in.ContextWindow.CurrentUsage)
	contextPct := 0
	if in.ContextWindow.UsedPercentage != nil {
		contextPct = int(*in.ContextWindow.UsedPercentage)
	}

	m := map[string]moduleResult{
		"$model":          {"", 0},
		"$context_bar":    {"", 0},
		"$context_tokens": {"", 0},
		"$context_pct":    {"", 0},
		"$cost":           {"", 0},
		"$duration":       {"", 0},
		"$status":         {"", 0},
		"$rate_5h":        {"", 0},
		"$rate_7d":        {"", 0},
	}

	// Model
	if shouldRenderModule(cfg.Model, termWidth) {
		if in.Model.DisplayName != "" {
			raw := applyFormat(cfg.Model.Format, in.Model.DisplayName, cfg.Model.Symbol)
			s := cachedParse(cfg.Model.Style)
			m["$model"] = moduleResult{s.Sprint(raw), len(raw)}
		}
	}

	// Context bar
	if shouldRenderModule(cfg.ContextBar.ModuleConfig, termWidth) {
		barWidth := cfg.ContextBar.Width
		if barWidth == 0 {
			barWidth = max(termWidth/4, 10)
		}
		if barWidth > 0 {
			filled := contextPct * barWidth / 100
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
			m["$context_bar"] = moduleResult{s.Sprint(raw), len(raw)}
		}
	}

	// Context tokens
	if shouldRenderModule(cfg.ContextTokens, termWidth) {
		value := format.SI(currentUsage) + "/" + format.SI(in.ContextWindow.ContextWindowSize) + " tokens"
		raw := applyFormat(cfg.ContextTokens.Format, value, cfg.ContextTokens.Symbol)
		s := cachedParse(cfg.ContextTokens.Style)
		m["$context_tokens"] = moduleResult{s.Sprint(raw), len(raw)}
	}

	// Context percentage
	if shouldRenderModule(cfg.ContextPct, termWidth) {
		value := fmt.Sprintf("%d", contextPct)
		raw := applyFormat(cfg.ContextPct.Format, value, cfg.ContextPct.Symbol)
		s := cachedParse(cfg.ContextPct.Style)
		m["$context_pct"] = moduleResult{s.Sprint(raw), len(raw)}
	}

	// Cost
	if shouldRenderModule(cfg.Cost.ModuleConfig, termWidth) {
		value := format.Cost(in.Cost.TotalCostUSD)
		raw := applyFormat(cfg.Cost.Format, value, cfg.Cost.Symbol)
		s := resolveThresholdStyle(cfg.Cost, in.Cost.TotalCostUSD)
		m["$cost"] = moduleResult{s.Sprint(raw), len(raw)}
	}

	// Duration
	if shouldRenderModule(cfg.Duration, termWidth) {
		value := format.Duration(in.Cost.TotalDurationMS)
		raw := applyFormat(cfg.Duration.Format, value, cfg.Duration.Symbol)
		s := cachedParse(cfg.Duration.Style)
		m["$duration"] = moduleResult{s.Sprint(raw), len(raw)}
	}

	// Status
	if shouldRenderModule(cfg.Status, termWidth) {
		value := status.Get()
		raw := applyFormat(cfg.Status.Format, value, cfg.Status.Symbol)
		s := cachedParse(cfg.Status.Style)
		m["$status"] = moduleResult{s.Sprint(raw), len(raw)}
	}

	// Rate limit 5h
	if shouldRenderModule(cfg.RateLimit5h.ModuleConfig, termWidth) {
		pct := in.RateLimits.FiveHour.UsedPercentage
		value := fmt.Sprintf("%.2f", pct)
		reset := format.TimeUntil(in.RateLimits.FiveHour.ResetsAt)
		raw := applyRateLimitFormat(cfg.RateLimit5h.Format, value, cfg.RateLimit5h.Symbol, reset)
		s := resolveThresholdStyle(cfg.RateLimit5h, float64(pct))
		m["$rate_5h"] = moduleResult{s.Sprint(raw), len(raw)}
	}

	// Rate limit 7d
	if shouldRenderModule(cfg.RateLimit7d.ModuleConfig, termWidth) {
		pct := in.RateLimits.SevenDay.UsedPercentage
		value := fmt.Sprintf("%.2f", pct)
		reset := format.TimeUntil(in.RateLimits.SevenDay.ResetsAt)
		raw := applyRateLimitFormat(cfg.RateLimit7d.Format, value, cfg.RateLimit7d.Symbol, reset)
		s := resolveThresholdStyle(cfg.RateLimit7d, float64(pct))
		m["$rate_7d"] = moduleResult{s.Sprint(raw), len(raw)}
	}

	return m
}

func shouldRenderModule(cfg config.ModuleConfig, termWidth int) bool {
	return !cfg.Disabled && (cfg.MinTermWidth == 0 || termWidth >= cfg.MinTermWidth) && (cfg.MaxTermWidth == 0 || termWidth <= cfg.MaxTermWidth)
}

// applyFormat applies a format string. Supports {value} and {symbol} placeholders.
// If format is empty, returns symbol + value.
func applyFormat(format, value, symbol string) string {
	if format == "" {
		return symbol + value
	}
	s := strings.Replace(format, "{value}", value, 1)
	s = strings.Replace(s, "{symbol}", symbol, 1)
	return s
}

// applyRateLimitFormat extends applyFormat with a {reset} placeholder for countdown display.
// If reset is empty (timestamp is zero or in the past), any text between the last
// non-space before {reset} and {reset} itself is removed to avoid dangling prefixes like "~".
func applyRateLimitFormat(f, value, symbol, reset string) string {
	s := applyFormat(f, value, symbol)
	if reset != "" {
		s = strings.Replace(s, "{reset}", reset, 1)
	} else {
		// Remove {reset} and any preceding non-space characters (e.g. "~")
		// that only make sense when a reset value is present.
		if idx := strings.Index(s, "{reset}"); idx >= 0 {
			// Walk back over non-space chars that prefix {reset}
			start := idx
			for start > 0 && s[start-1] != ' ' {
				start--
			}
			s = s[:start] + s[idx+len("{reset}"):]
		}
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
func renderSegment(seg string, modules map[string]moduleResult) string {
	result := seg
	hasContent := false
	for token, mod := range modules {
		if strings.Contains(result, token) {
			if mod.rendered != "" {
				hasContent = true
			}
			result = strings.ReplaceAll(result, token, mod.rendered)
		}
	}
	if !hasContent {
		return ""
	}
	return strings.TrimSpace(result)
}

// displayLen calculates the logical display width of a rendered segment
// by summing the raw lengths of the modules it contains plus literal text.
func displayLen(_ string, modules map[string]moduleResult, seg string) int {
	total := 0
	remaining := seg
	for remaining != "" {
		earliest := -1
		var earliestToken string
		for token := range modules {
			if idx := strings.Index(remaining, token); idx >= 0 && (earliest < 0 || idx < earliest) {
				earliest = idx
				earliestToken = token
			}
		}
		if earliest < 0 {
			total += len(remaining)
			break
		}
		total += earliest // literal text before token
		total += modules[earliestToken].rawLen
		remaining = remaining[earliest+len(earliestToken):]
	}
	return total
}
