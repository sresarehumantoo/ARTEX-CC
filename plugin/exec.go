package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"os/exec"
)

// DefaultTimeout is the per-invocation timeout for plugin execution.
const DefaultTimeout = 5 * time.Minute

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
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrText := strings.TrimSpace(stderr.String())
		if stderrText != "" {
			return Result{}, fmt.Errorf("plugin %s failed: %w\nstderr: %s", path, err, stderrText)
		}
		return Result{}, fmt.Errorf("plugin %s failed: %w", path, err)
	}

	var result Result
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return Result{}, fmt.Errorf("parsing plugin output from %s: %w\nraw: %s", path, err, stdout.String())
	}
	return result, nil
}
