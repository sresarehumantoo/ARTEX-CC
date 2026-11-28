package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func RegisterIntentTools(reg *Registry, store Store, explorationID int64) {
	reg.Register(makeStartIntent(store, explorationID))
	reg.Register(makeCompleteIntent(store))
	reg.Register(makeCancelIntent(store))
}

func makeStartIntent(store Store, explorationID int64) Tool {
	minuteUnit := "minutes"
	if IntentLease == time.Minute {
		minuteUnit = "minute"
	}
	return Tool{
		Name:     "start_intent",
		Category: CategoryExploration,
		Description: "Claim an open intent for this session and mark it running. Call this when you begin working on an intent. " +
			fmt.Sprintf("The claim is exclusive for %d %s and renews whenever you pass this intent as intent_id to record_fact, report_finding or record_coverage, with the same owner if you claimed with one. ", int(IntentLease.Minutes()), minuteUnit) +
			"If you go that long without recording anything for it, call start_intent again to renew it, or the intent reopens for other sessions. " +
			"Fails if another session holds a live claim, so pick a different intent. " +
			"When you take over an intent whose holder's claim lapsed, the result lists the unrestored target changes you inherit with their baselines: restoring them is now your job. " +
			"Pass adopt_changes with ids from unowned_target_changes to make this claim own their restoration too. " +
			"Pass the same owner on every claim call if you use one." + identityRequiredDesc(),
		Schema: obj(map[string]any{
			"node_id":       intp("intent node id"),
			"owner":         str(ownerArgDesc),
			"adopt_changes": arr(map[string]any{"type": "integer"}, "ids of unrestored target changes with no live owner to move under this intent"),
		}, "node_id"),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				NodeID       json.RawMessage   `json:"node_id"`
				Owner        string            `json:"owner"`
				AdoptChanges []json.RawMessage `json:"adopt_changes"`
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
			msg := fmt.Sprintf("intent %d is now running; %s by %s until %s",
				id, verb, OwnerLabel(claim.Owner), claim.ExpiresAt.UTC().Format(time.RFC3339))
			extra := map[string]any{}
			if claim.TookOver {
				inheritChanges(ctx, store, explorationID, id, claim.PreviousOwner, extra)
			}
			if ids := pidList(a.AdoptChanges); len(ids) > 0 {
				adoptChanges(ctx, store, explorationID, id, owner, ids, extra)
			}
			if len(extra) == 0 {
				return textResult(msg), nil
			}
			b, err := json.Marshal(extra)
			if err != nil {
				return errResult(err.Error()), nil
			}
			return textResult(msg + "\n" + string(b)), nil
		},
	}
}

// inheritChanges lists the unrestored changes a takeover hands to the new
// holder. The claim is already taken, so a failure here is reported, not
// returned.
func inheritChanges(ctx context.Context, store Store, explorationID, intentID int64, previous string, out map[string]any) {
	live, err := store.ListTargetChanges(ctx, explorationID, true)
	if err != nil {
		out["handoff_error"] = err.Error()
		return
	}
	var inherited []map[string]any
	for _, c := range live {
		if c.IntentID == intentID {
			inherited = append(inherited, c.Handoff())
		}
	}
	if len(inherited) == 0 {
		return
	}
	out["inherited_target_changes"] = inherited
	out["handoff"] = fmt.Sprintf("You took this intent over from %s, whose claim lapsed with %d unrestored target change(s). "+
		"Restoring them is now yours: put each resource back to its baseline (not a later change's original_fingerprint), "+
		"using the backup where one was kept, then call record_target_restore. last_observed and restore_attempts show earlier failed attempts.",
		OwnerLabel(previous), len(inherited))
}

func adoptChanges(ctx context.Context, store Store, explorationID, intentID int64, holder string, ids []int64, out map[string]any) {
	res, err := store.AdoptTargetChanges(ctx, explorationID, intentID, holder, ids)
	if err != nil {
		out["adopt_error"] = err.Error()
		return
	}
	adopted := make([]map[string]any, 0, len(res.Adopted))
	for _, c := range res.Adopted {
		adopted = append(adopted, c.Handoff())
	}
	out["adopted_changes"] = adopted
	if len(res.Refused) > 0 {
		refused := map[string]string{}
		for id, why := range res.Refused {
			refused[strconv.FormatInt(id, 10)] = why
		}
		out["not_adopted"] = refused
	}
}

func makeCompleteIntent(store Store) Tool {
	return Tool{
		Name:     "complete_intent",
		Category: CategoryExploration,
		Description: "Mark a running intent as done with a result summary. " +
			"Call this when you have finished working on an intent and recorded all facts/findings. " +
			"Refused while another session holds a live claim on the intent. " +
			"An open intent you never started is claimed and completed in one step, but start_intent it before working on it so other agents skip it.",
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
			neverStarted := node.State == "open"
			holder, res, ok := claimToClose(ctx, store, node, a.Owner)
			if !ok {
				return res, nil
			}

			merge := map[string]any{"completed_by": OwnerLabel(holder)}
			if result := strings.TrimSpace(a.Result); result != "" {
				merge["result"] = result
			}

			if err := store.UpdateNodeFields(ctx, id, "done", merge); err != nil {
				return errResult(err.Error()), nil
			}
			if neverStarted {
				return textResult(fmt.Sprintf("intent %d marked done; it was never started, so it was claimed and completed in one step. "+
					"Call start_intent before working an intent so other agents skip it.", id)), nil
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
			holder, res, ok := claimToClose(ctx, store, node, a.Owner)
			if !ok {
				return res, nil
			}

			merge := map[string]any{"cancelled_by": OwnerLabel(holder)}
			if reason := strings.TrimSpace(a.Reason); reason != "" {
				merge["cancel_reason"] = reason
			}

			if err := store.UpdateNodeFields(ctx, id, "stopped", merge); err != nil {
				return errResult(err.Error()), nil
			}
			return textResult(fmt.Sprintf("intent %d cancelled", id)), nil
		},
	}
}

func identityRequiredDesc() string {
	if !RequireIdentity {
		return ""
	}
	return " This server requires a verified X-Artex-Identity header for claims; without one, start_intent, complete_intent and cancel_intent are refused."
}

// claimToClose takes the claim on an open or running intent before it is
// closed, so the caller is its holder when it closes and the ownership check
// is atomic: it fails only while another holder has a live lease, including
// one taken on an open intent since the caller last looked.
func claimToClose(ctx context.Context, store Store, node *Node, owner string) (string, Result, bool) {
	holder, err := ClaimOwner(ctx, owner)
	if err != nil {
		return "", errResult(err.Error()), false
	}
	if node.State != "open" && node.State != "running" {
		return holder, Result{}, true
	}
	if _, err := store.ClaimIntent(ctx, node.ID, holder, IntentLease); err != nil {
		return "", errResult(err.Error()), false
	}
	return holder, Result{}, true
}
