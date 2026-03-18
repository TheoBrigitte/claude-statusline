// Package model defines the input types for Claude Code status line data.
package model

import "encoding/json"

// Input represents the JSON payload received from Claude Code via stdin.
type Input struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	CWD            string `json:"cwd"`
	Model          struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	} `json:"model"`
	Workspace struct {
		CurrentDir string   `json:"current_dir"`
		ProjectDir string   `json:"project_dir"`
		AddedDirs  []string `json:"added_dirs"`
	} `json:"workspace"`
	Version     string `json:"version"`
	OutputStyle struct {
		Name string `json:"name"`
	} `json:"output_style"`
	Cost struct {
		TotalCostUSD       float64 `json:"total_cost_usd"`
		TotalDurationMS    int     `json:"total_duration_ms"`
		TotalAPIDurationMS int     `json:"total_api_duration_ms"`
		TotalLinesAdded    int     `json:"total_lines_added"`
		TotalLinesRemoved  int     `json:"total_lines_removed"`
	} `json:"cost"`
	ContextWindow struct {
		TotalInputTokens    int             `json:"total_input_tokens"`
		TotalOutputTokens   int             `json:"total_output_tokens"`
		ContextWindowSize   int             `json:"context_window_size"`
		CurrentUsage        json.RawMessage `json:"current_usage"`
		UsedPercentage      *float64        `json:"used_percentage"`
		RemainingPercentage *float64        `json:"remaining_percentage"`
	} `json:"context_window"`
	Exceeds200kTokens bool `json:"exceeds_200k_tokens"`
}

// ParseCurrentUsage sums the values in current_usage, which can be either
// a single number or an object with numeric values (e.g. {"input": 100, "output": 50}).
func ParseCurrentUsage(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}

	var obj map[string]float64
	if json.Unmarshal(raw, &obj) == nil {
		total := 0.0
		for _, v := range obj {
			total += v
		}
		return int(total)
	}

	var n float64
	if json.Unmarshal(raw, &n) == nil {
		return int(n)
	}

	return 0
}
