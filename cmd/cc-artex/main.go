package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/SREsAreHumanToo/cc-artex/config"
	"github.com/SREsAreHumanToo/cc-artex/mcpserver"
	"github.com/SREsAreHumanToo/cc-artex/store"
	"github.com/SREsAreHumanToo/cc-artex/tools"
)

var version = "v0.1.0-dev"

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "serve":
		cmdServe(os.Args[2:])
	case "init":
		cmdInit(os.Args[2:])
	case "version":
		fmt.Println("cc-artex " + version)
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: cc-artex <serve|init|version>")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  serve    Start the MCP tool server")
	fmt.Fprintln(os.Stderr, "  init     Initialize a new engagement")
	fmt.Fprintln(os.Stderr, "  version  Print version")
	os.Exit(1)
}

func pgDSN(fs *flag.FlagSet) *string {
	return fs.String("pg", "", "PostgreSQL connection string (or CC_ARTEX_PG env)")
}

func resolvePG(val string) string {
	if val != "" {
		return val
	}
	if v := os.Getenv("CC_ARTEX_PG"); v != "" {
		return v
	}
	return ""
}

func cmdServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	listen := fs.String("listen", "localhost:9400", "HTTP listen address")
	stdio := fs.Bool("stdio", false, "use stdio transport instead of HTTP")
	pg := pgDSN(fs)
	dataDir := fs.String("data", ".", "data directory for traffic, evidence, etc.")
	fs.Parse(args)

	dsn := resolvePG(*pg)
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "error: --pg or CC_ARTEX_PG is required")
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	engCfg, err := config.LoadEngagement(*dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading engagement config: %v\n", err)
		os.Exit(1)
	}

	s, err := store.Open(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening store: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	reg := tools.NewRegistry()
	tools.RegisterAll(reg, s, engCfg.TaskID, engCfg.ExplorationID)

	srv := mcpserver.New(reg)
	if *stdio {
		if err := srv.ServeStdio(ctx); err != nil && ctx.Err() == nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	} else {
		fmt.Fprintf(os.Stderr, "cc-artex serving on %s\n", *listen)
		if err := srv.ServeHTTP(ctx, *listen); err != nil && ctx.Err() == nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}
}

func cmdInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	pg := pgDSN(fs)
	goal := fs.String("goal", "", "engagement goal")
	desc := fs.String("description", "", "target description")
	dataDir := fs.String("data", ".", "data directory for config file")
	fs.Parse(args)

	dsn := resolvePG(*pg)
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "error: --pg or CC_ARTEX_PG is required")
		os.Exit(1)
	}

	ctx := context.Background()

	s, err := store.Open(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening store: %v\n", err)
		os.Exit(1)
	}
	defer s.Close()

	taskID, explorationID, err := s.CreateEngagement(ctx, *goal, *desc)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating engagement: %v\n", err)
		os.Exit(1)
	}

	engCfg := config.EngagementConfig{
		TaskID:        taskID,
		ExplorationID: explorationID,
		Goal:          *goal,
		Description:   *desc,
	}
	if err := config.SaveEngagement(*dataDir, engCfg); err != nil {
		fmt.Fprintf(os.Stderr, "error saving config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Engagement created:\n")
	fmt.Printf("  Task ID:        %d\n", taskID)
	fmt.Printf("  Exploration ID: %d\n", explorationID)
	fmt.Printf("  Config written: %s/.cc-artex.json\n", *dataDir)
	fmt.Println()
	fmt.Println("Add to your Claude Code MCP config:")
	fmt.Println()
	fmt.Printf(`  { "mcpServers": { "cc-artex": { "command": "cc-artex", "args": ["serve", "--stdio", "--pg", %q] } } }`, dsn)
	fmt.Println()
}
