package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func cmdHealth(args []string) {
	fs := flag.NewFlagSet("health", flag.ExitOnError)
	pg := pgDSN(fs)
	fs.Parse(args)

	dsn := resolvePG(*pg)
	if dsn == "" {
		fmt.Println("PostgreSQL: NOT CONFIGURED")
		fmt.Println("  Set --pg or CC_ARTEX_PG")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		fmt.Printf("PostgreSQL: FAILED (%v)\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		fmt.Printf("PostgreSQL: FAILED (%v)\n", err)
		os.Exit(1)
	}
	fmt.Println("PostgreSQL: connected")

	var hasSchema bool
	err = db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'tasks')`).Scan(&hasSchema)
	if err != nil {
		fmt.Printf("Schema:     FAILED (%v)\n", err)
		os.Exit(1)
	}
	if !hasSchema {
		fmt.Println("Schema:     not initialized (run cc-artex init)")
		os.Exit(1)
	}

	var schemaVersion int
	err = db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&schemaVersion)
	if err != nil {
		fmt.Println("Schema:     migrated (version unknown)")
	} else {
		fmt.Printf("Schema:     migrated (v%d)\n", schemaVersion)
	}

	var count int
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks`).Scan(&count)
	if err != nil {
		fmt.Println("Engagements: unknown")
	} else {
		fmt.Printf("Engagements: %d\n", count)
	}
}
