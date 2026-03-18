// Package config loads and provides defaults for the claude-statusline TOML configuration.
//
// Config file discovery order:
//  1. --config <path> flag
//  2. ~/.config/claude-statusline.toml
//  3. Built-in defaults (no config required)
package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const defaultConfigName = "claude-statusline.toml"

// Config is the top-level configuration.
type Config struct {
	Separator string   `toml:"separator"`
	Lines     []string `toml:"lines"`
	Padding   int      `toml:"padding"`

	Model         ModuleConfig    `toml:"model"`
	ContextBar    ContextBarCfg   `toml:"context_bar"`
	ContextTokens ModuleConfig    `toml:"context_tokens"`
	ContextPct    ModuleConfig    `toml:"context_pct"`
	Cost          ThresholdConfig `toml:"cost"`
	Duration      ModuleConfig    `toml:"duration"`
	Status        ModuleConfig    `toml:"status"`
}

// ModuleConfig holds fields common to every module.
type ModuleConfig struct {
	Disabled     bool   `toml:"disabled"`
	Style        string `toml:"style"`
	Symbol       string `toml:"symbol"`
	Format       string `toml:"format"`
	MinTermWidth int    `toml:"min_term_width"`
	MaxTermWidth int    `toml:"max_term_width"`
}

// ThresholdConfig extends ModuleConfig with warn/critical colour thresholds.
type ThresholdConfig struct {
	ModuleConfig
	WarnThreshold     float64 `toml:"warn_threshold"`
	WarnStyle         string  `toml:"warn_style"`
	CriticalThreshold float64 `toml:"critical_threshold"`
	CriticalStyle     string  `toml:"critical_style"`
}

// ContextBarCfg adds bar-specific fields to ThresholdConfig.
type ContextBarCfg struct {
	ThresholdConfig
	Width     int    `toml:"width"`      // 0 = auto (termWidth/3, min 40)
	FillChar  string `toml:"fill_char"`  // default "#"
	EmptyChar string `toml:"empty_char"` // default "-"
}

// Default returns the built-in default configuration that reproduces the
// original hard-coded behaviour.
func Default() Config {
	return Config{
		Separator: " | ",
		Lines:     []string{"$model | $context_bar $context_tokens $context_pct | $cost | $duration | $status"},
		Padding:   5,

		Model: ModuleConfig{
			Style:        "cyan",
			Format:       "[{value}]",
			MinTermWidth: 80,
		},
		ContextBar: ContextBarCfg{
			ThresholdConfig: ThresholdConfig{
				ModuleConfig: ModuleConfig{
					Style:        "green",
					MinTermWidth: 60,
				},
				WarnThreshold:     40,
				WarnStyle:         "yellow",
				CriticalThreshold: 90,
				CriticalStyle:     "red",
			},
			FillChar:  "#",
			EmptyChar: "-",
		},
		ContextTokens: ModuleConfig{
			Format:       "({value})",
			MinTermWidth: 100,
		},
		ContextPct: ModuleConfig{
			Format: "{value}%",
		},
		Cost: ThresholdConfig{
			ModuleConfig: ModuleConfig{
				Style: "yellow",
			},
		},
		Duration: ModuleConfig{
			Symbol: "⏱️ ",
		},
		Status: ModuleConfig{},
	}
}

// Load reads the TOML config file. If path is empty, it searches the default
// location. Returns the default config merged with any file-level overrides.
func Load(path string) (Config, error) {
	cfg := Default()

	if path == "" {
		path = defaultPath()
	}
	if path == "" {
		return cfg, nil
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}

	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// defaultPath returns ~/.config/claude-statusline.toml if it exists, else "".
func defaultPath() string {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	p := filepath.Join(cfgDir, defaultConfigName)
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return ""
}
