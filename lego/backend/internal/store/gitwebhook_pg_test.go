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
	"errors"
	"os"
	"sync"
	"testing"
	"time"

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

func registerReplayTestEpoch(t *testing.T, st *PGStore, epochs ...GitWebhookReplayEpoch) {
	t.Helper()
	if _, err := st.MaintainGitWebhookReplayEpochs(context.Background(), epochs, time.Now()); err != nil {
		t.Fatalf("register replay epoch: %v", err)
	}
}

// codex round-8 #9: the git-webhook replay ledger — one claim per digest,
// concurrent duplicates collapse to exactly one winner, release frees a failed
// delivery for the host's retry.
func TestPGGitWebhookReplayClaims(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	claimA := GitWebhookReplayClaim{Scope: "workspace:test-claims", KeyClass: GitWebhookReplayKeyGitHub, Epoch: "epoch-claims", Digest: "digest-a"}
	registerReplayTestEpoch(t, st, GitWebhookReplayEpoch{KeyClass: claimA.KeyClass, Epoch: claimA.Epoch})

	fresh, err := st.ClaimGitWebhookDelivery(ctx, claimA)
	if err != nil || !fresh {
		t.Fatalf("first claim => fresh=%v err=%v, want fresh", fresh, err)
	}
	again, err := st.ClaimGitWebhookDelivery(ctx, claimA)
	if err != nil || again {
		t.Fatalf("re-claim => fresh=%v err=%v, want !fresh", again, err)
	}
	claimB := claimA
	claimB.Digest = "digest-b"
	other, err := st.ClaimGitWebhookDelivery(ctx, claimB)
	if err != nil || !other {
		t.Fatalf("distinct digest => fresh=%v err=%v, want fresh", other, err)
	}

	// Release frees exactly the failed delivery's digest.
	if err := st.ReleaseGitWebhookDelivery(ctx, claimA); err != nil {
		t.Fatal(err)
	}
	retried, err := st.ClaimGitWebhookDelivery(ctx, claimA)
	if err != nil || !retried {
		t.Fatalf("claim after release => fresh=%v err=%v, want fresh (retry unblocked)", retried, err)
	}

	// A pre-0104 replica can remain in service during the migration: its old
	// digest-only INSERT still resolves the unchanged primary key, and a new
	// replica recognizes that legacy claim as the same replay.
	if _, err := st.Pool.Exec(ctx, `INSERT INTO git_webhook_replays (digest) VALUES ('legacy-rollout') ON CONFLICT (digest) DO NOTHING`); err != nil {
		t.Fatalf("legacy rolling-upgrade insert: %v", err)
	}
	legacyClaim := claimA
	legacyClaim.Digest = "legacy-rollout"
	if fresh, err := st.ClaimGitWebhookDelivery(ctx, legacyClaim); err != nil || fresh {
		t.Fatalf("new claim over legacy digest => fresh=%v err=%v, want replay", fresh, err)
	}
}

