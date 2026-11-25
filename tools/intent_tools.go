package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func RegisterIntentTools(reg *Registry, store Store, explorationID int64) {
	reg.Register(makeStartIntent(store))
	reg.Register(makeCompleteIntent(store))
	reg.Register(makeCancelIntent(store))
}

func makeStartIntent(store Store) Tool {
	return Tool{
		Name:     "start_intent",
		Category: CategoryExploration,
		Description: "Claim an open intent for this session and mark it running. Call this when you begin working on an intent. " +
			fmt.Sprintf("The claim is exclusive for %d minutes; call again to renew it on long work. ", int(IntentLease.Minutes())) +
			"Fails if another session holds a live claim, so pick a different intent. " +
			"Pass the same owner on every claim call if you use one.",
		Schema: obj(map[string]any{
			"node_id": intp("intent node id"),
			"owner":   str(ownerArgDesc),
		}, "node_id"),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				NodeID json.RawMessage `json:"node_id"`
				Owner  string          `json:"owner"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			id := pid(a.NodeID)
			if id <= 0 {
				return errResult("node_id is required"), nil
			}

			node, err := store.NodeDetail(ctx, id)
			if err != nil {
				return errResult(err.Error()), nil
			}
			if node == nil {
				return errResult(fmt.Sprintf("node %d not found", id)), nil
			}
			if node.Kind != KindIntent {
				return errResult(fmt.Sprintf("node %d is a %s, not an intent", id, node.Kind)), nil
			}
			if node.State != "open" && node.State != "running" {
				return errResult(fmt.Sprintf("intent %d is %q, must be open to start", id, node.State)), nil
			}
			owner, err := ClaimOwner(ctx, a.Owner)
			if err != nil {
				return errResult(err.Error()), nil
			}

			claim, err := store.ClaimIntent(ctx, id, owner, IntentLease)
			if err != nil {
				return errResult(err.Error()), nil
			}
			verb := "claimed"
			if claim.Renewed {
				verb = "renewed"
			}
			return textResult(fmt.Sprintf("intent %d is now running; %s by %s until %s",
				id, verb, OwnerLabel(claim.Owner), claim.ExpiresAt.UTC().Format(time.RFC3339))), nil
		},
	}
}

func makeCompleteIntent(store Store) Tool {
	return Tool{
		Name:     "complete_intent",
		Category: CategoryExploration,
		Description: "Mark a running intent as done with a result summary. " +
			"Call this when you have finished working on an intent and recorded all facts/findings. " +
			"Refused while another session holds a live claim on the intent.",
		Schema: obj(map[string]any{
			"node_id": intp("intent node id"),
			"result":  str("brief result summary"),
			"owner":   str(ownerArgDesc),
		}, "node_id"),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				NodeID json.RawMessage `json:"node_id"`
				Result string          `json:"result"`
				Owner  string          `json:"owner"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			id := pid(a.NodeID)
			if id <= 0 {
				return errResult("node_id is required"), nil
			}

			node, err := store.NodeDetail(ctx, id)
			if err != nil {
				return errResult(err.Error()), nil
			}
			if node == nil {
				return errResult(fmt.Sprintf("node %d not found", id)), nil
			}
			if node.Kind != KindIntent {
				return errResult(fmt.Sprintf("node %d is a %s, not an intent", id, node.Kind)), nil
			}
			if node.State != "running" && node.State != "open" {
				return errResult(fmt.Sprintf("intent %d is %q, must be running or open to complete", id, node.State)), nil
			}
			if res, ok := claimRunning(ctx, store, node, a.Owner); !ok {
				return res, nil
			}

			var merge map[string]any
			if result := strings.TrimSpace(a.Result); result != "" {
				merge = map[string]any{"result": result}
			}

			if err := store.UpdateNodeFields(ctx, id, "done", merge); err != nil {
				return errResult(err.Error()), nil
			}
			return textResult(fmt.Sprintf("intent %d marked done", id)), nil
		},
	}
}

func makeCancelIntent(store Store) Tool {
	return Tool{
		Name:     "cancel_intent",
		Category: CategoryExploration,
		Description: "Cancel an intent that is no longer needed. " +
			"Use this when an intent is redundant, no longer applicable, or was created in error. " +
			"Refused while another session holds a live claim on the intent.",
		Schema: obj(map[string]any{
			"node_id": intp("intent node id"),
			"reason":  str("why this intent is being cancelled"),
			"owner":   str(ownerArgDesc),
		}, "node_id"),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				NodeID json.RawMessage `json:"node_id"`
				Reason string          `json:"reason"`
				Owner  string          `json:"owner"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			id := pid(a.NodeID)
			if id <= 0 {
				return errResult("node_id is required"), nil
			}

			node, err := store.NodeDetail(ctx, id)
			if err != nil {
				return errResult(err.Error()), nil
			}
			if node == nil {
				return errResult(fmt.Sprintf("node %d not found", id)), nil
			}
			if node.Kind != KindIntent {
				return errResult(fmt.Sprintf("node %d is a %s, not an intent", id, node.Kind)), nil
			}
			terminalStates := map[string]bool{"done": true, "stopped": true, "deleted": true}
			if terminalStates[node.State] {
				return errResult(fmt.Sprintf("intent %d is already %q", id, node.State)), nil
			}
			if res, ok := claimRunning(ctx, store, node, a.Owner); !ok {
				return res, nil
			}

			var merge map[string]any
			if reason := strings.TrimSpace(a.Reason); reason != "" {
				merge = map[string]any{"cancel_reason": reason}
			}

			if err := store.UpdateNodeFields(ctx, id, "stopped", merge); err != nil {
				return errResult(err.Error()), nil
			}
			return textResult(fmt.Sprintf("intent %d cancelled", id)), nil
		},
	}
}

// claimRunning takes the claim on a running intent before it is closed, which
// makes the ownership check atomic: it fails only while another holder has a
// live lease. Open intents have no holder to check.
func claimRunning(ctx context.Context, store Store, node *Node, owner string) (Result, bool) {
	if node.State != "running" {
		return Result{}, true
	}
	holder, err := ClaimOwner(ctx, owner)
	if err != nil {
		return errResult(err.Error()), false
	}
	if _, err := store.ClaimIntent(ctx, node.ID, holder, IntentLease); err != nil {
		return errResult(err.Error()), false
	}
	return Result{}, true
}
