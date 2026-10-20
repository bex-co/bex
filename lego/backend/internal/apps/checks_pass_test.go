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
	"strings"
	"sync"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/github"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w1/m181: autoDeployTrigger checksPass.

// fakeChecks is a ChecksSource: permitted answers ChecksPermitted, runs is
// every commit's check runs.
type fakeChecks struct {
	mu        sync.Mutex
	permitted bool
	runs      []github.CheckRun
	asked     []string
}

func (f *fakeChecks) ChecksPermitted(context.Context, string, string) (bool, error) {
	return f.permitted, nil
}

func (f *fakeChecks) CommitCheckRuns(_ context.Context, _, _, sha string) ([]github.CheckRun, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, sha)
	return f.runs, nil
}

func run(status, conclusion string) github.CheckRun {
	return github.CheckRun{Name: "ci", Status: status, Conclusion: conclusion}
}

func TestChecksVerdict(t *testing.T) {
	for _, c := range []struct {
		name string
		runs []github.CheckRun
		want checksState
	}{
		{"no checks never deploys", nil, checksPending},
		{"all pass", []github.CheckRun{run("completed", "success"), run("completed", "neutral"), run("completed", "skipped")}, checksPassed},
		{"one running", []github.CheckRun{run("completed", "success"), run("in_progress", "")}, checksPending},
		{"one queued", []github.CheckRun{run("queued", "")}, checksPending},
		{"a failure decides at once", []github.CheckRun{run("in_progress", ""), run("completed", "failure")}, checksFailed},
		{"cancelled fails", []github.CheckRun{run("completed", "cancelled")}, checksFailed},
		{"timed out fails", []github.CheckRun{run("completed", "timed_out")}, checksFailed},
		{"action required fails", []github.CheckRun{run("completed", "action_required")}, checksFailed},
	} {
		if got := checksVerdict(c.runs); got != c.want {
			t.Errorf("%s: verdict = %v, want %v", c.name, got, c.want)
		}
	}
}

// checksApp is a repo service on checksPass awaiting sha.
func checksApp(name, repo, sha string) *appv1alpha1.App {
	a := autoDeployApp(name, repo)
	a.Spec.AutoDeployTrigger = checksPassTrigger
	if sha != "" {
		a.Annotations = map[string]string{annotChecksPendingCommit: sha}
	}
	return a
}

func checkSuiteBody(t *testing.T, repo, branch, sha, action string) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"action": action,
		"check_suite": map[string]any{
			"head_branch": branch, "head_sha": sha,
			"head_commit": map[string]string{"message": "feat: ship", "timestamp": "2026-10-17T00:00:00Z"},
		},
		"repository": map[string]string{"clone_url": repo},
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

