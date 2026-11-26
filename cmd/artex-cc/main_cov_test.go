package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/store"
	_ "modernc.org/sqlite"
)

func testPGDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("ARTEX_TEST_PG")
	if dsn == "" {
		dsn = "dbname=artex_covtest sslmode=disable"
		db, err := sql.Open("pgx", dsn)
		if err != nil {
			t.Skip("no test PG available")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			db.Close()
			t.Skip("no test PG available: " + err.Error())
		}
		db.Close()
	}
	return dsn
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

// envEnabled

func TestEnvEnabled(t *testing.T) {
	for _, tt := range []struct {
		name  string
		value string
		want  bool
	}{
		{"true", "true", true},
		{"1", "1", true},
		{"false", "false", false},
		{"0", "0", false},
		{"empty", "", false},
		{"invalid", "nope", false},
		{"unset", "", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TEST_ENV_ENABLED", tt.value)
			if got := envEnabled("TEST_ENV_ENABLED"); got != tt.want {
				t.Errorf("envEnabled(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

// isLoopbackAddr

func TestIsLoopbackAddr(t *testing.T) {
	for _, tt := range []struct {
		addr string
		want bool
	}{
		{"localhost:9400", true},
		{"127.0.0.1:9400", true},
		{"[::1]:9400", true},
		{"0.0.0.0:9400", false},
		{"192.168.1.1:9400", false},
		{"example.com:443", false},
		{"localhost", true},
		{"127.0.0.1", true},
		{"LocalHost:9400", true},
		{"127.8.9.10:9400", true},
		{"[::1]:0", true},
		{":9400", false},
		{"[::]:9400", false},
		{"0.0.0.0", false},
		{"10.0.0.5:9400", false},
		{"[fe80::1]:9400", false},
		{"localhost.example.com:9400", false},
		{"myhost:9400", false},
		{"", false},
	} {
		t.Run(tt.addr, func(t *testing.T) {
			if got := isLoopbackAddr(tt.addr); got != tt.want {
				t.Errorf("isLoopbackAddr(%q) = %v, want %v", tt.addr, got, tt.want)
			}
		})
	}
}

// bearerAuth

func TestBearerAuth_ValidToken(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := bearerAuth("secret-token", inner)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestBearerAuth_WrongToken(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})
	h := bearerAuth("secret-token", inner)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestBearerAuth_MissingHeader(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	})
	h := bearerAuth("secret-token", inner)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// cmdServe --version

func TestCmdServe_VersionFlag(t *testing.T) {
	out := captureStdout(t, func() {
		cmdServe([]string{"--version"})
	})
	if !strings.Contains(out, "artex-cc ") {
		t.Errorf("expected 'artex-cc' in output: %q", out)
	}
	if !strings.Contains(out, version) {
		t.Errorf("expected version %q in output: %q", version, out)
	}
}

// cmdInit --version

func TestCmdInit_VersionFlag(t *testing.T) {
	out := captureStdout(t, func() {
		cmdInit([]string{"--version"})
	})
	if !strings.Contains(out, "artex-cc ") {
		t.Errorf("expected 'artex-cc' in output: %q", out)
	}
	if !strings.Contains(out, version) {
		t.Errorf("expected version %q in output: %q", version, out)
	}
}

// cmdInit --dry-run (goes through cmdInit's flag parsing and env resolution)

func TestCmdInit_DryRun_Text(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ARTEX_PG", "")
	out := captureStdout(t, func() {
		cmdInit([]string{
			"--dry-run",
			"--pg", "postgres://test/db",
			"--goal", "test goal",
			"--description", "test desc",
			"--data", dir,
		})
	})
	if !strings.Contains(out, "Dry run") {
		t.Errorf("expected 'Dry run' in output: %q", out)
	}
	if !strings.Contains(out, "test goal") {
		t.Errorf("expected goal in output: %q", out)
	}
	if !strings.Contains(out, "test desc") {
		t.Errorf("expected description in output: %q", out)
	}
}

func TestCmdInit_DryRun_JSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ARTEX_PG", "")
	out := captureStdout(t, func() {
		cmdInit([]string{
			"--dry-run",
			"--pg", "postgres://test/db",
			"--goal", "find sqli",
			"--description", "web app",
			"--data", dir,
			"--format", "json",
		})
	})
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out)
	}
	if result["dry_run"] != true {
		t.Error("expected dry_run = true")
	}
	if result["goal"] != "find sqli" {
		t.Errorf("goal = %v", result["goal"])
	}
}

// cmdInitDryRun direct tests

func TestCmdInitDryRun_Text(t *testing.T) {
	dir := t.TempDir()
	out := captureStdout(t, func() {
		cmdInitDryRun(dir, "postgres://test", "find bugs", "web app", "text")
	})
	if !strings.Contains(out, "Dry run") {
		t.Error("expected 'Dry run' in output")
	}
	if !strings.Contains(out, "find bugs") {
		t.Error("expected goal in output")
	}
	if !strings.Contains(out, "web app") {
		t.Error("expected description in output")
	}
	if !strings.Contains(out, ".artex.json") {
		t.Error("expected config path in output")
	}
	if !strings.Contains(out, "[create]") {
		t.Error("expected [create] action in output")
	}
}

func TestCmdInitDryRun_JSON(t *testing.T) {
	dir := t.TempDir()
	out := captureStdout(t, func() {
		cmdInitDryRun(dir, "postgres://test", "find bugs", "web app", "json")
	})
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out)
	}
	if result["dry_run"] != true {
		t.Error("dry_run should be true")
	}
	if result["pg_dsn"] != "postgres://test" {
		t.Errorf("pg_dsn = %v", result["pg_dsn"])
	}
	actions, ok := result["actions"].([]any)
	if !ok || len(actions) == 0 {
		t.Fatal("expected non-empty actions array")
	}
	first := actions[0].(map[string]any)
	if first["action"] != "create" {
		t.Errorf("first action = %q, want create", first["action"])
	}
}

