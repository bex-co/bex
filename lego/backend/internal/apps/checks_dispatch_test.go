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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/github"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestCheckedDispatchNeverConsumesBeforeRelease(t *testing.T) {
	svc, base := newService(nil, checksApp("api", checksRepo, shaA))
	svc.Checks = &fakeChecks{permitted: true, runs: []github.CheckRun{run("completed", "success")}}
	consumedWithoutRelease := false
	svc.Client = interceptor.NewClient(base.(client.WithWatch), interceptor.Funcs{
		Patch: func(ctx context.Context, cl client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			if err := cl.Patch(ctx, obj, patch, opts...); err != nil {
				return err
			}
			a := getApp(t, cl, "api")
			if a.Annotations[annotChecksPendingCommit] == "" && a.Spec.BuildCommit != shaA {
				consumedWithoutRelease = true
			}
			return nil
		},
	})
	h := &GitWebhook{Svc: svc, GitHubSecret: "gh-key"}
	postPush(t, h, "gh-key", "check_suite", checkSuiteBody(t, checksRepo, "main", shaA, "completed"))
	if consumedWithoutRelease {
		t.Fatal("accepted checked commit disappeared before its release was requested: a crash there loses the deployment")
	}
}

type checkedHistoryStore struct {
	*recordingStore
	failBefore, loseReply bool
}

func (s *checkedHistoryStore) EnsureCheckedDeploy(ctx context.Context, appID string, generation int64, commit store.CommitInfo) (store.Deploy, error) {
	if s.failBefore {
		return store.Deploy{}, errors.New("history unavailable")
	}
	d, err := s.recordingStore.EnsureCheckedDeploy(ctx, appID, generation, commit)
	if err == nil && s.loseReply {
		s.loseReply = false
		return store.Deploy{}, errors.New("history reply lost after commit")
	}
	return d, err
}

func TestCheckedWorkerRecoversAcceptedPushAfterRestart(t *testing.T) {
	svc, cl := newService(nil, checksApp("api", checksRepo, ""))
	h := &GitWebhook{Svc: svc, GitHubSecret: "gh-key"}
	postPush(t, h, "gh-key", "push", pushBody(t, checksRepo, "refs/heads/main"))
	if a := getApp(t, cl, "api"); a.Annotations[annotChecksPendingCommit] != shaA || a.Spec.BuildCommit != "" {
		t.Fatalf("push was not durably waiting: %+v", a)
	}
	// A new service/worker has no state from the accepted HTTP request.
	restarted, _ := newService(nil)
	restarted.Client = cl
	restarted.Checks = &fakeChecks{permitted: true, runs: []github.CheckRun{run("completed", "success")}}
	w := &CheckedDeployWorker{Svc: restarted}
	if err := w.tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	first := getApp(t, cl, "api")
	if first.Spec.BuildCommit != shaA || first.Annotations[annotChecksPendingCommit] != "" || first.Annotations[annotChecksDispatch] != "" {
		t.Fatalf("recovery did not request and settle the accepted release: %+v", first)
	}
	if err := w.tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if again := getApp(t, cl, "api"); again.Spec.RestartedAt != first.Spec.RestartedAt {
		t.Fatal("a recovery poll redeployed a consumed push")
	}
}

func TestCheckedReceiptRecoversHistoryWithoutRedeploying(t *testing.T) {
	for _, lostReply := range []bool{false, true} {
		t.Run(map[bool]string{false: "before database write", true: "after database commit"}[lostReply], func(t *testing.T) {
			st := &checkedHistoryStore{recordingStore: &recordingStore{}, failBefore: !lostReply, loseReply: lostReply}
			a := checksApp("api", checksRepo, shaA)
			a.Generation = 4
			a.Labels = map[string]string{core.LabelAppID: "srv-check", store.LabelManagedBy: store.ManagedByValue}
			svc, cl := newService(st, a)
			svc.Checks = &fakeChecks{permitted: true, runs: []github.CheckRun{run("completed", "success")}}
			h := &GitWebhook{Svc: svc, GitHubSecret: "gh-key"}
			postPush(t, h, "gh-key", "check_suite", checkSuiteBody(t, checksRepo, "main", shaA, "completed"))
			dispatched := getApp(t, cl, "api")
			if dispatched.Spec.BuildCommit != shaA || dispatched.Annotations[annotChecksDispatch] == "" {
				t.Fatalf("release has no durable history receipt: %+v", dispatched)
			}
			st.failBefore = false
			restarted, _ := newService(st)
			restarted.Client = cl
			w := &CheckedDeployWorker{Svc: restarted}
			if err := w.tick(context.Background()); err != nil {
				t.Fatal(err)
			}
			if err := w.tick(context.Background()); err != nil {
				t.Fatal(err)
			}
			recovered := getApp(t, cl, "api")
			if recovered.Annotations[annotChecksDispatch] != "" || recovered.Spec.RestartedAt != dispatched.Spec.RestartedAt {
				t.Fatal("history recovery lost its receipt or requested another release")
			}
			if len(st.deployCalls) != 1 || st.deployCalls[0].Generation != 5 || st.deployCalls[0].Commit != shaA {
				t.Fatalf("recovered history = %+v, want one original-generation row", st.deployCalls)
			}
		})
	}
}

