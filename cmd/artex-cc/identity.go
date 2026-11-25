package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/config"
	"github.com/SREsAreHumanToo/ARTEX-CC/identity"
)

const identityHeader = "X-Artex-Identity"

func cmdIdentity(args []string) {
	os.Exit(runIdentity(args, os.Stdin, os.Stdout, os.Stderr))
}

func identityUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: artex-cc identity <command> [flags]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  issue   Print a signed identity token for an HTTP MCP client")
	fmt.Fprintln(w, "  rotate  Replace identity.key, invalidating every issued token")
	fmt.Fprintln(w, "  show    Print the key ID, or decode and verify a token")
}

func runIdentity(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		identityUsage(stderr)
		return 1
	}
	switch args[0] {
	case "issue":
		return identityIssue(args[1:], stdout, stderr)
	case "rotate":
		return identityRotate(args[1:], stdout, stderr)
	case "show":
		return identityShow(args[1:], stdin, stdout, stderr)
	case "-h", "--help", "help":
		identityUsage(stderr)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown identity command: %s\n", args[0])
		identityUsage(stderr)
		return 1
	}
}

func parseFlags(fs *flag.FlagSet, args []string, stderr io.Writer) (int, bool) {
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0, false
		}
		return 2, false
	}
	return 0, true
}

func requireDataDir(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("data directory %s: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("data directory %s is not a directory", dir)
	}
	return nil
}

// parseTTL accepts Go durations plus a whole-day form such as 7d.
func parseTTL(s string) (time.Duration, error) {
	if days, ok := strings.CutSuffix(s, "d"); ok {
		n, err := strconv.Atoi(days)
		if err != nil {
			return 0, fmt.Errorf("invalid TTL %q", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid TTL %q (use a duration such as 12h or 7d)", s)
	}
	return d, nil
}

type mcpClientConfig struct {
	MCPServers map[string]mcpClientServer `json:"mcpServers"`
}

type mcpClientServer struct {
	Type    string            `json:"type"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

func identityIssue(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("identity issue", flag.ContinueOnError)
	dataDir := fs.String("data", ".", "data directory holding identity.key")
	name := fs.String("name", "", "identity name, one per concurrently running agent (required)")
	ttlFlag := fs.String("ttl", "7d", "token lifetime, at most 30d (e.g. 12h, 7d)")
	format := fs.String("format", "token", "output format: token, header or claude-json")
	url := fs.String("url", "http://localhost:9400/mcp", "MCP endpoint URL for --format claude-json")
	bearerEnv := fs.String("bearer-env", "", "for --format claude-json, send Authorization: Bearer ${VAR} from this environment variable")
	if code, ok := parseFlags(fs, args, stderr); !ok {
		return code
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n", fs.Arg(0))
		return 2
	}
	switch *format {
	case "token", "header", "claude-json":
	default:
		fmt.Fprintf(stderr, "unsupported format: %s (use token, header or claude-json)\n", *format)
		return 1
	}
	if *name == "" {
		fmt.Fprintln(stderr, "--name is required")
		return 1
	}
	if err := identity.ValidateName(*name); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if identity.Reserved(*name) {
		fmt.Fprintf(stderr, "identity name %q uses a prefix reserved for built-in issuers (chat/, eval/)\n", *name)
		return 1
	}
	ttl, err := parseTTL(*ttlFlag)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if ttl <= 0 || ttl > identity.MaxTTL {
		fmt.Fprintf(stderr, "%v: got %s\n", identity.ErrInvalidTTL, *ttlFlag)
		return 1
	}
	if err := requireDataDir(*dataDir); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	signer, err := identity.NewSigner(*dataDir)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	token, err := signer.Issue(*name, ttl)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	claims, err := signer.Verify(token)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	switch *format {
	case "token":
		fmt.Fprintln(stdout, token)
	case "header":
		fmt.Fprintf(stdout, "%s: %s\n", identityHeader, token)
	case "claude-json":
		headers := map[string]string{identityHeader: token}
		if *bearerEnv != "" {
			headers["Authorization"] = "Bearer ${" + *bearerEnv + "}"
		}
		cfg := mcpClientConfig{MCPServers: map[string]mcpClientServer{
			"artex": {Type: "http", URL: *url, Headers: headers},
		}}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(cfg); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}
	fmt.Fprintf(stderr, "issued identity %q under key %s, expires %s\n",
		claims.Name, claims.KeyID, claims.ExpiresAt.UTC().Format(time.RFC3339))
	return 0
}

func identityRotate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("identity rotate", flag.ContinueOnError)
	dataDir := fs.String("data", ".", "data directory holding identity.key")
	if code, ok := parseFlags(fs, args, stderr); !ok {
		return code
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "unexpected argument: %s\n", fs.Arg(0))
		return 2
	}
	if err := requireDataDir(*dataDir); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	prev, next, err := config.RotateIdentityKey(*dataDir)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	path := filepath.Join(*dataDir, config.IdentityKeyFile)
	if prev == nil {
		fmt.Fprintf(stdout, "wrote %s: key ID %s\n", path, identity.KeyID(next))
		return 0
	}
	fmt.Fprintf(stdout, "rotated %s: key ID %s (was %s)\n", path, identity.KeyID(next), identity.KeyID(prev))
	fmt.Fprintln(stdout, "Every token issued under the old key is now rejected. Running servers pick up the new key within a second.")
	return 0
}

type tokenReport struct {
	Name      string `json:"name,omitempty"`
	KeyID     string `json:"key_id,omitempty"`
	IssuedAt  string `json:"issued_at,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
	Valid     bool   `json:"valid"`
	Error     string `json:"error,omitempty"`
}

type showReport struct {
	KeyID string       `json:"key_id"`
	Token *tokenReport `json:"token,omitempty"`
}

func identityShow(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("identity show", flag.ContinueOnError)
	dataDir := fs.String("data", ".", "data directory holding identity.key")
	format := fs.String("format", "text", "output format: text or json")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: artex-cc identity show [flags] [token | -]")
		fmt.Fprintln(stderr, "")
		fmt.Fprintln(stderr, "With - the token is read from stdin, which keeps it out of shell history.")
		fs.PrintDefaults()
	}
	if code, ok := parseFlags(fs, args, stderr); !ok {
		return code
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "unsupported format: %s (use text or json)\n", *format)
		return 1
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(stderr, "show takes at most one token")
		return 2
	}
	if err := requireDataDir(*dataDir); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if _, err := os.Stat(filepath.Join(*dataDir, config.IdentityKeyFile)); err != nil {
		fmt.Fprintf(stderr, "no %s in %s (identity issue or identity rotate creates it)\n", config.IdentityKeyFile, *dataDir)
		return 1
	}
	signer, err := identity.NewSigner(*dataDir)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	keyID, err := signer.KeyID()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	report := showReport{KeyID: keyID}

	if fs.NArg() == 1 {
		token := fs.Arg(0)
		if token == "-" {
			line, err := bufio.NewReader(io.LimitReader(stdin, 64<<10)).ReadString('\n')
			if err != nil && err != io.EOF {
				fmt.Fprintln(stderr, err)
				return 1
			}
			token = line
		}
		report.Token = inspectToken(signer, cleanToken(token))
	}

	if *format == "json" {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		enc.Encode(report)
	} else {
		writeShowText(stdout, report)
	}
	if report.Token != nil && !report.Token.Valid {
		return 1
	}
	return 0
}

