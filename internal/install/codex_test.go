package install

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTarget(t *testing.T) {
	tests := []struct {
		value string
		want  Target
	}{
		{"", TargetClaude},
		{"claude", TargetClaude},
		{"CODEX", TargetCodex},
		{" all ", TargetAll},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got, err := ParseTarget(tt.value)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	_, err := ParseTarget("other")
	assert.Error(t, err)
}

func TestGetCodexConfigPath(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		t.Setenv("CODEX_HOME", "")
		home, err := os.UserHomeDir()
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(home, ".codex", "config.toml"), GetCodexConfigPath())
	})

	t.Run("CODEX_HOME", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("CODEX_HOME", dir)
		assert.Equal(t, filepath.Join(dir, "config.toml"), GetCodexConfigPath())
	})
}

func TestUpdateCodexConfig_NewFile(t *testing.T) {
	updated := UpdateCodexConfig("")

	assert.Equal(t, []string{
		"model-with-reasoning",
		"run-state",
		"context-used",
		"five-hour-limit",
		"weekly-limit",
		"git-branch",
		"project-name",
	}, codexStatusLineItems)
	assert.True(t, strings.HasPrefix(updated, "[tui]\nstatus_line = [\n"))
	for _, item := range codexStatusLineItems {
		assert.Contains(t, updated, `"`+item+`"`)
	}
	assert.True(t, strings.HasSuffix(updated, "\n"))
}

func TestUpdateCodexConfig_AddsTUISection(t *testing.T) {
	before := "# keep this comment\nmodel = \"gpt-5\"\n"
	updated := UpdateCodexConfig(before)

	assert.Contains(t, updated, before)
	assert.Contains(t, updated, "\n[tui]\nstatus_line = [")
}

func TestUpdateCodexConfig_PreservesExistingTUISettings(t *testing.T) {
	before := "[tui]\nanimations = false\n\n[features]\nhooks = true\n"
	updated := UpdateCodexConfig(before)

	assert.Contains(t, updated, "[tui]\nstatus_line = [")
	assert.Contains(t, updated, "animations = false")
	assert.Contains(t, updated, "[features]\nhooks = true")
}

func TestUpdateCodexConfig_ReplacesMultilineStatusLine(t *testing.T) {
	before := "[tui] # existing\nstatus_line = [\n  \"model\",\n  \"git-branch\",\n]\nshow_tooltips = false\n"
	updated := UpdateCodexConfig(before)

	assert.NotContains(t, updated, "  \"model\",\n")
	assert.Contains(t, updated, "  \"model-with-reasoning\",")
	assert.Contains(t, updated, "show_tooltips = false")
	assert.Equal(t, 1, strings.Count(updated, "status_line ="))
}

func TestUpdateCodexConfig_IgnoresBracketsInsideStrings(t *testing.T) {
	before := "[tui]\nstatus_line = [\"custom[segment\"]\nshow_tooltips = false\n"
	updated := UpdateCodexConfig(before)

	assert.Contains(t, updated, "show_tooltips = false")
	assert.Contains(t, updated, "  \"model-with-reasoning\",")
}

func TestUpdateCodexConfig_PreservesTrailingComment(t *testing.T) {
	before := "[tui]\nstatus_line = [\"model\"] # custom layout\n"
	updated := UpdateCodexConfig(before)

	assert.Contains(t, updated, "] # custom layout\n")
}

func TestUpdateCodexConfig_DoesNotReplaceSimilarKey(t *testing.T) {
	before := "[tui]\nstatus_line_use_colors = true\n"
	updated := UpdateCodexConfig(before)

	assert.Contains(t, updated, "status_line_use_colors = true")
	assert.Equal(t, 1, strings.Count(updated, "status_line ="))
}

func TestRunCodex_Confirm(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CODEX_HOME", dir)
	configPath := filepath.Join(dir, "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte("model = \"gpt-5\"\n"), 0644))

	var output bytes.Buffer
	err := RunTarget(&output, strings.NewReader("y\n"), TargetCodex)
	require.NoError(t, err)

	updated, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(updated), "model = \"gpt-5\"")
	assert.Contains(t, string(updated), "[tui]\nstatus_line = [")
	assert.Contains(t, output.String(), "Successfully configured the Codex status line")
}

func TestRunCodex_Cancel(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CODEX_HOME", dir)
	configPath := filepath.Join(dir, "config.toml")
	before := []byte("model = \"gpt-5\"\n")
	require.NoError(t, os.WriteFile(configPath, before, 0644))

	var output bytes.Buffer
	err := RunTarget(&output, strings.NewReader("n\n"), TargetCodex)
	require.NoError(t, err)

	after, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Equal(t, before, after)
	assert.Contains(t, output.String(), "cancelled")
}

func TestRunCodex_NoChanges(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CODEX_HOME", dir)
	configPath := filepath.Join(dir, "config.toml")
	before := []byte(UpdateCodexConfig(""))
	require.NoError(t, os.WriteFile(configPath, before, 0644))

	infoBefore, err := os.Stat(configPath)
	require.NoError(t, err)
	var output bytes.Buffer
	err = RunTarget(&output, strings.NewReader(""), TargetCodex)
	require.NoError(t, err)
	infoAfter, err := os.Stat(configPath)
	require.NoError(t, err)

	after, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Equal(t, before, after)
	assert.Equal(t, infoBefore.ModTime(), infoAfter.ModTime())
	assert.NotContains(t, output.String(), "Apply changes?")
	assert.Contains(t, output.String(), "already configured")
}
