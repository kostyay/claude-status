package install

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSettingsPath_Default(t *testing.T) {
	// Clear env var using t.Setenv for proper cleanup
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	path := GetSettingsPath()

	home, err := os.UserHomeDir()
	require.NoError(t, err)

	expected := filepath.Join(home, ".claude", "settings.json")
	assert.Equal(t, expected, path)
}

func TestGetSettingsPath_EnvOverride(t *testing.T) {
	customDir := "/custom/claude/config"
	t.Setenv("CLAUDE_CONFIG_DIR", customDir)

	path := GetSettingsPath()

	expected := filepath.Join(customDir, "settings.json")
	assert.Equal(t, expected, path)
}

func TestReadSettings_NewFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "settings.json")

	settings, err := ReadSettings(path)

	require.NoError(t, err)
	assert.Empty(t, settings)
}

func TestReadSettings_ExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "settings.json")

	content := `{"theme": "dark", "fontSize": 14}`
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)

	settings, err := ReadSettings(path)

	require.NoError(t, err)
	assert.Equal(t, "dark", settings["theme"])
	assert.Equal(t, float64(14), settings["fontSize"])
}

func TestReadSettings_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "settings.json")

	err := os.WriteFile(path, []byte(""), 0644)
	require.NoError(t, err)

	settings, err := ReadSettings(path)

	require.NoError(t, err)
	assert.Empty(t, settings)
}

func TestReadSettings_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "settings.json")

	err := os.WriteFile(path, []byte("not valid json"), 0644)
	require.NoError(t, err)

	_, err = ReadSettings(path)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JSON")
}

func TestReadSettings_PermissionDenied(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping test when running as root")
	}

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "settings.json")

	err := os.WriteFile(path, []byte(`{"theme": "dark"}`), 0644)
	require.NoError(t, err)

	// Remove read permission
	err = os.Chmod(path, 0000)
	require.NoError(t, err)
	t.Cleanup(func() {
		os.Chmod(path, 0644) // Restore for cleanup
	})

	_, err = ReadSettings(path)

	assert.Error(t, err)
}

func TestUpdateSettings_NewKey(t *testing.T) {
	settings := make(map[string]any)
	binaryPath := "/usr/local/bin/claude-status"

	UpdateSettings(settings, binaryPath)

	statusLine, ok := settings["statusLine"].(StatusLine)
	require.True(t, ok)
	assert.Equal(t, "command", statusLine.Type)
	assert.Equal(t, binaryPath, statusLine.Command)
	assert.Equal(t, 0, statusLine.Padding)
}

func TestUpdateSettings_ExistingKey(t *testing.T) {
	settings := map[string]any{
		"statusLine": map[string]any{
			"type":    "command",
			"command": "/old/path",
		},
	}
	newPath := "/new/path/claude-status"

	UpdateSettings(settings, newPath)

	statusLine, ok := settings["statusLine"].(StatusLine)
	require.True(t, ok)
	assert.Equal(t, newPath, statusLine.Command)
}

func TestUpdateSettings_PreservesOtherKeys(t *testing.T) {
	settings := map[string]any{
		"theme":    "dark",
		"fontSize": 14,
	}
	binaryPath := "/usr/local/bin/claude-status"

	UpdateSettings(settings, binaryPath)

	assert.Equal(t, "dark", settings["theme"])
	assert.Equal(t, 14, settings["fontSize"])
	assert.Contains(t, settings, "statusLine")
}

func TestShowDiff(t *testing.T) {
	var buf bytes.Buffer
	path := "/home/user/.claude/settings.json"
	before := map[string]any{}
	after := map[string]any{
		"statusLine": map[string]any{
			"type":    "command",
			"command": "/usr/bin/claude-status",
			"padding": float64(0),
		},
	}

	ShowDiff(&buf, path, before, after)

	output := buf.String()
	assert.Contains(t, output, path)
	assert.Contains(t, output, "--- before")
	assert.Contains(t, output, "+++ after")
	assert.Contains(t, output, "statusLine")
}

func TestShowDiff_NoChanges(t *testing.T) {
	var buf bytes.Buffer
	path := "/home/user/.claude/settings.json"
	settings := map[string]any{"theme": "dark"}

	ShowDiff(&buf, path, settings, settings)

	output := buf.String()
	assert.Contains(t, output, "No changes needed")
}