// cleanToken accepts the bare token or the header line that issue
// --format header prints.
func cleanToken(s string) string {
	s = strings.TrimSpace(s)
	if name, value, ok := strings.Cut(s, ":"); ok && strings.EqualFold(strings.TrimSpace(name), identityHeader) {
		s = strings.TrimSpace(value)
	}
	return s
}

func inspectToken(signer *identity.Signer, token string) *tokenReport {
	r := &tokenReport{}
	claims, err := identity.Decode(token)
	if err != nil {
		r.Error = err.Error()
		return r
	}
	r.Name = claims.Name
	r.KeyID = claims.KeyID
	r.IssuedAt = claims.IssuedAt.UTC().Format(time.RFC3339)
	r.ExpiresAt = claims.ExpiresAt.UTC().Format(time.RFC3339)
	if _, err := signer.Verify(token); err != nil {
		r.Error = err.Error()
		return r
	}
	r.Valid = true
	return r
}

func writeShowText(w io.Writer, r showReport) {
	fmt.Fprintf(w, "Key ID:     %s\n", r.KeyID)
	t := r.Token
	if t == nil {
		return
	}
	if t.IssuedAt != "" {
		if identity.ValidateName(t.Name) == nil {
			fmt.Fprintf(w, "Name:       %s\n", t.Name)
		} else {
			fmt.Fprintf(w, "Name:       %q\n", t.Name)
		}
		fmt.Fprintf(w, "Token key:  %s\n", t.KeyID)
		fmt.Fprintf(w, "Issued:     %s\n", t.IssuedAt)
		fmt.Fprintf(w, "Expires:    %s\n", t.ExpiresAt)
	}
	if t.Valid {
		fmt.Fprintln(w, "Valid:      yes")
	} else {
		fmt.Fprintf(w, "Valid:      no (%s)\n", t.Error)
	}
}
