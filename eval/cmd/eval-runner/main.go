package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/SREsAreHumanToo/ARTEX-CC/store"
	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

type Scenario struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Target      string   `yaml:"target"`
	Model       string   `yaml:"model"`
	Goal        string   `yaml:"goal"`
	Scope       []string `yaml:"scope"`
	Credentials struct {
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	} `yaml:"credentials"`
	Expected   ExpectedFindings `yaml:"expected_findings"`
	Assets     ExpectedAssets   `yaml:"expected_assets"`
	TimeoutRaw string           `yaml:"timeout"`
}

type ExpectedFindings struct {
	MinCount    int               `yaml:"min_count"`
	MustInclude []ExpectedFinding `yaml:"must_include"`
	MayInclude  []ExpectedFinding `yaml:"may_include"`
}

type ExpectedFinding struct {
	Type        string   `yaml:"type"`
	Description string   `yaml:"description"`
	Endpoints   []string `yaml:"endpoints"`
}

type ExpectedAssets struct {
	MinCount int      `yaml:"min_count"`
	Types    []string `yaml:"types"`
}

type Scorecard struct {
	Scenario        string          `json:"scenario"`
	Target          string          `json:"target"`
	StartedAt       string          `json:"started_at"`
	FinishedAt      string          `json:"finished_at"`
	DurationSeconds float64         `json:"duration_seconds"`
	TimedOut        bool            `json:"timed_out"`
	FindingsTotal   int             `json:"findings_total"`
	FindingsMatched int             `json:"findings_matched"`
	FindingsMissed  []string        `json:"findings_missed"`
	AssetsTotal     int             `json:"assets_total"`
	CoverageRecords int             `json:"coverage_records"`
	GoalsSet        int             `json:"goals_set"`
	IntentsCreated  int             `json:"intents_created"`
	IntentsFinished int             `json:"intents_finished"`
	Score           float64         `json:"score"`
	Details         ScorecardDetail `json:"details"`
	Instances       int             `json:"instances"`
	Transport       string          `json:"transport"`
	Lease           string          `json:"lease,omitempty"`
	Identity        bool            `json:"identity,omitempty"`
	TranscriptsDir  string          `json:"transcripts_dir,omitempty"`
	Runs            []InstanceRun   `json:"runs,omitempty"`
	Coordination    *Coordination   `json:"coordination,omitempty"`
}

type ScorecardDetail struct {
	FindingTypeScore  float64 `json:"finding_type_score"`
	FindingCountScore float64 `json:"finding_count_score"`
	AssetScore        float64 `json:"asset_score"`
	IntentScore       float64 `json:"intent_score"`
}

