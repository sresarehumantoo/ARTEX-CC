package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/mcpserver"
	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func TestMCPListenToken(t *testing.T) {
	for _, tt := range []struct {
		name    string
		addr    string
		token   string
		want    string
		wantErr bool
	}{
		{"loopback without token", "localhost:9400", "", "", false},
		{"loopback ip without token", "127.0.0.1:9400", "", "", false},
		{"loopback honors token", "localhost:9400", "s3cret", "s3cret", false},
		{"all interfaces without token", ":9400", "", "", true},
		{"unspecified v4 without token", "0.0.0.0:9400", "", "", true},
		{"unspecified v6 without token", "[::]:9400", "", "", true},
		{"lan ip without token", "192.168.1.20:9400", "", "", true},
		{"hostname without token", "artex.internal:9400", "", "", true},
		{"whitespace token is empty", ":9400", " \n", "", true},
		{"all interfaces with token", ":9400", "s3cret\n", "s3cret", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mcpListenToken(tt.addr, tt.token)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), "ARTEX_MCP_TOKEN") {
				t.Errorf("error %q does not say how to set a token", err)
			}
			if got != tt.want {
				t.Errorf("token = %q, want %q", got, tt.want)
			}
		})
	}
}

func startListenServer(t *testing.T, token string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	var middleware []func(http.Handler) http.Handler
	if token != "" {
		middleware = append(middleware, func(h http.Handler) http.Handler { return bearerAuth(token, h) })
	}
	srv := mcpserver.New(tools.NewRegistry())
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go srv.ServeHTTP(ctx, addr, middleware...)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err == nil {
			conn.Close()
			return addr
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("server did not start in time")
	return ""
}

func initializeStatus(t *testing.T, addr, auth string) int {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-03-26",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "test", "version": "1"},
		},
	})
	req, err := http.NewRequest("POST", "http://"+addr+"/mcp", strings.NewReader(string(body)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func TestServeHTTPRequiresToken(t *testing.T) {
	addr := startListenServer(t, "s3cret")
	for _, tt := range []struct {
		name string
		auth string
		want int
	}{
		{"missing", "", http.StatusUnauthorized},
		{"wrong", "Bearer nope", http.StatusUnauthorized},
		{"bare token", "s3cret", http.StatusUnauthorized},
		{"right", "Bearer s3cret", http.StatusOK},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := initializeStatus(t, addr, tt.auth); got != tt.want {
				t.Errorf("status = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestServeHTTPLoopbackWithoutToken(t *testing.T) {
	addr := startListenServer(t, "")
	if got := initializeStatus(t, addr, ""); got != http.StatusOK {
		t.Errorf("status = %d, want %d", got, http.StatusOK)
	}
}
