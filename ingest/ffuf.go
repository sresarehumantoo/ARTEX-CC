package ingest

import (
	"encoding/json"
	"fmt"
	"strings"
)

type ffufOutput struct {
	Results []ffufResult `json:"results"`
	Config  ffufConfig   `json:"config"`
}

type ffufResult struct {
	Status int    `json:"status"`
	Length int64  `json:"length"`
	URL    string `json:"url"`
}

type ffufConfig struct {
	URL    string `json:"url"`
	Method string `json:"method"`
}

// ParseFfuf parses ffuf JSON output into endpoint asset results.
func ParseFfuf(data string) (*Result, error) {
	data = strings.TrimSpace(data)

	var output ffufOutput
	if err := json.Unmarshal([]byte(data), &output); err != nil {
		return nil, fmt.Errorf("parse ffuf JSON: %w", err)
	}

	if len(output.Results) == 0 {
		return nil, fmt.Errorf("no results found in ffuf output")
	}

	r := &Result{}
	method := strings.ToUpper(output.Config.Method)
	if method == "" {
		method = "GET"
	}

	for _, fr := range output.Results {
		if fr.URL == "" {
			continue
		}
		r.Assets = append(r.Assets, Asset{
			Type:          "endpoint",
			URL:           fr.URL,
			Method:        method,
			StatusCode:    fr.Status,
			ContentLength: fr.Length,
		})
	}

	if len(r.Assets) == 0 {
		return nil, fmt.Errorf("no endpoints found in ffuf output")
	}
	return r, nil
}
