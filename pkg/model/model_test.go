package model

import (
	"encoding/json"
	"testing"
)

func TestParseCurrentUsage(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want int
	}{
		{"empty raw message", "", 0},
		{"null value", "null", 0},
		{"single number", "42", 42},
		{"single number float truncates", "42.9", 42},
		{"object sums all fields", `{"input_tokens":8500,"output_tokens":1200,"cache_creation_input_tokens":5000,"cache_read_input_tokens":2000}`, 16700},
		{"object with zeros", `{"input_tokens":0,"output_tokens":0}`, 0},
		{"object single field", `{"input_tokens":4096}`, 4096},
		{"invalid JSON falls back to 0", `not-json`, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var raw json.RawMessage
			if tt.raw != "" {
				raw = json.RawMessage(tt.raw)
			}
			if got := ParseCurrentUsage(raw); got != tt.want {
				t.Errorf("ParseCurrentUsage(%q) = %d, want %d", tt.raw, got, tt.want)
			}
		})
	}
}

func mustParseInput(t *testing.T, jsonStr string) Input {
	t.Helper()
	var in Input
	if err := json.Unmarshal([]byte(jsonStr), &in); err != nil {
		t.Fatalf("failed to parse test input: %v", err)
	}
	return in
}

func TestInputModel(t *testing.T) {
	in := mustParseInput(t, `{
		"model": {
			"id": "claude-opus-4-6[1m]",
			"display_name": "Opus 4.6 (1M context)"
		}
	}`)
	if in.Model.ID != "claude-opus-4-6[1m]" {
		t.Errorf("model.id = %q", in.Model.ID)
	}
	if in.Model.DisplayName != "Opus 4.6 (1M context)" {
		t.Errorf("model.display_name = %q", in.Model.DisplayName)
	}
}

func TestInputCost(t *testing.T) {
	in := mustParseInput(t, `{
		"cost": {
			"total_cost_usd": 0.24710,
			"total_duration_ms": 245000,
			"total_api_duration_ms": 128000,
			"total_lines_added": 478,
			"total_lines_removed": 112
		}
	}`)
	if in.Cost.TotalCostUSD != 0.24710 {
		t.Errorf("total_cost_usd = %f", in.Cost.TotalCostUSD)
	}
	if in.Cost.TotalDurationMS != 245000 {
		t.Errorf("total_duration_ms = %d", in.Cost.TotalDurationMS)
	}
	if in.Cost.TotalAPIDurationMS != 128000 {
		t.Errorf("total_api_duration_ms = %d", in.Cost.TotalAPIDurationMS)
	}
	if in.Cost.TotalLinesAdded != 478 {
		t.Errorf("total_lines_added = %d", in.Cost.TotalLinesAdded)
	}
	if in.Cost.TotalLinesRemoved != 112 {
		t.Errorf("total_lines_removed = %d", in.Cost.TotalLinesRemoved)
	}
}

func TestInputContextWindow(t *testing.T) {
	t.Run("populated percentages", func(t *testing.T) {
		in := mustParseInput(t, `{
			"context_window": {
				"total_input_tokens": 42000,
				"total_output_tokens": 12800,
				"context_window_size": 1000000,
				"current_usage": {
					"input_tokens": 21000,
					"output_tokens": 6400
				},
				"used_percentage": 7,
				"remaining_percentage": 93
			}
		}`)
		if in.ContextWindow.ContextWindowSize != 1000000 {
			t.Errorf("context_window_size = %d", in.ContextWindow.ContextWindowSize)
		}
		if in.ContextWindow.UsedPercentage == nil || *in.ContextWindow.UsedPercentage != 7 {
			t.Error("used_percentage should be 7")
		}
		if in.ContextWindow.RemainingPercentage == nil || *in.ContextWindow.RemainingPercentage != 93 {
			t.Error("remaining_percentage should be 93")
		}
		if got := ParseCurrentUsage(in.ContextWindow.CurrentUsage); got != 27400 {
			t.Errorf("current_usage sum = %d, want 27400", got)
		}
	})

	t.Run("null percentages before first API call", func(t *testing.T) {
		in := mustParseInput(t, `{
			"context_window": {
				"context_window_size": 200000,
				"current_usage": null,
				"used_percentage": null,
				"remaining_percentage": null
			}
		}`)
		if in.ContextWindow.UsedPercentage != nil {
			t.Errorf("used_percentage should be nil, got %v", *in.ContextWindow.UsedPercentage)
		}
		if in.ContextWindow.RemainingPercentage != nil {
			t.Errorf("remaining_percentage should be nil, got %v", *in.ContextWindow.RemainingPercentage)
		}
		contextPct := 0
		if in.ContextWindow.UsedPercentage != nil {
			contextPct = int(*in.ContextWindow.UsedPercentage)
		}
		if contextPct != 0 {
			t.Errorf("contextPct = %d, want 0", contextPct)
		}
	})
}

// Benchmarks

func BenchmarkParseCurrentUsageObject(b *testing.B) {
	raw := json.RawMessage(`{"input_tokens":21000,"output_tokens":6400,"cache_creation_input_tokens":5000}`)
	for b.Loop() {
		ParseCurrentUsage(raw)
	}
}

func BenchmarkParseCurrentUsageNumber(b *testing.B) {
	raw := json.RawMessage(`27400`)
	for b.Loop() {
		ParseCurrentUsage(raw)
	}
}
