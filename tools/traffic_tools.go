package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SREsAreHumanToo/ARTEX-CC/traffic"
)

func RegisterTrafficTools(reg *Registry, t *traffic.Traffic, store Store, taskID int64) {
	reg.Register(makeTrafficSearch(t))
	reg.Register(makeTrafficGet(t))
	reg.Register(makeTrafficBlob(t))
	reg.Register(makeTrafficMessages(t))
	reg.Register(makeTrafficReplay(t, store, taskID))
	reg.Register(makeTrafficHosts(t))
	reg.Register(makeTrafficHARExport(t))
	reg.Register(makeTrafficStats(t))
	reg.Register(makeTrafficTimeline(t))
	reg.Register(makeTrafficPatterns(t))
	reg.Register(makeTrafficClear(t))
	reg.Register(makeBindFindingTraffic(store, t.Exists))
}

func makeTrafficPatterns(t *traffic.Traffic) Tool {
	return Tool{
		Name:        "traffic_patterns",
		Category:    CategoryTraffic,
		Description: "Identify traffic patterns: most-requested paths (by url_template), hosts with the most HTTP errors (4xx/5xx with status breakdown), and endpoints that appeared recently. Optionally filter by host.",
		ReadOnly:    true,
		Schema: obj(map[string]any{
			"host": str("host substring filter (optional; empty analyzes all traffic)"),
		}),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				Host string `json:"host"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			patterns, err := t.Patterns(a.Host)
			if err != nil {
				return errResult(err.Error()), nil
			}
			return jsonResult(patterns)
		},
	}
}

func makeTrafficSearch(t *traffic.Traffic) Tool {
	return Tool{
		Name:        "traffic_search",
		Category:    CategoryTraffic,
		Description: traffic.TrafficSearchDescription,
		ReadOnly:    true,
		Schema: obj(map[string]any{
			"host":          str("target host (required): bare host, host:port, or full URL"),
			"query":         str("URL/metadata substring filter (optional)"),
			"body_contains": str("full-text body search (min 3 chars; optional)"),
			"limit":         intp("results per page, default 3, max 10"),
			"page":          intp("0-based page number for pagination"),
		}, "host"),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				Host         string `json:"host"`
				Query        string `json:"query"`
				BodyContains string `json:"body_contains"`
				Limit        int    `json:"limit"`
				Page         int    `json:"page"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			if strings.TrimSpace(a.Host) == "" {
				return errResult("host is required"), nil
			}
			if a.Limit <= 0 {
				a.Limit = 3
			}
			if a.Limit > 10 {
				a.Limit = 10
			}
			results, _, err := t.Page(traffic.PageQuery{
				Host:    a.Host,
				Query:   a.Query,
				Body:    a.BodyContains,
				RespMin: -1,
				RespMax: -1,
			}, a.Page, a.Limit)
			if err != nil {
				return errResult(err.Error()), nil
			}
			type row struct {
				ID      string `json:"id"`
				Method  string `json:"method"`
				URL     string `json:"url"`
				Status  int    `json:"status"`
				RespLen int    `json:"resp_len"`
			}
			out := make([]row, len(results))
			for i, m := range results {
				out[i] = row{ID: m.ID, Method: m.Method, URL: m.URL, Status: m.Status, RespLen: m.RespLen}
			}
			return jsonResult(map[string]any{"count": len(out), "results": out})
		},
	}
}

