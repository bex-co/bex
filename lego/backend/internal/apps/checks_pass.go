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
	"fmt"
	"log"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/github"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
	"github.com/bex-co/bex/lego/backend/internal/rollout"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// autoDeployTrigger checksPass (w1/m181, docs/render-artifacts/
// auto-deploy-checks-pass.md): a push that passes a service's filters records
// its commit as awaiting checks instead of deploying; a completed GitHub
// check_suite for that commit deploys it once all of its check runs pass.

// checksPassTrigger is spec.autoDeployTrigger's checks-gated value.
const checksPassTrigger = "checksPass"

// Pending acceptance and the release receipt live on the same App so a single
// optimistic write can request the release without losing accepted work.
const (
	annotChecksPendingCommit = "app.bex.co/checks-pending-commit"
	annotChecksPendingSource = "app.bex.co/checks-pending-source"
	annotChecksDispatch      = resourcemeta.AnnotCheckedDeployReceipt
)

type checkedSource struct {
	Repo   string           `json:"repo"`
	Branch string           `json:"branch"`
	Commit store.CommitInfo `json:"commit"`
}

type checkedDispatch struct {
	Generation int64            `json:"generation"`
	Commit     store.CommitInfo `json:"commit"`
	Repo       string           `json:"repo"`
	Branch     string           `json:"branch"`
	Image      string           `json:"image,omitempty"`
	RowVersion int64            `json:"rowVersion"`
}

var errCheckedSuperseded = errors.New("checked deployment superseded")

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
// skipped) deploys; any completed failure refuses deployment; anything
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

// awaitChecks persists source-bound acceptance before any remote check reads.
// A consumed predecessor does not supersede a newer push. Rival pending commits
// are ordered by the authoritative branch tip rather than event arrival order.
func (h *GitWebhook) awaitChecks(ctx context.Context, a *appv1alpha1.App, commit store.CommitInfo) error {
	source := checkedSource{Repo: a.Spec.Repo, Branch: a.Spec.Branch, Commit: commit}
	// Kubernetes annotations have a shared size budget; history keeps a bounded
	// commit summary rather than allowing an arbitrary push message to consume it.
	if len(source.Commit.Message) > 8192 {
		runes := []rune(source.Commit.Message)
		source.Commit.Message = string(runes[:min(len(runes), 2048)])
	}
	data, err := json.Marshal(source)
	if err != nil {
		return err
	}
	previous := a.Annotations[annotChecksPendingCommit]
	err = rollout.PatchAppLocked(ctx, h.Svc.Client, a, func(live *appv1alpha1.App) error {
		pending := live.Annotations[annotChecksPendingCommit]
		if !checksGated(live.Spec) || live.Spec.Repo != source.Repo || live.Spec.Branch != source.Branch ||
			!live.DeletionTimestamp.IsZero() {
			return errCheckedSuperseded
		}
		if (pending == commit.Hash && live.Annotations[annotChecksPendingSource] == string(data)) ||
			(pending == "" && live.Spec.BuildCommit == commit.Hash) {
			return errCheckedSuperseded // exact redelivery is already durable
		}
		if pending != "" && pending != commit.Hash {
			if h.Svc.Commits != nil {
				branch := source.Branch
				if branch == "" {
					branch = appv1alpha1.DefaultBranch
				}
				head, ok, err := h.Svc.Commits.ResolveCommit(ctx, h.Svc.AppWorkspace(ctx, live), source.Repo, branch)
				if err != nil {
					return err
				}
				if !ok {
					return fmt.Errorf("checked source branch is unavailable")
				}
				if head.Hash != commit.Hash {
					return errCheckedSuperseded
				}
			} else if pending != previous {
				return errCheckedSuperseded
			}
		}
		if live.Annotations == nil {
			live.Annotations = map[string]string{}
		}
		live.Annotations[annotChecksPendingCommit] = commit.Hash
		live.Annotations[annotChecksPendingSource] = string(data)
		return nil
	})
	if errors.Is(err, errCheckedSuperseded) {
		return nil
	}
	return err
}

