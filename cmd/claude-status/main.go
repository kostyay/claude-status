package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/kostyay/claude-status/internal/config"
	"github.com/kostyay/claude-status/internal/install"
	"github.com/kostyay/claude-status/internal/status"
	"github.com/kostyay/claude-status/internal/template"
)

var prefixFlag = flag.String("prefix", "", "Prefix to display at the start of the status line")
var prefixColorFlag = flag.String("prefix-color", "", "Color for the prefix (cyan, blue, green, yellow, red, magenta, gray)")

var installFlag = flag.Bool("install", false, "Run installation wizard")

func main() {
	flag.Parse()

	// Handle -install flag
	if *installFlag {
		if err := install.Run(os.Stdout, os.Stdin); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	os.Exit(runMain())
}

func runMain() int {
	if err := run(); err != nil {
		// Log error to stderr for debugging
		slog.Error("error", "err", err)
		// Graceful degradation - output minimal status
		fmt.Println("\033[31m[Claude] 📁 Unknown\033[0m")
		return 1
	}

	return 0
}

func run() error {
	// Load configuration
	cfg := config.Load()

	// Parse input from stdin
	var input status.Input
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		return fmt.Errorf("failed to parse input: %w", err)
	}

	// Build status data
	builder, err := status.NewBuilder(&cfg, input.Workspace.CurrentDir)
	if err != nil {
		return fmt.Errorf("failed to create builder: %w", err)
	}

	// Set prefix if provided
	if *prefixFlag != "" {
		builder.SetPrefix(*prefixFlag)

		// Set prefix color (default to cyan if not specified)
		colorName := *prefixColorFlag
		if colorName == "" {
			colorName = "cyan"
		}
		if colorCode, ok := template.ColorMap[colorName]; ok {
			builder.SetPrefixColor(colorCode)
		} else {
			slog.Warn("unknown prefix color, using cyan", "color", colorName)
			builder.SetPrefixColor(template.ColorMap["cyan"])
		}
	}

	data := builder.Build(input)

	// Render template
	engine, err := template.NewEngine(cfg.Template)
	if err != nil {
		// Log the template error and fall back to default
		slog.Warn("invalid template, using default", "err", err)
		engine, err = template.NewEngine(config.DefaultTemplate)
		if err != nil {
			return fmt.Errorf("failed to create template engine: %w", err)
		}
	}

	output, err := engine.Render(data)
	if err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	// Output the status line
	fmt.Println(output)

	// Optional logging
	if cfg.LoggingEnabled {
		logStatusLine(cfg, input, output)
	}

	return nil
}

// LogEntry represents a log entry in the status line log.
type LogEntry struct {
	Timestamp        string       `json:"timestamp"`
	InputData        status.Input `json:"input_data"`
	StatusLineOutput string       `json:"status_line_output"`
}

func logStatusLine(cfg config.Config, input status.Input, output string) {
	logPath := cfg.LogPath
	if logPath == "" {
		logPath = config.LogPath()
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		slog.Error("failed to create log directory", "err", err)
		return
	}

	// Read existing log data
	var logData []LogEntry
	if data, err := os.ReadFile(logPath); err == nil {
		if err := json.Unmarshal(data, &logData); err != nil {
			// Log file corrupted, start fresh
			slog.Warn("log file corrupted, starting fresh", "err", err)
			logData = nil
		}
	}

	// Append new entry
	logData = append(logData, LogEntry{
		Timestamp:        time.Now().Format(time.RFC3339),
		InputData:        input,
		StatusLineOutput: output,
	})

	// Write back
	data, err := json.MarshalIndent(logData, "", "  ")
	if err != nil {
		slog.Error("failed to marshal log data", "err", err)
		return
	}

	if err := os.WriteFile(logPath, data, 0644); err != nil {
		slog.Error("failed to write log file", "err", err)
	}
}
