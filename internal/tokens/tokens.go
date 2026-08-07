package tokens

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Metrics holds token usage statistics parsed from a transcript.
type Metrics struct {
	InputTokens         int64 // Total input tokens used
	OutputTokens        int64 // Total output tokens generated
	CacheReadTokens     int64 // Cache-read tokens (priced lower than input)
	CacheCreationTokens int64 // Cache-write tokens (priced higher than input)
	CachedTokens        int64 // Sum of cache read + creation (back-compat)
	TotalTokens         int64 // Sum of all tokens
	ContextLength       int64 // Current context window size (last message's input + cache)
}

// AutoCompactThreshold is the fraction of the context window at which Claude Code
// triggers auto-compaction. Used to derive UsableTokens and to scale used_percentage
// from the stdin context_window data.
const AutoCompactThreshold = 0.8

// suffix1M is the Claude Code marker on model IDs that signals 1M-token context mode.
const suffix1M = "[1m]"

// Has1MContext reports whether a model ID carries the 1M-context suffix.
func Has1MContext(modelID string) bool {
	return strings.Contains(strings.ToLower(modelID), suffix1M)
}

// NormalizeModelID strips Claude Code's context-mode suffix so the bare model ID
// can be used as a key against APIs like models.dev that don't know about it.
func NormalizeModelID(modelID string) string {
	if i := strings.Index(strings.ToLower(modelID), suffix1M); i >= 0 {
		return modelID[:i]
	}
	return modelID
}

// ContextConfig holds model-specific context limits.
type ContextConfig struct {
	MaxTokens    int64 // Maximum context window (1M for Opus 4.6/Sonnet 4.5/Sonnet 4 [1m], 200k otherwise)
	UsableTokens int64 // Usable context before auto-compact (AutoCompactThreshold of max)
}

// GetContextConfig infers context limits from a model ID, for when Claude Code
// does not report the session's window size. Models carrying the "[1m]" suffix
// get 1M, all others 200k.
func GetContextConfig(modelID string) ContextConfig {
	if Has1MContext(modelID) {
		return NewContextConfig(1_000_000)
	}
	return NewContextConfig(200_000)
}

// NewContextConfig derives the usable-token threshold from a context window size.
func NewContextConfig(maxTokens int64) ContextConfig {
	return ContextConfig{
		MaxTokens:    maxTokens,
		UsableTokens: int64(float64(maxTokens) * AutoCompactThreshold),
	}
}

// transcriptLine represents a single line in the JSONL transcript.
type transcriptLine struct {
	Type        string   `json:"type"`
	IsSidechain bool     `json:"isSidechain"`
	Message     *message `json:"message"`
}

// message represents the message field in a transcript line.
type message struct {
	Role  string `json:"role"`
	Usage *usage `json:"usage"`
}

// usage represents token usage in a message.
type usage struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
}

// ParseTranscript reads a JSONL transcript file and calculates token metrics.
// It skips sidechain messages (agent messages) and non-assistant messages.
func ParseTranscript(path string) (Metrics, error) {
	if path == "" {
		return Metrics{}, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return Metrics{}, err
	}
	defer file.Close()

	var m Metrics
	var lastContextLength int64

	scanner := bufio.NewScanner(file)
	// Increase buffer size for large lines (some messages can be very long)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var entry transcriptLine
		if err := json.Unmarshal(line, &entry); err != nil {
			// Skip malformed lines
			continue
		}

		// Skip sidechain (agent) messages and non-message entries
		if entry.IsSidechain || entry.Message == nil || entry.Message.Usage == nil {
			continue
		}

		u := entry.Message.Usage

		m.InputTokens += u.InputTokens
		m.OutputTokens += u.OutputTokens
		m.CacheReadTokens += u.CacheReadInputTokens
		m.CacheCreationTokens += u.CacheCreationInputTokens

		// Context length is the most recent message's input + cache reads + cache creations.
		lastContextLength = u.InputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens
	}

	if err := scanner.Err(); err != nil {
		return Metrics{}, err
	}

	m.CachedTokens = m.CacheReadTokens + m.CacheCreationTokens
	m.TotalTokens = m.InputTokens + m.OutputTokens + m.CachedTokens
	m.ContextLength = lastContextLength

	return m, nil
}

// ContextPercentage calculates the percentage of max context used.
func (m Metrics) ContextPercentage(cfg ContextConfig) float64 {
	if cfg.MaxTokens == 0 {
		return 0
	}
	pct := float64(m.ContextLength) / float64(cfg.MaxTokens) * 100
	if pct > 100 {
		return 100
	}
	return pct
}

// ContextPercentageUsable calculates the percentage of usable context used.
// This accounts for the 80% auto-compact threshold.
func (m Metrics) ContextPercentageUsable(cfg ContextConfig) float64 {
	if cfg.UsableTokens == 0 {
		return 0
	}
	pct := float64(m.ContextLength) / float64(cfg.UsableTokens) * 100
	if pct > 100 {
		return 100
	}
	return pct
}

// FormatTokens formats a token count in a human-readable way.
// e.g., 1234 -> "1.2k", 1234567 -> "1.2M"
func FormatTokens(count int64) string {
	if count >= 1_000_000 {
		return formatWithSuffix(float64(count)/1_000_000, "M")
	}
	if count >= 1_000 {
		return formatWithSuffix(float64(count)/1_000, "k")
	}
	return fmt.Sprintf("%d", count)
}

func formatWithSuffix(f float64, suffix string) string {
	// Format with one decimal place
	s := fmt.Sprintf("%.1f", f)
	// Remove trailing .0
	s = strings.TrimSuffix(s, ".0")
	return s + suffix
}
