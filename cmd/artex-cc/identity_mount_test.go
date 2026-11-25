package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/identity"
	"github.com/SREsAreHumanToo/ARTEX-CC/mcpserver"
	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
	"github.com/mark3labs/mcp-go/mcp"
)

const mountToken = "s3cret"

func holderRegistry() *tools.Registry {
	reg := tools.NewRegistry()
	reg.Register(tools.Tool{
		Name:     "holder",
		ReadOnly: true,
		Schema:   map[string]any{"type": "object", "properties": map[string]any{"owner": map[string]any{"type": "string"}}},
		Run: func(ctx context.Context, raw json.RawMessage) (tools.Result, error) {
			var a struct {
				Owner string `json:"owner"`
			}
			json.Unmarshal(raw, &a)
			owner, err := tools.ClaimOwner(ctx, a.Owner)
			if err != nil {
				return tools.Result{Content: "error: " + err.Error()}, nil
			}
			return tools.Result{Content: owner}, nil
		},
	})
	return reg
}

type mountPath struct {
	name string
	url  string
}

// mountPaths serves the holder tool on both /mcp stacks cmdServe builds,
// sharing one data directory.
func mountPaths(t *testing.T, dataDir string) []mountPath {
	t.Helper()
	signer, err := identity.NewSigner(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	web := mcpserver.New(holderRegistry())
	ts := httptest.NewServer(webMCPHandler(web, mountToken, signer))
	t.Cleanup(ts.Close)
	addr := serveListen(t, mcpserver.New(holderRegistry()), signer, mountToken)
	return []mountPath{
		{name: "web", url: ts.URL + "/mcp"},
		{name: "listen", url: "http://" + addr + "/mcp"},
	}
}

func mcpPost(t *testing.T, url string, h http.Header, sid string, body map[string]any) *http.Response {
	t.Helper()
	raw, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if sid != "" {
		req.Header.Set("Mcp-Session-Id", sid)
	}
	for k, v := range h {
		req.Header[k] = v
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func modernHolder(t *testing.T, url string, h http.Header, owner string) *http.Response {
	t.Helper()
	h = h.Clone()
	h.Set(mcp.HeaderProtocolVersion, mcp.ProtocolVersion20260728)
	h.Set(mcp.HeaderMethod, "tools/call")
	h.Set(mcp.HeaderName, "holder")
	return mcpPost(t, url, h, "", map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{
		"name":      "holder",
		"arguments": map[string]any{"owner": owner},
		"_meta": map[string]any{
			mcp.MetaKeyProtocolVersion:    mcp.ProtocolVersion20260728,
			mcp.MetaKeyClientInfo:         map[string]any{"name": "test", "version": "1"},
			mcp.MetaKeyClientCapabilities: map[string]any{},
		},
	}})
}

func legacyHolder(t *testing.T, url string, h http.Header, owner string) (*http.Response, string) {
	t.Helper()
	init := mcpPost(t, url, h, "", map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{
		"protocolVersion": "2025-03-26",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "test", "version": "1"},
	}})
	io.Copy(io.Discard, init.Body)
	init.Body.Close()
	sid := init.Header.Get("Mcp-Session-Id")
	if init.StatusCode != http.StatusOK {
		return init, ""
	}
	return mcpPost(t, url, h, sid, map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/call", "params": map[string]any{
		"name": "holder", "arguments": map[string]any{"owner": owner},
	}}), sid
}

func holderText(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("HTTP %d: %s", resp.StatusCode, raw)
	}
	var rr struct {
		Result *mcp.CallToolResult `json:"result"`
	}
	if err := json.Unmarshal(raw, &rr); err != nil || rr.Result == nil || len(rr.Result.Content) == 0 {
		t.Fatalf("decode: %v %s", err, raw)
	}
	text, _ := mcp.AsTextContent(rr.Result.Content[0])
	return text.Text
}

func authed(extra ...string) http.Header {
	h := http.Header{"Authorization": {"Bearer " + mountToken}}
	for i := 0; i+1 < len(extra); i += 2 {
		h.Set(extra[i], extra[i+1])
	}
	return h
}

func TestIdentityMountedOnBothHTTPPaths(t *testing.T) {
	dir := t.TempDir()
	signer, err := identity.NewSigner(dir)
	if err != nil {
		t.Fatal(err)
	}
	alice, err := signer.Issue("alice", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	other, err := identity.NewSigner(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	foreign, _ := other.Issue("alice", time.Hour)

	for _, p := range mountPaths(t, dir) {
		t.Run(p.name, func(t *testing.T) {
			withID := authed(mcpserver.IdentityHeader, alice)
			if got := holderText(t, modernHolder(t, p.url, withID, "")); got != "id:alice" {
				t.Errorf("modern with identity = %q, want id:alice", got)
			}
			if got := holderText(t, modernHolder(t, p.url, withID, "sub")); got != "id:alice/owner:sub" {
				t.Errorf("modern with identity and owner = %q", got)
			}
			resp, _ := legacyHolder(t, p.url, withID, "")
			if got := holderText(t, resp); got != "id:alice" {
				t.Errorf("legacy with identity = %q, want id:alice", got)
			}

			if got := holderText(t, modernHolder(t, p.url, authed(), "")); !strings.Contains(got, tools.ErrNoClaimOwner.Error()) {
				t.Errorf("modern without identity = %q, want ErrNoClaimOwner", got)
			}
			resp, sid := legacyHolder(t, p.url, authed(), "")
			if got := holderText(t, resp); got != sid {
				t.Errorf("legacy without identity = %q, want session %q", got, sid)
			}

			bad := authed(mcpserver.IdentityHeader, foreign)
			for era, resp := range map[string]*http.Response{
				"modern": modernHolder(t, p.url, bad, ""),
				"legacy": func() *http.Response { r, _ := legacyHolder(t, p.url, bad, ""); return r }(),
			} {
				resp.Body.Close()
				if resp.StatusCode != http.StatusBadRequest {
					t.Errorf("%s with foreign identity: HTTP %d, want 400", era, resp.StatusCode)
				}
			}

			for _, bearer := range []string{"", "Bearer wrong"} {
				bodies := map[string]string{}
				for name, token := range map[string]string{"none": "", "valid": alice, "foreign": foreign, "garbage": "not-a-token"} {
					h := http.Header{}
					if bearer != "" {
						h.Set("Authorization", bearer)
					}
					if token != "" {
						h.Set(mcpserver.IdentityHeader, token)
					}
					resp := modernHolder(t, p.url, h, "")
					raw, _ := io.ReadAll(resp.Body)
					resp.Body.Close()
					if resp.StatusCode != http.StatusUnauthorized {
						t.Errorf("bearer %q, %s identity: HTTP %d, want 401", bearer, name, resp.StatusCode)
					}
					bodies[name] = string(raw)
				}
				for name, body := range bodies {
					if body != bodies["none"] {
						t.Errorf("bearer %q rejection differs with %s identity: %q", bearer, name, bodies)
					}
				}
			}

			for name, h := range map[string]http.Header{
				"conversation only":         authed("X-Artex-Conversation", "conv-1.deadbeef"),
				"conversation and identity": authed("X-Artex-Conversation", "conv-1.deadbeef", mcpserver.IdentityHeader, alice),
			} {
				resp := modernHolder(t, p.url, h, "")
				raw, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				if resp.StatusCode != http.StatusBadRequest || !strings.Contains(string(raw), "start a new web chat conversation") {
					t.Errorf("%s: HTTP %d %s, want 400 telling the caller to start a new conversation", name, resp.StatusCode, raw)
				}
			}
		})
	}
}
