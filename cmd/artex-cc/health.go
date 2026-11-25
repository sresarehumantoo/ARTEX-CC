package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

type healthResult struct {
	Status string             `json:"status"`
	Checks map[string]hcEntry `json:"checks"`
}

type hcEntry struct {
	Status    string `json:"status"`
	LatencyMS int64  `json:"latency_ms,omitempty"`
	Detail    string `json:"detail,omitempty"`
	Error     string `json:"error,omitempty"`
}

func cmdHealth(args []string) {
	fs := flag.NewFlagSet("health", flag.ExitOnError)
	pg := pgDSN(fs)
	dataDir := fs.String("data", "", "data directory (for traffic DB health check)")
	format := fs.String("format", "text", "output format: text or json")
	timeout := fs.Duration("timeout", 5*time.Second, "health check timeout (e.g. 5s, 10s, 30s)")
	fs.Parse(args)

	if *format != "text" && *format != "json" {
		fmt.Fprintf(os.Stderr, "unsupported format: %s (use text or json)\n", *format)
		os.Exit(1)
	}

	result := runHealthChecks(*pg, *dataDir, *timeout)
	if *format == "json" {
		writeHealthJSON(os.Stdout, result)
	} else {
		writeHealthText(os.Stdout, result)
	}
	if result.Status != "ok" {
		os.Exit(1)
	}
}

func runHealthChecks(pgFlag, dataDir string, timeout time.Duration) healthResult {
	result := healthResult{
		Status: "ok",
		Checks: make(map[string]hcEntry),
	}

	dsn := resolvePG(pgFlag)
	if dsn == "" {
		result.Status = "fail"
		result.Checks["pg"] = hcEntry{Status: "fail", Error: "not configured (set --pg or ARTEX_PG)"}
		return result
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		result.Status = "fail"
		result.Checks["pg"] = hcEntry{Status: "fail", Error: err.Error()}
		return result
	}
	defer db.Close()

	start := time.Now()
	if err := db.PingContext(ctx); err != nil {
		result.Status = "fail"
		result.Checks["pg"] = hcEntry{Status: "fail", Error: err.Error()}
		return result
	}
	latency := time.Since(start).Milliseconds()
	result.Checks["pg"] = hcEntry{Status: "ok", LatencyMS: latency}

	var hasSchema bool
	err = db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'tasks')`).Scan(&hasSchema)
	if err != nil {
		result.Status = "fail"
		result.Checks["schema"] = hcEntry{Status: "fail", Error: err.Error()}
		return result
	}
	if !hasSchema {
		result.Status = "fail"
		result.Checks["schema"] = hcEntry{Status: "fail", Detail: "not initialized (run artex-cc init)"}
		return result
	}

	var schemaVersion int
	err = db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&schemaVersion)
	if err != nil {
		result.Checks["schema"] = hcEntry{Status: "ok", Detail: "migrated (version unknown)"}
	} else {
		result.Checks["schema"] = hcEntry{Status: "ok", Detail: fmt.Sprintf("migrated (v%d)", schemaVersion)}
	}

	var count int
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks`).Scan(&count)
	if err != nil {
		result.Checks["engagements"] = hcEntry{Status: "ok", Detail: "unknown"}
	} else {
		result.Checks["engagements"] = hcEntry{Status: "ok", Detail: fmt.Sprintf("%d", count)}
	}

	if dataDir != "" {
		checkTrafficDB(&result, dataDir)
	}

	return result
}

func checkTrafficDB(result *healthResult, dataDir string) {
	indexPath := filepath.Join(dataDir, "traffic", "_index", "index.sqlite")

	info, err := os.Stat(indexPath)
	if os.IsNotExist(err) {
		result.Checks["traffic_db"] = hcEntry{Status: "ok", Detail: "not present"}
		return
	}
	if err != nil {
		result.Checks["traffic_db"] = hcEntry{Status: "fail", Error: err.Error()}
		result.Status = "fail"
		return
	}

	db, err := sql.Open("sqlite", indexPath+"?mode=ro")
	if err != nil {
		result.Checks["traffic_db"] = hcEntry{Status: "fail", Error: err.Error()}
		result.Status = "fail"
		return
	}
	defer db.Close()

	var exchanges int
	if err := db.QueryRow("SELECT COUNT(*) FROM exchanges").Scan(&exchanges); err != nil {
		result.Checks["traffic_db"] = hcEntry{Status: "fail", Error: err.Error()}
		result.Status = "fail"
		return
	}

	sizeMB := float64(info.Size()) / (1024 * 1024)
	result.Checks["traffic_db"] = hcEntry{
		Status: "ok",
		Detail: fmt.Sprintf("%d exchanges, %.1f MB", exchanges, sizeMB),
	}
}

func writeHealthJSON(w io.Writer, r healthResult) {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(r)
}

func writeHealthText(w io.Writer, r healthResult) {
	if e, ok := r.Checks["pg"]; ok {
		if e.Status == "fail" {
			if e.Error == "not configured (set --pg or ARTEX_PG)" {
				fmt.Fprintln(w, "PostgreSQL: NOT CONFIGURED")
				fmt.Fprintln(w, "  Set --pg or ARTEX_PG")
			} else {
				fmt.Fprintf(w, "PostgreSQL: FAILED (%s)\n", e.Error)
			}
			return
		}
		fmt.Fprintln(w, "PostgreSQL: connected")
	}

	if e, ok := r.Checks["schema"]; ok {
		if e.Status == "fail" {
			if e.Error != "" {
				fmt.Fprintf(w, "Schema:     FAILED (%s)\n", e.Error)
			} else {
				fmt.Fprintf(w, "Schema:     %s\n", e.Detail)
			}
			return
		}
		fmt.Fprintf(w, "Schema:     %s\n", e.Detail)
	}

	if e, ok := r.Checks["engagements"]; ok {
		fmt.Fprintf(w, "Engagements: %s\n", e.Detail)
	}

	if e, ok := r.Checks["traffic_db"]; ok {
		if e.Status == "fail" {
			fmt.Fprintf(w, "Traffic DB: FAILED (%s)\n", e.Error)
		} else {
			fmt.Fprintf(w, "Traffic DB: %s\n", e.Detail)
		}
	}
}
