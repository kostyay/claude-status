// Package install provides functionality to install claude-status into Claude Code settings.
package install

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
)

// StatusLine represents the statusLine configuration object for Claude Code.
type StatusLine struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Padding int    `json:"padding"`
}

// Run executes the install flow: shows diff, prompts for confirmation, writes settings.
func Run(w io.Writer, r io.Reader) error {
	// Get the binary path
	binaryPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Resolve symlinks to get the real path
	binaryPath, err = filepath.EvalSymlinks(binaryPath)
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	// Get settings path
	settingsPath := GetSettingsPath()

	// Read existing settings
	beforeSettings, err := ReadSettings(settingsPath)
	if err != nil {
		return fmt.Errorf("failed to read settings: %w", err)
	}

	// Deep copy before settings for diff comparison
	beforeCopy := deepCopySettings(beforeSettings)

	// Update settings with statusLine
	UpdateSettings(beforeSettings, binaryPath)
	afterSettings := beforeSettings

	// Show diff
	ShowDiff(w, settingsPath, beforeCopy, afterSettings)

	// Prompt for confirmation
	if !PromptConfirm(w, r) {
		fmt.Fprintln(w, "Installation cancelled.")
		return nil
	}

	// Generate JSON for writing (with trailing newline for proper text file format)
	afterJSON, err := json.MarshalIndent(afterSettings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}
	afterJSON = append(afterJSON, '\n')

	// Write settings
	if err := WriteSettings(settingsPath, afterJSON); err != nil {
		return fmt.Errorf("failed to write settings: %w", err)
	}

	fmt.Fprintln(w, "Successfully installed claude-status!")
	return nil
}

// deepCopySettings creates a deep copy of settings map via JSON round-trip.
func deepCopySettings(settings map[string]any) map[string]any {
	data, _ := json.Marshal(settings)
	var copy map[string]any
	_ = json.Unmarshal(data, &copy)
	if copy == nil {
		return make(map[string]any)
	}
	return copy
}

// GetSettingsPath returns the path to Claude Code's settings.json.
// Respects CLAUDE_CONFIG_DIR environment variable.
func GetSettingsPath() string {
	configDir := os.Getenv("CLAUDE_CONFIG_DIR")
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ".claude/settings.json"
		}
		configDir = filepath.Join(home, ".claude")
	}
	return filepath.Join(configDir, "settings.json")
}

// ReadSettings reads and parses the settings file.
// Returns empty map if file doesn't exist.
func ReadSettings(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]any), nil
		}
		return nil, err
	}

	// Handle empty file
	if len(data) == 0 {
		return make(map[string]any), nil
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("invalid JSON in settings file: %w", err)
	}

	return settings, nil
}

// UpdateSettings adds or updates the statusLine configuration.
func UpdateSettings(settings map[string]any, binaryPath string) {
	settings["statusLine"] = StatusLine{
		Type:    "command",
		Command: binaryPath,
		Padding: 0,
	}
}

// ShowDiff displays the diff between before and after settings as JSON.
func ShowDiff(w io.Writer, path string, before, after map[string]any) {
	fmt.Fprintf(w, "Settings file: %s\n\n", path)

	beforeJSON, _ := json.MarshalIndent(before, "", "  ")
	afterJSON, _ := json.MarshalIndent(after, "", "  ")

	if string(beforeJSON) == string(afterJSON) {
		fmt.Fprintln(w, "No changes needed.")
		return
	}

	// Generate unified diff
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(beforeJSON)),
		B:        difflib.SplitLines(string(afterJSON)),
		FromFile: "before",
		ToFile:   "after",
		Context:  3,
	}

	result, _ := difflib.GetUnifiedDiffString(diff)

	// Colorize the diff output
	for _, line := range strings.Split(result, "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			fmt.Fprintf(w, "\033[32m%s\033[0m\n", line)
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			fmt.Fprintf(w, "\033[31m%s\033[0m\n", line)
		} else if strings.HasPrefix(line, "@@") {
			fmt.Fprintf(w, "\033[36m%s\033[0m\n", line)
		} else if line != "" {
			fmt.Fprintln(w, line)
		}
	}
	fmt.Fprintln(w)
}

// PromptConfirm asks the user to confirm the changes.
func PromptConfirm(w io.Writer, r io.Reader) bool {
	fmt.Fprint(w, "Apply changes? [y/N]: ")

	reader := bufio.NewReader(r)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}

// WriteSettings writes the settings to the file, creating directories if needed.
func WriteSettings(path string, data []byte) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
