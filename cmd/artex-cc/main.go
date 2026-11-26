package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/api"
	"github.com/SREsAreHumanToo/ARTEX-CC/config"
	"github.com/SREsAreHumanToo/ARTEX-CC/evidence"
	"github.com/SREsAreHumanToo/ARTEX-CC/identity"
	"github.com/SREsAreHumanToo/ARTEX-CC/mcpserver"
	"github.com/SREsAreHumanToo/ARTEX-CC/plugin"
	"github.com/SREsAreHumanToo/ARTEX-CC/store"
	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
	"github.com/SREsAreHumanToo/ARTEX-CC/traffic"
	"github.com/SREsAreHumanToo/ARTEX-CC/web"
)

var (
	version   = "v0.1.0-dev"
	commit    = "unknown"
	buildDate = "unknown"
)

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
	case "tools":
		cmdTools(os.Args[2:])
	case "identity":
		cmdIdentity(os.Args[2:])
	case "completion":
		cmdCompletion(os.Args[2:])
	case "version":
		cmdVersion(os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: artex-cc <command>")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "  serve       Start the MCP tool server")
	fmt.Fprintln(os.Stderr, "  init        Initialize a new engagement")
	fmt.Fprintln(os.Stderr, "  health      Check PostgreSQL connectivity")
	fmt.Fprintln(os.Stderr, "  tools       List the MCP tool catalog (--json for machine output)")
	fmt.Fprintln(os.Stderr, "  identity    Issue, rotate or inspect client identity tokens")
	fmt.Fprintln(os.Stderr, "  completion  Generate shell completion script")
	fmt.Fprintln(os.Stderr, "  version     Print version")
	os.Exit(1)
}

func pgDSN(fs *flag.FlagSet) *string {
	return fs.String("pg", "", "PostgreSQL connection string (or ARTEX_PG env)")
}

func resolvePG(val string) string {
	if val != "" {
		return val
	}
	if v := os.Getenv("ARTEX_PG"); v != "" {
		return v
	}
	return ""
}

func envEnabled(name string) bool {
	v, err := strconv.ParseBool(os.Getenv(name))
	return err == nil && v
}

// requireIdentity resolves --require-identity and ARTEX_REQUIRE_IDENTITY.
// An unparsable env value is an error rather than off, so a typo cannot
// silently disable the requirement. stdio is refused because a stdio client
// cannot send X-Artex-Identity, so every claim would fail.
func requireIdentity(flagSet, stdio bool, env string) (bool, error) {
	on := flagSet
	if v := strings.TrimSpace(env); v != "" && !on {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return false, fmt.Errorf("ARTEX_REQUIRE_IDENTITY=%q is not a boolean (use 1/true or 0/false)", env)
		}
		on = b
	}
	if on && stdio {
		return false, fmt.Errorf("--require-identity (or ARTEX_REQUIRE_IDENTITY) needs an HTTP transport: stdio clients cannot send X-Artex-Identity, so every intent claim would be refused; unset it for stdio, or serve with --listen or --web")
	}
	return on, nil
}

func isLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func setupURL(listen string) string {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return "http://localhost:9400/setup"
	}
	if ip := net.ParseIP(host); host == "" || (ip != nil && ip.IsUnspecified()) {
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, port) + "/setup"
}

// mcpListenToken returns the bearer token /mcp must require on a listener
// without --web, or "" when none is needed. Only literal loopback addresses
// may run without one; other hostnames are not resolved and count as exposed.
func mcpListenToken(addr, token string) (string, error) {
	token = strings.TrimSpace(token)
	if token == "" && !isLoopbackAddr(addr) {
		return "", fmt.Errorf("--listen %s is not a loopback address, so /mcp needs a bearer token: set ARTEX_MCP_TOKEN (for example ARTEX_MCP_TOKEN=$(openssl rand -hex 32)) and send it as \"Authorization: Bearer <token>\", or listen on localhost", addr)
	}
	return token, nil
}

func initLogging(debug bool) {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
}

func bearerAuth(token string, next http.Handler) http.Handler {
	want := []byte("Bearer " + token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), want) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func loadIdentitySigner(dataDir string) *identity.Signer {
	signer, err := identity.NewSigner(dataDir)
	if err != nil {
		slog.Error("loading identity key failed", "err", err)
		os.Exit(1)
	}
	return signer
}

// webMCPHandler is /mcp under --web: bearer auth, then identity.
func webMCPHandler(srv *mcpserver.Server, token string, signer *identity.Signer) http.Handler {
	return bearerAuth(token, srv.IdentityMiddleware(signer)(srv.StreamableHTTP()))
}

