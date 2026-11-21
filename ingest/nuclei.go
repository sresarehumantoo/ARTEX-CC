package ingest

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

type nucleiResult struct {
	TemplateID       string     `json:"template-id"`
	Info             nucleiInfo `json:"info"`
	Host             string     `json:"host"`
	MatchedAt        string     `json:"matched-at"`
	ExtractedResults []string   `json:"extracted-results"`
	MatcherName      string     `json:"matcher-name"`
}

type nucleiInfo struct {
	Name        string   `json:"name"`
	Severity    string   `json:"severity"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

// ParseNuclei parses nuclei JSONL output into asset and finding results.
func ParseNuclei(data string) (*Result, error) {
	r := &Result{}
	seenHosts := map[string]bool{}

	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var nr nucleiResult
		if err := json.Unmarshal([]byte(line), &nr); err != nil {
			continue
		}

		host := nr.Host
		if nr.MatchedAt != "" {
			host = nr.MatchedAt
		}

		if host != "" && !seenHosts[host] {
			seenHosts[host] = true
			if u, err := url.Parse(host); err == nil && u.Scheme != "" {
				r.Assets = append(r.Assets, Asset{
					Type: "service",
					URL:  u.Scheme + "://" + u.Host,
				})
			}
		}

		sev := strings.ToLower(nr.Info.Severity)
		switch sev {
		case "critical", "high", "medium", "low":
		case "info":
			continue
		default:
			sev = "low"
		}

		summary := nr.Info.Name
		if nr.Info.Description != "" {
			summary += ": " + nr.Info.Description
		}
		if len(summary) > 500 {
			summary = summary[:497] + "..."
		}

		evidence := fmt.Sprintf("template: %s, host: %s", nr.TemplateID, host)
		if nr.MatcherName != "" {
			evidence += fmt.Sprintf(", matcher: %s", nr.MatcherName)
		}
		if len(nr.ExtractedResults) > 0 {
			evidence += fmt.Sprintf(", extracted: %s", strings.Join(nr.ExtractedResults, ", "))
		}

		vulnClass := nr.TemplateID
		if len(nr.Info.Tags) > 0 {
			vulnClass = nr.Info.Tags[0]
		}

		r.Findings = append(r.Findings, Finding{
			VulnClass: vulnClass,
			Name:      nr.Info.Name,
			Severity:  sev,
			Summary:   summary,
			Evidence:  evidence,
			Host:      host,
		})
	}

	if len(r.Assets) == 0 && len(r.Findings) == 0 {
		return nil, fmt.Errorf("no results found in nuclei output")
	}
	return r, nil
}
