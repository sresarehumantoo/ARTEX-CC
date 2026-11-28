package api

import (
	"context"
	"sync"
	"testing"

	"github.com/SREsAreHumanToo/ARTEX-CC/identity"
)

func TestRunnerReleasesClaimsWhenClaudeExits(t *testing.T) {
	dataDir := t.TempDir()
	base, err := WriteMCPConfig(dataDir, "localhost:9400", "tok")
	if err != nil {
		t.Fatal(err)
	}
	signer, err := identity.NewSigner(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	store := newTestChatStore(t)
	r := NewRunner(writeFakeClaude(t, "exit 1\n"), base, dataDir, 2)
	r.SetIdentitySigner(signer)
	r.Bind(store, NewHub())
	var mu sync.Mutex
	var released []string
	r.SetClaimReleaser(func(_ context.Context, holder string) (int64, error) {
		mu.Lock()
		defer mu.Unlock()
		released = append(released, holder)
		return 1, nil
	})

	conv, err := store.Create(CreateConversationRequest{Title: "dies"})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Start(context.Background(), conv.ID, "", "", ""); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return !r.Running(conv.ID) })
	waitFor(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(released) == 1
	})
	mu.Lock()
	defer mu.Unlock()
	if released[0] != "id:chat/"+conv.ID {
		t.Errorf("released %q, want the conversation's identity holder", released[0])
	}
}