// listenMiddleware is the ServeHTTP middleware for plain --listen. ServeHTTP
// wraps in order, so bearer auth, appended last, runs first.
func listenMiddleware(srv *mcpserver.Server, signer *identity.Signer, token string) []func(http.Handler) http.Handler {
	middleware := []func(http.Handler) http.Handler{srv.IdentityMiddleware(signer)}
	if token != "" {
		middleware = append(middleware, func(h http.Handler) http.Handler { return bearerAuth(token, h) })
	}
	return middleware
}

// loopbackHostOnly refuses requests that arrived on a loopback address with a
// Host that is not a loopback name, so a DNS-rebound page cannot reach the
// listener as if it were same-origin. Connections on other local addresses
// pass, so remote access by hostname keeps working.
func loopbackHostOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		local, _ := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
		if local != nil && isLoopbackAddr(local.String()) && !api.LoopbackHost(r) {
			http.Error(w, "host not allowed", http.StatusMisdirectedRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func webHandler(mcp, restAPI http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/mcp", mcp)
	mux.Handle("/api/", restAPI)
	mux.Handle("/", web.Handler())
	return api.SecurityHeaders(mux)
}

func cmdServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	listen := fs.String("listen", "", "HTTP listen address (or ARTEX_LISTEN env, default localhost:9400)")
	stdio := fs.Bool("stdio", false, "use stdio transport instead of HTTP")
	webUI := fs.Bool("web", false, "enable web UI and REST API")
	authFlag := fs.Bool("auth", false, "require password login for the web UI (or ARTEX_AUTH env; always on for non-loopback listeners)")
	debug := fs.Bool("debug", false, "enable debug-level logging")
	showVersion := fs.Bool("version", false, "print version and exit")
	pg := pgDSN(fs)
	dataDir := fs.String("data", ".", "data directory for traffic, evidence, etc.")
	claudeBin := fs.String("claude-bin", "", "path to the claude CLI binary (or ARTEX_CLAUDE_BIN env, default claude)")
	proxyAddr := fs.String("proxy", "", "start the recording proxy on ADDR (e.g., :8788)")
	pluginDir := fs.String("plugin-dir", "", "plugin directory (or ARTEX_PLUGIN_DIR env, default ~/.artex/plugins)")
	noPlugins := fs.Bool("no-plugins", false, "disable plugin loading")
	shutdownSecs := fs.Int("timeout", 30, "graceful shutdown timeout in seconds")
	requireIdentityFlag := fs.Bool("require-identity", false, "refuse intent claims from callers without a verified X-Artex-Identity (or ARTEX_REQUIRE_IDENTITY env; HTTP only)")
	fs.Parse(args)

	if *showVersion {
		fmt.Println("artex-cc " + version)
		return
	}

	if *claudeBin == "" {
		*claudeBin = os.Getenv("ARTEX_CLAUDE_BIN")
	}

	if *listen == "" {
		if v := os.Getenv("ARTEX_LISTEN"); v != "" {
			*listen = v
		} else {
			*listen = "localhost:9400"
		}
	}

	var listenToken string
	if !*stdio && !*webUI {
		var err error
		listenToken, err = mcpListenToken(*listen, os.Getenv("ARTEX_MCP_TOKEN"))
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	}

	identityRequired, err := requireIdentity(*requireIdentityFlag, *stdio, os.Getenv("ARTEX_REQUIRE_IDENTITY"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	initLogging(*debug)

	dsn := resolvePG(*pg)
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "error: PostgreSQL connection string is required")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  Option 1: pass --pg flag")
		fmt.Fprintln(os.Stderr, "    artex-cc serve --pg 'postgres://user:pass@localhost/ccartex'")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  Option 2: set ARTEX_PG environment variable")
		fmt.Fprintln(os.Stderr, "    export ARTEX_PG='postgres://user:pass@localhost/ccartex'")
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	engCfg, err := config.LoadEngagement(*dataDir)
	if err != nil {
		slog.Error("failed to load engagement config", "err", err)
		os.Exit(1)
	}
	if err := config.PrepareDataDir(*dataDir); err != nil {
		slog.Warn("failed to restrict data directory permissions", "path", *dataDir, "err", err)
	}

	s, err := store.Open(ctx, dsn)
	if err != nil {
		slog.Error("failed to open store", "err", err)
		os.Exit(1)
	}

	if keyHex := os.Getenv("ARTEX_CREDENTIAL_KEY"); keyHex != "" {
		key, err := store.ParseCredentialKey(keyHex)
		if err != nil {
			slog.Error("invalid credential key", "err", err)
			os.Exit(1)
		}
		s.SetCredentialKey(key)
	} else {
		slog.Warn("ARTEX_CREDENTIAL_KEY is not set; credentials will be stored in plaintext")
	}

	if err := s.BackfillAssetURLs(ctx); err != nil {
		slog.Error("failed to backfill asset urls", "err", err)
		os.Exit(1)
	}

	if v := os.Getenv("ARTEX_INTENT_LEASE"); v != "" {
		lease, err := tools.ParseIntentLease(v)
		if err != nil {
			slog.Error("invalid intent lease", "err", err)
			os.Exit(1)
		}
		tools.IntentLease = lease
		slog.Info("intent claim lease set", "lease", lease)
	}
	if identityRequired {
		tools.RequireIdentity = true
		slog.Info("intent claims require a verified identity")
	}

	if key, err := config.OwnerLabelKey(*dataDir); err != nil {
		slog.Warn("claim holder labels will differ between processes", "err", err)
	} else {
		tools.SetOwnerLabeler(tools.NewOwnerLabeler(key))
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
		slog.Info("recording proxy started", "addr", t.ProxyAddr(), "ca_cert", t.CACertPath())
	}

	// srv is assigned below; plugin handlers only read it once serving starts.
	var srv *mcpserver.Server
	if !*noPlugins {
		pdir := *pluginDir
		if pdir == "" {
			pdir = os.Getenv("ARTEX_PLUGIN_DIR")
		}
		if pdir == "" {
			if home, err := os.UserHomeDir(); err == nil {
				pdir = filepath.Join(home, ".artex", "plugins")
			}
		}
		if pdir != "" {
			absPath := func(p string) string {
				if a, err := filepath.Abs(p); err == nil {
					return a
				}
				return p
			}
			var proxy *plugin.Proxy
			if t != nil {
				proxy = &plugin.Proxy{URL: t.ProxyAddr(), CACert: absPath(t.CACertPath())}
			}
			active := func() (int64, int64, *tools.Registry) { return srv.ActiveRegistry() }
			pctx := plugin.EngagementContext(s, active, absPath(*dataDir), proxy)
			if err := plugin.DiscoverAndRegister(reg, pdir, plugin.DefaultTimeout, pctx); err != nil {
				slog.Warn("plugin loading failed", "dir", pdir, "err", err)
			}
		}
	}

	var srvOpts []mcpserver.Option
	srvOpts = append(srvOpts, mcpserver.WithEngagement(s, engCfg.TaskID, engCfg.ExplorationID))
	srvOpts = append(srvOpts, mcpserver.WithEvidence(evStore))
	if t != nil {
		srvOpts = append(srvOpts, mcpserver.WithTraffic(t))
	}
	var restAPI *api.API
	var runner *api.Runner
	var mcpToken string
	var webIdentity *identity.Signer
	if *webUI {
		authRequired := *authFlag || envEnabled("ARTEX_AUTH") || !isLoopbackAddr(*listen)
		apiOpts := []api.Option{api.WithDataDir(*dataDir), api.WithAuthRequired(authRequired), api.WithToolList(reg.Tools())}
		if *claudeBin != "" {
			apiOpts = append(apiOpts, api.WithClaudeBin(*claudeBin))
		}
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			slog.Error("generating mcp token failed", "err", err)
			os.Exit(1)
		}
		mcpToken = hex.EncodeToString(tokenBytes)
		mcpConfig, err := api.WriteMCPConfig(*dataDir, *listen, mcpToken)
		if err != nil {
			slog.Error("writing mcp config failed", "err", err)
			os.Exit(1)
		}
		webIdentity = loadIdentitySigner(*dataDir)
		runner = api.NewRunner(*claudeBin, mcpConfig, *dataDir, 0)
		runner.SetIdentitySigner(webIdentity)
		apiOpts = append(apiOpts, api.WithRunner(runner))
		restAPI = api.New(s, t, engCfg.TaskID, engCfg.ExplorationID, apiOpts...)
	}
	changeOrigin := store.NewChangeOrigin()
	srvOpts = append(srvOpts, mcpserver.WithNotifier(func(name string, args json.RawMessage) {
		if restAPI != nil {
			restAPI.NotifyChange(name)
		}
		taskID, expID := srv.ActiveEngagement()
		pubCtx, pubCancel := context.WithTimeout(ctx, 2*time.Second)
		defer pubCancel()
		if err := s.PublishChange(pubCtx, store.NewChange(changeOrigin, name, taskID, expID, args)); err != nil {
			slog.Warn("publishing change failed", "tool", name, "err", err)
		}
	}))
	srvOpts = append(srvOpts, mcpserver.WithVersion(strings.TrimPrefix(version, "v")))
	srv = mcpserver.New(reg, srvOpts...)
	if restAPI != nil {
		webExploration := func() int64 { return engCfg.ExplorationID }
		go store.ListenChanges(ctx, dsn, changeOrigin, webExploration, func(c store.Change) {
			restAPI.NotifyChange(c.Tool)
		})
	}
	if *stdio {
		slog.Info("serving", "transport", "stdio")
		if err := srv.ServeStdio(ctx); err != nil && ctx.Err() == nil {
			slog.Error("stdio server failed", "err", err)
			os.Exit(1)
		}
	} else if *webUI {
		mcpHandler := webMCPHandler(srv, mcpToken, webIdentity)
		handler := loopbackHostOnly(webHandler(mcpHandler, restAPI.Handler()))
		httpServer := &http.Server{Addr: *listen, Handler: handler}
		setupToken, err := restAPI.IssueSetupToken()
		if err != nil {
			slog.Error("generating setup token failed", "err", err)
			os.Exit(1)
		}
		if setupToken != "" {
			// Written to stderr directly so the token stays out of the /api/logs buffer.
			fmt.Fprintf(os.Stderr, "\nNo web UI password is set. Open %s and enter this one-time setup token:\n\n    %s\n\n", setupURL(*listen), setupToken)
		}
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
			sdCtx, sdCancel := context.WithTimeout(context.Background(), time.Duration(*shutdownSecs)*time.Second)
			defer sdCancel()
			httpServer.Shutdown(sdCtx)
			runner.StopAll()
		}
	} else {
		middleware := listenMiddleware(srv, loadIdentitySigner(*dataDir), listenToken)
		middleware = append(middleware, loopbackHostOnly, api.SecurityHeaders)
		slog.Info("serving", "addr", *listen, "mcp_auth", listenToken != "")
		if err := srv.ServeHTTP(ctx, *listen, middleware...); err != nil && ctx.Err() == nil {
			slog.Error("http server failed", "err", err)
			os.Exit(1)
		}
	}

	shutdown(t, s, time.Duration(*shutdownSecs)*time.Second)
}

