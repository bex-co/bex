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

package apps

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// codex round-8 #9: the HMAC authenticates the delivery's bytes, not its
// freshness. These tests hold the replay ledger contract: the exact signed
// body — however it is resent — mutates exactly once.

// fakeReplayGuard is the in-memory WebhookReplayGuard: claims are a set, so a
// second claim of the same digest reports !fresh exactly like the ON CONFLICT
// DO NOTHING insert.
type fakeReplayGuard struct {
	mu       sync.Mutex
	claims   map[string]bool
	claimErr error
	capacity bool
}

func (f *fakeReplayGuard) ClaimGitWebhookDelivery(_ context.Context, claim store.GitWebhookReplayClaim) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.claimErr != nil {
		return false, f.claimErr
	}
	if f.capacity {
		return false, store.ErrGitWebhookReplayCapacity
	}
	if f.claims == nil {
		f.claims = map[string]bool{}
	}
	key := claim.Digest
	if f.claims[key] {
		return false, nil
	}
	f.claims[key] = true
	return true, nil
}

func (f *fakeReplayGuard) ReleaseGitWebhookDelivery(_ context.Context, claim store.GitWebhookReplayClaim) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := claim.Digest
	delete(f.claims, key)
	return nil
}

func (f *fakeReplayGuard) claimed() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.claims)
}