func checkedPendingSource(a *appv1alpha1.App, commit store.CommitInfo) (checkedSource, error) {
	source := checkedSource{Repo: a.Spec.Repo, Branch: a.Spec.Branch, Commit: commit}
	if raw := a.Annotations[annotChecksPendingSource]; raw != "" {
		if err := json.Unmarshal([]byte(raw), &source); err != nil {
			return checkedSource{}, err
		}
	}
	return source, nil
}

func checkedSourceMatches(a *appv1alpha1.App, source checkedSource) bool {
	return checksGated(a.Spec) && a.DeletionTimestamp.IsZero() &&
		a.Annotations[annotChecksPendingCommit] == source.Commit.Hash &&
		a.Spec.Repo == source.Repo && a.Spec.Branch == source.Branch
}

func clearCheckedPending(a *appv1alpha1.App) {
	delete(a.Annotations, annotChecksPendingCommit)
	delete(a.Annotations, annotChecksPendingSource)
}

func (s *Service) discardCheckedPending(ctx context.Context, a *appv1alpha1.App, pending, binding string) error {
	return rollout.PatchAppLocked(ctx, s.Client, a, func(live *appv1alpha1.App) error {
		if live.Annotations[annotChecksPendingCommit] != pending || live.Annotations[annotChecksPendingSource] != binding {
			return errCheckedSuperseded
		}
		clearCheckedPending(live)
		return nil
	})
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
		if h.deployIfChecksPass(ctx, a, commit) {
			redeployed = append(redeployed, a.Name)
		}
	}
	core.WriteJSON(w, http.StatusOK, map[string]any{"redeployed": redeployed})
}

// Suite delivery is a low-latency wake-up; periodic recovery handles crashes
// and transient failures without requiring another external suite event.
func (h *GitWebhook) deployIfChecksPass(ctx context.Context, a *appv1alpha1.App, commit store.CommitInfo) bool {
	deployed, err := h.Svc.dispatchCheckedCommit(ctx, a, commit)
	if err != nil {
		log.Printf("webhook: checked deploy %s: %v", a.Name, err)
	}
	return deployed
}

