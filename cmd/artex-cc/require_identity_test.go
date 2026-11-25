package main

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/SREsAreHumanToo/ARTEX-CC/identity"
	"github.com/SREsAreHumanToo/ARTEX-CC/mcpserver"
	"github.com/SREsAreHumanToo/ARTEX-CC/tools"
)

func TestRequireIdentitySetting(t *testing.T) {
	for _, tt := range []struct {
		name    string
		flag    bool
		stdio   bool
		env     string
		want    bool
		wantErr string
	}{
		{name: "default off"},
		{name: "flag", flag: true, want: true},
		{name: "env true", env: "true", want: true},
		{name: "env 1", env: " 1 ", want: true},
		{name: "env false", env: "false"},
		{name: "env 0", env: "0"},
		{name: "flag wins over env false", flag: true, env: "0", want: true},
		{name: "env typo", env: "yes", wantErr: "not a boolean"},
		{name: "flag on stdio", flag: true, stdio: true, wantErr: "stdio clients cannot send X-Artex-Identity"},
		{name: "env on stdio", env: "1", stdio: true, wantErr: "stdio clients cannot send X-Artex-Identity"},
		{name: "off on stdio", env: "0", stdio: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := requireIdentity(tt.flag, tt.stdio, tt.env)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("= %v, %v, want %v", got, err, tt.want)
			}
		})
	}
}

func TestServeStdioRefusesRequireIdentity(t *testing.T) {
	if args := os.Getenv("ARTEX_TEST_SERVE_ARGS"); args != "" {
		cmdServe(strings.Fields(args))
		os.Exit(0)
	}
	for name, c := range map[string]struct {
		args string
		env  string
	}{
		"flag": {args: "--stdio --require-identity"},
		"env":  {args: "--stdio", env: "1"},
	} {
		t.Run(name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestServeStdioRefusesRequireIdentity$")
			cmd.Env = append(os.Environ(),
				"ARTEX_TEST_SERVE_ARGS="+c.args+" --pg postgres://127.0.0.1:1/none --data "+t.TempDir(),
				"ARTEX_REQUIRE_IDENTITY="+c.env)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			err := cmd.Run()
			if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
				t.Fatalf("serve exited with %v, want status 1; stderr: %s", err, stderr.String())
			}
			if !strings.Contains(stderr.String(), "needs an HTTP transport") {
				t.Errorf("stderr = %q, want the stdio refusal", stderr.String())
			}
		})
	}
}

func TestRequireIdentityOnBothHTTPPaths(t *testing.T) {
	prev := tools.RequireIdentity
	tools.RequireIdentity = true
	t.Cleanup(func() { tools.RequireIdentity = prev })

	dir := t.TempDir()
	signer, err := identity.NewSigner(dir)
	if err != nil {
		t.Fatal(err)
	}
	alice, err := signer.Issue("alice", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	const chat = "chat/0b7c3c4e-5d3f-4b8e-9a51-2f0c6d1e8a90"
	conv, err := signer.Issue(chat, identity.DefaultTTL)
	if err != nil {
		t.Fatal(err)
	}
	refused := tools.ErrIdentityRequired.Error()

	for _, p := range mountPaths(t, dir) {
		t.Run(p.name, func(t *testing.T) {
			if got := holderText(t, modernHolder(t, p.url, authed(mcpserver.IdentityHeader, alice), "")); got != "id:alice" {
				t.Errorf("modern identity = %q", got)
			}
			resp, _ := legacyHolder(t, p.url, authed(mcpserver.IdentityHeader, conv), "")
			if got := holderText(t, resp); got != "id:"+chat {
				t.Errorf("legacy chat identity = %q", got)
			}
			if got := holderText(t, modernHolder(t, p.url, authed(), "f00d")); !strings.Contains(got, refused) {
				t.Errorf("modern owner only = %q, want refusal", got)
			}
			resp, _ = legacyHolder(t, p.url, authed(), "")
			if got := holderText(t, resp); !strings.Contains(got, refused) {
				t.Errorf("legacy session only = %q, want refusal", got)
			}
		})
	}
}
