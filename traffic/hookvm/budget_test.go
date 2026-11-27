package hookvm

import (
	"context"
	"errors"
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
