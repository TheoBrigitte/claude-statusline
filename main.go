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
	"path/filepath"
	"strconv"
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

	modules := renderModules(&cfg, &in, termWidth)

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

// moduleValue is what a module contributes before formatting and styling.
type moduleValue struct {
	// text replaces {value}. An empty text hides the module.
	text string
	// threshold is the number the module's warn/critical levels compare against.
	threshold float64
	// reset replaces {reset}, for modules displaying a countdown.
	reset string
}

// renderCtx is the state shared by every module while rendering one update.
type renderCtx struct {
	cfg          *config.Config
	in           *model.Input
	termWidth    int
	currentUsage int
	contextPct   int
}

// moduleDef describes one $token of the status line: where its configuration
// lives, and how to derive its value from an update. Adding a module means
// adding an entry here and a field on config.Config — nothing else.
type moduleDef struct {
	token string
	conf  func(*config.Config) config.ThresholdConfig
	value func(*renderCtx) moduleValue
	// countdown marks the modules whose format accepts {reset}.
	countdown bool
}

// plain adapts a module configuration that has no warn/critical levels.
func plain(c config.ModuleConfig) config.ThresholdConfig {
	return config.ThresholdConfig{ModuleConfig: c}
}

var moduleDefs = []moduleDef{
	{
		token: "$model",
		conf:  func(c *config.Config) config.ThresholdConfig { return plain(c.Model) },
		value: func(r *renderCtx) moduleValue { return moduleValue{text: r.in.Model.DisplayName} },
	}, {
		token: "$context_bar",
		conf:  func(c *config.Config) config.ThresholdConfig { return c.ContextBar.ThresholdConfig },
		value: func(r *renderCtx) moduleValue {
			return moduleValue{text: contextBar(r), threshold: float64(r.contextPct)}
		},
	}, {
		token: "$context_tokens",
		conf:  func(c *config.Config) config.ThresholdConfig { return plain(c.ContextTokens) },
		value: func(r *renderCtx) moduleValue {
			return moduleValue{text: format.SI(r.currentUsage) + "/" + format.SI(r.in.ContextWindow.ContextWindowSize) + " tokens"}
		},
	}, {
		token: "$context_pct",
		conf:  func(c *config.Config) config.ThresholdConfig { return plain(c.ContextPct) },
		value: func(r *renderCtx) moduleValue {
			return moduleValue{text: strconv.Itoa(r.contextPct), threshold: float64(r.contextPct)}
		},
	}, {
		token: "$cost",
		conf:  func(c *config.Config) config.ThresholdConfig { return c.Cost },
		value: func(r *renderCtx) moduleValue {
			return moduleValue{text: format.Cost(r.in.Cost.TotalCostUSD), threshold: r.in.Cost.TotalCostUSD}
		},
	}, {
		token: "$duration",
		conf:  func(c *config.Config) config.ThresholdConfig { return plain(c.Duration) },
		value: func(r *renderCtx) moduleValue {
			return moduleValue{text: format.Duration(r.in.Cost.TotalDurationMS)}
		},
	}, {
		token: "$status",
		conf:  func(c *config.Config) config.ThresholdConfig { return plain(c.Status) },
		value: func(_ *renderCtx) moduleValue { return moduleValue{text: status.Get()} },
	}, {
		token:     "$rate_5h",
		conf:      func(c *config.Config) config.ThresholdConfig { return c.RateLimit5h },
		value:     func(r *renderCtx) moduleValue { return rateLimit(r.in.RateLimits.FiveHour) },
		countdown: true,
	}, {
		token:     "$rate_7d",
		conf:      func(c *config.Config) config.ThresholdConfig { return c.RateLimit7d },
		value:     func(r *renderCtx) moduleValue { return rateLimit(r.in.RateLimits.SevenDay) },
		countdown: true,
	},

	// Modules over data Claude Code already sends and the status line used to
	// discard. None of them are on the default line: add their token to
	// [lines] to display them.
	{
		token: "$diff",
		conf:  func(c *config.Config) config.ThresholdConfig { return plain(c.Diff) },
		value: func(r *renderCtx) moduleValue {
			added, removed := r.in.Cost.TotalLinesAdded, r.in.Cost.TotalLinesRemoved
			if added == 0 && removed == 0 {
				return moduleValue{}
			}
			return moduleValue{text: "+" + strconv.Itoa(added) + "/-" + strconv.Itoa(removed)}
		},
	}, {
		token: "$dir",
		conf:  func(c *config.Config) config.ThresholdConfig { return plain(c.Dir) },
		value: func(r *renderCtx) moduleValue { return moduleValue{text: workingDir(r.in)} },
	}, {
		token: "$api_duration",
		conf:  func(c *config.Config) config.ThresholdConfig { return plain(c.APIDuration) },
		value: func(r *renderCtx) moduleValue {
			return moduleValue{text: format.Duration(r.in.Cost.TotalAPIDurationMS)}
		},
	}, {
		token: "$session_tokens",
		conf:  func(c *config.Config) config.ThresholdConfig { return plain(c.SessionTokens) },
		value: func(r *renderCtx) moduleValue {
			input, output := r.in.ContextWindow.TotalInputTokens, r.in.ContextWindow.TotalOutputTokens
			if input == 0 && output == 0 {
				return moduleValue{}
			}
			return moduleValue{text: "\u2191" + format.SI(input) + " \u2193" + format.SI(output)}
		},
	}, {
		token: "$version",
		conf:  func(c *config.Config) config.ThresholdConfig { return plain(c.Version) },
		value: func(r *renderCtx) moduleValue { return moduleValue{text: r.in.Version} },
	}, {
		token: "$output_style",
		conf:  func(c *config.Config) config.ThresholdConfig { return plain(c.OutputStyle) },
		value: func(r *renderCtx) moduleValue { return moduleValue{text: r.in.OutputStyle.Name} },
	},
}