func shutdown(t *traffic.Traffic, s *store.Store, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
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
	dryRun := fs.Bool("dry-run", false, "show what would be created without creating it")
	format := fs.String("format", "text", "output format: text or json")
	pg := pgDSN(fs)
	goal := fs.String("goal", "", "engagement goal")
	desc := fs.String("description", "", "target description")
	dataDir := fs.String("data", ".", "data directory for config file")
	fs.Parse(args)

	if *showVersion {
		fmt.Println("artex-cc " + version)
		return
	}

	if *format != "text" && *format != "json" {
		fmt.Fprintf(os.Stderr, "unsupported format: %s (use text or json)\n", *format)
		os.Exit(1)
	}

	initLogging(false)

	dsn := resolvePG(*pg)
	if dsn == "" {
		slog.Error("--pg or ARTEX_PG is required")
		os.Exit(1)
	}

	if *dryRun {
		cmdInitDryRun(*dataDir, dsn, *goal, *desc, *format)
		return
	}

	ctx := context.Background()

	s, err := store.Open(ctx, dsn)
	if err != nil {
		slog.Error("failed to open store", "err", err)
		os.Exit(1)
	}
	defer s.Close()

	if err := config.PrepareDataDir(*dataDir); err != nil {
		slog.Error("failed to create data directory", "path", *dataDir, "err", err)
		os.Exit(1)
	}

	certPath, err := generateCA(*dataDir)
	if err != nil {
		slog.Warn("failed to generate CA certificate", "err", err)
	}

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
		if err := s.DeleteEngagement(ctx, taskID); err != nil {
			slog.Error("failed to remove engagement", "task_id", taskID, "err", err)
		}
		os.Exit(1)
	}

	if *format == "json" {
		info := map[string]any{
			"task_id":        taskID,
			"exploration_id": explorationID,
			"config_path":    filepath.Join(*dataDir, ".artex.json"),
			"goal":           *goal,
			"description":    *desc,
		}
		if certPath != "" {
			info["ca_cert_path"] = certPath
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(info)
		return
	}

	fmt.Printf("Engagement created:\n")
	fmt.Printf("  Task ID:        %d\n", taskID)
	fmt.Printf("  Exploration ID: %d\n", explorationID)
	fmt.Printf("  Config written: %s/.artex.json\n", *dataDir)
	fmt.Println()
	if certPath != "" {
		fmt.Printf("  CA certificate: %s\n", certPath)
		fmt.Println()
		fmt.Println("Trust the CA certificate so the recording proxy can intercept HTTPS:")
		fmt.Println()
		fmt.Println("  Linux:   sudo cp " + certPath + " /usr/local/share/ca-certificates/artex.crt && sudo update-ca-certificates")
		fmt.Println("  macOS:   sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain " + certPath)
		fmt.Println("  Windows: certutil -addstore Root " + certPath)
		fmt.Println()
	}
	printMCPSnippets(os.Stdout, dsn, *dataDir)
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  --pg        PostgreSQL connection string (or set ARTEX_PG)")
	fmt.Println("  --data      directory for traffic captures, evidence, and config")
	fmt.Println("  --proxy     start the MITM recording proxy on ADDR (e.g. :8788)")
	fmt.Println("  --debug     enable debug-level logging")
}