func makeTrafficGet(t *traffic.Traffic) Tool {
	return Tool{
		Name:        "traffic_get",
		Category:    CategoryTraffic,
		Description: traffic.TrafficGetDescription,
		ReadOnly:    true,
		Schema: obj(map[string]any{
			"id": str("exchange id from traffic_search"),
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

			rewritten, ruleIDs, _ := t.RewriteInfo(a.ID)
			req, resp, err := t.Get(a.ID)
			if err != nil {
				return errResult(err.Error()), nil
			}

			const maxGet = 32 * 1024
			req = clip(req, maxGet)
			resp = clip(resp, maxGet)

			var sb strings.Builder
			if rewritten {
				fmt.Fprintf(&sb, "REWRITTEN by rule ids %v\n\n", ruleIDs)
			}
			sb.WriteString("--- REQUEST ---\n")
			sb.WriteString(req)
			sb.WriteString("\n\n--- RESPONSE ---\n")
			sb.WriteString(resp)
			return textResult(sb.String()), nil
		},
	}
}

func makeTrafficBlob(t *traffic.Traffic) Tool {
	return Tool{
		Name:        "traffic_blob",
		Category:    CategoryTraffic,
		Description: "Read a segment of an oversized request or response body by its blob hash (shown in traffic_get as @blob sha256:<hash>). Returns at most 8 KiB per call; use offset to page through.",
		ReadOnly:    true,
		Schema: obj(map[string]any{
			"hash":   str("sha256 blob hash from traffic_get output"),
			"offset": intp("byte offset to start reading (default 0)"),
			"limit":  intp("bytes to read, max 8192 (default 8192)"),
		}, "hash"),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				Hash   string `json:"hash"`
				Offset int64  `json:"offset"`
				Limit  int64  `json:"limit"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			if strings.TrimSpace(a.Hash) == "" {
				return errResult("hash is required"), nil
			}
			if a.Limit <= 0 || a.Limit > 8192 {
				a.Limit = 8192
			}
			data, total, err := t.BlobRange(a.Hash, a.Offset, a.Limit)
			if err != nil {
				return errResult(err.Error()), nil
			}
			return jsonResult(map[string]any{
				"total":  total,
				"offset": a.Offset,
				"length": len(data),
				"data":   string(data),
			})
		},
	}
}

func makeTrafficMessages(t *traffic.Traffic) Tool {
	return Tool{
		Name:        "traffic_messages",
		Category:    CategoryTraffic,
		Description: "List WebSocket or SSE messages of a captured stream exchange. Use after traffic_get shows status 101 (WebSocket) or text/event-stream (SSE).",
		ReadOnly:    true,
		Schema: obj(map[string]any{
			"id":        str("exchange id"),
			"direction": str("filter: c2s (client to server), s2c (server to client), or empty for both"),
			"limit":     intp("messages per page, default 50, max 200"),
			"offset":    intp("0-based offset for pagination"),
		}, "id"),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				ID        string `json:"id"`
				Direction string `json:"direction"`
				Limit     int    `json:"limit"`
				Offset    int    `json:"offset"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			if strings.TrimSpace(a.ID) == "" {
				return errResult("id is required"), nil
			}
			msgs, total, err := t.StreamMessagesByDirection(a.ID, a.Direction, a.Offset, a.Limit)
			if err != nil {
				return errResult(err.Error()), nil
			}
			type row struct {
				Seq       int64  `json:"seq"`
				Direction string `json:"direction"`
				Type      string `json:"type"`
				Size      int    `json:"size"`
				Payload   string `json:"payload"`
				Event     string `json:"event,omitempty"`
				Blob      string `json:"blob,omitempty"`
				Truncated bool   `json:"truncated,omitempty"`
			}
			out := make([]row, len(msgs))
			for i, m := range msgs {
				out[i] = row{
					Seq:       m.Seq,
					Direction: m.Direction,
					Type:      m.Type,
					Size:      m.Size,
					Payload:   truncRunes(m.Payload, 1024),
					Event:     m.Event,
					Blob:      m.Blob,
					Truncated: m.Truncated,
				}
			}
			return jsonResult(map[string]any{"total": total, "messages": out})
		},
	}
}

