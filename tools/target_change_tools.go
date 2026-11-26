package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
)

var TargetChangeKinds = []string{"modify", "create", "delete", "other"}

// TargetChangeInput is one change an agent is about to make on a target.
type TargetChangeInput struct {
	Target              string
	Resource            string
	Kind                string
	OriginalFingerprint string
	Backup              string
	Note                string
	IntentID            int64
	RecordedBy          string
}

// TargetChange is a recorded change to a target. Baseline is the original
// fingerprint of the first change recorded on the same target and resource
// while earlier ones were still unrestored, so it is the true pre-engagement
// state even when later recorders saw an already modified resource.
type TargetChange struct {
	ID                  int64      `json:"id"`
	Target              string     `json:"target"`
	Resource            string     `json:"resource"`
	Kind                string     `json:"kind"`
	IntentID            int64      `json:"intent_id,omitempty"`
	OriginalFingerprint string     `json:"original_fingerprint"`
	Baseline            string     `json:"baseline"`
	BaselineChangeID    int64      `json:"baseline_change_id"`
	Backup              string     `json:"backup,omitempty"`
	Note                string     `json:"note,omitempty"`
	RecordedBy          string     `json:"recorded_by"`
	RecordedAt          time.Time  `json:"recorded_at"`
	RestoredAt          *time.Time `json:"restored_at,omitempty"`
	RestoredBy          string     `json:"restored_by,omitempty"`
	LastObserved        string     `json:"last_observed,omitempty"`
	RestoreAttempts     int        `json:"restore_attempts,omitempty"`
}

// Summary is the compact form shown in graph_overview and list_changes.
func (c TargetChange) Summary() map[string]any {
	m := map[string]any{
		"id":                 c.ID,
		"target":             c.Target,
		"resource":           c.Resource,
		"kind":               c.Kind,
		"baseline":           c.Baseline,
		"baseline_change_id": c.BaselineChangeID,
		"recorded_by":        c.RecordedBy,
		"recorded_at":        c.RecordedAt.UTC().Format(time.RFC3339),
	}
	if c.IntentID != 0 {
		m["intent_id"] = c.IntentID
	}
	if c.RestoreAttempts > 0 {
		m["last_observed"] = c.LastObserved
		m["restore_attempts"] = c.RestoreAttempts
	}
	return m
}

// TargetChangeRecord is the result of RecordTargetChange. Existing holds the
// changes on the same target and resource that were unrestored when this one
// was recorded.
type TargetChangeRecord struct {
	Change   TargetChange
	Existing []TargetChange
}

const (
	RestoreStatusRestored        = "restored"
	RestoreStatusNotRestored     = "not_restored"
	RestoreStatusAlreadyRestored = "already_restored"
)

// TargetRestoreOutcome is what RecordTargetRestore decided for one change.
type TargetRestoreOutcome struct {
	ChangeID         int64   `json:"change_id"`
	Target           string  `json:"target"`
	Resource         string  `json:"resource"`
	Status           string  `json:"status"`
	Observed         string  `json:"observed"`
	Baseline         string  `json:"baseline"`
	BaselineChangeID int64   `json:"baseline_change_id"`
	AlsoRestored     []int64 `json:"also_restored,omitempty"`
	Message          string  `json:"message"`
}

