package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

// DefaultTimeout is the per-invocation timeout for plugin execution.
const DefaultTimeout = 5 * time.Minute

// MaxOutputBytes caps how much a plugin may write to stdout. A plugin that
// exceeds it is killed and the call fails.
const MaxOutputBytes = 8 << 20

// Exec calls the plugin binary at path, sending inv as JSON on stdin and
// parsing the JSON result from stdout. The timeout caps the total wall time.
func Exec(ctx context.Context, path string, inv Invocation, timeout time.Duration) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	payload, err := json.Marshal(inv)
	if err != nil {
		return Result{}, fmt.Errorf("marshaling invocation: %w", err)
	}

	cmd := exec.CommandContext(ctx, path)
	cmd.Stdin = bytes.NewReader(payload)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return Result{}, fmt.Errorf("plugin %s: %w", path, err)
	}
	if err := cmd.Start(); err != nil {
		return Result{}, fmt.Errorf("plugin %s failed: %w", path, err)
	}
	stdout, readErr := io.ReadAll(io.LimitReader(pipe, MaxOutputBytes+1))
	if len(stdout) > MaxOutputBytes {
		cancel()
		_ = cmd.Wait()
		return Result{}, fmt.Errorf("plugin %s wrote more than %d bytes to stdout", path, MaxOutputBytes)
	}
	err = cmd.Wait()
	if err == nil && readErr != nil {
		err = readErr
	}
	if err != nil {
		stderrText := strings.TrimSpace(stderr.String())
		if stderrText != "" {
			return Result{}, fmt.Errorf("plugin %s failed: %w\nstderr: %s", path, err, stderrText)
		}
		return Result{}, fmt.Errorf("plugin %s failed: %w", path, err)
	}

	var result Result
	if err := json.Unmarshal(stdout, &result); err != nil {
		return Result{}, fmt.Errorf("parsing plugin output from %s: %w\nraw: %s", path, err, stdout)
	}
	return result, nil
}
