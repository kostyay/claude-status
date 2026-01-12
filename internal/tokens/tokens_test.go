package tokens

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetContextConfig(t *testing.T) {
	tests := []struct {
		name       string
		modelID    string
		wantMax    int64
		wantUsable int64
	}{
		{
			name:       "Sonnet 4 model (200k default)",
			modelID:    "claude-sonnet-4-20250514",
			wantMax:    200_000,
			wantUsable: 160_000,
		},
		{
			name:       "Sonnet 4.5 model (200k default)",
			modelID:    "claude-sonnet-4-5-20250929",
			wantMax:    200_000,
			wantUsable: 160_000,
		},
		{
			name:       "Sonnet 4.5 with [1m] suffix",
			modelID:    "claude-sonnet-4-5-20250929[1m]",
			wantMax:    1_000_000,
			wantUsable: 800_000,
		},
		{
			name:       "Standard model",
			modelID:    "claude-opus-4-5-20251101",
			wantMax:    200_000,
			wantUsable: 160_000,
		},
		{
			name:       "Empty model",
			modelID:    "",
			wantMax:    200_000,
			wantUsable: 160_000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := GetContextConfig(tt.modelID)
			if cfg.MaxTokens != tt.wantMax {
				t.Errorf("MaxTokens = %d, want %d", cfg.MaxTokens, tt.wantMax)
			}
			if cfg.UsableTokens != tt.wantUsable {
				t.Errorf("UsableTokens = %d, want %d", cfg.UsableTokens, tt.wantUsable)
			}
		})
	}
}

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		count int64
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{999, "999"},
		{1000, "1k"},
		{1234, "1.2k"},
		{10000, "10k"},
		{10500, "10.5k"},
		{100000, "100k"},
		{999900, "999.9k"},
		{1000000, "1M"},
		{1500000, "1.5M"},
		{10000000, "10M"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := FormatTokens(tt.count)
			if got != tt.want {
				t.Errorf("FormatTokens(%d) = %q, want %q", tt.count, got, tt.want)
			}
		})
	}
}

