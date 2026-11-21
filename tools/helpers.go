package tools

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func obj(props map[string]any, required ...string) map[string]any {
	m := map[string]any{"type": "object", "properties": props}
	req := make([]string, 0, len(required))
	for _, r := range required {
		if strings.TrimSpace(r) != "" {
			req = append(req, r)
		}
	}
	if len(req) > 0 {
		m["required"] = req
	}
	return m
}

func str(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }

func intp(desc string) map[string]any {
	return map[string]any{"type": "integer", "description": desc}
}

func arr(items map[string]any, desc string) map[string]any {
	return map[string]any{"type": "array", "items": items, "description": desc}
}

func enm(values []string, desc string) map[string]any {
	return map[string]any{"type": "string", "enum": values, "description": desc}
}

func jsonResult(v any) (Result, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return Result{Content: string(b)}, nil
}

func errResult(msg string) Result {
	return Result{Content: msg, IsError: true}
}

func textResult(msg string) Result {
	return Result{Content: msg}
}

func pid(raw json.RawMessage) int64 {
	if len(raw) == 0 {
		return 0
	}
	var n int64
	if json.Unmarshal(raw, &n) == nil {
		return n
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		v, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		return v
	}
	return 0
}

func pidList(raw []json.RawMessage) []int64 {
	var out []int64
	for _, r := range raw {
		if v := pid(r); v > 0 {
			out = append(out, v)
		}
	}
	return out
}

const (
	maxTextLen  = 10_000
	maxBatchLen = 500
)

var validSeverities = map[string]bool{
	"critical": true, "high": true, "medium": true, "low": true,
}

func validateTextLen(name, value string) error {
	if len(value) > maxTextLen {
		return fmt.Errorf("%s exceeds maximum length (%d bytes)", name, maxTextLen)
	}
	return nil
}

func splitLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func firstLine(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	runes := []rune(s)
	if len(runes) > maxRunes {
		return string(runes[:maxRunes-1]) + "…"
	}
	return s
}

func truncRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}

func formatBatchResult(ids []int64, errs map[string]string) (Result, error) {
	out := map[string]any{"ids": ids}
	if len(errs) > 0 {
		out["errors"] = errs
	}
	return jsonResult(out)
}

func batchOrSingle[T any](batch []T, single T) ([]T, bool) {
	if len(batch) > 0 {
		return batch, true
	}
	return []T{single}, false
}

func runBatch[T any](items []T, fn func(T) (int64, error)) ([]int64, map[string]string) {
	ids := make([]int64, len(items))
	errs := map[string]string{}
	for i, it := range items {
		id, err := fn(it)
		if err != nil {
			errs[strconv.Itoa(i)] = err.Error()
			continue
		}
		ids[i] = id
	}
	return ids, errs
}

func singleOrBatchReturn(ids []int64, errs map[string]string, isBatch bool, noun string) (Result, error) {
	if !isBatch {
		if e, bad := errs["0"]; bad {
			return errResult(e), nil
		}
		return textResult(fmt.Sprintf("%s created: %d", noun, ids[0])), nil
	}
	return formatBatchResult(ids, errs)
}