func TestCheckedSourceIsRevalidatedAfterRemoteReads(t *testing.T) {
	for _, change := range []string{"off", "repo", "branch", "suspended", "newer push", "replacement"} {
		t.Run(change, func(t *testing.T) {
			initial := checksApp("api", checksRepo, "")
			initial.UID = "initial"
			svc, base := newService(nil, initial)
			h := &GitWebhook{Svc: svc, GitHubSecret: "gh-key"}
			if err := h.awaitChecks(context.Background(), getApp(t, base, "api"), store.CommitInfo{Hash: shaA}); err != nil {
				t.Fatal(err)
			}
			svc.Checks = &changingChecks{change: func() {
				a := getApp(t, base, "api")
				switch change {
				case "off":
					a.Spec.AutoDeploy = false
				case "repo":
					a.Spec.Repo += "-other"
				case "branch":
					a.Spec.Branch = "other"
				case "suspended":
					a.Spec.Suspended = true
				case "newer push":
					a.Annotations[annotChecksPendingCommit] = shaB
				case "replacement":
					if err := base.Delete(context.Background(), a); err != nil {
						t.Fatal(err)
					}
					a.ResourceVersion = ""
					a.UID = "replacement"
					if err := base.Create(context.Background(), a); err != nil {
						t.Fatal(err)
					}
					return
				}
				if err := base.Update(context.Background(), a); err != nil {
					t.Fatal(err)
				}
			}}
			postPush(t, h, "gh-key", "check_suite", checkSuiteBody(t, checksRepo, "main", shaA, "completed"))
			if got := getApp(t, base, "api"); got.Spec.RestartedAt != "" {
				t.Fatalf("stale checked work deployed after %s", change)
			}
		})
	}
}

type changingChecks struct{ change func() }

func (*changingChecks) ChecksPermitted(context.Context, string, string) (bool, error) {
	return true, nil
}
func (c *changingChecks) CommitCheckRuns(context.Context, string, string, string) ([]github.CheckRun, error) {
	c.change()
	return []github.CheckRun{run("completed", "success")}, nil
}

func TestCheckedWorkerRejectsDelayedOldPushAndAcceptsForcePush(t *testing.T) {
	for _, head := range []string{shaA, shaB} {
		t.Run(head[:1], func(t *testing.T) {
			svc, cl := newService(nil, checksApp("api", checksRepo, shaA))
			svc.Checks = &fakeChecks{permitted: true, runs: []github.CheckRun{run("completed", "success")}}
			svc.Commits = commitsAt(head)
			w := &CheckedDeployWorker{Svc: svc}
			if err := w.tick(context.Background()); err != nil {
				t.Fatal(err)
			}
			got := getApp(t, cl, "api")
			if head == shaB && got.Spec.BuildCommit != "" {
				t.Fatal("delayed old push rolled back the branch")
			}
			if head == shaA && got.Spec.BuildCommit != shaA {
				t.Fatal("authoritative force-pushed branch was not deployed")
			}
			if got.Annotations[annotChecksPendingCommit] != "" {
				t.Fatal("terminal pending source was not settled")
			}
		})
	}
}

