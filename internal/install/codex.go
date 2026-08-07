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

	after := []byte(UpdateCodexConfig(string(before)))
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
// unrelated TOML text, including comments and formatting.
func UpdateCodexConfig(content string) string {
	lines := strings.Split(content, "\n")

	tuiStart := -1
	tuiEnd := len(lines)
	for i, line := range lines {
		if tomlHeader(line) == "tui" {
			tuiStart = i
			for j := i + 1; j < len(lines); j++ {
				if tomlHeader(lines[j]) != "" {
					tuiEnd = j
					break
				}
			}
			break
		}
	}

	if tuiStart == -1 {
		trimmed := strings.TrimRight(content, "\r\n")
		if trimmed != "" {
			trimmed += "\n\n"
		}
		return trimmed + "[tui]\n" + codexStatusLineBlock("") + "\n"
	}

	for i := tuiStart + 1; i < tuiEnd; i++ {
		if !isStatusLineAssignment(lines[i]) {
			continue
		}
		end := assignmentEnd(lines, i, tuiEnd)
		block := codexStatusLineBlock(trailingComment(lines[end]))
		updated := make([]string, 0, len(lines)-(end-i)+len(strings.Split(block, "\n")))
		updated = append(updated, lines[:i]...)
		updated = append(updated, strings.Split(block, "\n")...)
		updated = append(updated, lines[end+1:]...)
		return strings.Join(updated, "\n")
	}

	block := codexStatusLineBlock("")
	updated := make([]string, 0, len(lines)+len(codexStatusLineItems)+3)
	updated = append(updated, lines[:tuiStart+1]...)
	updated = append(updated, strings.Split(block, "\n")...)
	updated = append(updated, lines[tuiStart+1:]...)
	return strings.Join(updated, "\n")
}

func codexStatusLineBlock(comment string) string {
	var b strings.Builder
	b.WriteString("status_line = [\n")
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

func tomlHeader(line string) string {
	line = strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
	if len(line) < 3 || !strings.HasPrefix(line, "[") || !strings.HasSuffix(line, "]") || strings.HasPrefix(line, "[[") {
		return ""
	}
	return strings.TrimSpace(line[1 : len(line)-1])
}

func isStatusLineAssignment(line string) bool {
	line = strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
	if !strings.HasPrefix(line, "status_line") {
		return false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(line, "status_line"))
	return strings.HasPrefix(rest, "=")
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
