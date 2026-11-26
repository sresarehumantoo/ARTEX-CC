package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	defaultChangesLimit = 100
	maxChangesLimit     = 500
)

// ChangeCursor is a keyset position in the engagement's change stream,
// ordered by (At, Kind, ID). The tie-breakers matter because a bulk insert
// stamps every row with the same transaction time.
type ChangeCursor struct {
	At   time.Time
	Kind string
	ID   int64
}

func (c ChangeCursor) String() string {
	return fmt.Sprintf("c%d.%s.%d", c.At.UnixMicro(), c.Kind, c.ID)
}

// ParseChangeCursor accepts a cursor from String or an RFC 3339 timestamp.
func ParseChangeCursor(s string) (ChangeCursor, error) {
	s = strings.TrimSpace(s)
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return ChangeCursor{At: t}, nil
	}
	parts := strings.Split(strings.TrimPrefix(s, "c"), ".")
	if !strings.HasPrefix(s, "c") || len(parts) != 3 {
		return ChangeCursor{}, fmt.Errorf("expected a cursor from list_changes or graph_overview, or an RFC 3339 timestamp: %q", s)
	}
	micros, err1 := strconv.ParseInt(parts[0], 10, 64)
	id, err2 := strconv.ParseInt(parts[2], 10, 64)
	if err1 != nil || err2 != nil {
		return ChangeCursor{}, fmt.Errorf("malformed cursor %q", s)
	}
	return ChangeCursor{At: time.UnixMicro(micros), Kind: parts[1], ID: id}, nil
}

// Change is one created or updated row. Kind is finding, fact, intent, goal,
// hint or asset; Fields is what the agent sees.
type Change struct {
	Kind   string
	Cursor ChangeCursor
	Fields map[string]any
}

// HeldClaim is a running intent owned by the calling session.
type HeldClaim struct {
	NodeID    int64          `json:"node_id"`
	ExpiresAt time.Time      `json:"claim_expires_at"`
	Expired   bool           `json:"expired,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
}

var changeGroups = map[string]string{
	"finding": "findings",
	"fact":    "facts",
	"intent":  "intents",
	"goal":    "goals",
	"hint":    "hints",
	"asset":   "assets",
}

func RegisterChangeTools(reg *Registry, store Store, taskID, explorationID int64) {
	reg.Register(makeListChanges(store, taskID, explorationID))
}

func makeListChanges(store Store, taskID, explorationID int64) Tool {
	return Tool{
		Name:     "list_changes",
		Category: CategoryExploration,
		Description: "What changed since you last looked: new or updated findings, facts, intents (with state and claim holder), goals, hints and assets, oldest first. " +
			"Pass since as the changes_cursor from graph_overview, the cursor from your previous list_changes call, or an RFC 3339 timestamp. " +
			"Also returns unrestored_target_changes, changes to targets that nobody has restored yet (see record_target_change), and session, your own holder label as it appears in claimed_by, and my_claims, the intents you hold with their expiry; renew one by calling start_intent again. " +
			"Pass the same owner you pass to start_intent, if any; without a session or owner, session and my_claims are replaced by claims_error. " +
			"When several agents share an engagement, poll this instead of re-reading graph_overview.",
		ReadOnly: true,
		Schema: obj(map[string]any{
			"since": str("cursor from graph_overview (changes_cursor) or a previous list_changes call, or an RFC 3339 timestamp"),
			"limit": intp(fmt.Sprintf("max changes to return, default %d, max %d", defaultChangesLimit, maxChangesLimit)),
			"owner": str(ownerArgDesc),
		}, "since"),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				Since string `json:"since"`
				Limit int    `json:"limit"`
				Owner string `json:"owner"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			if strings.TrimSpace(a.Since) == "" {
				return errResult("since is required: pass changes_cursor from graph_overview or the cursor from your last list_changes call"), nil
			}
			after, err := ParseChangeCursor(a.Since)
			if err != nil {
				return errResult("invalid since: " + err.Error()), nil
			}
			limit := a.Limit
			if limit <= 0 {
				limit = defaultChangesLimit
			}
			limit = min(limit, maxChangesLimit)

			changes, err := store.ListChanges(ctx, explorationID, taskID, after, limit+1)
			if err != nil {
				return errResult(err.Error()), nil
			}
			truncated := len(changes) > limit
			if truncated {
				changes = changes[:limit]
			}

			unrestored, err := store.ListTargetChanges(ctx, explorationID, true)
			if err != nil {
				return errResult(err.Error()), nil
			}

			var claims []HeldClaim
			owner, ownerErr := ClaimOwner(ctx, a.Owner)
			if ownerErr == nil {
				if claims, err = store.ListClaims(ctx, explorationID, owner); err != nil {
					return errResult(err.Error()), nil
				}
			}

			cursor := after
			grouped := map[string][]map[string]any{}
			for _, c := range changes {
				grouped[changeGroups[c.Kind]] = append(grouped[changeGroups[c.Kind]], c.Fields)
				cursor = c.Cursor
			}

			out := map[string]any{
				"cursor":    cursor.String(),
				"count":     len(changes),
				"truncated": truncated,
			}
			if ownerErr != nil {
				out["claims_error"] = ownerErr.Error()
			} else {
				out["session"] = OwnerLabel(owner)
				out["my_claims"] = claims
			}
			if len(unrestored) > 0 {
				out["unrestored_target_changes"] = UnrestoredSummaries(unrestored)
			}
			for k, v := range grouped {
				out[k] = v
			}
			if truncated {
				out["note"] = "more changes pending; call list_changes again with this cursor"
			}
			return jsonResult(out)
		},
	}
}
