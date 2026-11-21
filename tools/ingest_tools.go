package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SREsAreHumanToo/cc-artex/ingest"
)

func RegisterIngestTools(reg *Registry, store Store, taskID, explorationID int64) {
	reg.Register(makeIngestScannerOutput(store, taskID, explorationID))
}

func makeIngestScannerOutput(store Store, taskID, explorationID int64) Tool {
	return Tool{
		Name: "ingest_scanner_output",
		Description: "Parse output from common security scanners and import the results as assets and/or findings. " +
			"Supported scanners: nmap (XML or greppable), nuclei (JSONL), httpx (JSONL), ffuf (JSON).",
		Schema: obj(map[string]any{
			"scanner": enm([]string{"nmap", "nuclei", "httpx", "ffuf"}, "scanner that produced the output"),
			"output":  str("the scanner output to parse"),
		}, "scanner", "output"),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				Scanner string `json:"scanner"`
				Output  string `json:"output"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			scanner := strings.ToLower(strings.TrimSpace(a.Scanner))
			if scanner == "" {
				return errResult("scanner is required"), nil
			}
			output := strings.TrimSpace(a.Output)
			if output == "" {
				return errResult("output is required"), nil
			}

			var result *ingest.Result
			var err error
			switch scanner {
			case "nmap":
				result, err = ingest.ParseNmap(output)
			case "nuclei":
				result, err = ingest.ParseNuclei(output)
			case "httpx":
				result, err = ingest.ParseHttpx(output)
			case "ffuf":
				result, err = ingest.ParseFfuf(output)
			default:
				return errResult(fmt.Sprintf("unsupported scanner: %s", scanner)), nil
			}
			if err != nil {
				return errResult(fmt.Sprintf("parse %s output: %v", scanner, err)), nil
			}

			assetsCreated := 0
			findingsCreated := 0
			var errors []string

			for _, asset := range result.Assets {
				input := ingestAssetToInput(asset)
				if _, err := store.InsertAsset(ctx, taskID, input); err != nil {
					errors = append(errors, fmt.Sprintf("asset %s: %v", ingestAssetRef(asset), err))
					continue
				}
				assetsCreated++
			}

			for _, finding := range result.Findings {
				input := FindingInput{
					TaskID:        taskID,
					ExplorationID: explorationID,
					VulnClass:     finding.VulnClass,
					Name:          finding.Name,
					Severity:      finding.Severity,
					Summary:       finding.Summary,
					Evidence:      finding.Evidence,
					Worker:        "scanner:" + scanner,
				}
				if _, err := store.AddFinding(ctx, input); err != nil {
					errors = append(errors, fmt.Sprintf("finding %s: %v", finding.Name, err))
					continue
				}
				findingsCreated++
			}

			out := map[string]any{
				"assets_created":   assetsCreated,
				"findings_created": findingsCreated,
			}
			if len(errors) > 0 {
				out["errors"] = errors
			}
			return jsonResult(out)
		},
	}
}

func ingestAssetToInput(a ingest.Asset) AssetInput {
	input := AssetInput{
		Type:   a.Type,
		Domain: a.Domain,
		IP:     a.IP,
	}
	if a.Port > 0 {
		input.Port = a.Port
	}
	if a.Proto != "" {
		input.Proto = a.Proto
	}
	if a.ServiceName != "" {
		input.ServiceName = a.ServiceName
	}
	if a.URL != "" {
		input.URL = a.URL
	}
	if a.Method != "" {
		input.Method = a.Method
	}
	if len(a.Technologies) > 0 {
		input.Technologies = a.Technologies
	}
	if a.StatusCode > 0 {
		sc := a.StatusCode
		input.StatusCode = &sc
	}
	if a.PageTitle != "" {
		input.PageTitle = a.PageTitle
	}
	if a.ContentLength > 0 {
		cl := a.ContentLength
		input.ContentLength = &cl
	}
	return input
}

func ingestAssetRef(a ingest.Asset) string {
	if a.URL != "" {
		return a.URL
	}
	if a.Domain != "" {
		return a.Domain
	}
	if a.IP != "" {
		return a.IP
	}
	return a.Type
}
