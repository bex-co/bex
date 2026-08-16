/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package store

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func newReplayTestStore(t *testing.T) *PGStore {
	t.Helper()
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(context.Background(), uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return NewPGStore(pool)
}

// codex round-8 #9: the git-webhook replay ledger — one claim per digest,
// concurrent duplicates collapse to exactly one winner, release frees a failed
// delivery for the host's retry.
func TestPGGitWebhookReplayClaims(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()

	fresh, err := st.ClaimGitWebhookDelivery(ctx, "digest-a")
	if err != nil || !fresh {
		t.Fatalf("first claim => fresh=%v err=%v, want fresh", fresh, err)
	}
	again, err := st.ClaimGitWebhookDelivery(ctx, "digest-a")
	if err != nil || again {
		t.Fatalf("re-claim => fresh=%v err=%v, want !fresh", again, err)
	}
	other, err := st.ClaimGitWebhookDelivery(ctx, "digest-b")
	if err != nil || !other {
		t.Fatalf("distinct digest => fresh=%v err=%v, want fresh", other, err)
	}

	// Release frees exactly the failed delivery's digest.
	if err := st.ReleaseGitWebhookDelivery(ctx, "digest-a"); err != nil {
		t.Fatal(err)
	}
	retried, err := st.ClaimGitWebhookDelivery(ctx, "digest-a")
	if err != nil || !retried {
		t.Fatalf("claim after release => fresh=%v err=%v, want fresh (retry unblocked)", retried, err)
	}
}

// Concurrent duplicates of one delivery: the ON CONFLICT insert makes exactly
// one winner — the atomicity the webhook's dedup leans on.
func TestPGGitWebhookReplayConcurrentClaimsCollapse(t *testing.T) {
	st := newReplayTestStore(t)
	const workers = 8
	var wg sync.WaitGroup
	winners := make(chan bool, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fresh, err := st.ClaimGitWebhookDelivery(context.Background(), "digest-race")
			if err != nil {
				t.Errorf("claim: %v", err)
			}
			winners <- fresh
		}()
	}
	wg.Wait()
	close(winners)
	won := 0
	for fresh := range winners {
		if fresh {
			won++
		}
	}
	if won != 1 {
		t.Fatalf("concurrent claims produced %d winners, want exactly 1", won)
	}
}