func TestMetrics_ContextPercentage(t *testing.T) {
	tests := []struct {
		name          string
		contextLength int64
		cfg           ContextConfig
		want          float64
	}{
		{
			name:          "50% of 200k",
			contextLength: 100_000,
			cfg:           ContextConfig{MaxTokens: 200_000, UsableTokens: 160_000},
			want:          50.0,
		},
		{
			name:          "Over 100%",
			contextLength: 250_000,
			cfg:           ContextConfig{MaxTokens: 200_000, UsableTokens: 160_000},
			want:          100.0, // Capped at 100
		},
		{
			name:          "Zero max tokens",
			contextLength: 100_000,
			cfg:           ContextConfig{MaxTokens: 0, UsableTokens: 0},
			want:          0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Metrics{ContextLength: tt.contextLength}
			got := m.ContextPercentage(tt.cfg)
			if got != tt.want {
				t.Errorf("ContextPercentage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMetrics_ContextPercentageUsable(t *testing.T) {
	tests := []struct {
		name          string
		contextLength int64
		cfg           ContextConfig
		want          float64
	}{
		{
			name:          "50% of usable (160k)",
			contextLength: 80_000,
			cfg:           ContextConfig{MaxTokens: 200_000, UsableTokens: 160_000},
			want:          50.0,
		},
		{
			name:          "100% of usable",
			contextLength: 160_000,
			cfg:           ContextConfig{MaxTokens: 200_000, UsableTokens: 160_000},
			want:          100.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Metrics{ContextLength: tt.contextLength}
			got := m.ContextPercentageUsable(tt.cfg)
			if got != tt.want {
				t.Errorf("ContextPercentageUsable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseTranscript(t *testing.T) {
	// Create a temporary JSONL file for testing
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "test.jsonl")

	jsonlContent := `{"type":"summary","summary":"Test session"}
{"parentUuid":null,"isSidechain":false,"type":"user","message":{"role":"user","content":"Hello"}}
{"parentUuid":"123","isSidechain":false,"type":"assistant","message":{"role":"assistant","usage":{"input_tokens":100,"output_tokens":50,"cache_read_input_tokens":500,"cache_creation_input_tokens":200}}}
{"parentUuid":"456","isSidechain":true,"type":"assistant","message":{"role":"assistant","usage":{"input_tokens":1000,"output_tokens":500}}}
{"parentUuid":"789","isSidechain":false,"type":"assistant","message":{"role":"assistant","usage":{"input_tokens":150,"output_tokens":75,"cache_read_input_tokens":600,"cache_creation_input_tokens":100}}}
`

	if err := os.WriteFile(transcriptPath, []byte(jsonlContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	metrics, err := ParseTranscript(transcriptPath)
	if err != nil {
		t.Fatalf("ParseTranscript() error = %v", err)
	}

	// Should have accumulated from non-sidechain assistant messages
	// First assistant: 100 input, 50 output, 500+200=700 cached
	// Second assistant (sidechain): skipped
	// Third assistant: 150 input, 75 output, 600+100=700 cached
	// Total: 250 input, 125 output, 1400 cached
	wantInput := int64(250)
	wantOutput := int64(125)
	wantCached := int64(1400)
	wantTotal := wantInput + wantOutput + wantCached

	if metrics.InputTokens != wantInput {
		t.Errorf("InputTokens = %d, want %d", metrics.InputTokens, wantInput)
	}
	if metrics.OutputTokens != wantOutput {
		t.Errorf("OutputTokens = %d, want %d", metrics.OutputTokens, wantOutput)
	}
	if metrics.CachedTokens != wantCached {
		t.Errorf("CachedTokens = %d, want %d", metrics.CachedTokens, wantCached)
	}
	if metrics.TotalTokens != wantTotal {
		t.Errorf("TotalTokens = %d, want %d", metrics.TotalTokens, wantTotal)
	}

	// Context length should be from the last message: 150 + 600 + 100 = 850
	wantContextLength := int64(850)
	if metrics.ContextLength != wantContextLength {
		t.Errorf("ContextLength = %d, want %d", metrics.ContextLength, wantContextLength)
	}
}

func TestParseTranscript_EmptyPath(t *testing.T) {
	metrics, err := ParseTranscript("")
	if err != nil {
		t.Errorf("ParseTranscript(\"\") error = %v, want nil", err)
	}
	if metrics.TotalTokens != 0 {
		t.Errorf("TotalTokens = %d, want 0", metrics.TotalTokens)
	}
}

func TestParseTranscript_NonExistentFile(t *testing.T) {
	_, err := ParseTranscript("/nonexistent/path/file.jsonl")
	if err == nil {
		t.Error("ParseTranscript() expected error for non-existent file")
	}
}

func TestParseTranscript_MalformedJSONL(t *testing.T) {
	// Create a temporary JSONL file with some malformed lines
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "malformed.jsonl")

	// Mix of valid and invalid JSON lines
	jsonlContent := `{"type":"summary","summary":"Test session"}
not valid json at all
{"parentUuid":"123","isSidechain":false,"type":"assistant","message":{"role":"assistant","usage":{"input_tokens":100,"output_tokens":50}}}
{"invalid json structure
{"parentUuid":"456","isSidechain":false,"type":"assistant","message":{"role":"assistant","usage":{"input_tokens":200,"output_tokens":100}}}
`

	if err := os.WriteFile(transcriptPath, []byte(jsonlContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Should not error - malformed lines are skipped
	metrics, err := ParseTranscript(transcriptPath)
	if err != nil {
		t.Fatalf("ParseTranscript() error = %v, want nil (should skip malformed lines)", err)
	}

	// Should have accumulated tokens from valid lines only
	// First valid assistant: 100 input, 50 output
	// Second valid assistant: 200 input, 100 output
	// Total: 300 input, 150 output
	wantInput := int64(300)
	wantOutput := int64(150)

	if metrics.InputTokens != wantInput {
		t.Errorf("InputTokens = %d, want %d", metrics.InputTokens, wantInput)
	}
	if metrics.OutputTokens != wantOutput {
		t.Errorf("OutputTokens = %d, want %d", metrics.OutputTokens, wantOutput)
	}
}

func TestParseTranscript_UserMessagesIgnored(t *testing.T) {
	// Create a temporary JSONL file that includes user messages
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "with_user.jsonl")

	jsonlContent := `{"type":"summary","summary":"Test session"}
{"parentUuid":null,"isSidechain":false,"type":"user","message":{"role":"user","content":"Hello"}}
{"parentUuid":"123","isSidechain":false,"type":"assistant","message":{"role":"assistant","usage":{"input_tokens":100,"output_tokens":50}}}
{"parentUuid":"456","isSidechain":false,"type":"user","message":{"role":"user","content":"Another message"}}
`

	if err := os.WriteFile(transcriptPath, []byte(jsonlContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	metrics, err := ParseTranscript(transcriptPath)
	if err != nil {
		t.Fatalf("ParseTranscript() error = %v", err)
	}

	// Should only count assistant message tokens
	if metrics.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100 (user messages should be ignored)", metrics.InputTokens)
	}
	if metrics.OutputTokens != 50 {
		t.Errorf("OutputTokens = %d, want 50 (user messages should be ignored)", metrics.OutputTokens)
	}
}
