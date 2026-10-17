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
	"fmt"
	"log"
	"net/http"
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/github"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// autoDeployTrigger checksPass (w1/m181, docs/render-artifacts/
// auto-deploy-checks-pass.md): a push that passes a service's filters records
// its commit as awaiting checks instead of deploying; a completed GitHub
// check_suite for that commit deploys it once all of its check runs pass.

// checksPassTrigger is spec.autoDeployTrigger's checks-gated value.
const checksPassTrigger = "checksPass"

// annotChecksPendingCommit holds the latest pushed commit a checksPass service
// awaits checks for. A newer push overwrites it, so a late result for an
// older commit never deploys; removing it with an optimistic update is what
// makes one deploy per commit even when two suites finish together.
const annotChecksPendingCommit = "app.bex.co/checks-pending-commit"

// ChecksSource reads a commit's GitHub check runs through the workspace's
// GitHub App connection. The github service satisfies it structurally.
type ChecksSource interface {
	// ChecksPermitted reports whether repoURL is reached through a connection
	// whose installation granted Checks: read.
	ChecksPermitted(ctx context.Context, workspaceID, repoURL string) (bool, error)
	// CommitCheckRuns lists sha's latest check run of every check.
	CommitCheckRuns(ctx context.Context, workspaceID, repoURL, sha string) ([]github.CheckRun, error)
}

type checksState int

const (
	checksPending checksState = iota
	checksPassed
	checksFailed
)

// checksVerdict applies Render's rule: all checks pass (success, neutral or
// skipped) deploys; any completed failure drops the commit at once; anything
// still running waits. No check runs at all waits too: Render never deploys a
// commit with zero checks.
func checksVerdict(runs []github.CheckRun) checksState {
	if len(runs) == 0 {
		return checksPending
	}
	state := checksPassed
	for _, run := range runs {
		if run.Status != "completed" {
			state = checksPending
			continue
		}
		switch run.Conclusion {
		case "success", "neutral", "skipped":
		default:
			return checksFailed
		}
	}
	return state
}

// requireChecksPermitted refuses checksPass for a service whose commits bex
// cannot gate on: an image service has none, and a repository reached without
// Checks: read would wait forever without saying so.
func (s *Service) requireChecksPermitted(ctx context.Context, a *appv1alpha1.App) error {
	if a.Spec.Repo == "" {
		return fmt.Errorf("%w: autoDeployTrigger %q needs a Git repository; a prebuilt image has no commits to check", core.ErrBadRequest, checksPassTrigger)
	}
	permitted := false
	if s.Checks != nil {
		var err error
		if permitted, err = s.Checks.ChecksPermitted(ctx, s.AppWorkspace(ctx, a), a.Spec.Repo); err != nil {
			return fmt.Errorf("checking GitHub checks access: %w", err)
		}
	}
	if !permitted {
		return fmt.Errorf("%w: autoDeployTrigger %q needs the repository connected through the bex GitHub App with the Checks: read permission granted (and the Check suite event subscribed); grant it in the App's GitHub settings, or use %q", core.ErrBadRequest, checksPassTrigger, "commit")
	}
	return nil
}

// checksGated reports a spec whose pushes wait for their commit's checks.
func checksGated(spec appv1alpha1.AppSpec) bool {
	return spec.AutoDeploy && spec.AutoDeployTrigger == checksPassTrigger
}

// awaitChecks records sha as the commit a checksPass service waits on,
// replacing any older one. It is metadata only: no release, no generation.
func (h *GitWebhook) awaitChecks(ctx context.Context, a *appv1alpha1.App, sha string) error {
	if a.Annotations[annotChecksPendingCommit] == sha {
		return nil
	}
	base := a.DeepCopy()
	if a.Annotations == nil {
		a.Annotations = map[string]string{}
	}
	a.Annotations[annotChecksPendingCommit] = sha
	return h.Svc.Client.Patch(ctx, a, client.MergeFrom(base))
}

// takeChecksPending removes the awaited commit if it is still sha, with the
// App's resourceVersion as a precondition, so of two deliveries racing on one
// commit exactly one removes it and reports true. A conflict from any other
// write (an operator status update) re-reads and tries again rather than
// stranding the commit.
func (h *GitWebhook) takeChecksPending(ctx context.Context, a *appv1alpha1.App, sha string) (bool, error) {
	for range 5 {
		var live appv1alpha1.App
		if err := h.Svc.Client.Get(ctx, client.ObjectKeyFromObject(a), &live); err != nil {
			return false, client.IgnoreNotFound(err)
		}
		if live.Annotations[annotChecksPendingCommit] != sha {
			return false, nil
		}
		base := live.DeepCopy()
		delete(live.Annotations, annotChecksPendingCommit)
		err := h.Svc.Client.Patch(ctx, &live, client.MergeFromWithOptions(base, client.MergeFromWithOptimisticLock{}))
		if apierrors.IsConflict(err) {
			continue
		}
		if err != nil {
			return false, err
		}
		*a = live
		return true, nil
	}
	return false, fmt.Errorf("claiming checked commit %s for %s: the App kept changing", sha, a.Name)
}