func TestCmdInitDryRun_ExistingConfig(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".artex.json"), []byte("{}"), 0o644)

	out := captureStdout(t, func() {
		cmdInitDryRun(dir, "postgres://test", "goal", "desc", "json")
	})
	var result map[string]any
	json.Unmarshal([]byte(out), &result)
	actions := result["actions"].([]any)
	first := actions[0].(map[string]any)
	if first["action"] != "overwrite" {
		t.Errorf("first action = %q, want overwrite", first["action"])
	}
}

func TestCmdInitDryRun_ExistingCA(t *testing.T) {
	dir := t.TempDir()
	caDir := filepath.Join(dir, "traffic", "_ca")
	os.MkdirAll(caDir, 0o755)
	os.WriteFile(filepath.Join(caDir, "mitmproxy-ca.pem"), []byte("key"), 0o600)
	os.WriteFile(filepath.Join(caDir, "mitmproxy-ca-cert.pem"), []byte("cert"), 0o644)

	out := captureStdout(t, func() {
		cmdInitDryRun(dir, "postgres://test", "goal", "desc", "json")
	})
	var result map[string]any
	json.Unmarshal([]byte(out), &result)
	actions := result["actions"].([]any)
	foundExists := false
	for _, a := range actions {
		act := a.(map[string]any)
		if act["action"] == "exists" {
			foundExists = true
		}
	}
	if !foundExists {
		t.Error("expected at least one 'exists' action for existing CA")
	}
}

func TestCmdInitDryRun_ExistingCA_Text(t *testing.T) {
	dir := t.TempDir()
	caDir := filepath.Join(dir, "traffic", "_ca")
	os.MkdirAll(caDir, 0o755)
	os.WriteFile(filepath.Join(caDir, "mitmproxy-ca.pem"), []byte("key"), 0o600)

	out := captureStdout(t, func() {
		cmdInitDryRun(dir, "postgres://test", "goal", "desc", "text")
	})
	if !strings.Contains(out, "[exists]") {
		t.Errorf("expected [exists] in output: %q", out)
	}
}

// shutdown

func TestShutdown_NilTraffic(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	s := &store.Store{DB: db}
	shutdown(nil, s, time.Second)
}

// writeHealthText additional branches