func TestFailedCheckedDispatchCannotRestoreOverNewerPush(t *testing.T) {
	svc, base := newService(nil, checksApp("api", checksRepo, shaA))
	svc.Checks = &fakeChecks{permitted: true, runs: []github.CheckRun{run("completed", "success")}}
	injected := false
	svc.Client = interceptor.NewClient(base.(client.WithWatch), interceptor.Funcs{
		Patch: func(ctx context.Context, cl client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			data, err := patch.Data(obj)
			if err != nil {
				return err
			}
			var body struct {
				Spec appv1alpha1.AppSpec `json:"spec"`
			}
			if err := json.Unmarshal(data, &body); err != nil {
				return err
			}
			if !injected && body.Spec.BuildCommit == shaA {
				injected = true
				newer := getApp(t, cl, "api")
				if newer.Annotations == nil {
					newer.Annotations = map[string]string{}
				}
				newer.Annotations[annotChecksPendingCommit] = shaB
				if err := cl.Update(ctx, newer); err != nil {
					return err
				}
				return errors.New("release write unavailable")
			}
			return cl.Patch(ctx, obj, patch, opts...)
		},
	})
	h := &GitWebhook{Svc: svc, GitHubSecret: "gh-key"}
	postPush(t, h, "gh-key", "check_suite", checkSuiteBody(t, checksRepo, "main", shaA, "completed"))
	if !injected {
		t.Fatal("did not reach release dispatch")
	}
	if got := getApp(t, base, "api"); got.Annotations[annotChecksPendingCommit] != shaB {
		t.Fatalf("pending commit = %q, want newer %s", got.Annotations[annotChecksPendingCommit], shaB)
	}
}

// B's candidate snapshot may precede A's atomic release. Consumption is not a
// rival push: after the optimistic retry, B must still become durable.
func TestCheckedAcceptanceSurvivesPredecessorConsumption(t *testing.T) {
	svc, cl := newService(nil, checksApp("api", checksRepo, shaA))
	h := &GitWebhook{Svc: svc}
	candidate := getApp(t, cl, "api")
	consumed := candidate.DeepCopy()
	clearCheckedPending(consumed)
	consumed.Spec.BuildCommit = shaA
	if err := cl.Update(context.Background(), consumed); err != nil {
		t.Fatal(err)
	}
	if err := h.awaitChecks(context.Background(), candidate, store.CommitInfo{Hash: shaB}); err != nil {
		t.Fatal(err)
	}
	if got := getApp(t, cl, "api"); got.Annotations[annotChecksPendingCommit] != shaB {
		t.Fatal("newer accepted push was lost after predecessor consumption")
	}
}

func TestCheckedAcceptanceDoesNotReplaceBranchTipWithDelayedPush(t *testing.T) {
	svc, cl := newService(nil, checksApp("api", checksRepo, shaB))
	svc.Commits = commitsAt(shaB)
	h := &GitWebhook{Svc: svc}
	if err := h.awaitChecks(context.Background(), getApp(t, cl, "api"), store.CommitInfo{Hash: shaA}); err != nil {
		t.Fatal(err)
	}
	if got := getApp(t, cl, "api"); got.Annotations[annotChecksPendingCommit] != shaB {
		t.Fatal("delayed old arrival replaced the actual branch tip")
	}
	// An ancestry-independent force push is accepted in either SHA direction.
	svc.Commits = commitsAt(shaA)
	if err := h.awaitChecks(context.Background(), getApp(t, cl, "api"), store.CommitInfo{Hash: shaA}); err != nil {
		t.Fatal(err)
	}
	if got := getApp(t, cl, "api"); got.Annotations[annotChecksPendingCommit] != shaA {
		t.Fatal("force-pushed branch tip was rejected")
	}
}

func TestCheckedAcceptanceDuplicateDoesNotPatch(t *testing.T) {
	svc, base := newService(nil, checksApp("api", checksRepo, ""))
	h := &GitWebhook{Svc: svc}
	commit := store.CommitInfo{Hash: shaA}
	if err := h.awaitChecks(context.Background(), getApp(t, base, "api"), commit); err != nil {
		t.Fatal(err)
	}
	patches := 0
	svc.Client = interceptor.NewClient(base.(client.WithWatch), interceptor.Funcs{Patch: func(ctx context.Context, cl client.WithWatch, obj client.Object, p client.Patch, opts ...client.PatchOption) error {
		patches++
		return cl.Patch(ctx, obj, p, opts...)
	}})
	if err := h.awaitChecks(context.Background(), getApp(t, base, "api"), commit); err != nil {
		t.Fatal(err)
	}
	if patches != 0 {
		t.Fatalf("duplicate acceptance made %d patches", patches)
	}
}

type checkedImageStore struct {
	*recordingStore
	clears int
}

func (s *checkedImageStore) ClearCheckedDeployImage(ctx context.Context, appID, repo, branch, image string, version int64) (bool, error) {
	s.clears++
	return true, nil
}

