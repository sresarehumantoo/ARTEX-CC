package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func cmdHealth(args []string) {
	fs := flag.NewFlagSet("health", flag.ExitOnError)
	pg := pgDSN(fs)
	format := fs.String("format", "text", "output format: text or json")
	timeout := fs.Duration("timeout", 5*time.Second, "health check timeout (e.g. 5s, 10s, 30s)")
	fs.Parse(args)

	useJSON := *format == "json"

	dsn := resolvePG(*pg)
	if dsn == "" {
		if useJSON {
			printHealthJSON("NOT CONFIGURED", "", -1, "set --pg or CC_ARTEX_PG")
		} else {
			fmt.Println("PostgreSQL: NOT CONFIGURED")
			fmt.Println("  Set --pg or CC_ARTEX_PG")
		}
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		if useJSON {
			printHealthJSON("FAILED", "", -1, err.Error())
		} else {
			fmt.Printf("PostgreSQL: FAILED (%v)\n", err)
		}
		os.Exit(1)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		if useJSON {
			printHealthJSON("FAILED", "", -1, err.Error())
		} else {
			fmt.Printf("PostgreSQL: FAILED (%v)\n", err)
		}
		os.Exit(1)
	}

	var hasSchema bool
	err = db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'tasks')`).Scan(&hasSchema)
	if err != nil {
		if useJSON {
			printHealthJSON("connected", "FAILED", -1, err.Error())
		} else {
			fmt.Println("PostgreSQL: connected")
			fmt.Printf("Schema:     FAILED (%v)\n", err)
		}
		os.Exit(1)
	}
	if !hasSchema {
		if useJSON {
			printHealthJSON("connected", "not initialized", -1, "")
		} else {
			fmt.Println("PostgreSQL: connected")
			fmt.Println("Schema:     not initialized (run cc-artex init)")
		}
		os.Exit(1)
	}

	schemaVersion := -1
	err = db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&schemaVersion)
	if err != nil {
		schemaVersion = -1
	}

	engagements := -1
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks`).Scan(&engagements)
	if err != nil {
		engagements = -1
	}

	if useJSON {
		schema := "migrated"
		if schemaVersion >= 0 {
			schema = fmt.Sprintf("migrated (v%d)", schemaVersion)
		}
		printHealthJSON("connected", schema, engagements, "")
	} else {
		fmt.Println("PostgreSQL: connected")
		if schemaVersion >= 0 {
			fmt.Printf("Schema:     migrated (v%d)\n", schemaVersion)
		} else {
			fmt.Println("Schema:     migrated (version unknown)")
		}
		if engagements >= 0 {
			fmt.Printf("Engagements: %d\n", engagements)
		} else {
			fmt.Println("Engagements: unknown")
		}
	}
}

type healthResult struct {
	PostgreSQL  string `json:"postgresql"`
	Schema      string `json:"schema,omitempty"`
	Engagements int    `json:"engagements,omitempty"`
	Error       string `json:"error,omitempty"`
}

func printHealthJSON(pgStatus, schema string, engagements int, errMsg string) {
	r := healthResult{PostgreSQL: pgStatus, Schema: schema}
	if engagements >= 0 {
		r.Engagements = engagements
	}
	if errMsg != "" {
		r.Error = errMsg
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(r)
}