// printMCPSnippets never writes the DSN password: init output often lands in
// logs (docker logs, CI), so a password must be filled in by the reader.
func printMCPSnippets(w io.Writer, dsn, dataDir string) {
	shown := store.RedactDSN(dsn)
	fmt.Fprintln(w, "Add to your Claude Code MCP config (~/.claude.json or project .mcp.json):")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Minimal (no proxy, data in cwd):")
	fmt.Fprintln(w)
	fmt.Fprintf(w, `  {
    "mcpServers": {
      "artex": {
        "command": "artex-cc",
        "args": ["serve", "--stdio", "--pg", %q]
      }
    }
  }`, shown)
	fmt.Fprintln(w)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Full (recording proxy + explicit data directory):")
	fmt.Fprintln(w)
	fmt.Fprintf(w, `  {
    "mcpServers": {
      "artex": {
        "command": "artex-cc",
        "args": [
          "serve", "--stdio",
          "--pg", %q,
          "--data", %q,
          "--proxy", ":8788"
        ]
      }
    }
  }`, shown, dataDir)
	fmt.Fprintln(w)
	if shown != dsn {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "The password in --pg is shown as ***. Replace it with the real one, or drop")
		fmt.Fprintln(w, `--pg and set it through the server's environment: "env": {"ARTEX_PG": "<dsn>"}.`)
	}
}