// postPushDelivery is postPush with an explicit X-GitHub-Delivery header — the
// unsigned header a replay can freely change.
func postPushDelivery(t *testing.T, h *GitWebhook, sigSecret, event, delivery string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/v1/webhooks/git", strings.NewReader(string(body)))
	req.Header.Set("X-Hub-Signature-256", sign(sigSecret, body))
	if event != "" {
		req.Header.Set("X-GitHub-Event", event)
	}
	if delivery != "" {
		req.Header.Set("X-GitHub-Delivery", delivery)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func isReplay(rec *httptest.ResponseRecorder) bool {
	var payload struct {
		Replayed bool `json:"replayed"`
	}
	return json.Unmarshal(rec.Body.Bytes(), &payload) == nil && payload.Replayed
}

// A captured signed body, resent verbatim, is answered 200 and mutates nothing
// — one claim, one redeploy.
func TestWebhookReplayIsSkipped(t *testing.T) {
	const repo = "https://github.com/x/app"
	svc, cl := newService(nil, autoDeployApp("api", repo))
	replays := &fakeReplayGuard{}
	h := &GitWebhook{Svc: svc, Secret: "manual-key", Replays: replays}
	body := pushBody(t, repo, "refs/heads/main")

	if rec := postPushDelivery(t, h, "manual-key", "push", "d-1", body); rec.Code != http.StatusOK {
		t.Fatalf("first delivery => %d: %s", rec.Code, rec.Body)
	}
	first := getApp(t, cl, "api").Spec.RestartedAt
	if first == "" {
		t.Fatal("first delivery should redeploy")
	}

	rec := postPushDelivery(t, h, "manual-key", "push", "d-2", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("replay => %d, want 200 (stop the host retrying)", rec.Code)
	}
	if !isReplay(rec) {
		t.Fatalf("replay response should carry replayed=true, got %s", rec.Body)
	}
	if got := getApp(t, cl, "api").Spec.RestartedAt != first; got {
		t.Error("a replayed delivery must not stamp a second deploy generation")
	}
	if n := replays.claimed(); n != 1 {
		t.Fatalf("claims = %d, want 1 (the replay must not add one)", n)
	}
}

// The delivery HEADER is unsigned — a replay that changes it (d-1 → d-2 in the
// test above) is still deduplicated because the claim is keyed on the signed
// bytes; this test makes that explicit against the header-keyed fact path.
func TestWebhookReplayChangingDeliveryHeaderStillSkipped(t *testing.T) {
	const repo = "https://github.com/x/app"
	svc, _ := newService(nil, autoDeployApp("api", repo))
	replays := &fakeReplayGuard{}
	h := &GitWebhook{Svc: svc, GitHubSecret: "gh-key", Replays: replays}
	body := pushBody(t, repo, "refs/heads/main")

	if rec := postPushDelivery(t, h, "gh-key", "push", "aaa", body); rec.Code != http.StatusOK {
		t.Fatalf("first delivery => %d", rec.Code)
	}
	rec := postPushDelivery(t, h, "gh-key", "push", "bbb", body)
	if rec.Code != http.StatusOK || !isReplay(rec) {
		t.Fatalf("header-swapped replay must still be skipped, got %d %s", rec.Code, rec.Body)
	}
}

// A genuinely different delivery body is NOT a replay.
func TestWebhookDistinctDeliveryIsNotAReplay(t *testing.T) {
	const repo = "https://github.com/x/app"
	svc, _ := newService(nil, autoDeployApp("api", repo))
	replays := &fakeReplayGuard{}
	h := &GitWebhook{Svc: svc, Secret: "manual-key", Replays: replays}

	if rec := postPushDelivery(t, h, "manual-key", "push", "d-1", pushBody(t, repo, "refs/heads/main")); rec.Code != http.StatusOK {
		t.Fatalf("first push => %d", rec.Code)
	}
	rec := postPushDelivery(t, h, "manual-key", "push", "d-2", pushBody(t, repo, "refs/heads/release"))
	if rec.Code != http.StatusOK {
		t.Fatalf("distinct push => %d", rec.Code)
	}
	if isReplay(rec) {
		t.Fatal("a distinct signed body is a new delivery, not a replay")
	}
	if n := replays.claimed(); n != 2 {
		t.Fatalf("claims = %d, want 2", n)
	}
}

// The delete event mutates too (facts + autoDeploy-off patches) and claims the
// same way.
func TestWebhookDeleteEventReplayIsSkipped(t *testing.T) {
	const repo = "https://github.com/x/app"
	svc, _ := newService(nil, autoDeployApp("api", repo))
	replays := &fakeReplayGuard{}
	h := &GitWebhook{Svc: svc, GitHubSecret: "gh-key", Replays: replays}
	body, err := json.Marshal(map[string]any{
		"ref":      "main",
		"ref_type": "branch",
		"repository": map[string]string{
			"clone_url": repo, "ssh_url": repo, "html_url": repo, "url": repo,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if rec := postPushDelivery(t, h, "gh-key", "delete", "d-1", body); rec.Code != http.StatusOK {
		t.Fatalf("first delete => %d: %s", rec.Code, rec.Body)
	}
	rec := postPushDelivery(t, h, "gh-key", "delete", "d-2", body)
	if rec.Code != http.StatusOK || !isReplay(rec) {
		t.Fatalf("replayed delete must be skipped, got %d %s", rec.Code, rec.Body)
	}
}

// An unreachable ledger fails closed (500), never an unclaimed mutation.
func TestWebhookReplayClaimFailureFailsClosed(t *testing.T) {
	const repo = "https://github.com/x/app"
	svc, _ := newService(nil, autoDeployApp("api", repo))
	replays := &fakeReplayGuard{claimErr: errors.New("ledger down")}
	h := &GitWebhook{Svc: svc, Secret: "manual-key", Replays: replays}

	if rec := postPushDelivery(t, h, "manual-key", "push", "d-1", pushBody(t, repo, "refs/heads/main")); rec.Code < 500 {
		t.Fatalf("claim failure => %d, want 5xx (fail closed)", rec.Code)
	}
}

// claimReplay releases the claim when the mutation branch answered 5xx (the git
// host will redeliver; that retry must not be swallowed) and keeps it on a
// completed 2xx answer.
func TestWebhookReplayClaimReleasedOnHardFailure(t *testing.T) {
	replays := &fakeReplayGuard{}
	h := &GitWebhook{Replays: replays}
	body := []byte(`{"ref":"refs/heads/main"}`)

	// 2xx: the claim stays — a completed delivery IS the processed state.
	w, finish, ok := h.claimReplay(context.Background(), httptest.NewRecorder(), keyManual, "manual:single-tenant", body)
	if !ok {
		t.Fatal("first claim must succeed")
	}
	w.WriteHeader(http.StatusOK)
	finish()
	if n := replays.claimed(); n != 1 {
		t.Fatalf("claims after 200 = %d, want 1", n)
	}

	// 5xx: the claim is released so the host's retry can process.
	replays2 := &fakeReplayGuard{}
	h2 := &GitWebhook{Replays: replays2}
	w2, finish2, ok2 := h2.claimReplay(context.Background(), httptest.NewRecorder(), keyManual, "manual:single-tenant", body)
	if !ok2 {
		t.Fatal("second claim must succeed (fresh guard)")
	}
	w2.WriteHeader(http.StatusBadGateway)
	finish2()
	if n := replays2.claimed(); n != 0 {
		t.Fatalf("claims after 502 = %d, want 0 (released for the retry)", n)
	}
}

// A valid push to a repository bex does not track is a no-op before replay
// admission, so ordinary activity on other installation-granted repositories
// cannot grow the ledger.
func TestWebhookUnmatchedRepositoryDoesNotClaimReplay(t *testing.T) {
	svc, _ := newService(nil, autoDeployApp("api", "https://github.com/x/tracked"))
	replays := &fakeReplayGuard{}
	h := &GitWebhook{Svc: svc, GitHubSecret: "gh-key", Replays: replays}

	rec := postPushDelivery(t, h, "gh-key", "push", "d-1", pushBody(t, "https://github.com/x/unrelated", "refs/heads/main"))
	if rec.Code != http.StatusOK {
		t.Fatalf("unmatched push => %d: %s", rec.Code, rec.Body)
	}
	if n := replays.claimed(); n != 0 {
		t.Fatalf("unmatched push claims = %d, want 0", n)
	}
}

func TestWebhookReplayCapacityFailsClosed(t *testing.T) {
	const repo = "https://github.com/x/app"
	svc, cl := newService(nil, autoDeployApp("api", repo))
	replays := &fakeReplayGuard{capacity: true}
	h := &GitWebhook{Svc: svc, GitHubSecret: "gh-key", Replays: replays}

	rec := postPushDelivery(t, h, "gh-key", "push", "d-1", pushBody(t, repo, "refs/heads/main"))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("capacity response = %d, want 503: %s", rec.Code, rec.Body)
	}
	if getApp(t, cl, "api").Spec.RestartedAt != "" {
		t.Fatal("capacity exhaustion must fail before mutation")
	}
}

func TestWebhookSigningKeyRotationCreatesNewEpochAndRejectsOldSignature(t *testing.T) {
	const repo = "https://github.com/x/app"
	svc, _ := newService(nil, autoDeployApp("api", repo))
	replays := &fakeReplayGuard{}
	body := pushBody(t, repo, "refs/heads/main")

	oldHandler := &GitWebhook{Svc: svc, GitHubSecret: "old-key", Replays: replays}
	if rec := postPushDelivery(t, oldHandler, "old-key", "push", "d-1", body); rec.Code != http.StatusOK {
		t.Fatalf("old epoch delivery = %d: %s", rec.Code, rec.Body)
	}
	newHandler := &GitWebhook{Svc: svc, GitHubSecret: "new-key", Replays: replays}
	if rec := postPushDelivery(t, newHandler, "old-key", "push", "d-2", body); rec.Code != http.StatusUnauthorized {
		t.Fatalf("old signature after rotation = %d, want 401", rec.Code)
	}
	if rec := postPushDelivery(t, newHandler, "new-key", "push", "d-3", body); rec.Code != http.StatusOK || !isReplay(rec) {
		t.Fatalf("same body under overlapping new epoch = %d %s, want replayed 200", rec.Code, rec.Body)
	}
	newBody := pushBody(t, repo, "refs/heads/release")
	if rec := postPushDelivery(t, newHandler, "new-key", "push", "d-4", newBody); rec.Code != http.StatusOK || isReplay(rec) {
		t.Fatalf("distinct delivery under new epoch = %d %s, want fresh 200", rec.Code, rec.Body)
	}
	if n := replays.claimed(); n != 2 {
		t.Fatalf("claims across old/new epochs = %d, want 2 before lease retirement", n)
	}
}

// Without a ledger (store-less operation) the webhook keeps its prior
// replayable behavior — byte-identical single-tenant mode.
func TestWebhookWithoutReplayGuardStaysPermissive(t *testing.T) {
	const repo = "https://github.com/x/app"
	svc, _ := newService(nil, autoDeployApp("api", repo))
	h := &GitWebhook{Svc: svc, Secret: "manual-key"}
	body := pushBody(t, repo, "refs/heads/main")

	for i, delivery := range []string{"d-1", "d-2"} {
		rec := postPushDelivery(t, h, "manual-key", "push", delivery, body)
		if rec.Code != http.StatusOK || isReplay(rec) {
			t.Fatalf("delivery %d without a guard => %d %s (prior behavior)", i+1, rec.Code, rec.Body)
		}
	}
}
