package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain_ExitNonZeroOnError(t *testing.T) {
	tmp := t.TempDir()
	env := append(os.Environ(),
		"XDG_CACHE_HOME="+filepath.Join(tmp, "cache"),
		"XDG_CONFIG_HOME="+filepath.Join(tmp, "config"),
		"XDG_DATA_HOME="+filepath.Join(tmp, "data"),
	)

	cmd := exec.Command("go", "run", "./cmd/claude-status")
	cmd.Dir = filepath.Clean("../..")
	cmd.Env = env
	cmd.Stdin = strings.NewReader("not valid json")

	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit code, got success (output: %s)", string(out))
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected ExitError, got %T", err)
	}
	if exitErr.ExitCode() == 0 {
		t.Fatalf("expected non-zero exit code, got 0 (output: %s)", string(out))
	}
	if !strings.Contains(string(out), "[Claude]") {
		t.Fatalf("expected fallback output to contain [Claude], got: %s", string(out))
	}
}
