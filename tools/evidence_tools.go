package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/evidence"
)

// RegisterEvidenceTools registers evidence management tools.
func RegisterEvidenceTools(reg *Registry, evidenceStore *evidence.Store, store Store, taskID int64) {
	reg.Register(makeSaveEvidence(evidenceStore, store, taskID))
	reg.Register(makeListEvidence(evidenceStore, taskID))
	reg.Register(makeGetEvidence(evidenceStore))
	reg.Register(makeCaptureCommandOutput(evidenceStore, store, taskID))
}

func makeSaveEvidence(es *evidence.Store, store Store, taskID int64) Tool {
	return Tool{
		Name:     "save_evidence",
		Category: CategoryEvidence,
		Description: "Save a piece of evidence (screenshot, PoC output, config extract, etc.) " +
			"linked to a finding. Content is base64-encoded.",
		Schema: obj(map[string]any{
			"finding_id":     intp("finding to attach this evidence to"),
			"kind":           enm([]string{"screenshot", "poc_output", "request_response", "config_file", "log_extract", "other"}, "type of evidence"),
			"filename":       str("original filename for the artifact"),
			"content_base64": str("base64-encoded file content"),
		}, "finding_id", "kind", "filename", "content_base64"),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				FindingID     json.RawMessage `json:"finding_id"`
				Kind          string          `json:"kind"`
				Filename      string          `json:"filename"`
				ContentBase64 string          `json:"content_base64"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}

			findingID := pid(a.FindingID)
			if findingID <= 0 {
				return errResult("finding_id is required"), nil
			}
			if strings.TrimSpace(a.Kind) == "" {
				return errResult("kind is required"), nil
			}
			if strings.TrimSpace(a.Filename) == "" {
				return errResult("filename is required"), nil
			}
			if strings.TrimSpace(a.ContentBase64) == "" {
				return errResult("content_base64 is required"), nil
			}

			const maxEvidenceBase64Len = 50 * 1024 * 1024 * 4 / 3
			if len(a.ContentBase64) > maxEvidenceBase64Len {
				return errResult("evidence too large (50 MB limit)"), nil
			}

			exists, err := store.FindingExists(ctx, findingID)
			if err != nil {
				return errResult("check finding: " + err.Error()), nil
			}
			if !exists {
				return errResult(fmt.Sprintf("finding %d not found", findingID)), nil
			}

			if len(a.ContentBase64) > maxEvidenceBase64Len {
				return errResult(fmt.Sprintf("content_base64 exceeds %dMB limit", maxEvidenceBytes/(1024*1024))), nil
			}

			data, err := base64.StdEncoding.DecodeString(a.ContentBase64)
			if err != nil {
				return errResult("invalid base64: " + err.Error()), nil
			}

			ev, err := es.Save(ctx, taskID, findingID, a.Kind, a.Filename, bytes.NewReader(data))
			if err != nil {
				return errResult("save evidence: " + err.Error()), nil
			}

			return jsonResult(map[string]any{
				"id":     ev.ID,
				"sha256": ev.SHA256,
				"size":   ev.Size,
				"path":   ev.Filename,
			})
		},
	}
}

func makeListEvidence(es *evidence.Store, taskID int64) Tool {
	return Tool{
		Name:        "list_evidence",
		Category:    CategoryEvidence,
		Description: "List evidence artifacts for the current engagement, optionally filtered by finding.",
		ReadOnly:    true,
		Schema: obj(map[string]any{
			"finding_id": intp("filter by finding id (optional)"),
		}),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				FindingID json.RawMessage `json:"finding_id"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}

			var findingIDPtr *int64
			if fid := pid(a.FindingID); fid > 0 {
				findingIDPtr = &fid
			}

			items, err := es.List(ctx, taskID, findingIDPtr)
			if err != nil {
				return errResult("list evidence: " + err.Error()), nil
			}

			out := make([]map[string]any, 0, len(items))
			for _, ev := range items {
				out = append(out, map[string]any{
					"id":         ev.ID,
					"finding_id": ev.FindingID,
					"kind":       ev.Kind,
					"filename":   ev.Filename,
					"size":       ev.Size,
					"sha256":     ev.SHA256,
					"created_at": ev.CreatedAt,
				})
			}
			return jsonResult(map[string]any{
				"count":    len(out),
				"evidence": out,
			})
		},
	}
}

