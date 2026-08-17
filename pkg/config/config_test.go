package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultPreservesOriginalBehaviour(t *testing.T) {
	cfg := Default()

	if cfg.Separator != " | " {
		t.Errorf("separator = %q, want %q", cfg.Separator, " | ")
	}
	if cfg.Padding != 5 {
		t.Errorf("padding = %d, want 5", cfg.Padding)
	}
	if len(cfg.Lines) != 1 {
		t.Fatalf("lines len = %d, want 1", len(cfg.Lines))
	}

	// Model defaults
	if cfg.Model.Style != "cyan" {
		t.Errorf("model.style = %q, want cyan", cfg.Model.Style)
	}
	if cfg.Model.MinTermWidth != 80 {
		t.Errorf("model.min_term_width = %d, want 80", cfg.Model.MinTermWidth)
	}
	if cfg.Model.Format != "[{value}]" {
		t.Errorf("model.format = %q, want [value]", cfg.Model.Format)
	}

	// Context bar defaults
	if cfg.ContextBar.Style != "green" {
		t.Errorf("context_bar.style = %q, want green", cfg.ContextBar.Style)
	}
	if cfg.ContextBar.WarnThreshold != 40 {
		t.Errorf("context_bar.warn_threshold = %f, want 40", cfg.ContextBar.WarnThreshold)
	}
	if cfg.ContextBar.CriticalThreshold != 90 {
		t.Errorf("context_bar.critical_threshold = %f, want 90", cfg.ContextBar.CriticalThreshold)
	}
	if cfg.ContextBar.FillChar != "#" {
		t.Errorf("context_bar.fill_char = %q, want #", cfg.ContextBar.FillChar)
	}
	if cfg.ContextBar.EmptyChar != "-" {
		t.Errorf("context_bar.empty_char = %q, want -", cfg.ContextBar.EmptyChar)
	}

	// Cost defaults
	if cfg.Cost.Style != "yellow" {
		t.Errorf("cost.style = %q, want yellow", cfg.Cost.Style)
	}

	// Duration defaults
	if cfg.Duration.Symbol != "⏱️ " {
		t.Errorf("duration.symbol = %q, want ⏱️ ", cfg.Duration.Symbol)
	}

	// Context tokens/pct format
	if cfg.ContextTokens.Format != "({value})" {
		t.Errorf("context_tokens.format = %q, want ({value})", cfg.ContextTokens.Format)
	}
	if cfg.ContextPct.Format != "{value}%" {
		t.Errorf("context_pct.format = %q, want {value}%%", cfg.ContextPct.Format)
	}
}

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	cfg, err := Load("/nonexistent/path/to/config.toml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Separator != " | " {
		t.Errorf("expected defaults, got separator %q", cfg.Separator)
	}
}

func TestLoadCustomConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.toml")

	content := `
separator = "  "
lines = ["$model  $cost"]
padding = 3

[model]
style = "bold fg:#7aa2f7"
symbol = "󰚩 "
format = "{symbol}{value}"

[cost]
style = "green"
warn_threshold = 2.0
warn_style = "bold yellow"
critical_threshold = 5.0
critical_style = "bold red"

[context_bar]
width = 20
fill_char = "█"
empty_char = "░"
style = "fg:#7dcfff"
warn_threshold = 50.0
warn_style = "fg:#e0af68"
critical_threshold = 80.0
critical_style = "bold fg:#f7768e"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil { //nolint:gosec // G302: config file, world-readable is fine
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Separator != "  " {
		t.Errorf("separator = %q, want %q", cfg.Separator, "  ")
	}
	if cfg.Padding != 3 {
		t.Errorf("padding = %d, want 3", cfg.Padding)
	}
	if len(cfg.Lines) != 1 || cfg.Lines[0] != "$model  $cost" {
		t.Errorf("lines = %v", cfg.Lines)
	}
	if cfg.Model.Symbol != "󰚩 " {
		t.Errorf("model.symbol = %q", cfg.Model.Symbol)
	}
	if cfg.Model.Style != "bold fg:#7aa2f7" {
		t.Errorf("model.style = %q", cfg.Model.Style)
	}
	if cfg.Cost.WarnThreshold != 2.0 {
		t.Errorf("cost.warn_threshold = %f", cfg.Cost.WarnThreshold)
	}
	if cfg.ContextBar.Width != 20 {
		t.Errorf("context_bar.width = %d", cfg.ContextBar.Width)
	}
	if cfg.ContextBar.FillChar != "█" {
		t.Errorf("context_bar.fill_char = %q", cfg.ContextBar.FillChar)
	}
}

func TestLoadInvalidTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.toml")
	if err := os.WriteFile(path, []byte("not valid [toml {{{"), 0o644); err != nil { //nolint:gosec // G302: config file, world-readable is fine
		t.Fatalf("failed to write invalid TOML: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Error("expected error for invalid TOML")
	}
}

// TestLoadOptionalModules covers the module tables that are configurable but
// absent from the default line.
func TestLoadOptionalModules(t *testing.T) {
	path := filepath.Join(t.TempDir(), "optional.toml")
	contents := `
lines = ["$dir | $diff | $session_tokens"]

[diff]
style = "green"
format = "({value})"

[dir]
style = "bold blue"
min_term_width = 60

[api_duration]
disabled = true

[session_tokens]
symbol = "tok "

[version]
format = "{value}"

[output_style]
style = "italic"
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Diff.Style != "green" || cfg.Diff.Format != "({value})" {
		t.Errorf("diff = %+v", cfg.Diff)
	}
	if cfg.Dir.Style != "bold blue" || cfg.Dir.MinTermWidth != 60 {
		t.Errorf("dir = %+v", cfg.Dir)
	}
	if !cfg.APIDuration.Disabled {
		t.Error("api_duration should be disabled")
	}
	if cfg.SessionTokens.Symbol != "tok " {
		t.Errorf("session_tokens.symbol = %q", cfg.SessionTokens.Symbol)
	}
	// An override replaces the default rather than merging with it.
	if cfg.Version.Format != "{value}" {
		t.Errorf("version.format = %q, want the override", cfg.Version.Format)
	}
	if cfg.OutputStyle.Style != "italic" {
		t.Errorf("output_style.style = %q", cfg.OutputStyle.Style)
	}
}

// TestDefaultLineOnlyUsesCoreModules pins that the optional modules stay off
// the default output until a user asks for them.
func TestDefaultLineOnlyUsesCoreModules(t *testing.T) {
	cfg := Default()
	for _, token := range []string{"$diff", "$dir", "$api_duration", "$session_tokens", "$version", "$output_style"} {
		for _, line := range cfg.Lines {
			if strings.Contains(line, token) {
				t.Errorf("%s is on the default line %q", token, line)
			}
		}
	}
}
