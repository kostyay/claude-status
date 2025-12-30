package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := Default()

	if cfg.Template != DefaultTemplate {
		t.Errorf("Template = %q, want %q", cfg.Template, DefaultTemplate)
	}
	if cfg.GitHubWorkflow != "build_and_test" {
		t.Errorf("GitHubWorkflow = %q, want %q", cfg.GitHubWorkflow, "build_and_test")
	}
	if cfg.GitHubTTL != 60 {
		t.Errorf("GitHubTTL = %d, want %d", cfg.GitHubTTL, 60)
	}
	if cfg.LoggingEnabled != false {
		t.Errorf("LoggingEnabled = %v, want %v", cfg.LoggingEnabled, false)
	}
	if cfg.LogPath != "" {
		t.Errorf("LogPath = %q, want %q", cfg.LogPath, "")
	}
}

func TestLoadConfig_ValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	content := `{
		"template": "custom template",
		"github_workflow": "ci",
		"github_ttl": 120,
		"logging_enabled": true,
		"log_path": "/custom/log.json"
	}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := LoadFrom(path)

	if cfg.Template != "custom template" {
		t.Errorf("Template = %q, want %q", cfg.Template, "custom template")
	}
	if cfg.GitHubWorkflow != "ci" {
		t.Errorf("GitHubWorkflow = %q, want %q", cfg.GitHubWorkflow, "ci")
	}
	if cfg.GitHubTTL != 120 {
		t.Errorf("GitHubTTL = %d, want %d", cfg.GitHubTTL, 120)
	}
	if cfg.LoggingEnabled != true {
		t.Errorf("LoggingEnabled = %v, want %v", cfg.LoggingEnabled, true)
	}
	if cfg.LogPath != "/custom/log.json" {
		t.Errorf("LogPath = %q, want %q", cfg.LogPath, "/custom/log.json")
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if err := os.WriteFile(path, []byte("{ invalid json }"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := LoadFrom(path)

	// Should fall back to defaults
	if cfg.Template != DefaultTemplate {
		t.Errorf("Template = %q, want %q (default)", cfg.Template, DefaultTemplate)
	}
	if cfg.GitHubWorkflow != "build_and_test" {
		t.Errorf("GitHubWorkflow = %q, want %q (default)", cfg.GitHubWorkflow, "build_and_test")
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	path := "/nonexistent/path/config.json"
	cfg := LoadFrom(path)

	// Should return defaults
	if cfg.Template != DefaultTemplate {
		t.Errorf("Template = %q, want %q (default)", cfg.Template, DefaultTemplate)
	}
	if cfg.GitHubWorkflow != "build_and_test" {
		t.Errorf("GitHubWorkflow = %q, want %q (default)", cfg.GitHubWorkflow, "build_and_test")
	}
}

func TestLoadConfig_PartialConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	// Only set some fields
	content := `{"github_ttl": 30}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := LoadFrom(path)

	// Specified field should be set
	if cfg.GitHubTTL != 30 {
		t.Errorf("GitHubTTL = %d, want %d", cfg.GitHubTTL, 30)
	}
	// Unspecified fields should have defaults
	if cfg.Template != DefaultTemplate {
		t.Errorf("Template = %q, want %q (default)", cfg.Template, DefaultTemplate)
	}
	if cfg.GitHubWorkflow != "build_and_test" {
		t.Errorf("GitHubWorkflow = %q, want %q (default)", cfg.GitHubWorkflow, "build_and_test")
	}
}

func TestLoadConfig_EmptyTemplate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	// Empty template should use default
	content := `{"template": ""}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := LoadFrom(path)

	if cfg.Template != DefaultTemplate {
		t.Errorf("Template = %q, want %q (default)", cfg.Template, DefaultTemplate)
	}
}

func TestLoadConfig_LoggingEnabledFalse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	// Explicitly set logging_enabled to false
	content := `{"logging_enabled": false}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := LoadFrom(path)

	if cfg.LoggingEnabled != false {
		t.Errorf("LoggingEnabled = %v, want %v", cfg.LoggingEnabled, false)
	}
}

func TestXDGPaths(t *testing.T) {
	// These tests verify that paths are constructed correctly
	// The actual XDG values depend on the environment

	cacheDir := CacheDir()
	if !filepath.IsAbs(cacheDir) {
		t.Errorf("CacheDir() = %q, want absolute path", cacheDir)
	}
	if filepath.Base(cacheDir) != "claude-status" {
		t.Errorf("CacheDir() base = %q, want %q", filepath.Base(cacheDir), "claude-status")
	}

	configDir := ConfigDir()
	if !filepath.IsAbs(configDir) {
		t.Errorf("ConfigDir() = %q, want absolute path", configDir)
	}
	if filepath.Base(configDir) != "claude-status" {
		t.Errorf("ConfigDir() base = %q, want %q", filepath.Base(configDir), "claude-status")
	}

	dataDir := DataDir()
	if !filepath.IsAbs(dataDir) {
		t.Errorf("DataDir() = %q, want absolute path", dataDir)
	}
	if filepath.Base(dataDir) != "claude-status" {
		t.Errorf("DataDir() base = %q, want %q", filepath.Base(dataDir), "claude-status")
	}
}

func TestXDGPaths_EnvOverride(t *testing.T) {
	// Save original env values
	origCache := os.Getenv("XDG_CACHE_HOME")
	origConfig := os.Getenv("XDG_CONFIG_HOME")
	origData := os.Getenv("XDG_DATA_HOME")

	defer func() {
		os.Setenv("XDG_CACHE_HOME", origCache)
		os.Setenv("XDG_CONFIG_HOME", origConfig)
		os.Setenv("XDG_DATA_HOME", origData)
	}()

	// Note: The xdg library caches values at init time, so we can only
	// verify that our functions return paths under the current XDG dirs.
	// A full test would require restarting the process with new env vars.

	cacheDir := CacheDir()
	if cacheDir == "" {
		t.Error("CacheDir() returned empty string")
	}

	configDir := ConfigDir()
	if configDir == "" {
		t.Error("ConfigDir() returned empty string")
	}

	dataDir := DataDir()
	if dataDir == "" {
		t.Error("DataDir() returned empty string")
	}
}