// renderModules renders every module the configuration refers to, into a map
// keyed by $token name.
func renderModules(cfg *config.Config, in *model.Input, termWidth int) map[string]string {
	r := renderCtx{
		cfg:          cfg,
		in:           in,
		termWidth:    termWidth,
		currentUsage: model.ParseCurrentUsage(in.ContextWindow.CurrentUsage),
	}
	if in.ContextWindow.UsedPercentage != nil {
		r.contextPct = int(*in.ContextWindow.UsedPercentage)
	}

	used := referencedModules(cfg.Lines)

	m := make(map[string]string, len(moduleDefs))
	for i := range moduleDefs {
		if !used[i] {
			continue
		}
		def := &moduleDefs[i]
		// A referenced token always gets an entry, so a module hidden by its
		// configuration disappears instead of leaving its name on screen.
		m[def.token] = ""

		mcfg := def.conf(cfg)
		if !shouldRenderModule(mcfg.ModuleConfig, termWidth) {
			continue
		}
		v := def.value(&r)
		if v.text == "" {
			continue
		}

		raw := applyFormat(mcfg.Format, v.text, mcfg.Symbol)
		if def.countdown {
			raw = applyRateLimitFormat(mcfg.Format, v.text, mcfg.Symbol, v.reset)
		}
		m[def.token] = resolveThresholdStyle(mcfg, v.threshold).Sprint(raw)
	}
	return m
}

// tokenIndex locates a module by its $token. Built once, at startup.
var tokenIndex = func() map[string]int {
	idx := make(map[string]int, len(moduleDefs))
	for i := range moduleDefs {
		idx[moduleDefs[i].token] = i
	}
	return idx
}()

// referencedModules reports, per module, whether the line templates mention it.
//
// Skipping the modules a configuration never uses keeps their cost off every
// render — for $status that cost is an HTTP request. The templates are scanned
// once here rather than once per module, so the module list can grow without
// the scan growing with it.
func referencedModules(lines []string) []bool {
	used := make([]bool, len(moduleDefs))
	for _, line := range lines {
		for i := 0; i < len(line); i++ {
			if line[i] != '$' {
				continue
			}
			end := i + 1
			for end < len(line) && isTokenByte(line[end]) {
				end++
			}
			// Slicing for the lookup does not copy the token.
			if m, ok := tokenIndex[line[i:end]]; ok {
				used[m] = true
			}
			i = end - 1
		}
	}
	return used
}

// isTokenByte reports whether b can appear in a $token name.
func isTokenByte(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9')
}

// contextBar draws the usage bar, without its symbol — applyFormat prepends
// that. It returns "" when the bar is configured down to nothing.
func contextBar(r *renderCtx) string {
	barWidth := r.cfg.ContextBar.Width
	if barWidth == 0 {
		barWidth = max(r.termWidth/4, 10)
	}
	if barWidth <= 0 {
		return ""
	}

	fill, empty := r.cfg.ContextBar.FillChar, r.cfg.ContextBar.EmptyChar
	if fill == "" {
		fill = "#"
	}
	if empty == "" {
		empty = "-"
	}

	// The percentage comes from the session JSON, so clamp it: an out-of-range
	// value would ask strings.Repeat for a negative count and take the whole
	// status line down with a panic.
	filled := min(max(r.contextPct*barWidth/100, 0), barWidth)
	return strings.Repeat(fill, filled) + strings.Repeat(empty, barWidth-filled)
}

// rateLimit renders one usage bucket with its countdown to reset.
func rateLimit(rl model.RateLimit) moduleValue {
	return moduleValue{
		text:      strconv.FormatFloat(rl.UsedPercentage, 'f', 2, 64),
		threshold: rl.UsedPercentage,
		reset:     format.TimeUntil(rl.ResetsAt),
	}
}

// workingDir returns the base name of the directory the session runs in.
func workingDir(in *model.Input) string {
	dir := in.Workspace.CurrentDir
	if dir == "" {
		dir = in.CWD
	}
	if dir == "" {
		return ""
	}
	return filepath.Base(dir)
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