func makeTrafficReplay(t *traffic.Traffic, store Store, taskID int64) Tool {
	return Tool{
		Name:     "traffic_replay",
		Category: CategoryTraffic,
		Description: "Replay a captured HTTP request (optionally with modifications). " +
			"Fetches the original request from the given exchange id, applies any header or body overrides, " +
			"sends it through the recording proxy, and returns the new response.",
		Schema: obj(map[string]any{
			"id":             str("exchange id to replay"),
			"modify_headers": map[string]any{"type": "object", "description": "header overrides: {name: value} to set, {name: \"\"} to remove", "additionalProperties": map[string]any{"type": "string"}},
			"modify_url":     str("override the request URL (optional)"),
			"modify_method":  str("override the HTTP method (optional)"),
			"modify_body":    str("override the request body (optional)"),
			"apply_rules":    map[string]any{"type": "boolean", "description": "apply capture rewrite rules to the replay (default false)"},
		}, "id"),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				ID            string            `json:"id"`
				ModifyHeaders map[string]string `json:"modify_headers"`
				ModifyURL     string            `json:"modify_url"`
				ModifyMethod  string            `json:"modify_method"`
				ModifyBody    *string           `json:"modify_body"`
				ApplyRules    bool              `json:"apply_rules"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			if strings.TrimSpace(a.ID) == "" {
				return errResult("id is required"), nil
			}

			req, _, err := t.Get(a.ID)
			if err != nil {
				return errResult("exchange not found: " + err.Error()), nil
			}
			method, reqURL, headers, body := parseRawRequest(req)

			replayTarget := reqURL
			if a.ModifyURL != "" {
				replayTarget = a.ModifyURL
			}
			if store != nil && replayTarget != "" {
				ok, reason, _, scopeErr := store.IsInScope(ctx, taskID, replayTarget)
				if scopeErr != nil {
					return errResult("scope check failed: " + scopeErr.Error()), nil
				}
				if !ok {
					return errResult(fmt.Sprintf("replay blocked: target is out of scope (%s)", reason)), nil
				}
			}

			if a.ModifyMethod != "" {
				method = a.ModifyMethod
			}
			if a.ModifyURL != "" {
				reqURL = a.ModifyURL
			}
			for k, v := range a.ModifyHeaders {
				if v == "" {
					delete(headers, k)
				} else {
					headers[k] = v
				}
			}
			if a.ModifyBody != nil {
				body = []byte(*a.ModifyBody)
			}

			var replayHeaders []traffic.ReplayHeader
			for k, v := range headers {
				replayHeaders = append(replayHeaders, traffic.ReplayHeader{Name: k, Value: v})
			}

			result, err := t.Replay(ctx, traffic.ReplayRequest{
				Method:     method,
				URL:        reqURL,
				Headers:    replayHeaders,
				Body:       body,
				ApplyRules: a.ApplyRules,
			})
			if err != nil {
				return errResult("replay failed: " + err.Error()), nil
			}

			const maxReplay = 32 * 1024
			raw := result.Raw
			if len(raw) > maxReplay {
				raw = raw[:maxReplay] + fmt.Sprintf("\n... [truncated, %d bytes total]", len(result.Raw))
			}

			return jsonResult(map[string]any{
				"status":      result.Status,
				"duration_ms": result.DurationMS,
				"recorded":    result.Recorded,
				"truncated":   result.Truncated,
				"raw":         raw,
			})
		},
	}
}

func makeTrafficHosts(t *traffic.Traffic) Tool {
	return Tool{
		Name:        "traffic_hosts",
		Category:    CategoryTraffic,
		Description: "List all recorded target hosts with their exchange counts, most recent activity first.",
		ReadOnly:    true,
		Schema:      obj(map[string]any{}),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			hosts, err := t.Hosts()
			if err != nil {
				return errResult(err.Error()), nil
			}
			return jsonResult(map[string]any{"count": len(hosts), "hosts": hosts})
		},
	}
}

func makeTrafficHARExport(t *traffic.Traffic) Tool {
	return Tool{
		Name:        "traffic_har_export",
		Category:    CategoryTraffic,
		Description: "Export captured traffic as a HAR 1.2 JSON document. Optionally filter by host.",
		ReadOnly:    true,
		Schema: obj(map[string]any{
			"host":   str("host substring filter (optional; empty exports all)"),
			"method": str("HTTP method filter (optional)"),
			"status": str("status code or class filter, e.g. 200 or 4xx (optional)"),
		}),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				Host   string `json:"host"`
				Method string `json:"method"`
				Status string `json:"status"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}

			doc, meta, err := t.ExportHAR(traffic.PageQuery{
				Host:    a.Host,
				Method:  a.Method,
				Status:  a.Status,
				RespMin: -1,
				RespMax: -1,
			}, traffic.HARExportOptions{})
			if err != nil {
				return errResult(err.Error()), nil
			}

			raw, err := json.Marshal(doc)
			if err != nil {
				return errResult("marshal HAR: " + err.Error()), nil
			}
			return jsonResult(map[string]any{
				"har":          json.RawMessage(raw),
				"exported":     meta.Exported,
				"totalMatched": meta.TotalMatched,
				"truncated":    meta.Truncated,
			})
		},
	}
}