func main() {
	pgDSN := flag.String("pg", "", "PostgreSQL connection string (or ARTEX_PG env)")
	artexBin := flag.String("artex", "artex-cc", "path to artex-cc binary")
	claudeBin := flag.String("claude", "claude", "path to claude CLI binary")
	outputDir := flag.String("output", ".", "directory for scorecard output")
	skipClaude := flag.Bool("dry-run", false, "set up engagement but skip the Claude run (useful for testing the harness)")
	targetOverride := flag.String("target", "", "override scenario target (URL or IP)")
	modelOverride := flag.String("model", "", "override scenario model for Claude CLI")
	mcpConfigFlag := flag.String("mcp-config", "", "path to existing MCP config file (skip generating one)")
	skipHealth := flag.Bool("skip-health", false, "skip the target health check")
	instances := flag.Int("instances", 1, "number of concurrent claude instances on the engagement")
	transport := flag.String("transport", "stdio", "MCP transport: stdio (one serve process per instance) or http (one shared serve --listen)")
	killAfter := flag.String("kill-after", "", "kill one instance after a delay, as INSTANCE:DURATION (e.g. 2:3m)")
	lease := flag.String("lease", "", "intent lease passed to every serve process as ARTEX_INTENT_LEASE (needs a server that supports it)")
	var extraEnv envFlag
	flag.Var(&extraEnv, "env", "KEY=VALUE added to every claude instance's environment; repeatable (e.g. TELEPORT_HOME=/path/to/empty/dir)")
	identityFlag := flag.Bool("identity", true, "with --transport http, give each instance its own X-Artex-Identity instead of asking it to pick an owner (false keeps the anonymous owner path)")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: eval-runner [flags] <scenario.yml>")
		os.Exit(1)
	}

	dsn := *pgDSN
	if dsn == "" {
		dsn = os.Getenv("ARTEX_PG")
	}
	if dsn == "" {
		dsn = "postgres://ccartex_eval:ccartex_eval@localhost:5433/ccartex_eval?sslmode=disable"
	}

	scenario, timeout, err := loadScenario(flag.Arg(0))
	if err != nil {
		log.Fatalf("load scenario: %v", err)
	}

	if *instances < 1 {
		log.Fatalf("--instances must be at least 1")
	}
	if *transport != "stdio" && *transport != "http" {
		log.Fatalf("--transport must be stdio or http, got %q", *transport)
	}
	kill, err := parseKillAfter(*killAfter, *instances)
	if err != nil {
		log.Fatalf("--kill-after: %v", err)
	}
	if *lease != "" {
		if _, err := time.ParseDuration(*lease); err != nil {
			log.Fatalf("--lease: %v", err)
		}
	}

	if *targetOverride != "" {
		scenario.Target = *targetOverride
	}
	if *modelOverride != "" {
		scenario.Model = *modelOverride
	}

	slog.Info("loaded scenario", "name", scenario.Name, "target", scenario.Target, "model", scenario.Model, "timeout", timeout)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if !*skipHealth {
		if err := waitForTarget(ctx, scenario.Target, 60*time.Second); err != nil {
			log.Fatalf("target not reachable: %v", err)
		}
		slog.Info("target is reachable", "target", scenario.Target)
	} else {
		slog.Info("skipping health check", "target", scenario.Target)
	}

	s, err := store.Open(ctx, dsn)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer s.Close()

	taskID, explorationID, err := s.CreateEngagement(ctx, scenario.Goal, scenario.Description)
	if err != nil {
		log.Fatalf("create engagement: %v", err)
	}
	slog.Info("engagement created", "task_id", taskID, "exploration_id", explorationID)

	for _, sc := range scenario.Scope {
		if err := s.AddTaskScope(ctx, taskID, scopeKind(sc), sc, "eval scenario", "manual"); err != nil {
			log.Fatalf("add scope %q: %v", sc, err)
		}
	}

	dataDir, err := os.MkdirTemp("", "eval-runner-*")
	if err != nil {
		log.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(dataDir)

	if err := writeEngagementConfig(dataDir, taskID, explorationID, scenario); err != nil {
		log.Fatalf("write engagement config: %v", err)
	}

	startedAt := time.Now()
	stem := fmt.Sprintf("%s-%s", sanitize(scenario.Name), startedAt.Format("20060102-150405"))
	rc := runConfig{
		claudeBin:      *claudeBin,
		artexBin:       *artexBin,
		dsn:            dsn,
		dataDir:        dataDir,
		mcpConfig:      *mcpConfigFlag,
		instances:      *instances,
		transport:      *transport,
		lease:          *lease,
		extraEnv:       extraEnv,
		identity:       *identityFlag && *transport == "http" && *mcpConfigFlag == "",
		runID:          startedAt.Format("20060102-150405"),
		kill:           kill,
		transcriptsDir: filepath.Join(*outputDir, "transcripts-"+stem),
	}

	var runs []InstanceRun
	if !*skipClaude {
		runs, err = runInstances(ctx, rc, scenario, timeout)
		if err != nil {
			slog.Warn("claude run ended", "err", err)
		}
	} else {
		slog.Info("dry-run mode: skipping Claude execution")
	}

	finishedAt := time.Now()
	timedOut := finishedAt.Sub(startedAt) >= timeout

	scorecard, err := score(ctx, s, explorationID, taskID, scenario, startedAt, finishedAt, timedOut)
	if err != nil {
		log.Fatalf("scoring failed: %v", err)
	}
	scorecard.Instances = *instances
	scorecard.Transport = *transport
	scorecard.Lease = *lease
	scorecard.Identity = rc.identity
	scorecard.Runs = runs
	if len(runs) > 0 {
		scorecard.TranscriptsDir = rc.transcriptsDir
		leaseDur, _ := time.ParseDuration(*lease)
		renewal := *mcpConfigFlag == "" && serverRenewsClaims(ctx, *artexBin)
		coord, err := scoreRun(ctx, s, explorationID, runs, leaseDur, renewal)
		if err != nil {
			slog.Warn("coordination scoring failed", "err", err)
		} else {
			scorecard.Coordination = coord
			summary := coord.Summary()
			fmt.Fprint(os.Stderr, "\n"+summary+"\n")
			if err := os.WriteFile(filepath.Join(rc.transcriptsDir, "coordination.txt"), []byte(summary), 0600); err != nil {
				slog.Warn("failed to write coordination summary", "err", err)
			}
		}
	}

	out, _ := json.MarshalIndent(scorecard, "", "  ")
	fmt.Println(string(out))

	outPath := filepath.Join(*outputDir, "scorecard-"+stem+".json")
	if err := os.WriteFile(outPath, out, 0644); err != nil {
		slog.Warn("failed to write scorecard file", "path", outPath, "err", err)
	} else {
		slog.Info("scorecard written", "path", outPath)
	}
}

