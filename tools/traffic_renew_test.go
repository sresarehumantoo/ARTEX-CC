package tools

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestHostClaimRenewerThrottlesPerHost(t *testing.T) {
	var mu sync.Mutex
	calls := map[string]int{}
	done := make(chan struct{}, 10)
	r := NewHostClaimRenewer(func(_ context.Context, host string, lease time.Duration) ([]int64, error) {
		mu.Lock()
		calls[host]++
		mu.Unlock()
		if lease != IntentLease {
			t.Errorf("lease = %v, want %v", lease, IntentLease)
		}
		done <- struct{}{}
		return nil, nil
	})
	r.Touch("App.Example.test")
	r.Touch("app.example.test")
	r.Touch("other.test")
	for range 2 {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("renewal did not run")
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if calls["app.example.test"] != 1 || calls["other.test"] != 1 || len(calls) != 2 {
		t.Errorf("renewals = %v, want one per host within the interval", calls)
	}
}
