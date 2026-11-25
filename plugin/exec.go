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

// MaxErrorTextBytes caps how much plugin stderr or unparseable stdout is
// quoted in an error. Excess stderr is drained and discarded so the plugin
// never blocks on a full pipe.
const MaxErrorTextBytes = 4 << 10

type cappedBuffer struct {
	buf     bytes.Buffer
	dropped int
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	room := MaxErrorTextBytes - c.buf.Len()
	if room > len(p) {
		room = len(p)
	}
	c.buf.Write(p[:room])
	c.dropped += len(p) - room
	return len(p), nil
}

func (c *cappedBuffer) String() string {
	return truncateNote(strings.TrimSpace(c.buf.String()), c.dropped)
}

func truncateNote(s string, dropped int) string {
	if dropped > 0 {
		return fmt.Sprintf("%s ... (%d more bytes)", s, dropped)
	}
	return s
}

func quoteOutput(b []byte) string {
	if len(b) <= MaxErrorTextBytes {
		return string(b)
	}
	return truncateNote(string(b[:MaxErrorTextBytes]), len(b)-MaxErrorTextBytes)
}

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
	var stderr cappedBuffer
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
		if stderrText := stderr.String(); stderrText != "" {
			return Result{}, fmt.Errorf("plugin %s failed: %w\nstderr: %s", path, err, stderrText)
		}
		return Result{}, fmt.Errorf("plugin %s failed: %w", path, err)
	}

	var result Result
	if err := json.Unmarshal(stdout, &result); err != nil {
		return Result{}, fmt.Errorf("parsing plugin output from %s: %w\nraw: %s", path, err, quoteOutput(stdout))
	}
	return result, nil
}