// FingerprintsMatch compares fingerprints ignoring surrounding whitespace and
// case, so hex digests in either case match.
func FingerprintsMatch(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func RegisterTargetChangeTools(reg *Registry, store Store, explorationID int64) {
	reg.Register(makeRecordTargetChange(store, explorationID))
	reg.Register(makeRecordTargetRestore(store, explorationID))
}

func changeRecorder(ctx context.Context, owner string) (label, identityErr string) {
	o, err := ClaimOwner(ctx, owner)
	if err != nil {
		return "unidentified", err.Error()
	}
	return OwnerLabel(o), ""
}

func makeRecordTargetChange(store Store, explorationID int64) Tool {
	return Tool{
		Name:     "record_target_change",
		Category: CategoryExploration,
		Description: "Record a change to a target BEFORE you make it: a file, registry key, service, scheduled task, account or config you are about to modify, create or delete on a host. " +
			"Pass original_fingerprint, evidence of the resource's current state (e.g. sha256:<hex> of the file, or absent when you are creating it), and where you keep a backup. " +
			"If another unrestored change on the same target and resource exists, the result carries warning and existing_changes: the state you observed is NOT the original. " +
			"Targets match ignoring case; resources match ignoring case only when they are Windows paths or registry keys, so POSIX paths differing in case are different resources. " +
			"Do not take your own backup as the baseline; restore to the returned baseline (the first recorded original fingerprint) and coordinate with the holders in recorded_by. " +
			"After undoing the change, call record_target_restore with the fingerprint you observe.",
		Schema: obj(map[string]any{
			"target":               str("host the change is on, written the same way every time (asset label, hostname or IP)"),
			"resource":             str("what changes, e.g. a file path, registry key, service name, account or config setting"),
			"kind":                 enm(TargetChangeKinds, "kind of change"),
			"original_fingerprint": str("evidence of the original state, e.g. sha256:<hex> of the file, absent for a resource you create, or a value you read"),
			"backup":               str("where you keep a copy of the original, or the original content when it is short"),
			"note":                 str("what you are changing and why"),
			"intent_id":            intp("intent this change is made for"),
			"owner":                str(ownerArgDesc),
		}, "target", "resource", "kind", "original_fingerprint"),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				Target              string          `json:"target"`
				Resource            string          `json:"resource"`
				Kind                string          `json:"kind"`
				OriginalFingerprint string          `json:"original_fingerprint"`
				Backup              string          `json:"backup"`
				Note                string          `json:"note"`
				IntentID            json.RawMessage `json:"intent_id"`
				Owner               string          `json:"owner"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			for _, f := range []struct{ name, v string }{
				{"target", a.Target}, {"resource", a.Resource}, {"original_fingerprint", a.OriginalFingerprint},
			} {
				if strings.TrimSpace(f.v) == "" {
					return errResult(f.name + " is required"), nil
				}
			}
			kind := strings.ToLower(strings.TrimSpace(a.Kind))
			if !slices.Contains(TargetChangeKinds, kind) {
				return errResult(fmt.Sprintf("kind must be one of %s", strings.Join(TargetChangeKinds, ", "))), nil
			}
			label, identityErr := changeRecorder(ctx, a.Owner)

			rec, err := store.RecordTargetChange(ctx, explorationID, TargetChangeInput{
				Target:              strings.TrimSpace(a.Target),
				Resource:            strings.TrimSpace(a.Resource),
				Kind:                kind,
				OriginalFingerprint: strings.TrimSpace(a.OriginalFingerprint),
				Backup:              a.Backup,
				Note:                a.Note,
				IntentID:            pid(a.IntentID),
				RecordedBy:          label,
			})
			if err != nil {
				return errResult(err.Error()), nil
			}

			c := rec.Change
			out := map[string]any{
				"change_id":          c.ID,
				"target":             c.Target,
				"resource":           c.Resource,
				"kind":               c.Kind,
				"baseline":           c.Baseline,
				"baseline_change_id": c.BaselineChangeID,
				"recorded_by":        c.RecordedBy,
			}
			if identityErr != "" {
				out["identity_error"] = identityErr
			}
			if len(rec.Existing) > 0 {
				existing := make([]map[string]any, 0, len(rec.Existing))
				var others []string
				for _, e := range rec.Existing {
					m := e.Summary()
					m["original_fingerprint"] = e.OriginalFingerprint
					if e.Backup != "" {
						m["backup"] = e.Backup
					}
					if e.RecordedBy == label && label != "unidentified" {
						m["yours"] = true
					} else {
						others = append(others, fmt.Sprintf("#%d by %s", e.ID, e.RecordedBy))
					}
					existing = append(existing, m)
				}
				out["existing_changes"] = existing
				out["original_matches_baseline"] = FingerprintsMatch(c.OriginalFingerprint, c.Baseline)
				who := "you"
				if len(others) > 0 {
					who = strings.Join(others, ", ")
				}
				out["warning"] = fmt.Sprintf("RESOURCE ALREADY MODIFIED: %d unrestored change(s) on %s %s (%s). "+
					"The state you observed is not the original. The baseline is %s, first recorded in change #%d; "+
					"do not use your own backup as the baseline. Coordinate with the other holders and restore to the baseline, "+
					"then call record_target_restore with the fingerprint you observe.",
					len(rec.Existing), c.Target, c.Resource, who, c.Baseline, c.BaselineChangeID)
			}
			return jsonResult(out)
		},
	}
}

func makeRecordTargetRestore(store Store, explorationID int64) Tool {
	return Tool{
		Name:     "record_target_restore",
		Category: CategoryExploration,
		Description: "Record that you restored changed resources on a target. Pass the change ids from record_target_change and the fingerprint you observe now, computed the same way as the original. " +
			"The server compares it with the baseline, the first recorded original fingerprint for that target and resource. " +
			"Only an exact match marks the change restored, together with every other unrestored change on the same resource; " +
			"a mismatch is recorded as a failed attempt and the change stays unrestored. Never report a host restored while graph_overview or list_changes lists unrestored_target_changes.",
		Schema: obj(map[string]any{
			"change_ids":           arr(map[string]any{"type": "integer"}, "change ids from record_target_change"),
			"observed_fingerprint": str("fingerprint of the resource as it is now, after restoring, e.g. sha256:<hex> or absent"),
			"owner":                str(ownerArgDesc),
		}, "change_ids", "observed_fingerprint"),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				ChangeIDs           []json.RawMessage `json:"change_ids"`
				ObservedFingerprint string            `json:"observed_fingerprint"`
				Owner               string            `json:"owner"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			ids := pidList(a.ChangeIDs)
			if len(ids) == 0 {
				return errResult("change_ids is required"), nil
			}
			if strings.TrimSpace(a.ObservedFingerprint) == "" {
				return errResult("observed_fingerprint is required"), nil
			}
			label, identityErr := changeRecorder(ctx, a.Owner)

			outcomes, err := store.RecordTargetRestore(ctx, explorationID, ids, strings.TrimSpace(a.ObservedFingerprint), label)
			if err != nil {
				return errResult(err.Error()), nil
			}
			live, err := store.ListTargetChanges(ctx, explorationID, true)
			if err != nil {
				return errResult(err.Error()), nil
			}
			allRestored := true
			for _, o := range outcomes {
				if o.Status == RestoreStatusNotRestored {
					allRestored = false
				}
			}
			out := map[string]any{
				"results":                   outcomes,
				"all_restored":              allRestored,
				"unrestored_target_changes": len(live),
			}
			if identityErr != "" {
				out["identity_error"] = identityErr
			}
			return jsonResult(out)
		},
	}
}

// UnrestoredSummaries is the compact list shown in graph_overview and list_changes.
func UnrestoredSummaries(changes []TargetChange) []map[string]any {
	out := make([]map[string]any, 0, len(changes))
	for _, c := range changes {
		out = append(out, c.Summary())
	}
	return out
}
