package tools

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

type renewCalls struct {
	mu    sync.Mutex
	hosts []string
	block chan struct{}
}

func (c *renewCalls) renew(_ context.Context, host string, _ time.Duration) ([]int64, error) {
	if c.block != nil {
		<-c.block
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hosts = append(c.hosts, host)
	return nil, nil
}

func (c *renewCalls) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.hosts)
}

func TestHostClaimRenewerThrottlesPerHost(t *testing.T) {
	calls := &renewCalls{}
	r := NewHostClaimRenewer(calls.renew)
	clock := time.Unix(1_000_000, 0)
	r.now = func() time.Time { return clock }

	r.Touch("App.Example.test.")
	r.wg.Wait()
	r.Touch("app.example.test")
	r.wg.Wait()
	if n := calls.count(); n != 1 {
		t.Fatalf("second touch within the interval renewed again: %d calls", n)
	}
	r.Touch("other.test")
	r.wg.Wait()
	if n := calls.count(); n != 2 {
		t.Fatalf("another host was throttled: %d calls", n)
	}
	clock = clock.Add(trafficRenewInterval)
	r.Touch("app.example.test")
	r.wg.Wait()
	if n := calls.count(); n != 3 || calls.hosts[0] != "app.example.test" {
		t.Errorf("after the interval: %v, want a third renewal of app.example.test", calls.hosts)
	}
}

func TestHostClaimRenewerRunsOneQueryAtATime(t *testing.T) {
	calls := &renewCalls{block: make(chan struct{})}
	r := NewHostClaimRenewer(calls.renew)
	r.Touch("a.test")
	for i := range 50 {
		r.Touch(fmt.Sprintf("h%d.test", i))
	}
	close(calls.block)
	r.wg.Wait()
	if n := calls.count(); n != 1 {
		t.Errorf("%d renewals ran while one was in flight, want 1", n)
	}
	r.Touch("h0.test")
	r.wg.Wait()
	if n := calls.count(); n != 2 {
		t.Errorf("a host dropped while busy was not retried on its next exchange: %d calls", n)
	}
}

func TestHostClaimRenewerPrunesOldHosts(t *testing.T) {
	calls := &renewCalls{}
	r := NewHostClaimRenewer(calls.renew)
	clock := time.Unix(1_000_000, 0)
	r.now = func() time.Time { return clock }
	for i := range maxTrafficHosts {
		r.Touch(fmt.Sprintf("h%d.test", i))
		r.wg.Wait()
	}
	clock = clock.Add(trafficRenewInterval)
	r.Touch("new.test")
	r.wg.Wait()
	if len(r.last) != 1 {
		t.Errorf("throttle map holds %d hosts after pruning, want 1", len(r.last))
	}
}
