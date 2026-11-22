package main

import (
	"flag"
	"testing"
	"time"
)

func TestHealthTimeoutFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want time.Duration
	}{
		{"default", []string{}, 5 * time.Second},
		{"custom 10s", []string{"-timeout", "10s"}, 10 * time.Second},
		{"custom 30s", []string{"-timeout", "30s"}, 30 * time.Second},
		{"custom 500ms", []string{"-timeout", "500ms"}, 500 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := flag.NewFlagSet("health", flag.ContinueOnError)
			_ = pgDSN(fs)
			timeout := fs.Duration("timeout", 5*time.Second, "health check timeout (e.g. 5s, 10s, 30s)")
			if err := fs.Parse(tt.args); err != nil {
				t.Fatalf("parse: %v", err)
			}
			if *timeout != tt.want {
				t.Errorf("timeout = %v, want %v", *timeout, tt.want)
			}
		})
	}
}