func TestWriteHealthText_TrafficDB_OK(t *testing.T) {
	r := healthResult{
		Status: "ok",
		Checks: map[string]hcEntry{
			"pg":          {Status: "ok", LatencyMS: 5},
			"schema":      {Status: "ok", Detail: "migrated (v2)"},
			"engagements": {Status: "ok", Detail: "3"},
			"traffic_db":  {Status: "ok", Detail: "42 exchanges, 1.5 MB"},
		},
	}
	var buf bytes.Buffer
	writeHealthText(&buf, r)
	out := buf.String()
	if !strings.Contains(out, "Traffic DB: 42 exchanges") {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestWriteHealthText_TrafficDB_Fail(t *testing.T) {
	r := healthResult{
		Status: "fail",
		Checks: map[string]hcEntry{
			"pg":          {Status: "ok", LatencyMS: 5},
			"schema":      {Status: "ok", Detail: "migrated (v2)"},
			"engagements": {Status: "ok", Detail: "3"},
			"traffic_db":  {Status: "fail", Error: "database locked"},
		},
	}
	var buf bytes.Buffer
	writeHealthText(&buf, r)
	out := buf.String()
	if !strings.Contains(out, "Traffic DB: FAILED (database locked)") {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestWriteHealthText_SchemaFailWithError(t *testing.T) {
	r := healthResult{
		Status: "fail",
		Checks: map[string]hcEntry{
			"pg":     {Status: "ok", LatencyMS: 5},
			"schema": {Status: "fail", Error: "relation does not exist"},
		},
	}
	var buf bytes.Buffer
	writeHealthText(&buf, r)
	out := buf.String()
	if !strings.Contains(out, "Schema:     FAILED (relation does not exist)") {
		t.Errorf("unexpected output: %q", out)
	}
}

// checkTrafficDB with real SQLite

func TestCheckTrafficDB_ValidSQLite(t *testing.T) {
	dir := t.TempDir()
	indexDir := filepath.Join(dir, "traffic", "_index")
	os.MkdirAll(indexDir, 0o755)

	dbPath := filepath.Join(indexDir, "index.sqlite")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	db.Exec("CREATE TABLE exchanges (id INTEGER PRIMARY KEY)")
	db.Exec("INSERT INTO exchanges (id) VALUES (1)")
	db.Exec("INSERT INTO exchanges (id) VALUES (2)")
	db.Exec("INSERT INTO exchanges (id) VALUES (3)")
	db.Close()

	result := healthResult{Status: "ok", Checks: make(map[string]hcEntry)}
	checkTrafficDB(&result, dir)

	e, ok := result.Checks["traffic_db"]
	if !ok {
		t.Fatal("expected traffic_db check")
	}
	if e.Status != "ok" {
		t.Errorf("status = %q, want ok", e.Status)
	}
	if !strings.Contains(e.Detail, "3 exchanges") {
		t.Errorf("detail = %q, expected '3 exchanges'", e.Detail)
	}
}

func TestCheckTrafficDB_NoExchangesTable(t *testing.T) {
	dir := t.TempDir()
	indexDir := filepath.Join(dir, "traffic", "_index")
	os.MkdirAll(indexDir, 0o755)

	dbPath := filepath.Join(indexDir, "index.sqlite")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	db.Exec("CREATE TABLE other (id INTEGER PRIMARY KEY)")
	db.Close()

	result := healthResult{Status: "ok", Checks: make(map[string]hcEntry)}
	checkTrafficDB(&result, dir)

	e := result.Checks["traffic_db"]
	if e.Status != "fail" {
		t.Errorf("status = %q, want fail", e.Status)
	}
	if result.Status != "fail" {
		t.Errorf("overall status = %q, want fail", result.Status)
	}
}

func TestCheckTrafficDB_CorruptFile(t *testing.T) {
	dir := t.TempDir()
	indexDir := filepath.Join(dir, "traffic", "_index")
	os.MkdirAll(indexDir, 0o755)

	dbPath := filepath.Join(indexDir, "index.sqlite")
	os.WriteFile(dbPath, []byte("not a sqlite file"), 0o644)

	result := healthResult{Status: "ok", Checks: make(map[string]hcEntry)}
	checkTrafficDB(&result, dir)

	e := result.Checks["traffic_db"]
	if e.Status != "fail" {
		t.Errorf("status = %q, want fail", e.Status)
	}
}

// PG-dependent tests (skipped when no PG is available)

func TestRunHealthChecks_FullSuccess(t *testing.T) {
	dsn := testPGDSN(t)

	s, err := store.Open(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	r := runHealthChecks(dsn, t.TempDir(), 10*time.Second)
	if r.Status != "ok" {
		t.Errorf("status = %q, want ok", r.Status)
	}
	if r.Checks["pg"].Status != "ok" {
		t.Errorf("pg status = %q, want ok", r.Checks["pg"].Status)
	}
	if r.Checks["pg"].LatencyMS < 0 {
		t.Errorf("pg latency = %d", r.Checks["pg"].LatencyMS)
	}
	if r.Checks["schema"].Status != "ok" {
		t.Errorf("schema status = %q, want ok", r.Checks["schema"].Status)
	}
	if !strings.Contains(r.Checks["schema"].Detail, "migrated") {
		t.Errorf("schema detail = %q", r.Checks["schema"].Detail)
	}
	if r.Checks["engagements"].Status != "ok" {
		t.Errorf("engagements status = %q, want ok", r.Checks["engagements"].Status)
	}
}

func TestRunHealthChecks_WithTrafficDir(t *testing.T) {
	dsn := testPGDSN(t)

	s, err := store.Open(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	dir := t.TempDir()
	indexDir := filepath.Join(dir, "traffic", "_index")
	os.MkdirAll(indexDir, 0o755)
	db, err := sql.Open("sqlite", filepath.Join(indexDir, "index.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	db.Exec("CREATE TABLE exchanges (id INTEGER PRIMARY KEY)")
	db.Close()

	r := runHealthChecks(dsn, dir, 10*time.Second)
	if r.Status != "ok" {
		t.Errorf("status = %q, want ok", r.Status)
	}
	if _, ok := r.Checks["traffic_db"]; !ok {
		t.Error("expected traffic_db check")
	}
}

func TestCmdInit_FullFlow_JSON(t *testing.T) {
	dsn := testPGDSN(t)
	dir := t.TempDir()
	t.Setenv("ARTEX_PG", "")

	out := captureStdout(t, func() {
		cmdInit([]string{
			"--pg", dsn,
			"--goal", "coverage test engagement",
			"--description", "test target for coverage",
			"--data", dir,
			"--format", "json",
		})
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, out)
	}
	taskID, ok := result["task_id"]
	if !ok {
		t.Fatal("expected task_id in output")
	}
	if taskID.(float64) <= 0 {
		t.Errorf("task_id = %v", taskID)
	}
	if _, ok := result["exploration_id"]; !ok {
		t.Fatal("expected exploration_id in output")
	}
	if _, err := os.Stat(filepath.Join(dir, ".artex.json")); err != nil {
		t.Errorf("config file not created: %v", err)
	}
}

func TestCmdInit_FullFlow_Text(t *testing.T) {
	dsn := testPGDSN(t)
	dir := t.TempDir()
	t.Setenv("ARTEX_PG", "")

	out := captureStdout(t, func() {
		cmdInit([]string{
			"--pg", dsn,
			"--goal", "coverage text flow",
			"--description", "test target",
			"--data", dir,
		})
	})

	if !strings.Contains(out, "Engagement created") {
		t.Errorf("expected 'Engagement created' in output: %q", out)
	}
	if !strings.Contains(out, "Task ID:") {
		t.Errorf("expected 'Task ID:' in output: %q", out)
	}
	if !strings.Contains(out, "Exploration ID:") {
		t.Errorf("expected 'Exploration ID:' in output: %q", out)
	}
	if !strings.Contains(out, "mcpServers") {
		t.Errorf("expected MCP config example in output: %q", out)
	}
}

func TestCmdInit_CreatesMissingDataDir(t *testing.T) {
	dsn := testPGDSN(t)
	dir := filepath.Join(t.TempDir(), "new", "data")
	t.Setenv("ARTEX_PG", "")

	captureStdout(t, func() {
		cmdInit([]string{"--pg", dsn, "--goal", "missing data dir", "--data", dir, "--format", "json"})
	})

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o711 {
		t.Errorf("data dir mode = %o, want 711", perm)
	}
	if _, err := os.Stat(filepath.Join(dir, ".artex.json")); err != nil {
		t.Errorf("config file not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "traffic", "_ca", "mitmproxy-ca-cert.pem")); err != nil {
		t.Errorf("CA certificate not created: %v", err)
	}
}

func TestCmdInit_SaveFailureLeavesNoEngagement(t *testing.T) {
	if args := os.Getenv("ARTEX_TEST_INIT_ARGS"); args != "" {
		cmdInit(strings.Fields(args))
		os.Exit(0)
	}
	dsn := testPGDSN(t)
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".artex.json"), 0o700); err != nil {
		t.Fatal(err)
	}
	goal := "orphan-check-" + filepath.Base(filepath.Dir(dir))

	cmd := exec.Command(os.Args[0], "-test.run=^TestCmdInit_SaveFailureLeavesNoEngagement$")
	cmd.Env = append(os.Environ(),
		"ARTEX_PG="+dsn,
		"ARTEX_TEST_INIT_ARGS=--goal "+goal+" --data "+dir)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
		t.Fatalf("init exited with %v, want status 1; stderr: %s", err, stderr.String())
	}
	if !strings.Contains(stderr.String(), "failed to save config") {
		t.Errorf("stderr = %q, want the save failure", stderr.String())
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM tasks WHERE goal = $1`, goal).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("%d engagement rows left after a failed init, want 0", n)
	}
}

func TestShutdown_WithStore(t *testing.T) {
	dsn := testPGDSN(t)
	s, err := store.Open(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	shutdown(nil, s, 5*time.Second)
}

func TestSetupURL(t *testing.T) {
	for addr, want := range map[string]string{
		":9400":          "http://localhost:9400/setup",
		"0.0.0.0:9400":   "http://localhost:9400/setup",
		"[::]:9400":      "http://localhost:9400/setup",
		"localhost:8080": "http://localhost:8080/setup",
		"10.0.0.5:9400":  "http://10.0.0.5:9400/setup",
		"[fe80::1]:9400": "http://[fe80::1]:9400/setup",
		"artex.lan:9400": "http://artex.lan:9400/setup",
	} {
		if got := setupURL(addr); got != want {
			t.Errorf("setupURL(%q) = %q, want %q", addr, got, want)
		}
	}
}

func TestPrintMCPSnippets_RedactsPassword(t *testing.T) {
	var buf bytes.Buffer
	printMCPSnippets(&buf, "postgres://artex:hunter2@db:5432/ccartex", "/app/data")
	out := buf.String()
	if strings.Contains(out, "hunter2") {
		t.Fatalf("password leaked into init output:\n%s", out)
	}
	if !strings.Contains(out, `"postgres://artex:***@db:5432/ccartex"`) {
		t.Errorf("expected redacted DSN in snippet:\n%s", out)
	}
	if !strings.Contains(out, "ARTEX_PG") {
		t.Errorf("expected substitution note:\n%s", out)
	}
}

func TestPrintMCPSnippets_NoPasswordUnchanged(t *testing.T) {
	var buf bytes.Buffer
	printMCPSnippets(&buf, "postgres://localhost/ccartex", ".")
	out := buf.String()
	if !strings.Contains(out, `"postgres://localhost/ccartex"`) {
		t.Errorf("expected DSN verbatim:\n%s", out)
	}
	if strings.Contains(out, "***") {
		t.Errorf("unexpected redaction note:\n%s", out)
	}
}

func TestCmdInit_DryRun_JSONRedactsPassword(t *testing.T) {
	t.Setenv("ARTEX_PG", "")
	out := captureStdout(t, func() {
		cmdInit([]string{"--dry-run", "--pg", "postgres://artex:hunter2@db/ccartex", "--data", t.TempDir(), "--format", "json"})
	})
	if strings.Contains(out, "hunter2") {
		t.Fatalf("password leaked into dry-run output:\n%s", out)
	}
}
