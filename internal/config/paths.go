package config

import (
	"path/filepath"

	"github.com/adrg/xdg"
)

const appName = "claude-status"

// CacheDir returns the XDG cache directory for claude-status.
func CacheDir() string {
	return filepath.Join(xdg.CacheHome, appName)
}

// ConfigDir returns the XDG config directory for claude-status.
func ConfigDir() string {
	return filepath.Join(xdg.ConfigHome, appName)
}

// DataDir returns the XDG data directory for claude-status.
func DataDir() string {
	return filepath.Join(xdg.DataHome, appName)
}

// ConfigPath returns the full path to the config file.
func ConfigPath() string {
	return filepath.Join(ConfigDir(), "config.json")
}

// CachePath returns the full path to the cache file.
func CachePath() string {
	return filepath.Join(CacheDir(), "cache.json")
}

// LogPath returns the default path to the log file.
func LogPath() string {
	return filepath.Join(DataDir(), "status_line.json")
}