// Concurrent duplicates of one delivery: the ON CONFLICT insert makes exactly
// one winner — the atomicity the webhook's dedup leans on.
func TestPGGitWebhookReplayConcurrentClaimsCollapse(t *testing.T) {
	st := newReplayTestStore(t)
	claim := GitWebhookReplayClaim{Scope: "workspace:test-race", KeyClass: GitWebhookReplayKeyGitHub, Epoch: "epoch-race", Digest: "digest-race"}
	registerReplayTestEpoch(t, st, GitWebhookReplayEpoch{KeyClass: claim.KeyClass, Epoch: claim.Epoch})
	const workers = 8
	var wg sync.WaitGroup
	winners := make(chan bool, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fresh, err := st.ClaimGitWebhookDelivery(context.Background(), claim)
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

func TestPGGitWebhookReplayScopeCapIsExact(t *testing.T) {
	st := newReplayTestStore(t)
	newEpochStore := NewPGStore(st.Pool)
	ctx := context.Background()
	claim := GitWebhookReplayClaim{Scope: "workspace:test-cap", KeyClass: GitWebhookReplayKeyGitHub, Epoch: "epoch-cap-a"}
	registerReplayTestEpoch(t, st, GitWebhookReplayEpoch{KeyClass: claim.KeyClass, Epoch: claim.Epoch})
	registerReplayTestEpoch(t, newEpochStore, GitWebhookReplayEpoch{KeyClass: claim.KeyClass, Epoch: "epoch-cap-b"})
	for _, digest := range []string{"cap-a", "cap-b"} {
		claim.Digest = digest
		fresh, err := st.claimGitWebhookDelivery(ctx, claim, 2)
		if err != nil || !fresh {
			t.Fatalf("claim %s => fresh=%v err=%v", digest, fresh, err)
		}
	}
	claim.Epoch = "epoch-cap-b"
	claim.Digest = "cap-c"
	if fresh, err := newEpochStore.claimGitWebhookDelivery(ctx, claim, 2); !errors.Is(err, ErrGitWebhookReplayCapacity) || fresh {
		t.Fatalf("over-cap claim => fresh=%v err=%v, want capacity", fresh, err)
	}
	claim.Epoch = "epoch-cap-a"
	claim.Digest = "cap-a"
	if fresh, err := st.claimGitWebhookDelivery(ctx, claim, 2); err != nil || fresh {
		t.Fatalf("duplicate at cap => fresh=%v err=%v, want replay without capacity error", fresh, err)
	}
}

func TestPGGitWebhookReplayEpochPurgesOnlyAfterLeaseExpires(t *testing.T) {
	st := newReplayTestStore(t)
	oldReplica := NewPGStore(st.Pool)
	newReplica := NewPGStore(st.Pool)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	oldEpoch := GitWebhookReplayEpoch{KeyClass: GitWebhookReplayKeyGitHub, Epoch: "epoch-old"}
	newEpoch := GitWebhookReplayEpoch{KeyClass: GitWebhookReplayKeyGitHub, Epoch: "epoch-new"}
	oldClaim := GitWebhookReplayClaim{Scope: "workspace:test-epoch", KeyClass: oldEpoch.KeyClass, Epoch: oldEpoch.Epoch, Digest: "epoch-old-digest"}
	newClaim := GitWebhookReplayClaim{Scope: oldClaim.Scope, KeyClass: newEpoch.KeyClass, Epoch: newEpoch.Epoch, Digest: "epoch-new-digest"}
	// Production registers each replica's accepted epochs synchronously before
	// serving. Mirror that ordering so neither replica can mistake the other's
	// just-created epoch for retired during a rolling rotation.
	if _, err := oldReplica.MaintainGitWebhookReplayEpochs(ctx, []GitWebhookReplayEpoch{oldEpoch}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := newReplica.MaintainGitWebhookReplayEpochs(ctx, []GitWebhookReplayEpoch{newEpoch}, now); err != nil {
		t.Fatal(err)
	}
	if fresh, err := oldReplica.ClaimGitWebhookDelivery(ctx, oldClaim); err != nil || !fresh {
		t.Fatalf("old claim => fresh=%v err=%v", fresh, err)
	}
	if fresh, err := newReplica.ClaimGitWebhookDelivery(ctx, newClaim); err != nil || !fresh {
		t.Fatalf("new claim => fresh=%v err=%v", fresh, err)
	}
	// Claims refresh leases using the real clock. Anchor simulated expiry after
	// those writes so a slow database cannot leave the old lease still live.
	now = time.Now().UTC()
	if _, err := st.Pool.Exec(ctx, `INSERT INTO git_webhook_replays (digest) VALUES ('legacy-retained') ON CONFLICT DO NOTHING`); err != nil {
		t.Fatal(err)
	}
	if purged, err := newReplica.MaintainGitWebhookReplayEpochs(ctx, []GitWebhookReplayEpoch{newEpoch}, now); err != nil || purged != 0 {
		t.Fatalf("maintenance while old epoch live => purged=%d err=%v", purged, err)
	}
	if purged, err := newReplica.MaintainGitWebhookReplayEpochs(ctx, []GitWebhookReplayEpoch{newEpoch}, now.Add(GitWebhookReplayLeaseTTL+time.Second)); err != nil || purged < 1 {
		t.Fatalf("maintenance after old lease expiry => purged=%d err=%v", purged, err)
	}
	var oldExists, newExists, legacyExists bool
	if err := st.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM git_webhook_replays WHERE scope = $1 AND epoch = $2)`, oldClaim.Scope, oldClaim.Epoch).Scan(&oldExists); err != nil {
		t.Fatal(err)
	}
	if err := st.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM git_webhook_replays WHERE scope = $1 AND epoch = $2)`, newClaim.Scope, newClaim.Epoch).Scan(&newExists); err != nil {
		t.Fatal(err)
	}
	if err := st.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM git_webhook_replays WHERE digest = 'legacy-retained' AND epoch = 'legacy')`).Scan(&legacyExists); err != nil {
		t.Fatal(err)
	}
	if oldExists || !newExists || !legacyExists {
		t.Fatalf("post-retirement rows old=%v new=%v legacy=%v, want false/true/true", oldExists, newExists, legacyExists)
	}
	if _, err := oldReplica.MaintainGitWebhookReplayEpochs(ctx, []GitWebhookReplayEpoch{oldEpoch}, now.Add(GitWebhookReplayLeaseTTL+2*time.Second)); !errors.Is(err, ErrGitWebhookReplayEpochRetired) {
		t.Fatalf("retired old replica re-registration err=%v, want retired epoch", err)
	}
	if fresh, err := oldReplica.ClaimGitWebhookDelivery(ctx, oldClaim); !errors.Is(err, ErrGitWebhookReplayEpochRetired) || fresh {
		t.Fatalf("retired old replica claim => fresh=%v err=%v, want fail closed", fresh, err)
	}
}