func makeGetEvidence(es *evidence.Store) Tool {
	return Tool{
		Name:        "get_evidence",
		Category:    CategoryEvidence,
		Description: "Read evidence content by ID. Returns base64-encoded data.",
		ReadOnly:    true,
		Schema: obj(map[string]any{
			"id": str("evidence id"),
		}, "id"),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			if strings.TrimSpace(a.ID) == "" {
				return errResult("id is required"), nil
			}

			ev, err := es.Get(ctx, a.ID)
			if err != nil {
				return errResult(err.Error()), nil
			}

			rc, err := es.Read(ctx, a.ID)
			if err != nil {
				return errResult(err.Error()), nil
			}
			defer rc.Close()

			var buf bytes.Buffer
			if _, err := buf.ReadFrom(rc); err != nil {
				return errResult("read evidence: " + err.Error()), nil
			}

			return jsonResult(map[string]any{
				"id":             ev.ID,
				"filename":       ev.Filename,
				"kind":           ev.Kind,
				"content_base64": base64.StdEncoding.EncodeToString(buf.Bytes()),
				"sha256":         ev.SHA256,
			})
		},
	}
}

const (
	maxEvidenceBytes     = 50 * 1024 * 1024
	maxEvidenceBase64Len = maxEvidenceBytes * 4 / 3
	commandTimeout       = 30 * time.Second
)

func makeCaptureCommandOutput(es *evidence.Store, store Store, taskID int64) Tool {
	return Tool{
		Name:     "capture_command_output",
		Category: CategoryEvidence,
		Description: "Run a command and save its combined stdout+stderr as evidence (kind=poc_output). " +
			"The command must only target in-scope hosts. Returns evidence ID, exit code, and an output preview.",
		Schema: obj(map[string]any{
			"finding_id":  intp("finding to attach this evidence to"),
			"command":     str("shell command to execute"),
			"description": str("what this command demonstrates"),
		}, "finding_id", "command"),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				FindingID   json.RawMessage `json:"finding_id"`
				Command     string          `json:"command"`
				Description string          `json:"description"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}

			findingID := pid(a.FindingID)
			if findingID <= 0 {
				return errResult("finding_id is required"), nil
			}
			if strings.TrimSpace(a.Command) == "" {
				return errResult("command is required"), nil
			}

			exists, err := store.FindingExists(ctx, findingID)
			if err != nil {
				return errResult("check finding: " + err.Error()), nil
			}
			if !exists {
				return errResult(fmt.Sprintf("finding %d not found", findingID)), nil
			}

			targets := extractTargets(a.Command)
			for _, target := range targets {
				inScope, reason, _, err := store.IsInScope(ctx, taskID, target)
				if err != nil {
					return errResult(fmt.Sprintf("scope check for %q: %s", target, err.Error())), nil
				}
				if !inScope {
					return errResult(fmt.Sprintf("command targets out-of-scope host %q: %s", target, reason)), nil
				}
			}

			cmdCtx, cancel := context.WithTimeout(ctx, commandTimeout)
			defer cancel()

			cmd := exec.CommandContext(cmdCtx, "sh", "-c", a.Command)
			output, execErr := cmd.CombinedOutput()

			exitCode := 0
			if execErr != nil {
				if exitErr, ok := execErr.(*exec.ExitError); ok {
					exitCode = exitErr.ExitCode()
				} else if cmdCtx.Err() != nil {
					return errResult("command timed out after 30s"), nil
				} else {
					return errResult("exec: " + execErr.Error()), nil
				}
			}

			desc := a.Description
			if desc == "" {
				desc = a.Command
			}
			filename := "poc_" + sanitizeForFilename(desc) + ".txt"

			header := fmt.Sprintf("$ %s\n# exit_code: %d\n# description: %s\n\n", a.Command, exitCode, desc)
			content := header + string(output)

			ev, err := es.Save(ctx, taskID, findingID, "poc_output", filename, strings.NewReader(content))
			if err != nil {
				return errResult("save evidence: " + err.Error()), nil
			}

			preview := string(output)
			if len(preview) > 500 {
				preview = preview[:500]
			}

			return jsonResult(map[string]any{
				"evidence_id":    ev.ID,
				"exit_code":      exitCode,
				"output_preview": preview,
			})
		},
	}
}

var (
	ipv4Re   = regexp.MustCompile(`\b(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})\b`)
	domainRe = regexp.MustCompile(`\b([a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\.[a-zA-Z]{2,})+)\b`)
)

func extractTargets(command string) []string {
	seen := map[string]bool{}
	var targets []string

	for _, m := range ipv4Re.FindAllString(command, -1) {
		ip := net.ParseIP(m)
		if ip == nil {
			continue
		}
		if ip.IsLoopback() || ip.IsUnspecified() {
			continue
		}
		if !seen[m] {
			seen[m] = true
			targets = append(targets, m)
		}
	}

	for _, m := range domainRe.FindAllString(command, -1) {
		m = strings.ToLower(m)
		if m == "github.com" || m == "golang.org" {
			continue
		}
		if !seen[m] {
			seen[m] = true
			targets = append(targets, m)
		}
	}

	return targets
}

func sanitizeForFilename(s string) string {
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, s)
	if len(s) > 50 {
		s = s[:50]
	}
	return s
}