func TestRejectedCheckedDispatchDoesNotClearManagedRollback(t *testing.T) {
	for _, change := range []string{"off", "suspended", "newer push"} {
		t.Run(change, func(t *testing.T) {
			st := &checkedImageStore{recordingStore: &recordingStore{}}
			initial := checksApp("api", checksRepo, shaA)
			initial.Spec.Image = "registry.test/rollback:v1"
			initial.Labels = map[string]string{core.LabelAppID: "srv-check", store.LabelManagedBy: store.ManagedByValue}
			svc, cl := newService(st, initial)
			svc.Checks = &changingChecks{change: func() {
				a := getApp(t, cl, "api")
				switch change {
				case "off":
					a.Spec.AutoDeploy = false
				case "suspended":
					a.Spec.Suspended = true
				case "newer push":
					a.Annotations[annotChecksPendingCommit] = shaB
				}
				if err := cl.Update(context.Background(), a); err != nil {
					t.Fatal(err)
				}
			}}
			_, _ = svc.dispatchCheckedCommit(context.Background(), getApp(t, cl, "api"), store.CommitInfo{Hash: shaA})
			if st.clears != 0 {
				t.Fatalf("rejected dispatch cleared database rollback %d times", st.clears)
			}
			if got := getApp(t, cl, "api"); got.Spec.Image != initial.Spec.Image {
				t.Fatal("rejected dispatch changed rollback image")
			}
		})
	}
}

func TestCheckedWorkerRotatesPastBatchOfWaitingChecks(t *testing.T) {
	objs := make([]*appv1alpha1.App, 0, 51)
	for i := 0; i < 51; i++ {
		objs = append(objs, checksApp(fmt.Sprintf("api-%02d", i), checksRepo, shaA))
	}
	svc, cl := newService(nil, objs...)
	checks := &fakeChecks{permitted: true}
	svc.Checks = checks
	w := &CheckedDeployWorker{Svc: svc}
	if err := w.tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	checks.runs = []github.CheckRun{run("completed", "success")}
	if err := w.tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := getApp(t, cl, "api-50"); got.Spec.BuildCommit != shaA {
		t.Fatal("waiting first batch starved later accepted work")
	}
}

type unavailableCheckedSource struct{}

func (unavailableCheckedSource) ResolveCommit(context.Context, string, string, string) (store.CommitInfo, bool, error) {
	return store.CommitInfo{}, false, errors.New("GitHub unavailable")
}

func TestCheckedIntakeFailureCanBeRedeliveredWithoutGenerationChurn(t *testing.T) {
	checked := checksApp("checked", checksRepo, shaA)
	ordinary := checksApp("ordinary", checksRepo, "")
	ordinary.Spec.AutoDeployTrigger = "commit"
	svc, cl := newService(nil, checked, ordinary)
	svc.Commits = unavailableCheckedSource{}
	replays := &fakeReplayGuard{}
	h := &GitWebhook{Svc: svc, GitHubSecret: "gh-key", Replays: replays}
	body := bytes.ReplaceAll(pushBody(t, checksRepo, "refs/heads/main"), []byte(shaA), []byte(shaB))
	rec := postPush(t, h, "gh-key", "push", body)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("intake outage => %d, want retryable refusal: %s", rec.Code, rec.Body)
	}
	if replays.claimed() != 0 {
		t.Fatal("failed intake retained its replay claim")
	}
	if got := getApp(t, cl, "ordinary"); got.Spec.RestartedAt != "" {
		t.Fatal("failed intake already redeployed another service")
	}
	svc.Commits = commitsAt(shaB)
	rec = postPush(t, h, "gh-key", "push", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("redelivery => %d: %s", rec.Code, rec.Body)
	}
	if got := getApp(t, cl, "checked"); got.Annotations[annotChecksPendingCommit] != shaB {
		t.Fatal("successful redelivery did not durably accept B")
	}
	// Restarted recovery needs neither another push nor another suite delivery.
	svc.Checks = &fakeChecks{permitted: true, runs: []github.CheckRun{run("completed", "success")}}
	w := &CheckedDeployWorker{Svc: svc}
	if err := w.tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := getApp(t, cl, "checked"); got.Spec.BuildCommit != shaB {
		t.Fatal("accepted redelivery did not recover")
	}
	first := getApp(t, cl, "ordinary").Spec.RestartedAt
	rec = postPush(t, h, "gh-key", "push", body)
	if rec.Code != http.StatusOK || getApp(t, cl, "ordinary").Spec.RestartedAt != first {
		t.Fatal("duplicate delivery churned an ordinary service")
	}
}

