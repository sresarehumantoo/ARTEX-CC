package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

type targetChangeMockStore struct {
	mockStore
	gotInput    TargetChangeInput
	existing    []TargetChange
	gotIDs      []int64
	gotObserved string
	gotBy       string
	outcomes    []TargetRestoreOutcome
	live        []TargetChange
}

func (m *targetChangeMockStore) RecordTargetChange(_ context.Context, _ int64, in TargetChangeInput) (*TargetChangeRecord, error) {
	m.gotInput = in
	c := TargetChange{
		ID: 9, Target: in.Target, Resource: in.Resource, Kind: in.Kind,
		OriginalFingerprint: in.OriginalFingerprint, Baseline: in.OriginalFingerprint, BaselineChangeID: 9,
		RecordedBy: in.RecordedBy, RecordedAt: time.Now(),
	}
	if len(m.existing) > 0 {
		c.Baseline, c.BaselineChangeID = m.existing[0].Baseline, m.existing[0].BaselineChangeID
	}
	return &TargetChangeRecord{Change: c, Existing: m.existing}, nil
}

func (m *targetChangeMockStore) RecordTargetRestore(_ context.Context, _ int64, ids []int64, observed, by string) ([]TargetRestoreOutcome, error) {
	m.gotIDs, m.gotObserved, m.gotBy = ids, observed, by
	return m.outcomes, nil
}

func (m *targetChangeMockStore) ListTargetChanges(_ context.Context, _ int64, _ bool) ([]TargetChange, error) {
	return m.live, nil
}

func runTool(t *testing.T, tool Tool, ctx context.Context, in string) (Result, map[string]any) {
	t.Helper()
	r, err := tool.Run(ctx, json.RawMessage(in))
	if err != nil {
		t.Fatal(err)
	}
	if r.IsError {
		return r, nil
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(r.Content), &out); err != nil {
		t.Fatalf("decode %q: %v", r.Content, err)
	}
	return r, out
}

const scriptChange = `{"target":" WIN01 ","resource":"C:\\ProgramData\\WinforgeMaint\\rotate-logs.ps1","kind":"Modify","original_fingerprint":"sha256:aaa","backup":"C:\\Temp\\r.bak","intent_id":4}`

func TestRecordTargetChangeFirst(t *testing.T) {
	t.Parallel()
	m := &targetChangeMockStore{}
	ctx := WithSessionID(context.Background(), "sess-a")
	r, out := runTool(t, makeRecordTargetChange(m, 1), ctx, scriptChange)
	if r.IsError {
		t.Fatalf("error: %s", r.Content)
	}
	in := m.gotInput
	if in.Target != "WIN01" || in.Kind != "modify" || in.IntentID != 4 || in.Backup != `C:\Temp\r.bak` || in.RecordedBy != OwnerLabel("sess-a") {
		t.Errorf("input = %+v", in)
	}
	if out["baseline"] != "sha256:aaa" || out["change_id"] != float64(9) {
		t.Errorf("out = %v", out)
	}
	if _, ok := out["warning"]; ok {
		t.Errorf("unexpected warning: %v", out["warning"])
	}
}

func TestRecordTargetChangeWarnsOnLiveChange(t *testing.T) {
	t.Parallel()
	m := &targetChangeMockStore{existing: []TargetChange{{
		ID: 3, Target: "WIN01", Resource: "rotate-logs.ps1", Kind: "modify",
		OriginalFingerprint: "sha256:orig", Baseline: "sha256:orig", BaselineChangeID: 3,
		RecordedBy: "h-other", Backup: `C:\Temp\first.bak`, RecordedAt: time.Now(),
	}}}
	ctx := WithSessionID(context.Background(), "sess-b")
	_, out := runTool(t, makeRecordTargetChange(m, 1), ctx,
		`{"target":"WIN01","resource":"rotate-logs.ps1","kind":"modify","original_fingerprint":"sha256:payload"}`)
	warning, _ := out["warning"].(string)
	for _, want := range []string{"RESOURCE ALREADY MODIFIED", "#3 by h-other", "sha256:orig", "do not use your own backup"} {
		if !strings.Contains(warning, want) {
			t.Errorf("warning missing %q: %s", want, warning)
		}
	}
	if out["baseline"] != "sha256:orig" || out["original_matches_baseline"] != false {
		t.Errorf("out = %v", out)
	}
	existing, _ := out["existing_changes"].([]any)
	if len(existing) != 1 {
		t.Fatalf("existing_changes = %v", out["existing_changes"])
	}
	e := existing[0].(map[string]any)
	if e["original_fingerprint"] != "sha256:orig" || e["recorded_by"] != "h-other" || e["backup"] != `C:\Temp\first.bak` || e["yours"] != nil {
		t.Errorf("existing = %v", e)
	}
}

