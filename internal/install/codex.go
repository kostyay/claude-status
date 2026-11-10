package install

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
)

// codexStatusLineItems is ordered by narrow-screen priority. Codex truncates
// the footer from the right, so session and usage data come before workspace
// identifiers.
var codexStatusLineItems = []string{
	"model-with-reasoning",
	"run-state",
	"context-used",
	"five-hour-limit",
	"weekly-limit",
	"git-branch",
	"pull-request-number",
	"branch-changes",
	"project-name",
}

// GetCodexConfigPath returns the user-level Codex config.toml path.
// CODEX_HOME overrides the default ~/.codex directory.
func GetCodexConfigPath() string {
	configDir := os.Getenv("CODEX_HOME")
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ".codex/config.toml"
		}
		configDir = filepath.Join(home, ".codex")
	}
	return filepath.Join(configDir, "config.toml")
}

func runCodex(w io.Writer, r io.Reader) error {
	configPath := GetCodexConfigPath()
	before, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read Codex config: %w", err)
	}

	updated, err := UpdateCodexConfig(string(before))
	if err != nil {
		return fmt.Errorf("failed to update %s: %w", configPath, err)
	}
	after := []byte(updated)
	showTextDiff(w, configPath, before, after)
	if bytes.Equal(before, after) {
		fmt.Fprintln(w, "Codex status line is already configured.")
		return nil
	}

	if !PromptConfirm(w, r) {
		fmt.Fprintln(w, "Installation cancelled.")
		return nil
	}

	if err := WriteSettings(configPath, after); err != nil {
		return fmt.Errorf("failed to write Codex config: %w", err)
	}

	fmt.Fprintln(w, "Successfully configured the Codex status line!")
	return nil
}

// UpdateCodexConfig adds or replaces tui.status_line while preserving all
// unrelated TOML text, including comments and formatting. It writes into an
// existing [tui] table, or beside root-level tui.* dotted keys, since adding a
// [tui] header there would define the table twice.
func UpdateCodexConfig(content string) (string, error) {
	lines := strings.Split(content, "\n")
	rootEnd := sectionEnd(lines, 0)
	for i := rootEnd; i < len(lines); i++ {
		if tomlHeader(lines[i]) == "tui" {
			return upsertStatusLine(lines, i+1, sectionEnd(lines, i+1), "status_line"), nil
		}
	}

	for i := range rootEnd {
		switch key := assignmentKey(lines[i]); {
		case key == "tui":
			return "", fmt.Errorf(
				"tui is an inline table on line %d; add status_line to it manually", i+1)
		case strings.HasPrefix(key, "tui."):
			return upsertStatusLine(lines, i, rootEnd, "tui.status_line"), nil
		}
	}

	trimmed := strings.TrimRight(content, "\r\n")
	if trimmed != "" {
		trimmed += "\n\n"
	}
	return trimmed + "[tui]\n" + codexStatusLineBlock("status_line", "") + "\n", nil
}

// upsertStatusLine replaces the key's assignment within lines[from:to], or inserts
// it at from when absent.
func upsertStatusLine(lines []string, from, to int, key string) string {
	start, end, comment := from, from, "" // lines[start:end] is replaced; empty means insert
	for i := from; i < to; i++ {
		if assignmentKey(lines[i]) == key {
			last := assignmentEnd(lines, i, to)
			start, end, comment = i, last+1, trailingComment(lines[last])
			break
		}
	}
	block := strings.Split(codexStatusLineBlock(key, comment), "\n")
	updated := make([]string, 0, len(lines)+len(block))
	updated = append(updated, lines[:start]...)
	updated = append(updated, block...)
	updated = append(updated, lines[end:]...)
	return strings.Join(updated, "\n")
}

func sectionEnd(lines []string, from int) int {
	for i := from; i < len(lines); i++ {
		if tomlHeader(lines[i]) != "" {
			return i
		}
	}
	return len(lines)
}

func codexStatusLineBlock(key, comment string) string {
	var b strings.Builder
	b.WriteString(key + " = [\n")
	for _, item := range codexStatusLineItems {
		fmt.Fprintf(&b, "  %q,\n", item)
	}
	b.WriteString("]")
	if comment != "" {
		b.WriteByte(' ')
		b.WriteString(comment)
	}
	return b.String()
}

// tomlHeader returns the name inside a [table] or [[array]] header line, or "".
func tomlHeader(line string) string {
	line = strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
	if len(line) < 3 || !strings.HasPrefix(line, "[") || !strings.HasSuffix(line, "]") {
		return ""
	}
	return strings.TrimSpace(line[1 : len(line)-1])
}

// assignmentKey returns the whitespace-free key of a "key = value" line, or "".
func assignmentKey(line string) string {
	key, _, ok := strings.Cut(line, "=")
	if !ok {
		return ""
	}
	return strings.Join(strings.Fields(key), "")
}

func assignmentEnd(lines []string, start, limit int) int {
	depth := 0
	sawArray := false
	inDoubleQuote := false
	inSingleQuote := false
	escaped := false
	for i := start; i < limit; i++ {
		for _, r := range lines[i] {
			if escaped {
				escaped = false
				continue
			}
			if inDoubleQuote {
				switch r {
				case '\\':
					escaped = true
				case '"':
					inDoubleQuote = false
				}
				continue
			}
			if inSingleQuote {
				if r == '\'' {
					inSingleQuote = false
				}
				continue
			}
			if r == '#' {
				break
			}

			switch r {
			case '"':
				inDoubleQuote = true
			case '\'':
				inSingleQuote = true
			case '[':
				depth++
				sawArray = true
			case ']':
				depth--
			}
		}
		if !sawArray || depth <= 0 {
			return i
		}
	}
	return limit - 1
}

func trailingComment(line string) string {
	inDoubleQuote := false
	inSingleQuote := false
	escaped := false
	for i, r := range line {
		if escaped {
			escaped = false
			continue
		}
		if inDoubleQuote {
			switch r {
			case '\\':
				escaped = true
			case '"':
				inDoubleQuote = false
			}
			continue
		}
		if inSingleQuote {
			if r == '\'' {
				inSingleQuote = false
			}
			continue
		}
		switch r {
		case '"':
			inDoubleQuote = true
		case '\'':
			inSingleQuote = true
		case '#':
			return strings.TrimSpace(line[i:])
		}
	}
	return ""
}

func showTextDiff(w io.Writer, path string, before, after []byte) {
	fmt.Fprintf(w, "Config file: %s\n\n", path)
	if string(before) == string(after) {
		fmt.Fprintln(w, "No changes needed.")
		return
	}

	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(before)),
		B:        difflib.SplitLines(string(after)),
		FromFile: "before",
		ToFile:   "after",
		Context:  3,
	}
	result, _ := difflib.GetUnifiedDiffString(diff)
	writeColorizedDiff(w, result)
}