func TestCheckedIntakeFailureRetainsCompletedWorkspaceClaim(t *testing.T) {
	ordinary := checksApp("ordinary", checksRepo, "")
	ordinary.Spec.AutoDeployTrigger = "commit"
	ordinary.Labels = map[string]string{core.LabelTenant: "tea-a"}
	checked := checksApp("checked", checksRepo, shaA)
	checked.Labels = map[string]string{core.LabelTenant: "tea-b"}
	svc, cl := newService(nil, ordinary, checked)
	svc.Commits = unavailableCheckedSource{}
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	svc.Clock = func() time.Time { return now }
	replays := &fakeReplayGuard{}
	h := &GitWebhook{Svc: svc, GitHubSecret: "gh-key", Replays: replays, Installations: fakeInstallations{7: {"tea-a", "tea-b"}}}
	ev := newPush(checksRepo, []string{"main.go"})
	ev.After, ev.Installation.ID = shaB, 7
	body, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	if rec := postPush(t, h, "gh-key", "push", body); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("later workspace intake failure => %d, want 503: %s", rec.Code, rec.Body)
	}
	first := getApp(t, cl, "ordinary")
	if first.Spec.BuildCommit != shaB || first.Spec.RestartedAt == "" {
		t.Fatal("first workspace did not complete its ordinary deployment")
	}
	if n := replays.claimed(); n != 1 {
		t.Fatalf("claims after later workspace failure = %d, want completed workspace only", n)
	}
	if got := getApp(t, cl, "checked"); got.Annotations[annotChecksPendingCommit] != shaA {
		t.Fatal("failed intake changed the later workspace's acceptance")
	}

	svc.Commits = commitsAt(shaB)
	svc.Checks = &fakeChecks{permitted: true, runs: []github.CheckRun{run("completed", "success")}}
	now = now.Add(time.Minute)
	if rec := postPush(t, h, "gh-key", "push", body); rec.Code != http.StatusOK {
		t.Fatalf("recovered redelivery => %d: %s", rec.Code, rec.Body)
	}
	if got := getApp(t, cl, "ordinary"); got.Spec.RestartedAt != first.Spec.RestartedAt {
		t.Fatal("later workspace retry redeployed the already completed ordinary service")
	}
	if got := getApp(t, cl, "checked"); got.Spec.BuildCommit != shaB {
		t.Fatal("redelivery did not complete the failed workspace's checked deployment")
	}
	if n := replays.claimed(); n != 2 {
		t.Fatalf("claims after recovery = %d, want both completed workspaces", n)
	}
}

func TestCheckedPartialIntakeRedeliveryDoesNotRepeatRecoveredRelease(t *testing.T) {
	accepted := checksApp("a-accepted", checksRepo, "")
	failing := checksApp("z-failing", checksRepo, shaA)
	svc, cl := newService(nil, accepted, failing)
	svc.Commits = unavailableCheckedSource{}
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	svc.Clock = func() time.Time { return now }
	replays := &fakeReplayGuard{}
	h := &GitWebhook{Svc: svc, GitHubSecret: "gh-key", Replays: replays}
	body := bytes.ReplaceAll(pushBody(t, checksRepo, "refs/heads/main"), []byte(shaA), []byte(shaB))
	if rec := postPush(t, h, "gh-key", "push", body); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("partial intake failure => %d, want 503: %s", rec.Code, rec.Body)
	}
	if n := replays.claimed(); n != 0 {
		t.Fatalf("partial intake retained %d claims, want retryable scope", n)
	}
	if got := getApp(t, cl, "a-accepted"); got.Annotations[annotChecksPendingCommit] != shaB || got.Spec.RestartedAt != "" {
		t.Fatal("first checked acceptance was not durable before the later intake failure")
	}

	// Recovery may consume the partial acceptance before GitHub redelivers it.
	svc.Commits = commitsAt(shaB)
	svc.Checks = &fakeChecks{permitted: true, runs: []github.CheckRun{run("completed", "success")}}
	w := &CheckedDeployWorker{Svc: svc}
	if err := w.tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	first := getApp(t, cl, "a-accepted")
	if first.Spec.BuildCommit != shaB || first.Spec.RestartedAt == "" || first.Annotations[annotChecksPendingCommit] != "" {
		t.Fatal("worker did not consume the accepted commit before redelivery")
	}
	now = now.Add(time.Minute)
	if rec := postPush(t, h, "gh-key", "push", body); rec.Code != http.StatusOK {
		t.Fatalf("partial intake redelivery => %d: %s", rec.Code, rec.Body)
	}
	if got := getApp(t, cl, "a-accepted"); got.Spec.RestartedAt != first.Spec.RestartedAt || got.Annotations[annotChecksPendingCommit] != "" {
		t.Fatal("redelivery requested a second release of the recovered checked commit")
	}
	if got := getApp(t, cl, "z-failing"); got.Spec.BuildCommit != shaB {
		t.Fatal("redelivery did not deploy the previously failed checked acceptance")
	}
}