func (s *Service) dispatchCheckedCommit(ctx context.Context, a *appv1alpha1.App, commit store.CommitInfo) (bool, error) {
	sha := commit.Hash
	if err := core.RereadApp(ctx, s.Client, a); err != nil {
		return false, err
	}
	if err := s.finishCheckedDispatch(ctx, a); err != nil {
		return false, err
	}
	if s.Checks == nil || a.Annotations[annotChecksPendingCommit] != sha {
		return false, nil
	}
	source, err := checkedPendingSource(a, commit)
	if err != nil {
		return false, fmt.Errorf("decode checked source: %w", err)
	}
	binding := a.Annotations[annotChecksPendingSource]
	if !checkedSourceMatches(a, source) {
		return false, s.discardCheckedPending(ctx, a, sha, binding)
	}
	if a.Spec.Suspended {
		return false, nil
	}
	permitted, err := s.Checks.ChecksPermitted(ctx, s.AppWorkspace(ctx, a), source.Repo)
	if err != nil {
		return false, err
	}
	if !permitted {
		return false, nil
	}
	runs, err := s.Checks.CommitCheckRuns(ctx, s.AppWorkspace(ctx, a), source.Repo, sha)
	if err != nil {
		return false, err
	}
	if checksVerdict(runs) != checksPassed {
		return false, nil
	}
	// Arrival order cannot establish Git order. Re-read the actual branch tip;
	// a legitimate force push may move it in either ancestry direction.
	if s.Commits != nil {
		branch := source.Branch
		if branch == "" {
			branch = appv1alpha1.DefaultBranch
		}
		head, ok, err := s.Commits.ResolveCommit(ctx, s.AppWorkspace(ctx, a), source.Repo, branch)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, fmt.Errorf("checked source branch is unavailable")
		}
		if head.Hash != sha {
			return false, s.discardCheckedPending(ctx, a, sha, binding)
		}
	}
	if err := s.RequireBillingMutation(ctx, a.Labels[core.LabelTenant]); err != nil {
		return false, err
	}
	cloneSecret, err := s.ensureCloneSecret(ctx, a)
	if err != nil {
		return false, err
	}
	pullSecret, err := s.ensureExternalRegistryPullSecret(ctx, a)
	if err != nil {
		return false, err
	}
	image := a.Spec.Image
	var rowVersion int64
	if s.Store != nil && managedAppID(a) != "" {
		version, matches, err := s.Store.CheckedDeploySourceVersion(ctx, managedAppID(a), source.Repo, source.Branch, image)
		if err != nil {
			return false, err
		}
		if !matches {
			return false, nil
		}
		rowVersion = version
	}
	err = rollout.PatchAppLocked(ctx, s.Client, a, func(live *appv1alpha1.App) error {
		if !checkedSourceMatches(live, source) || live.Spec.Suspended || live.Spec.Image != image ||
			live.Annotations[annotChecksPendingSource] != binding || live.Annotations[annotChecksDispatch] != "" {
			return errCheckedSuperseded
		}
		generation := live.Generation + 1
		receipt, err := json.Marshal(checkedDispatch{Generation: generation, Commit: source.Commit, Repo: source.Repo, Branch: source.Branch, Image: image, RowVersion: rowVersion})
		if err != nil {
			return err
		}
		clearCheckedPending(live)
		if live.Annotations == nil {
			live.Annotations = map[string]string{}
		}
		live.Annotations[annotChecksDispatch] = string(receipt)
		if s.Store != nil && managedAppID(live) != "" {
			live.Annotations[resourcemeta.AnnotCheckedImageVersion] = strconv.FormatInt(rowVersion, 10)
		}
		stampReleaseGeneration(live, generation)
		if cloneSecret != "" {
			live.Spec.CloneSecret = cloneSecret
		}
		live.Spec.ExternalRegistryPullSecret = pullSecret
		live.Spec.Image = ""
		live.Spec.BuildCommit = sha
		live.Spec.RestartedAt = s.Now().UTC().Format(time.RFC3339Nano)
		resourcemeta.Touch(live, s.Now())
		return nil
	})
	if errors.Is(err, errCheckedSuperseded) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	s.notifyDeployStarted(ctx, a, core.AppPublicName(a))
	return true, s.finishCheckedDispatch(ctx, a)
}

// The receipt is written with the release, and cleared only after an idempotent
// history write. A lost database reply or API restart therefore cannot lose the
// row or request the release again. New pushes may await alongside this receipt.
func (s *Service) finishCheckedDispatch(ctx context.Context, a *appv1alpha1.App) error {
	raw := a.Annotations[annotChecksDispatch]
	if raw == "" {
		return nil
	}
	var receipt checkedDispatch
	if err := json.Unmarshal([]byte(raw), &receipt); err != nil {
		return err
	}
	if receipt.Generation <= 0 || !validGitObjectID(receipt.Commit.Hash) {
		return fmt.Errorf("invalid checked deploy receipt")
	}
	if s.Store != nil && managedAppID(a) != "" {
		if _, err := s.Store.ClearCheckedDeployImage(ctx, managedAppID(a), receipt.Repo, receipt.Branch, receipt.Image, receipt.RowVersion); err != nil {
			return err
		}
		if _, err := s.Store.EnsureCheckedDeploy(ctx, managedAppID(a), receipt.Generation, receipt.Commit); err != nil {
			return err
		}
	}
	err := rollout.PatchAppLocked(ctx, s.Client, a, func(live *appv1alpha1.App) error {
		if live.Annotations[annotChecksDispatch] != raw {
			return errCheckedSuperseded
		}
		delete(live.Annotations, annotChecksDispatch)
		return nil
	})
	if errors.Is(err, errCheckedSuperseded) {
		return nil
	}
	return err
}