func loadScenario(path string) (Scenario, time.Duration, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Scenario{}, 0, err
	}
	var sc Scenario
	if err := yaml.Unmarshal(data, &sc); err != nil {
		return Scenario{}, 0, fmt.Errorf("parse %s: %w", path, err)
	}
	timeout := 15 * time.Minute
	if sc.TimeoutRaw != "" {
		d, err := time.ParseDuration(sc.TimeoutRaw)
		if err != nil {
			return Scenario{}, 0, fmt.Errorf("parse timeout %q: %w", sc.TimeoutRaw, err)
		}
		timeout = d
	}
	return sc, timeout, nil
}

func waitForTarget(ctx context.Context, target string, maxWait time.Duration) error {
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		return waitForHTTP(ctx, target, maxWait)
	}
	return waitForTCP(ctx, target, maxWait)
}

func waitForHTTP(ctx context.Context, target string, maxWait time.Duration) error {
	deadline := time.Now().Add(maxWait)
	healthURL := strings.TrimRight(target, "/")
	for time.Now().Before(deadline) {
		req, _ := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode < 500 {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return fmt.Errorf("target %s not reachable after %s", target, maxWait)
}

func waitForTCP(ctx context.Context, host string, maxWait time.Duration) error {
	ports := []string{"22", "445", "88", "80", "443"}
	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		for _, port := range ports {
			addr := net.JoinHostPort(host, port)
			conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
			if err == nil {
				conn.Close()
				slog.Info("TCP port open", "addr", addr)
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return fmt.Errorf("target %s not reachable on any common port after %s", host, maxWait)
}

func writeEngagementConfig(dir string, taskID, explorationID int64, sc Scenario) error {
	cfg := map[string]any{
		"task_id":        taskID,
		"exploration_id": explorationID,
		"goal":           sc.Goal,
		"description":    sc.Description,
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ".artex.json"), data, 0600)
}

func score(ctx context.Context, s *store.Store, explorationID, taskID int64, sc Scenario, startedAt, finishedAt time.Time, timedOut bool) (*Scorecard, error) {
	overview, err := s.GraphOverview(ctx, explorationID)
	if err != nil {
		return nil, fmt.Errorf("graph overview: %w", err)
	}

	findings, err := s.ListFindings(ctx, explorationID)
	if err != nil {
		return nil, fmt.Errorf("list findings: %w", err)
	}

	assets, _, err := s.ListAssets(ctx, tools.AssetFilter{TaskID: taskID, Limit: 1000})
	if err != nil {
		return nil, fmt.Errorf("list assets: %w", err)
	}

	finishedIntents, _ := s.CountFinishedIntents(ctx, explorationID)
	openIntents, _ := s.CountOpenIntents(ctx, explorationID)

	matched, missed := matchFindings(findings, sc.Expected.MustInclude)

	findingTypeScore := 0.0
	if len(sc.Expected.MustInclude) > 0 {
		findingTypeScore = float64(matched) / float64(len(sc.Expected.MustInclude))
	}

	findingCountScore := 0.0
	if sc.Expected.MinCount > 0 {
		ratio := float64(len(findings)) / float64(sc.Expected.MinCount)
		if ratio > 1 {
			ratio = 1
		}
		findingCountScore = ratio
	}

	assetScore := 0.0
	if sc.Assets.MinCount > 0 {
		ratio := float64(len(assets)) / float64(sc.Assets.MinCount)
		if ratio > 1 {
			ratio = 1
		}
		assetScore = ratio
	} else if len(assets) > 0 {
		assetScore = 1.0
	}

	intentScore := 0.0
	totalIntents := finishedIntents + openIntents
	if totalIntents > 0 {
		intentScore = float64(finishedIntents) / float64(totalIntents)
	}

	overallScore := findingTypeScore*0.40 + findingCountScore*0.25 + assetScore*0.15 + intentScore*0.20

	coverageCount := coverageEntries(overview.Coverage)

	return &Scorecard{
		Scenario:        sc.Name,
		Target:          sc.Target,
		StartedAt:       startedAt.Format(time.RFC3339),
		FinishedAt:      finishedAt.Format(time.RFC3339),
		DurationSeconds: finishedAt.Sub(startedAt).Seconds(),
		TimedOut:        timedOut,
		FindingsTotal:   len(findings),
		FindingsMatched: matched,
		FindingsMissed:  missed,
		AssetsTotal:     len(assets),
		CoverageRecords: coverageCount,
		GoalsSet:        len(overview.Goals),
		IntentsCreated:  totalIntents,
		IntentsFinished: finishedIntents,
		Score:           overallScore,
		Details: ScorecardDetail{
			FindingTypeScore:  findingTypeScore,
			FindingCountScore: findingCountScore,
			AssetScore:        assetScore,
			IntentScore:       intentScore,
		},
	}, nil
}

// envFlag collects repeatable --env KEY=VALUE flags.
type envFlag []string

func (e *envFlag) String() string { return strings.Join(*e, ",") }

func (e *envFlag) Set(v string) error {
	k, _, ok := strings.Cut(v, "=")
	if !ok || strings.TrimSpace(k) == "" {
		return fmt.Errorf("want KEY=VALUE, got %q", v)
	}
	*e = append(*e, v)
	return nil
}

// scopeKind picks the task scope kind for a scenario scope entry: a CIDR, an
// IP (optionally with a port), or otherwise one exact host, which subdomain
// scope matches (port stripped) without widening to a registrable domain.
func scopeKind(entry string) string {
	entry = strings.TrimSpace(entry)
	if _, _, err := net.ParseCIDR(entry); err == nil {
		return "cidr"
	}
	host := entry
	if h, _, err := net.SplitHostPort(entry); err == nil {
		host = h
	}
	if net.ParseIP(strings.Trim(host, "[]")) != nil {
		return "ip"
	}
	return "subdomain"
}

// coverageEntries reads the ledger row count from graph_overview's coverage
// map: an int straight from the store, a float64 after a JSON round trip.
func coverageEntries(cov map[string]any) int {
	switch n := cov["entries"].(type) {
	case int:
		return n
	case float64:
		return int(n)
	}
	return 0
}

func matchFindings(findings []tools.Finding, expected []ExpectedFinding) (matched int, missed []string) {
	for _, exp := range expected {
		found := false
		for _, f := range findings {
			if matchesFindingType(f, exp.Type) {
				found = true
				break
			}
		}
		if found {
			matched++
		} else {
			missed = append(missed, exp.Type)
		}
	}
	return matched, missed
}

func matchesFindingType(f tools.Finding, expectedType string) bool {
	norm := tools.NormalizeVulnClass(expectedType)
	vulnNorm := tools.NormalizeVulnClass(f.VulnClass)
	summaryNorm := strings.ToLower(f.Summary)

	if strings.Contains(vulnNorm, norm) || strings.Contains(norm, vulnNorm) {
		return true
	}

	aliases := map[string][]string{
		"sqlinjection":             {"sqli", "sql injection", "sql_injection"},
		"xss":                      {"cross-site scripting", "cross site scripting", "reflected xss", "stored xss", "dom xss"},
		"commandinjection":         {"command injection", "os command", "rce", "remote code execution"},
		"directorytraversal":       {"path traversal", "directory traversal", "lfi", "local file inclusion", "file read"},
		"ssrf":                     {"server-side request forgery", "server side request forgery", "ssrf"},
		"brokenaccesscontrol":      {"access control", "idor", "insecure direct object", "unauthorized", "authentication bypass", "missing auth", "unauthenticated"},
		"sensitivedataexposure":    {"sensitive data", "information disclosure", "data exposure", "data leak"},
		"hardcodedcredentials":     {"hardcoded", "default credentials", "weak password", "default password"},
		"fileinclusion":            {"file inclusion", "lfi", "rfi"},
		"csrf":                     {"cross-site request forgery", "csrf"},
		"fileupload":               {"file upload", "unrestricted upload"},
		"bruteforce":               {"brute force", "brute_force"},
		"securitymisconfiguration": {"misconfiguration", "misconfig"},
		"injection":                {"injection"},
	}

	for key, synonyms := range aliases {
		if norm == key || strings.Contains(key, norm) {
			for _, syn := range synonyms {
				if strings.Contains(vulnNorm, strings.ReplaceAll(syn, " ", "")) ||
					strings.Contains(summaryNorm, syn) {
					return true
				}
			}
		}
	}
	return false
}

func sanitize(s string) string {
	r := strings.NewReplacer(" ", "-", "/", "-", "\\", "-")
	return strings.ToLower(r.Replace(s))
}