const (
	checksRepo = "https://github.com/x/app"
	shaA       = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	shaB       = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

// A push to a checksPass service records the commit instead of deploying it.
func TestPushToChecksPassServiceAwaitsChecks(t *testing.T) {
	svc, cl := newService(nil, checksApp("api", checksRepo, ""))
	h := &GitWebhook{Svc: svc, GitHubSecret: "gh-key"}
	if rec := postPush(t, h, "gh-key", "push", pushBody(t, checksRepo, "refs/heads/main")); rec.Code != http.StatusOK {
		t.Fatalf("push => %d: %s", rec.Code, rec.Body)
	}
	got := getApp(t, cl, "api")
	if got.Spec.RestartedAt != "" {
		t.Error("a checksPass push must not deploy before its checks pass")
	}
	if got.Annotations[annotChecksPendingCommit] != shaA {
		t.Errorf("awaited commit = %q, want the pushed %s", got.Annotations[annotChecksPendingCommit], shaA)
	}
}

// A completed suite with all checks passing deploys the awaited commit once;
// a duplicate delivery or a second suite finishing does not deploy again.
func TestCheckSuiteDeploysTheAwaitedCommitOnce(t *testing.T) {
	svc, cl := newService(nil, checksApp("api", checksRepo, shaA))
	svc.Checks = &fakeChecks{permitted: true, runs: []github.CheckRun{run("completed", "success")}}
	h := &GitWebhook{Svc: svc, GitHubSecret: "gh-key"}
	body := checkSuiteBody(t, checksRepo, "main", shaA, "completed")

	rec := postPush(t, h, "gh-key", "check_suite", body)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"api"`) {
		t.Fatalf("check_suite => %d: %s, want api redeployed", rec.Code, rec.Body)
	}
	first := getApp(t, cl, "api")
	if first.Spec.RestartedAt == "" || first.Spec.BuildCommit != shaA {
		t.Fatalf("deploy = restartedAt %q buildCommit %q, want a deploy of %s", first.Spec.RestartedAt, first.Spec.BuildCommit, shaA)
	}
	if _, ok := first.Annotations[annotChecksPendingCommit]; ok {
		t.Error("the awaited commit must be consumed by its deploy")
	}

	if rec := postPush(t, h, "gh-key", "check_suite", body); !strings.Contains(rec.Body.String(), `"redeployed":[]`) {
		t.Errorf("a second suite for the same commit = %s, want no deploy", rec.Body)
	}
	if again := getApp(t, cl, "api"); again.Spec.RestartedAt != first.Spec.RestartedAt {
		t.Error("a second suite redeployed the same commit")
	}
}

func TestCheckSuiteWaitsThroughRunningAndFailedChecks(t *testing.T) {
	checks := &fakeChecks{permitted: true, runs: []github.CheckRun{run("completed", "success"), run("in_progress", "")}}
	svc, cl := newService(nil, checksApp("api", checksRepo, shaA))
	svc.Checks = checks
	h := &GitWebhook{Svc: svc, GitHubSecret: "gh-key"}
	body := checkSuiteBody(t, checksRepo, "main", shaA, "completed")
	awaiting := func() (string, bool) {
		got := getApp(t, cl, "api")
		return got.Annotations[annotChecksPendingCommit], got.Spec.RestartedAt != ""
	}

	postPush(t, h, "gh-key", "check_suite", body)
	if sha, deployed := awaiting(); deployed || sha != shaA {
		t.Fatalf("with a check still running: deployed %v awaited %q, want still waiting", deployed, sha)
	}

	// A failed check never deploys, but the commit stays awaited: a re-run
	// that passes deploys it, and a newer push would replace it.
	checks.runs = []github.CheckRun{run("completed", "success"), run("completed", "failure")}
	postPush(t, h, "gh-key", "check_suite", body)
	if sha, deployed := awaiting(); deployed || sha != shaA {
		t.Fatalf("after a failure: deployed %v awaited %q, want no deploy and still awaited", deployed, sha)
	}
	checks.runs = []github.CheckRun{run("completed", "success"), run("completed", "success")}
	postPush(t, h, "gh-key", "check_suite", body)
	if sha, deployed := awaiting(); !deployed || sha != "" {
		t.Fatalf("after a passing re-run: deployed %v awaited %q, want deployed and consumed", deployed, sha)
	}
}

// A commit CI already checked on another branch, then fast-forwarded to the
// tracked branch, gets no further suite: the push deploys it at once. Its
// suites carry the other branch, which must not stop a later one either.
func TestAlreadyPassingCommitDeploysOnPushAndSuitesIgnoreTheirBranch(t *testing.T) {
	svc, cl := newService(nil, checksApp("api", checksRepo, ""))
	svc.Checks = &fakeChecks{permitted: true, runs: []github.CheckRun{run("completed", "success")}}
	h := &GitWebhook{Svc: svc, GitHubSecret: "gh-key"}
	if rec := postPush(t, h, "gh-key", "push", pushBody(t, checksRepo, "refs/heads/main")); !strings.Contains(rec.Body.String(), `"api"`) {
		t.Fatalf("push of an already-passing commit = %s, want api redeployed", rec.Body)
	}
	if got := getApp(t, cl, "api"); got.Spec.BuildCommit != shaA || got.Annotations[annotChecksPendingCommit] != "" {
		t.Fatalf("deploy = buildCommit %q awaited %q", got.Spec.BuildCommit, got.Annotations[annotChecksPendingCommit])
	}

	svc2, cl2 := newService(nil, checksApp("api", checksRepo, shaA))
	svc2.Checks = &fakeChecks{permitted: true, runs: []github.CheckRun{run("completed", "success")}}
	h2 := &GitWebhook{Svc: svc2, GitHubSecret: "gh-key"}
	postPush(t, h2, "gh-key", "check_suite", checkSuiteBody(t, checksRepo, "feature/x", shaA, "completed"))
	if getApp(t, cl2, "api").Spec.BuildCommit != shaA {
		t.Error("a suite reporting the branch CI ran on must still deploy the awaited commit")
	}
}

// Any other write between the read and the claim (an operator status update)
// is retried, not taken for a rival claim that would strand the commit.
func TestCheckedDispatchRetriesAConflict(t *testing.T) {
	base := fakeClient(checksApp("api", checksRepo, shaA)).(client.WithWatch)
	conflicts := 0
	cl := interceptor.NewClient(base, interceptor.Funcs{
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			if conflicts == 0 {
				conflicts++
				return apierrors.NewConflict(schema.GroupResource{Group: "app.bex.co", Resource: "apps"}, obj.GetName(), errors.New("stale"))
			}
			return c.Patch(ctx, obj, patch, opts...)
		},
	})
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}, Checks: &fakeChecks{permitted: true, runs: []github.CheckRun{run("completed", "success")}}}
	h := &GitWebhook{Svc: svc}
	a := checksApp("api", checksRepo, shaA)
	if deployed := h.deployIfChecksPass(context.Background(), a, store.CommitInfo{Hash: shaA}); !deployed || conflicts != 1 {
		t.Fatalf("deployed %v after %d conflicts, want successful retry", deployed, conflicts)
	}
	if h.deployIfChecksPass(context.Background(), a, store.CommitInfo{Hash: shaA}) {
		t.Fatal("duplicate dispatch")
	}
}

// A Blueprint sync that turns checksPass on proves the gate, as the verb does.
func TestBlueprintSyncTurningOnChecksPassNeedsChecksAccess(t *testing.T) {
	existing := autoDeployApp("api", checksRepo)
	svc, _ := newService(nil, existing)
	final := existing.DeepCopy()
	final.Spec.AutoDeployTrigger = checksPassTrigger
	_, err := svc.patchChangedStackService(context.Background(), CreateRequest{}, existing, final, stackChanges{})
	if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "Checks: read") {
		t.Fatalf("sync without checks access = %v, want a refusal naming Checks: read", err)
	}
}

// Only the latest pushed commit can deploy: a suite for an older commit, a
// suite still running, or a forged non-suite body never does.
func TestCheckSuiteIgnoresEverythingButTheAwaitedCompletedSuite(t *testing.T) {
	for _, c := range []struct {
		name, event, sha, action, key string
	}{
		{"older commit", "check_suite", shaB, "completed", "gh-key"},
		{"suite still running", "check_suite", shaA, "requested", "gh-key"},
		{"manual key cannot deliver suites", "check_suite", shaA, "completed", "manual-key"},
	} {
		t.Run(c.name, func(t *testing.T) {
			checks := &fakeChecks{permitted: true, runs: []github.CheckRun{run("completed", "success")}}
			svc, cl := newService(nil, checksApp("api", checksRepo, shaA))
			svc.Checks = checks
			h := &GitWebhook{Svc: svc, Secret: "manual-key", GitHubSecret: "gh-key"}
			if rec := postPush(t, h, c.key, c.event, checkSuiteBody(t, checksRepo, "main", c.sha, c.action)); rec.Code != http.StatusOK {
				t.Fatalf("=> %d: %s", rec.Code, rec.Body)
			}
			if getApp(t, cl, "api").Spec.RestartedAt != "" || len(checks.asked) != 0 {
				t.Errorf("deployed or read checks (asked %v)", checks.asked)
			}
		})
	}

	// A signed push body replayed under the check_suite header is no suite.
	svc, cl := newService(nil, checksApp("api", checksRepo, shaA))
	svc.Checks = &fakeChecks{permitted: true, runs: []github.CheckRun{run("completed", "success")}}
	h := &GitWebhook{Svc: svc, GitHubSecret: "gh-key"}
	postPush(t, h, "gh-key", "check_suite", pushBody(t, checksRepo, "refs/heads/main"))
	if getApp(t, cl, "api").Spec.RestartedAt != "" {
		t.Error("a push body under a check_suite header deployed")
	}
}

// Selecting checksPass proves the gate can work: an image service has no
// commits, and a repository without Checks: read is refused by name.
func TestChecksPassNeedsARepositoryWithChecksAccess(t *testing.T) {
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})
	repoApp := autoDeployApp("api", checksRepo)
	svc, cl := newService(nil, repoApp)

	if _, err := svc.SetAutoDeployTrigger(ctx, "api", "checksPass"); !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "Checks: read") {
		t.Fatalf("no checks access = %v, want a refusal naming Checks: read", err)
	}
	svc.Checks = &fakeChecks{permitted: false}
	if _, err := svc.SetAutoDeployTrigger(ctx, "api", "checksPass"); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("installation without the permission = %v, want refused", err)
	}

	svc.Checks = &fakeChecks{permitted: true}
	view, err := svc.SetAutoDeployTrigger(ctx, "api", "checksPass")
	if err != nil || view.AutoDeployTrigger != "checksPass" || !view.AutoDeploy {
		t.Fatalf("permitted = %+v, %v; want checksPass", view, err)
	}
	if got := getApp(t, cl, "api").Spec; !got.AutoDeploy || got.AutoDeployTrigger != checksPassTrigger {
		t.Errorf("stored spec = autoDeploy %v trigger %q", got.AutoDeploy, got.AutoDeployTrigger)
	}

	// commit and off clear the gate and forget an awaited commit.
	stored := getApp(t, cl, "api")
	stored.Annotations = map[string]string{annotChecksPendingCommit: shaA}
	if err := cl.Update(ctx, stored); err != nil {
		t.Fatal(err)
	}
	if view, err := svc.SetAutoDeployTrigger(ctx, "api", "commit"); err != nil || view.AutoDeployTrigger != "commit" {
		t.Fatalf("commit = %+v, %v", view, err)
	}
	if got := getApp(t, cl, "api"); got.Spec.AutoDeployTrigger != "" || got.Annotations[annotChecksPendingCommit] != "" {
		t.Errorf("after commit: trigger %q awaited %q, want both cleared", got.Spec.AutoDeployTrigger, got.Annotations[annotChecksPendingCommit])
	}

	image := sampleApp("img")
	svc2, _ := newService(nil, image)
	svc2.Checks = &fakeChecks{permitted: true}
	if _, err := svc2.SetAutoDeployTrigger(ctx, "img", "checksPass"); !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "prebuilt image") {
		t.Errorf("image service checksPass = %v, want refused", err)
	}
}

// TestBlueprintChecksPassTrigger is the capability registry's fixture for the
// translated checksPass value: a repo service's checksPass parses, and an
// export of a checksPass service writes it back.
func TestBlueprintChecksPassTrigger(t *testing.T) {
	source, problems := CompileBlueprintSource(`
services:
  - type: web
    name: api
    runtime: docker
    repo: https://github.com/x/app
    autoDeployTrigger: checksPass
`)
	if len(problems) != 0 || source == nil {
		t.Fatalf("checksPass compile problems = %+v", problems)
	}
	req, _, err := parseService(blueprintParseOverrides{}, bexService{
		Name: "api", Type: "web", Runtime: "docker", Repo: checksRepo, AutoDeployTrigger: "checksPass",
	})
	if err != nil || deref(req.AutoDeployChecksPass) != true {
		t.Fatalf("parse = %+v, %v", req.AutoDeployChecksPass, err)
	}
	spec, err := specFromCreate(req)
	if err != nil || spec.AutoDeployTrigger != checksPassTrigger || !spec.AutoDeploy {
		t.Fatalf("spec = trigger %q autoDeploy %v, %v", spec.AutoDeployTrigger, spec.AutoDeploy, err)
	}
	if _, problems := CompileBlueprintSource(`
services:
  - type: web
    name: img
    runtime: image
    image: {url: nginx:1}
    autoDeployTrigger: checksPass
`); findBlueprintProblem(problems, "BLUEPRINT_CAPABILITY_INCOMPATIBLE") == nil {
		t.Errorf("checksPass on a prebuilt image = %+v, want incompatible", problems)
	}
}