func cmdInitDryRun(dataDir, dsn, goal, desc, format string) {
	configPath := filepath.Join(dataDir, ".artex.json")
	caDir := filepath.Join(dataDir, "traffic", "_ca")
	caCertPath := filepath.Join(caDir, "mitmproxy-ca-cert.pem")
	caKeyPath := filepath.Join(caDir, "mitmproxy-ca.pem")

	type planned struct {
		Path   string `json:"path"`
		Action string `json:"action"`
	}
	var actions []planned

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		actions = append(actions, planned{configPath, "create"})
	} else {
		actions = append(actions, planned{configPath, "overwrite"})
	}

	if _, err := os.Stat(caKeyPath); os.IsNotExist(err) {
		actions = append(actions, planned{caKeyPath, "create"})
		actions = append(actions, planned{caCertPath, "create"})
	} else {
		actions = append(actions, planned{caCertPath, "exists"})
	}

	if format == "json" {
		out := map[string]any{
			"dry_run":     true,
			"pg_dsn":      store.RedactDSN(dsn),
			"goal":        goal,
			"description": desc,
			"actions":     actions,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(out)
		return
	}

	fmt.Println("Dry run: the following actions would be performed:")
	fmt.Println()
	fmt.Printf("  Database:  connect to PostgreSQL and create engagement\n")
	fmt.Printf("  Goal:      %s\n", goal)
	fmt.Printf("  Desc:      %s\n", desc)
	fmt.Println()
	for _, a := range actions {
		fmt.Printf("  [%s] %s\n", a.Action, a.Path)
	}
}

func cmdVersion(args []string) {
	fs := flag.NewFlagSet("version", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "output as JSON")
	fs.Parse(args)

	if *jsonOut {
		info := map[string]string{
			"version":    version,
			"commit":     commit,
			"build_date": buildDate,
			"go_version": runtime.Version(),
			"os":         runtime.GOOS,
			"arch":       runtime.GOARCH,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(info)
	} else {
		fmt.Println("artex-cc " + version)
	}
}
