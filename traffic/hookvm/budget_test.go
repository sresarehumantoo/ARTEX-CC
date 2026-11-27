package hookvm

import (
	"context"
	"errors"
	"runtime"
	"testing"
)

func TestRangeOverBodyStoppedByMemoryBudget(t *testing.T) {
	prog := compile(t, "request", `package hook
func OnRequest(req *Request) error {
	for _, b := range req.Body {
		_ = b
	}
	return nil
}`)
	err := prog.RunRequest(context.Background(), &Request{Body: make([]byte, 256<<10)}, RunOptions{MaxMemory: 64 << 10})
	if !errors.Is(err, ErrMemoryLimit) {
		t.Fatalf("err = %v, want ErrMemoryLimit", err)
	}
}

func TestEqualityKeepsFloatFraction(t *testing.T) {
	prog := compile(t, "request", `package hook
func mark(b bool) string {
	if b {
		return "T"
	}
	return "F"
}
func OnRequest(req *Request) error {
	x := 1.5
	n := 1
	req.SetHeader("X-Eq", mark(x == 1)+mark(x != n)+mark(-0.5 == 0)+mark(2.0 == 2)+mark(n == 1.0))
	return nil
}`)
	req := &Request{}
	if err := prog.RunRequest(context.Background(), req, RunOptions{}); err != nil {
		t.Fatal(err)
	}
	if got := req.Headers["X-Eq"][0]; got != "FTFTT" {
		t.Fatalf("X-Eq = %q, want FTFTT", got)
	}
}

// Each case replaces 8 KiB of matches with 8 KiB apiece, a 64 MiB result. The
// budget has to stop the run before that result is built, not after.
func TestReplaceAllResultStoppedByMemoryBudget(t *testing.T) {
	srcs := map[string]string{
		"strings": `package hook
import "strings"
func OnRequest(req *Request) error {
	s := strings.Repeat("a", 8*1024)
	_ = strings.ReplaceAll(s, "a", strings.Repeat("b", 8*1024))
	return nil
}`,
		"bytes": `package hook
import "bytes"
func OnRequest(req *Request) error {
	b := bytes.Repeat([]byte("a"), 8*1024)
	_ = bytes.ReplaceAll(b, []byte("a"), bytes.Repeat([]byte("b"), 8*1024))
	return nil
}`,
		"regexp": `package hook
import (
	"regexp"
	"strings"
)
func OnRequest(req *Request) error {
	s := strings.Repeat("a", 8*1024)
	_ = regexp.ReplaceAllString("a", s, strings.Repeat("b", 8*1024))
	return nil
}`,
	}
	for name, src := range srcs {
		t.Run(name, func(t *testing.T) {
			prog := compile(t, "request", src)
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			err := prog.RunRequest(context.Background(), &Request{}, RunOptions{MaxMemory: 1 << 20})
			runtime.ReadMemStats(&after)
			if !errors.Is(err, ErrMemoryLimit) {
				t.Fatalf("err = %v, want ErrMemoryLimit", err)
			}
			// Race builds drop sync.Pool items, so regexp allocates fresh
			// matcher state for every one of the 8K matches.
			if name == "regexp" && raceEnabled {
				return
			}
			if got := after.TotalAlloc - before.TotalAlloc; got > 16<<20 {
				t.Fatalf("the run allocated %d MiB under a 1 MiB budget", got>>20)
			}
		})
	}
}
