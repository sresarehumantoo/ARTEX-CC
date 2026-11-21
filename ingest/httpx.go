package ingest

import (
	"encoding/json"
	"fmt"
	"strings"
)

type httpxResult struct {
	URL           string   `json:"url"`
	StatusCode    int      `json:"status_code"`
	ContentLength int64    `json:"content_length"`
	Title         string   `json:"title"`
	Tech          []string `json:"tech"`
	WebServer     string   `json:"webserver"`
	Host          string   `json:"host"`
	Port          string   `json:"port"`
	Scheme        string   `json:"scheme"`
}

// ParseHttpx parses httpx JSONL output into service asset results.
func ParseHttpx(data string) (*Result, error) {
	r := &Result{}
	seenURLs := map[string]bool{}

	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var hr httpxResult
		if err := json.Unmarshal([]byte(line), &hr); err != nil {
			continue
		}

		svcURL := hr.URL
		if svcURL == "" {
			continue
		}
		if seenURLs[svcURL] {
			continue
		}
		seenURLs[svcURL] = true

		techs := hr.Tech
		if hr.WebServer != "" {
			found := false
			for _, t := range techs {
				if strings.EqualFold(t, hr.WebServer) {
					found = true
					break
				}
			}
			if !found {
				techs = append(techs, hr.WebServer)
			}
		}

		port := 0
		if hr.Port != "" {
			fmt.Sscanf(hr.Port, "%d", &port)
		}

		r.Assets = append(r.Assets, Asset{
			Type:          "service",
			URL:           svcURL,
			IP:            hr.Host,
			Port:          port,
			Technologies:  techs,
			StatusCode:    hr.StatusCode,
			PageTitle:     hr.Title,
			ContentLength: hr.ContentLength,
		})
	}

	if len(r.Assets) == 0 {
		return nil, fmt.Errorf("no results found in httpx output")
	}
	return r, nil
}