func makeTrafficStats(t *traffic.Traffic) Tool {
	return Tool{
		Name:        "traffic_stats",
		Category:    CategoryTraffic,
		Description: "Aggregate statistics about all captured traffic: total exchanges, unique hosts, status code distribution, method distribution, top 10 hosts, time range, and average response size.",
		ReadOnly:    true,
		Schema:      obj(map[string]any{}),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			stats, err := t.Stats()
			if err != nil {
				return errResult(err.Error()), nil
			}
			return jsonResult(stats)
		},
	}
}

func makeTrafficTimeline(t *traffic.Traffic) Tool {
	return Tool{
		Name:        "traffic_timeline",
		Category:    CategoryTraffic,
		Description: "Show traffic activity over time for a host, bucketed by minute, hour (default), or day. Useful for spotting rate limiting, session expiry, and WAF patterns.",
		ReadOnly:    true,
		Schema: obj(map[string]any{
			"host":   str("target host (required)"),
			"bucket": enm([]string{"minute", "hour", "day"}, "time bucket granularity (default hour)"),
		}, "host"),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				Host   string `json:"host"`
				Bucket string `json:"bucket"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			if strings.TrimSpace(a.Host) == "" {
				return errResult("host is required"), nil
			}
			buckets, err := t.Timeline(a.Host, a.Bucket)
			if err != nil {
				return errResult(err.Error()), nil
			}
			return jsonResult(map[string]any{"host": a.Host, "buckets": buckets})
		},
	}
}

// parseRawRequest extracts method, URL, headers and body from a raw HTTP
// request as returned by Traffic.Get. The request line carries only the
// request-URI (/path?query), so the full URL is rebuilt from the Host header.
func parseRawRequest(raw string) (method, reqURL string, headers map[string]string, body []byte) {
	headers = map[string]string{}
	parts := strings.SplitN(raw, "\n\n", 2)
	head := parts[0]
	if len(parts) == 2 {
		body = []byte(parts[1])
	}

	lines := strings.Split(head, "\n")
	if len(lines) == 0 {
		return
	}

	requestLine := strings.Fields(lines[0])
	if len(requestLine) >= 2 {
		method = requestLine[0]
		reqURL = requestLine[1]
	}

	for _, line := range lines[1:] {
		if idx := strings.IndexByte(line, ':'); idx > 0 {
			name := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])
			headers[name] = value
		}
	}

	if !strings.Contains(reqURL, "://") {
		host := headers["Host"]
		if host != "" {
			reqURL = "https://" + host + reqURL
		}
	}
	return
}

func makeTrafficClear(t *traffic.Traffic) Tool {
	return Tool{
		Name:        "traffic_clear",
		Category:    CategoryTraffic,
		Description: "Delete all captured traffic for a given host substring. Returns the number of deleted exchanges.",
		Schema: obj(map[string]any{
			"host": str("host substring to match for deletion"),
		}, "host"),
		Run: func(ctx context.Context, in json.RawMessage) (Result, error) {
			var a struct {
				Host string `json:"host"`
			}
			if err := json.Unmarshal(in, &a); err != nil {
				return errResult("invalid input: " + err.Error()), nil
			}
			host := strings.TrimSpace(a.Host)
			if host == "" {
				return errResult("host is required"), nil
			}
			n, err := t.DeleteHost(host)
			if err != nil {
				return errResult(err.Error()), nil
			}
			return jsonResult(map[string]any{
				"deleted": n,
				"host":    host,
			})
		},
	}
}

func clip(s string, max int) string {
	if len(s) <= max {
		return s
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + fmt.Sprintf("\n... [truncated, %d bytes total] ...", len(s))
}
