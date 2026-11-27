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
