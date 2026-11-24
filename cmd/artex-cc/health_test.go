package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestWriteHealthJSON_AllOK(t *testing.T) {
	r := healthResult{
		Status: "ok",
		Checks: map[string]hcEntry{
			"pg":          {Status: "ok", LatencyMS: 12},
			"schema":      {Status: "ok", Detail: "migrated (v3)"},
			"engagements": {Status: "ok", Detail: "5"},
		},
	}
	var buf bytes.Buffer
	writeHealthJSON(&buf, r)

	var got healthResult
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got.Status != "ok" {
		t.Errorf("status = %q, want ok", got.Status)
	}
	if got.Checks["pg"].Status != "ok" {
		t.Errorf("pg status = %q, want ok", got.Checks["pg"].Status)
	}
	if got.Checks["pg"].LatencyMS != 12 {
		t.Errorf("pg latency = %d, want 12", got.Checks["pg"].LatencyMS)
	}
	if got.Checks["schema"].Detail != "migrated (v3)" {
		t.Errorf("schema detail = %q, want %q", got.Checks["schema"].Detail, "migrated (v3)")
	}
	if got.Checks["engagements"].Detail != "5" {
		t.Errorf("engagements detail = %q, want %q", got.Checks["engagements"].Detail, "5")
	}
}

func TestWriteHealthJSON_Fail(t *testing.T) {
	r := healthResult{
		Status: "fail",
		Checks: map[string]hcEntry{
			"pg": {Status: "fail", Error: "connection refused"},
		},
	}
	var buf bytes.Buffer
	writeHealthJSON(&buf, r)

	var got healthResult
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got.Status != "fail" {
		t.Errorf("status = %q, want fail", got.Status)
	}
	if got.Checks["pg"].Error != "connection refused" {
		t.Errorf("pg error = %q, want %q", got.Checks["pg"].Error, "connection refused")
	}
}

func TestWriteHealthText_AllOK(t *testing.T) {
	r := healthResult{
		Status: "ok",
		Checks: map[string]hcEntry{
			"pg":          {Status: "ok", LatencyMS: 5},
			"schema":      {Status: "ok", Detail: "migrated (v2)"},
			"engagements": {Status: "ok", Detail: "10"},
		},
	}
	var buf bytes.Buffer
	writeHealthText(&buf, r)
	out := buf.String()

	want := "PostgreSQL: connected\nSchema:     migrated (v2)\nEngagements: 10\n"
	if out != want {
		t.Errorf("text output:\ngot:  %q\nwant: %q", out, want)
	}
}

func TestWriteHealthText_NotConfigured(t *testing.T) {
	r := healthResult{
		Status: "fail",
		Checks: map[string]hcEntry{
			"pg": {Status: "fail", Error: "not configured (set --pg or ARTEX_PG)"},
		},
	}
	var buf bytes.Buffer
	writeHealthText(&buf, r)
	out := buf.String()

	want := "PostgreSQL: NOT CONFIGURED\n  Set --pg or ARTEX_PG\n"
	if out != want {
		t.Errorf("text output:\ngot:  %q\nwant: %q", out, want)
	}
}

func TestWriteHealthText_PGFailed(t *testing.T) {
	r := healthResult{
		Status: "fail",
		Checks: map[string]hcEntry{
			"pg": {Status: "fail", Error: "dial tcp: connection refused"},
		},
	}
	var buf bytes.Buffer
	writeHealthText(&buf, r)
	out := buf.String()

	want := "PostgreSQL: FAILED (dial tcp: connection refused)\n"
	if out != want {
		t.Errorf("text output:\ngot:  %q\nwant: %q", out, want)
	}
}

func TestRunHealthChecks_NoDSN(t *testing.T) {
	os.Unsetenv("ARTEX_PG")
	r := runHealthChecks("", t.TempDir(), 5*time.Second)
	if r.Status != "fail" {
		t.Errorf("status = %q, want fail", r.Status)
	}
	if r.Checks["pg"].Status != "fail" {
		t.Errorf("pg status = %q, want fail", r.Checks["pg"].Status)
	}
}

func TestCheckTrafficDB_NotPresent(t *testing.T) {
	result := healthResult{
		Status: "ok",
		Checks: make(map[string]hcEntry),
	}
	checkTrafficDB(&result, t.TempDir())
	if result.Status != "ok" {
		t.Errorf("status = %q, want ok", result.Status)
	}
	e, ok := result.Checks["traffic_db"]
	if !ok {
		t.Fatal("expected traffic_db check")
	}
	if e.Detail != "not present" {
		t.Errorf("detail = %q, want 'not present'", e.Detail)
	}
}

func TestWriteHealthJSON_OmitsZeroLatency(t *testing.T) {
	r := healthResult{
		Status: "fail",
		Checks: map[string]hcEntry{
			"pg": {Status: "fail", Error: "connection refused"},
		},
	}
	var buf bytes.Buffer
	writeHealthJSON(&buf, r)

	var raw map[string]json.RawMessage
	json.Unmarshal(buf.Bytes(), &raw)
	var checks map[string]map[string]json.RawMessage
	json.Unmarshal(raw["checks"], &checks)

	if _, has := checks["pg"]["latency_ms"]; has {
		t.Error("latency_ms should be omitted when zero")
	}
}