// checkSuiteEvent is the subset of GitHub's check_suite payload a deploy needs.
type checkSuiteEvent struct {
	Action     string `json:"action"`
	CheckSuite struct {
		HeadSHA    string `json:"head_sha"`
		HeadCommit struct {
			Message   string `json:"message"`
			Timestamp string `json:"timestamp"`
		} `json:"head_commit"`
	} `json:"check_suite"`
	Installation struct {
		ID int64 `json:"id"`
	} `json:"installation"`
	Repository gitRepository `json:"repository"`
}

// serveCheckSuite deploys a checksPass service's awaited commit once a
// completed check suite leaves all of its checks passing. Only GitHub App
// deliveries carry check suites. The signature authenticates the body, not
// the event header, so the body must itself be a completed check suite with a
// valid head commit before anything is looked up. The branch is not matched:
// the awaited commit was pushed to the tracked branch, while its suite may
// carry the branch CI first ran it on.
func (h *GitWebhook) serveCheckSuite(w http.ResponseWriter, r *http.Request, body []byte, key verifiedKey) {
	var ev checkSuiteEvent
	if key != keyGitHubApp || h.Svc.Checks == nil || json.Unmarshal(body, &ev) != nil || ev.Action != "completed" {
		core.WriteJSON(w, http.StatusOK, map[string]string{"ignored": "check_suite"})
		return
	}
	sha := ev.CheckSuite.HeadSHA
	if !validGitObjectID(sha) || isZeroSHA(sha) {
		core.WriteJSON(w, http.StatusOK, map[string]string{"ignored": "check_suite"})
		return
	}
	ctx := r.Context()
	// GitHub sends a suite per CI app per commit for every installed
	// repository; almost none name an awaited commit, so look for one before
	// resolving the installation's workspaces.
	var list appv1alpha1.AppList
	if err := h.Svc.Client.List(ctx, &list); err != nil {
		core.WriteErr(w, err)
		return
	}
	awaiting := make([]appv1alpha1.App, 0)
	for i := range list.Items {
		a := &list.Items[i]
		if checksGated(a.Spec) && a.Annotations[annotChecksPendingCommit] == sha && repoURLsMatch(a.Spec.Repo, ev.Repository.urls()...) {
			awaiting = append(awaiting, *a)
		}
	}
	redeployed := []string{}
	if len(awaiting) == 0 {
		core.WriteJSON(w, http.StatusOK, map[string]any{"redeployed": redeployed})
		return
	}
	scopes, proceed := h.scopesFor(ctx, key, ev.Installation.ID)
	if !proceed {
		core.WriteJSON(w, http.StatusOK, map[string]any{"redeployed": redeployed})
		return
	}
	commit := newCommitInfo(sha, ev.CheckSuite.HeadCommit.Message, ev.CheckSuite.HeadCommit.Timestamp)
	for i := range awaiting {
		a := &awaiting[i]
		if !slices.ContainsFunc(scopes, func(scope string) bool { return scope == "" || a.Labels[core.LabelTenant] == scope }) {
			continue
		}
		if h.deployIfChecksPass(ctx, a, sha, commit) {
			redeployed = append(redeployed, a.Name)
		}
	}
	core.WriteJSON(w, http.StatusOK, map[string]any{"redeployed": redeployed})
}

// deployIfChecksPass reads the awaited commit's checks and deploys it once
// they all pass. A failed check, a still-running one, or a read error leaves
// it awaiting: a re-run that passes, or the next completed suite, asks again,
// and a newer push replaces it. Like the push path, a per-service failure
// never fails the delivery, which GitHub would retry.
func (h *GitWebhook) deployIfChecksPass(ctx context.Context, a *appv1alpha1.App, sha string, commit store.CommitInfo) bool {
	if h.Svc.Checks == nil {
		return false
	}
	runs, err := h.Svc.Checks.CommitCheckRuns(ctx, h.Svc.AppWorkspace(ctx, a), a.Spec.Repo, sha)
	if err != nil {
		log.Printf("webhook: checks for %s at %s: %v", a.Name, sha, err)
		return false
	}
	switch checksVerdict(runs) {
	case checksPending:
		return false
	case checksFailed:
		log.Printf("webhook: %s not deployed: a check failed for %s", a.Name, sha)
		return false
	}
	if err := h.Svc.RequireBillingMutation(ctx, a.Labels[core.LabelTenant]); err != nil {
		log.Printf("webhook: skip checked deploy %s: billing enforced: %v", a.Name, err)
		return false
	}
	taken, err := h.takeChecksPending(ctx, a, sha)
	if err != nil || !taken {
		if err != nil {
			log.Printf("webhook: claim checked commit for %s at %s: %v", a.Name, sha, err)
		}
		return false
	}
	if _, err := h.Svc.redeployFetched(ctx, a, commit); err != nil {
		log.Printf("webhook: checked deploy %s: %v", a.Name, err)
		// Put the commit back so the next suite can try again.
		if err := h.awaitChecks(ctx, a, sha); err != nil {
			log.Printf("webhook: restore awaited commit for %s: %v", a.Name, err)
		}
		return false
	}
	return true
}
