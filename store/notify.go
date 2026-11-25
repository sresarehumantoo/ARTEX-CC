package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ChangeChannel is the NOTIFY channel that carries tool-driven changes
// between artex-cc processes sharing one database.
const ChangeChannel = "artex_changes"

// PostgreSQL rejects NOTIFY payloads of 8000 bytes or more.
const maxChangePayload = 7999

// Change describes a successful mutating tool call in some artex-cc process.
type Change struct {
	Tool          string `json:"tool"`
	TaskID        int64  `json:"task_id,omitempty"`
	ExplorationID int64  `json:"exploration_id"`
	NodeID        int64  `json:"node_id,omitempty"`
	Origin        string `json:"origin"`
}

// NewChangeOrigin returns a random identifier for this process, used to drop
// its own notifications when they echo back through LISTEN.
func NewChangeOrigin() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// NewChange builds a Change for a tool call, taking the intent node ID from
// the arguments of intent tools.
func NewChange(origin, tool string, taskID, explorationID int64, args json.RawMessage) Change {
	c := Change{Tool: tool, TaskID: taskID, ExplorationID: explorationID, Origin: origin}
	if strings.Contains(tool, "intent") {
		c.NodeID = argNodeID(args)
	}
	return c
}

func argNodeID(args json.RawMessage) int64 {
	var a struct {
		NodeID json.RawMessage `json:"node_id"`
	}
	if json.Unmarshal(args, &a) != nil || len(a.NodeID) == 0 {
		return 0
	}
	var n int64
	if json.Unmarshal(a.NodeID, &n) == nil {
		return n
	}
	var s string
	if json.Unmarshal(a.NodeID, &s) == nil {
		n, _ = strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	}
	return n
}

// EncodeChange serializes c as a NOTIFY payload.
func EncodeChange(c Change) (string, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	if len(b) > maxChangePayload {
		return "", fmt.Errorf("change payload is %d bytes, limit %d", len(b), maxChangePayload)
	}
	return string(b), nil
}

// DecodeChange parses a NOTIFY payload produced by EncodeChange.
func DecodeChange(payload string) (Change, error) {
	var c Change
	err := json.Unmarshal([]byte(payload), &c)
	return c, err
}

// Relevant reports whether a process with the given origin, bound to
// explorationID, should act on c.
func (c Change) Relevant(origin string, explorationID int64) bool {
	return c.Origin != origin && c.ExplorationID == explorationID
}

// PublishChange sends c to every process listening on ChangeChannel.
func (s *Store) PublishChange(ctx context.Context, c Change) error {
	payload, err := EncodeChange(c)
	if err != nil {
		return err
	}
	_, err = s.ExecContext(ctx, "SELECT pg_notify($1, $2)", ChangeChannel, payload)
	return err
}

// ListenChanges holds a dedicated connection listening on ChangeChannel and
// calls handle for each change from another process on the exploration that
// active returns. It reconnects with backoff until ctx is done.
func ListenChanges(ctx context.Context, dsn, origin string, active func() int64, handle func(Change)) {
	const maxBackoff = 30 * time.Second
	backoff := time.Second
	for {
		connected, err := listenChanges(ctx, dsn, origin, active, handle)
		if ctx.Err() != nil {
			return
		}
		if connected {
			backoff = time.Second
		}
		slog.Warn("change listener disconnected", "err", err, "retry_in", backoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, maxBackoff)
	}
}

func listenChanges(ctx context.Context, dsn, origin string, active func() int64, handle func(Change)) (bool, error) {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return false, err
	}
	defer conn.Close(context.Background())
	if _, err := conn.Exec(ctx, "LISTEN "+ChangeChannel); err != nil {
		return false, err
	}
	slog.Debug("change listener connected", "channel", ChangeChannel)
	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			return true, err
		}
		c, err := DecodeChange(n.Payload)
		if err != nil {
			slog.Debug("ignoring malformed change notification", "err", err)
			continue
		}
		if c.Relevant(origin, active()) {
			handle(c)
		}
	}
}
