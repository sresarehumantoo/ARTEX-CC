package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/SREsAreHumanToo/cc-artex/api"
	"github.com/SREsAreHumanToo/cc-artex/config"
	"github.com/SREsAreHumanToo/cc-artex/evidence"
	"github.com/SREsAreHumanToo/cc-artex/mcpserver"
	"github.com/SREsAreHumanToo/cc-artex/store"
	"github.com/SREsAreHumanToo/cc-artex/tools"
	"github.com/SREsAreHumanToo/cc-artex/traffic"
	"github.com/SREsAreHumanToo/cc-artex/web"
	"github.com/mark3labs/mcp-go/server"
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
	case "health":
		cmdHealth(os.Args[2:])
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
	fmt.Fprintln(os.Stderr, "usage: cc-artex <serve|init|health|version>")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  serve    Start the MCP tool server")
	fmt.Fprintln(os.Stderr, "  init     Initialize a new engagement")
	fmt.Fprintln(os.Stderr, "  health   Check PostgreSQL connectivity")
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

func initLogging(debug bool) {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
}

func cmdServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	listen := fs.String("listen", "localhost:9400", "HTTP listen address")
	stdio := fs.Bool("stdio", false, "use stdio transport instead of HTTP")
	webUI := fs.Bool("web", false, "enable web UI and REST API")
	debug := fs.Bool("debug", false, "enable debug-level logging")
	showVersion := fs.Bool("version", false, "print version and exit")
	pg := pgDSN(fs)
	dataDir := fs.String("data", ".", "data directory for traffic, evidence, etc.")
	proxyAddr := fs.String("proxy", "", "start the recording proxy on ADDR (e.g., :8788)")
	fs.Parse(args)

	if *showVersion {
		fmt.Println("cc-artex " + version)
		return
	}

	initLogging(*debug)

	dsn := resolvePG(*pg)
	if dsn == "" {
		slog.Error("--pg or CC_ARTEX_PG is required")
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	engCfg, err := config.LoadEngagement(*dataDir)
	if err != nil {
		slog.Error("failed to load engagement config", "err", err)
		os.Exit(1)
	}

	s, err := store.Open(ctx, dsn)
	if err != nil {
		slog.Error("failed to open store", "err", err)
		os.Exit(1)
	}

	if keyHex := os.Getenv("CC_ARTEX_CREDENTIAL_KEY"); keyHex != "" {
		key, err := store.ParseCredentialKey(keyHex)
		if err != nil {
			slog.Error("invalid credential key", "err", err)
			os.Exit(1)
		}
		s.SetCredentialKey(key)
	} else {
		slog.Warn("CC_ARTEX_CREDENTIAL_KEY is not set; credentials will be stored in plaintext")
	}

	reg := tools.NewRegistry()
	tools.RegisterAll(reg, s, engCfg.TaskID, engCfg.ExplorationID)

	evStore := evidence.NewStore(*dataDir)
	tools.RegisterEvidenceTools(reg, evStore, s, engCfg.TaskID)

	var t *traffic.Traffic
	if *proxyAddr != "" {
		trafficDir := filepath.Join(*dataDir, "traffic")
		t, err = traffic.Open(trafficDir, *proxyAddr)
		if err != nil {
			slog.Error("failed to open traffic proxy", "err", err)
			os.Exit(1)
		}
		go func() {
			if err := t.Start(); err != nil && ctx.Err() == nil {
				slog.Error("traffic proxy failed", "err", err)
			}
		}()
		tools.RegisterTrafficTools(reg, t, s, engCfg.TaskID)
		slog.Info("recording proxy started", "addr", *proxyAddr, "ca_cert", t.CACertPath())
	}

	var srvOpts []mcpserver.Option
	srvOpts = append(srvOpts, mcpserver.WithEngagement(s, engCfg.TaskID, engCfg.ExplorationID))
	srvOpts = append(srvOpts, mcpserver.WithEvidence(evStore))
	if t != nil {
		srvOpts = append(srvOpts, mcpserver.WithTraffic(t))
	}
	var restAPI *api.API
	if *webUI {
		restAPI = api.New(s, t, engCfg.TaskID, engCfg.ExplorationID)
		srvOpts = append(srvOpts, mcpserver.WithNotifier(restAPI.NotifyChange))
	}
	srv := mcpserver.New(reg, srvOpts...)
	if *stdio {
		slog.Info("serving", "transport", "stdio")
		if err := srv.ServeStdio(ctx); err != nil && ctx.Err() == nil {
			slog.Error("stdio server failed", "err", err)
			os.Exit(1)
		}
	} else if *webUI {
		mux := http.NewServeMux()

		mcpHandler := server.NewStreamableHTTPServer(srv.MCPServer())
		mux.Handle("/mcp", mcpHandler)

		mux.Handle("/api/", restAPI.Handler())

		mux.Handle("/", web.Handler())

		httpServer := &http.Server{Addr: *listen, Handler: mux}
		errCh := make(chan error, 1)
		go func() { errCh <- httpServer.ListenAndServe() }()

		slog.Info("serving", "addr", *listen, "web", true)
		select {
		case err := <-errCh:
			if err != nil && ctx.Err() == nil {
				slog.Error("http server failed", "err", err)
				os.Exit(1)
			}
		case <-ctx.Done():
			httpServer.Shutdown(context.Background())
		}
	} else {
		slog.Info("serving", "addr", *listen)
		if err := srv.ServeHTTP(ctx, *listen); err != nil && ctx.Err() == nil {
			slog.Error("http server failed", "err", err)
			os.Exit(1)
		}
	}

	shutdown(t, s)
}