func TestPromptConfirm(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"yes lowercase", "y\n", true},
		{"yes full", "yes\n", true},
		{"yes uppercase", "Y\n", true},
		{"YES full uppercase", "YES\n", true},
		{"no lowercase", "n\n", false},
		{"no full", "no\n", false},
		{"empty input", "\n", false},
		{"random text", "maybe\n", false},
		{"whitespace around yes", "  y  \n", true},
		{"EOF (empty reader)", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			input := strings.NewReader(tt.input)

			result := PromptConfirm(&buf, input)

			assert.Equal(t, tt.expected, result)
			assert.Contains(t, buf.String(), "Apply changes?")
		})
	}
}

func TestWriteSettings(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subdir", "settings.json")
	data := []byte(`{"statusLine": {}}`)

	err := WriteSettings(path, data)

	require.NoError(t, err)

	// Verify file was written
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, data, content)
}

func TestWriteSettings_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "deep", "nested", "dir", "settings.json")

	err := WriteSettings(path, []byte("{}"))

	require.NoError(t, err)
	assert.FileExists(t, path)
}

func TestRun_Integration_Confirm(t *testing.T) {
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "settings.json")

	// Set env to use temp dir
	t.Setenv("CLAUDE_CONFIG_DIR", tmpDir)

	// Create existing settings
	existingSettings := map[string]any{"theme": "dark"}
	data, _ := json.MarshalIndent(existingSettings, "", "  ")
	err := os.WriteFile(settingsPath, data, 0644)
	require.NoError(t, err)

	// Simulate user input "y"
	var output bytes.Buffer
	input := strings.NewReader("y\n")

	err = Run(&output, input)
	require.NoError(t, err)

	// Verify output
	assert.Contains(t, output.String(), "Settings file:")
	assert.Contains(t, output.String(), "Successfully installed")

	// Verify file was updated
	updatedData, err := os.ReadFile(settingsPath)
	require.NoError(t, err)

	// Verify trailing newline
	assert.True(t, len(updatedData) > 0 && updatedData[len(updatedData)-1] == '\n',
		"settings file should end with newline")

	var settings map[string]any
	err = json.Unmarshal(updatedData, &settings)
	require.NoError(t, err)

	assert.Equal(t, "dark", settings["theme"])
	assert.Contains(t, settings, "statusLine")

	statusLine := settings["statusLine"].(map[string]any)
	assert.Equal(t, "command", statusLine["type"])
	assert.Equal(t, float64(0), statusLine["padding"])
}

func TestRun_Integration_Cancel(t *testing.T) {
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "settings.json")

	t.Setenv("CLAUDE_CONFIG_DIR", tmpDir)

	// Create existing settings
	existingSettings := map[string]any{"theme": "dark"}
	data, _ := json.MarshalIndent(existingSettings, "", "  ")
	err := os.WriteFile(settingsPath, data, 0644)
	require.NoError(t, err)

	// Simulate user input "n"
	var output bytes.Buffer
	input := strings.NewReader("n\n")

	err = Run(&output, input)
	require.NoError(t, err)

	// Verify output
	assert.Contains(t, output.String(), "cancelled")

	// Verify file was NOT updated
	updatedData, err := os.ReadFile(settingsPath)
	require.NoError(t, err)

	var settings map[string]any
	err = json.Unmarshal(updatedData, &settings)
	require.NoError(t, err)

	assert.NotContains(t, settings, "statusLine")
}

func TestRun_Integration_NewFile(t *testing.T) {
	tmpDir := t.TempDir()

	t.Setenv("CLAUDE_CONFIG_DIR", tmpDir)

	// Simulate user input "y"
	var output bytes.Buffer
	input := strings.NewReader("y\n")

	err := Run(&output, input)
	require.NoError(t, err)

	// Verify file was created
	settingsPath := filepath.Join(tmpDir, "settings.json")
	assert.FileExists(t, settingsPath)

	data, err := os.ReadFile(settingsPath)
	require.NoError(t, err)

	var settings map[string]any
	err = json.Unmarshal(data, &settings)
	require.NoError(t, err)

	assert.Contains(t, settings, "statusLine")
}