func TestRecordTargetChangeMarksOwnLiveChange(t *testing.T) {
	t.Parallel()
	ctx := WithSessionID(context.Background(), "sess-c")
	m := &targetChangeMockStore{existing: []TargetChange{{
		ID: 5, Baseline: "sha256:orig", BaselineChangeID: 5, RecordedBy: OwnerLabel("sess-c"),
	}}}
	_, out := runTool(t, makeRecordTargetChange(m, 1), ctx,
		`{"target":"h","resource":"r","kind":"modify","original_fingerprint":"sha256:orig"}`)
	e := out["existing_changes"].([]any)[0].(map[string]any)
	if e["yours"] != true {
		t.Errorf("existing = %v", e)
	}
	if !strings.Contains(out["warning"].(string), "(you)") || out["original_matches_baseline"] != true {
		t.Errorf("out = %v", out)
	}
}

func TestRecordTargetChangeWithoutHolderStillRecords(t *testing.T) {
	t.Parallel()
	m := &targetChangeMockStore{}
	_, out := runTool(t, makeRecordTargetChange(m, 1), context.Background(),
		`{"target":"h","resource":"r","kind":"create","original_fingerprint":"absent"}`)
	if m.gotInput.RecordedBy != "unidentified" || out["identity_error"] == nil {
		t.Errorf("recorded_by = %q, out = %v", m.gotInput.RecordedBy, out)
	}
}

func TestRecordTargetChangeValidation(t *testing.T) {
	t.Parallel()
	tool := makeRecordTargetChange(&targetChangeMockStore{}, 1)
	ctx := WithSessionID(context.Background(), "s")
	for in, want := range map[string]string{
		`{"resource":"r","kind":"modify","original_fingerprint":"x"}`:              "target is required",
		`{"target":"h","kind":"modify","original_fingerprint":"x"}`:                "resource is required",
		`{"target":"h","resource":"r","kind":"modify","original_fingerprint":""}`:  "original_fingerprint is required",
		`{"target":"h","resource":"r","kind":"tamper","original_fingerprint":"x"}`: "kind must be one of",
	} {
		r, _ := runTool(t, tool, ctx, in)
		if !r.IsError || !strings.Contains(r.Content, want) {
			t.Errorf("%s: got %v %q, want error %q", in, r.IsError, r.Content, want)
		}
	}
}

func TestRecordTargetRestore(t *testing.T) {
	t.Parallel()
	m := &targetChangeMockStore{
		outcomes: []TargetRestoreOutcome{{ChangeID: 3, Status: RestoreStatusNotRestored, Observed: "sha256:payload", Baseline: "sha256:orig"}},
		live:     []TargetChange{{ID: 3}, {ID: 4}},
	}
	ctx := WithSessionID(context.Background(), "sess-a")
	r, out := runTool(t, makeRecordTargetRestore(m, 1), ctx, `{"change_ids":[3],"observed_fingerprint":" sha256:payload "}`)
	if r.IsError {
		t.Fatal(r.Content)
	}
	if len(m.gotIDs) != 1 || m.gotIDs[0] != 3 || m.gotObserved != "sha256:payload" || m.gotBy != OwnerLabel("sess-a") {
		t.Errorf("store got ids=%v observed=%q by=%q", m.gotIDs, m.gotObserved, m.gotBy)
	}
	if out["all_restored"] != false || out["unrestored_target_changes"] != float64(2) {
		t.Errorf("out = %v", out)
	}

	m.outcomes = []TargetRestoreOutcome{{ChangeID: 3, Status: RestoreStatusRestored}}
	m.live = nil
	_, out = runTool(t, makeRecordTargetRestore(m, 1), ctx, `{"change_ids":[3],"observed_fingerprint":"sha256:orig"}`)
	if out["all_restored"] != true || out["unrestored_target_changes"] != float64(0) {
		t.Errorf("out = %v", out)
	}

	for _, in := range []string{`{"observed_fingerprint":"x"}`, `{"change_ids":[3]}`} {
		if r, _ := runTool(t, makeRecordTargetRestore(m, 1), ctx, in); !r.IsError {
			t.Errorf("%s: want error", in)
		}
	}
}

func TestListChangesShowsUnrestoredTargetChanges(t *testing.T) {
	t.Parallel()
	m := &targetChangeMockStore{live: []TargetChange{{ID: 3, Target: "WIN01", Resource: "r", Baseline: "sha256:orig", RecordedBy: "h-x"}}}
	ctx := WithSessionID(context.Background(), "s")
	_, out := runTool(t, makeListChanges(m, 7, 1), ctx, `{"since":"2026-01-01T00:00:00Z"}`)
	list, _ := out["unrestored_target_changes"].([]any)
	if len(list) != 1 || list[0].(map[string]any)["baseline"] != "sha256:orig" {
		t.Errorf("unrestored_target_changes = %v", out["unrestored_target_changes"])
	}

	m.live = nil
	_, out = runTool(t, makeListChanges(m, 7, 1), ctx, `{"since":"2026-01-01T00:00:00Z"}`)
	if _, ok := out["unrestored_target_changes"]; ok {
		t.Errorf("unexpected unrestored_target_changes: %v", out)
	}
}