const shutdownTimeout = 5 * time.Second

func shutdown(t *traffic.Traffic, s *store.Store) {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if t != nil {
		if err := t.Shutdown(ctx); err != nil {
			slog.Error("traffic proxy shutdown failed", "err", err)
		}
		if err := t.Close(); err != nil {
			slog.Error("traffic close failed", "err", err)
		}
	}
	if err := s.Close(); err != nil {
		slog.Error("store close failed", "err", err)
	}
}

func cmdInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	showVersion := fs.Bool("version", false, "print version and exit")
	pg := pgDSN(fs)
	goal := fs.String("goal", "", "engagement goal")
	desc := fs.String("description", "", "target description")
	dataDir := fs.String("data", ".", "data directory for config file")
	fs.Parse(args)

	if *showVersion {
		fmt.Println("cc-artex " + version)
		return
	}

	initLogging(false)

	dsn := resolvePG(*pg)
	if dsn == "" {
		slog.Error("--pg or CC_ARTEX_PG is required")
		os.Exit(1)
	}

	ctx := context.Background()

	s, err := store.Open(ctx, dsn)
	if err != nil {
		slog.Error("failed to open store", "err", err)
		os.Exit(1)
	}
	defer s.Close()

	taskID, explorationID, err := s.CreateEngagement(ctx, *goal, *desc)
	if err != nil {
		slog.Error("failed to create engagement", "err", err)
		os.Exit(1)
	}

	engCfg := config.EngagementConfig{
		TaskID:        taskID,
		ExplorationID: explorationID,
		Goal:          *goal,
		Description:   *desc,
	}
	if err := config.SaveEngagement(*dataDir, engCfg); err != nil {
		slog.Error("failed to save config", "err", err)
		os.Exit(1)
	}

	certPath, err := generateCA(*dataDir)
	if err != nil {
		slog.Warn("failed to generate CA certificate", "err", err)
	}

	fmt.Printf("Engagement created:\n")
	fmt.Printf("  Task ID:        %d\n", taskID)
	fmt.Printf("  Exploration ID: %d\n", explorationID)
	fmt.Printf("  Config written: %s/.cc-artex.json\n", *dataDir)
	fmt.Println()
	if certPath != "" {
		fmt.Printf("  CA certificate: %s\n", certPath)
		fmt.Println()
		fmt.Println("Trust the CA certificate so the recording proxy can intercept HTTPS:")
		fmt.Println()
		fmt.Println("  Linux:   sudo cp " + certPath + " /usr/local/share/ca-certificates/cc-artex.crt && sudo update-ca-certificates")
		fmt.Println("  macOS:   sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain " + certPath)
		fmt.Println("  Windows: certutil -addstore Root " + certPath)
		fmt.Println()
	}
	fmt.Println("Add to your Claude Code MCP config (~/.claude.json or project .mcp.json):")
	fmt.Println()
	fmt.Println("Minimal (no proxy, data in cwd):")
	fmt.Println()
	fmt.Printf(`  {
    "mcpServers": {
      "cc-artex": {
        "command": "cc-artex",
        "args": ["serve", "--stdio", "--pg", %q]
      }
    }
  }`, dsn)
	fmt.Println()
	fmt.Println()
	fmt.Println("Full (recording proxy + explicit data directory):")
	fmt.Println()
	fmt.Printf(`  {
    "mcpServers": {
      "cc-artex": {
        "command": "cc-artex",
        "args": [
          "serve", "--stdio",
          "--pg", %q,
          "--data", %q,
          "--proxy", ":8788"
        ]
      }
    }
  }`, dsn, *dataDir)
	fmt.Println()
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  --pg        PostgreSQL connection string (or set CC_ARTEX_PG)")
	fmt.Println("  --data      directory for traffic captures, evidence, and config")
	fmt.Println("  --proxy     start the MITM recording proxy on ADDR (e.g. :8788)")
	fmt.Println("  --debug     enable debug-level logging")
}
