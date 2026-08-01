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
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// webhook.go closes the push-to-deploy loop (roadmap item 2): a git host's push
// webhook, authenticated by a shared-secret HMAC-SHA256 signature (GitHub /
// Gitea's X-Hub-Signature-256), redeploys every App built from the pushed repo
// and branch. It is the one caller of the unauthorized redeploy — the signature
// check is the authorization, since a git-host callback carries no OpenFGA
// identity — so it lives outside the api package's auth gate (server.go mounts
// it directly, ahead of the /v1/ auth wildcard).

const maxWebhookBody = 1 << 20 // 1 MiB — push payloads are small; cap to bound HMAC work.

// GitWebhook is the HMAC-verified push endpoint. It accepts two independent
// HMAC keys: Secret (the shared BEX_WEBHOOK_SECRET, manual per-repo webhooks) and
// GitHubSecret (BEX_GITHUB_WEBHOOK_SECRET, the GitHub App's app-wide webhook, so
// installed repos redeploy with zero per-repo config — docs/ADR026-github-integration.md).
// A delivery is accepted if it verifies under EITHER key; the endpoint 503s only
// when BOTH are empty (never accept unsigned pushes).
type GitWebhook struct {
	Svc          *Service
	Secret       string
	GitHubSecret string
}

// configured reports whether at least one HMAC key is set.
func (h *GitWebhook) configured() bool { return h.Secret != "" || h.GitHubSecret != "" }

// verify reports whether the signature matches under any configured key. Each
// validSignature call is constant-time (hmac.Equal); both configured keys are
// evaluated with no early return, so a remote timing analysis can't distinguish
// WHICH key matched — only that at least one did (w6/004).
func (h *GitWebhook) verify(sig string, body []byte) bool {
	var ok byte
	for _, secret := range []string{h.Secret, h.GitHubSecret} {
		if secret != "" && validSignature(secret, sig, body) {
			ok = 1
		}
	}
	return ok == 1
}

// pushEvent is the slice of a GitHub/Gitea push payload the webhook needs: which
// ref moved, which repository it belongs to (matched against App.spec.repo),
// which files each pushed commit touched (matched against App.spec.rootDir
// and App.spec.buildFilter, monorepo support) — GitHub and Gitea both carry
// added/removed/modified paths per commit in this shape — and the head commit,
// which becomes the deploy row's provenance without a round trip to the git host.
type pushEvent struct {
	Ref         string `json:"ref"` // e.g. refs/heads/main
	After       string `json:"after"`
	Deleted     bool   `json:"deleted"` // true when this push deleted the branch (git push --delete)
	DeliveryKey string `json:"-"`
	Repository  struct {
		CloneURL string `json:"clone_url"`
		SSHURL   string `json:"ssh_url"`
		HTMLURL  string `json:"html_url"`
		URL      string `json:"url"`
	} `json:"repository"`
	HeadCommit struct {
		ID        string `json:"id"`
		URL       string `json:"url"`
		Message   string `json:"message"`
		Timestamp string `json:"timestamp"` // ISO 8601, both hosts
	} `json:"head_commit"`
	Commits []struct {
		Added    []string `json:"added"`
		Removed  []string `json:"removed"`
		Modified []string `json:"modified"`
	} `json:"commits"`
}

// deleteEvent is the slice of GitHub's `delete` webhook payload the webhook needs
// (w7/m66): a branch (or tag) was deleted through the UI or API. GitHub delivers
// git-push deletions as a `push` with deleted=true (handled in ServeHTTP) and
// UI/API deletions as this separate `delete` event — a distinct type the app must
// also subscribe to (docs/ADR026-github-integration.md). ref is the plain branch
// name here, not a refs/heads/ ref.
type deleteEvent struct {
	Ref        string `json:"ref"`
	RefType    string `json:"ref_type"` // "branch" | "tag"; only branch deletions matter
	Repository struct {
		CloneURL string `json:"clone_url"`
		SSHURL   string `json:"ssh_url"`
		HTMLURL  string `json:"html_url"`
		URL      string `json:"url"`
	} `json:"repository"`
}

// commitInfo lifts the payload's head commit into the deploy row's provenance
// shape. Best-effort like every CommitInfo source: an absent head_commit (or an
// unparseable timestamp) degrades field-by-field, never fails the redeploy.
func (ev pushEvent) commitInfo() store.CommitInfo {
	if ev.HeadCommit.ID == "" {
		return store.CommitInfo{}
	}
	info := store.CommitInfo{Hash: ev.HeadCommit.ID, Message: ev.HeadCommit.Message}
	if at, err := time.Parse(time.RFC3339, ev.HeadCommit.Timestamp); err == nil {
		utc := at.UTC()
		info.AuthorAt = &utc
	}
	return info
}

// changedPaths flattens the added/removed/modified paths across every commit
// in the push.
func (ev pushEvent) changedPaths() []string {
	var paths []string
	for _, c := range ev.Commits {
		paths = append(paths, c.Added...)
		paths = append(paths, c.Removed...)
		paths = append(paths, c.Modified...)
	}
	return paths
}

// ServeHTTP verifies the signature, resolves the pushed repo+branch to matching
// Apps, and redeploys each. An absent/mismatched signature is 401 with no action.
func (h *GitWebhook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.configured() {
		core.WriteErrStatus(w, http.StatusServiceUnavailable, "git webhook not configured (BEX_WEBHOOK_SECRET / BEX_GITHUB_WEBHOOK_SECRET unset)")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBody))
	if err != nil {
		core.WriteErrStatus(w, http.StatusBadRequest, "cannot read body")
		return
	}
	if !h.verify(r.Header.Get("X-Hub-Signature-256"), body) {
		core.WriteErrStatus(w, http.StatusUnauthorized, "invalid or missing signature")
		return
	}
	// The GitHub App's one app-wide webhook also delivers lifecycle events
	// (ping on setup, installation/installation_repositories on grant changes).
	// They're validly signed, so don't 401 them — just no-op with 200. A branch
	// deletion arrives as its own `delete` event (w7/m66); an absent event header
	// (Gitea, or a manual GitHub push webhook) is treated as a push, preserving
	// the pre-App behavior byte-for-byte.
	event := r.Header.Get("X-GitHub-Event")
	if event == "delete" {
		h.serveDelete(w, r, body)
		return
	}
	if event != "" && event != "push" {
		core.WriteJSON(w, http.StatusOK, map[string]string{"ignored": event})
		return
	}
	var ev pushEvent
	if err := json.Unmarshal(body, &ev); err != nil {
		core.WriteErrStatus(w, http.StatusBadRequest, "malformed push payload")
		return
	}
	ev.DeliveryKey = deliveryKey(r, body)
	branch := strings.TrimPrefix(ev.Ref, "refs/heads/")
	// A branch-delete push (git push --delete: deleted=true, or an all-zero
	// `after`) carries no commit to build — record branch_deleted and disable
	// auto-deploy for services tracking it rather than attempting a redeploy.
	if ev.Deleted || isZeroSHA(ev.After) {
		urls := []string{ev.Repository.CloneURL, ev.Repository.SSHURL, ev.Repository.HTMLURL, ev.Repository.URL}
		h.writeBranchDeleted(r.Context(), w, urls, branch, ev.DeliveryKey)
		return
	}
	redeployed, err := h.redeployMatching(r.Context(), ev, branch)
	if err != nil {
		core.WriteErr(w, err)
		return
	}
	core.WriteJSON(w, http.StatusOK, map[string]any{"redeployed": redeployed})
}

// serveDelete handles GitHub's `delete` event — a UI/API branch (or tag)
// deletion. Only branch deletions produce a branch_deleted event; a tag deletion
// is a signed no-op.
func (h *GitWebhook) serveDelete(w http.ResponseWriter, r *http.Request, body []byte) {
	var ev deleteEvent
	if err := json.Unmarshal(body, &ev); err != nil {
		core.WriteErrStatus(w, http.StatusBadRequest, "malformed delete payload")
		return
	}
	if ev.RefType != "branch" {
		core.WriteJSON(w, http.StatusOK, map[string]string{"ignored": "delete " + ev.RefType})
		return
	}
	branch := strings.TrimPrefix(ev.Ref, "refs/heads/")
	urls := []string{ev.Repository.CloneURL, ev.Repository.SSHURL, ev.Repository.HTMLURL, ev.Repository.URL}
	h.writeBranchDeleted(r.Context(), w, urls, branch, deliveryKey(r, body))
}

// writeBranchDeleted records branch_deleted for every matching service and
// writes the shared `{branchDeleted: [...]}` response — the one tail both the
// `delete` event and the push-with-deleted path funnel through.
func (h *GitWebhook) writeBranchDeleted(ctx context.Context, w http.ResponseWriter, urls []string, branch, deliveryKey string) {
	deleted, err := h.recordBranchDeleted(ctx, urls, branch, deliveryKey)
	if err != nil {
		core.WriteErr(w, err)
		return
	}
	core.WriteJSON(w, http.StatusOK, map[string]any{"branchDeleted": deleted})
}

// deliveryKey is the delivery's stable identity for fact idempotency: GitHub's
// X-GitHub-Delivery header, or a body hash for a host (Gitea) that omits it.
func deliveryKey(r *http.Request, body []byte) string {
	if k := r.Header.Get("X-GitHub-Delivery"); k != "" {
		return k
	}
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:])
}

// isZeroSHA reports whether sha is git's all-zero object id, which a push
// carries in `after` when it deletes the branch. Empty is not a delete.
func isZeroSHA(sha string) bool {
	if sha == "" {
		return false
	}
	return strings.Trim(sha, "0") == ""
}

// recordBranchDeleted records a branch_deleted event for every App tracking the
// deleted branch on the pushed repository, and disables its auto-deploy — the
// branch it built from is gone, so Render (and bex) stop auto-deploying it. It
// reads/writes through the raw client like redeployMatching: the HMAC signature
// already authorized this call. Returns the affected service names.
func (h *GitWebhook) recordBranchDeleted(ctx context.Context, urls []string, branch, deliveryKey string) ([]string, error) {
	if branch == "" {
		return []string{}, nil
	}
	var list appv1alpha1.AppList
	if err := h.Svc.Client.List(ctx, &list, client.InNamespace(h.Svc.Namespace)); err != nil {
		return nil, err
	}
	affected := []string{}
	for i := range list.Items {
		a := &list.Items[i]
		if a.Spec.Repo == "" || !repoURLsMatch(a.Spec.Repo, urls...) || !branchMatches(a.Spec.Branch, branch) {
			continue
		}
		h.recordBranchDeletedFact(ctx, a, branch, deliveryKey)
		if a.Spec.AutoDeploy {
			if err := h.disableAutoDeploy(ctx, a); err != nil {
				log.Printf("webhook: disable auto-deploy for %s after branch delete: %v", a.Name, err)
			}
		}
		affected = append(affected, a.Name)
	}
	return affected, nil
}

// recordBranchDeletedFact appends the closed branch_deleted fact — the deleted
// branch name in branch_from, no commit, no value. Idempotent by delivery+app.
func (h *GitWebhook) recordBranchDeletedFact(ctx context.Context, app *appv1alpha1.App, branch, deliveryKey string) {
	if h.Svc.EventFacts == nil {
		return
	}
	appID := managedAppID(app)
	if appID == "" {
		return
	}
	fact := store.ServiceEventFact{
		SourceKey:  fmt.Sprintf("git:%s:%s:branch_deleted", deliveryKey, appID),
		AppID:      appID,
		Type:       store.EventFactBranchDeleted,
		At:         h.Svc.Now().UTC(),
		BranchFrom: branch,
	}
	if _, err := h.Svc.EventFacts.InsertServiceEventFact(ctx, fact); err != nil {
		log.Printf("events: record branch delete for %s: %v", appID, err)
	}
}

// disableAutoDeploy clears spec.autoDeploy on the App — a direct CR patch, the
// same field SetAutoDeploy owns (not projection-owned, so it survives resync).
func (h *GitWebhook) disableAutoDeploy(ctx context.Context, app *appv1alpha1.App) error {
	patch := client.MergeFrom(app.DeepCopy())
	app.Spec.AutoDeploy = false
	return h.Svc.Client.Patch(ctx, app, patch)
}

// redeployMatching lists every App and redeploys those whose spec.repo matches
// the pushed repository and whose tracked branch matches the pushed ref. It
// reads/writes through the raw client (not the authorized List/verbs): the HMAC
// signature already authorized this call.
func (h *GitWebhook) redeployMatching(ctx context.Context, ev pushEvent, branch string) ([]string, error) {
	var list appv1alpha1.AppList
	if err := h.Svc.Client.List(ctx, &list, client.InNamespace(h.Svc.Namespace)); err != nil {
		return nil, err
	}
	paths := ev.changedPaths()
	redeployed := []string{}
	for i := range list.Items {
		a := &list.Items[i]
		// autoDeploy gates push-triggered redeploys (Render's autoDeploy: no):
		// an App that opted out is never bumped by a push, only by an explicit deploy.
		if !a.Spec.AutoDeploy {
			continue
		}
		if a.Spec.Repo == "" || !repoMatches(a.Spec.Repo, ev) || !branchMatches(a.Spec.Branch, branch) {
			continue
		}
		if commitHasSkipPhrase(ev.HeadCommit.Message) {
			h.recordCommitIgnored(ctx, a, ev, store.EventReasonSkipPhrase)
			continue
		}
		if !rootDirMatches(a.Spec.RootDir, paths) {
			h.recordCommitIgnored(ctx, a, ev, store.EventReasonRootDirectory)
			continue
		}
		if !buildFilterMatches(a.Spec.BuildFilter, paths) {
			h.recordCommitIgnored(ctx, a, ev, store.EventReasonBuildFilter)
			continue
		}
		if _, err := h.Svc.redeploy(ctx, a.Name, ev.commitInfo()); err != nil {
			// Log but do not propagate: a 5xx response causes the git host to
			// retry the delivery, re-triggering redeploy on apps that were already
			// bumped (each retry stamps a new spec.restartedAt, incrementing the
			// CR generation and triggering a full rebuild — the generation churn
			// that can fill the Zot registry volume). Return 200 with the
			// partial list instead.
			log.Printf("webhook: redeploy %s: %v", a.Name, err)
			continue
		}
		redeployed = append(redeployed, a.Name)
	}
	return redeployed, nil
}

func commitHasSkipPhrase(message string) bool {
	message = strings.ToLower(message)
	for _, phrase := range []string{"[skip render]", "[render skip]"} {
		if strings.Contains(message, phrase) {
			return true
		}
	}
	return false
}

func (h *GitWebhook) recordCommitIgnored(ctx context.Context, app *appv1alpha1.App, ev pushEvent, reason string) {
	if h.Svc.EventFacts == nil {
		return
	}
	appID := managedAppID(app)
	if appID == "" {
		return
	}
	commitID := ev.HeadCommit.ID
	if commitID == "" {
		commitID = ev.After
	}
	commitURL := ev.HeadCommit.URL
	if commitURL != "" && !core.ValidAbsoluteHTTPURL(commitURL) {
		commitURL = ""
	}
	fact := store.ServiceEventFact{
		SourceKey: fmt.Sprintf("git:%s:%s:ignored", ev.DeliveryKey, appID),
		AppID:     appID, Type: store.EventFactCommitIgnored, At: h.Svc.Now().UTC(),
		ReasonCode: reason, CommitID: commitID, CommitURL: commitURL,
	}
	if _, err := h.Svc.EventFacts.InsertServiceEventFact(ctx, fact); err != nil {
		log.Printf("events: record ignored commit for %s: %v", appID, err)
	}
}

// repoMatches reports whether an App's spec.repo names the pushed repository,
// comparing against every URL form the payload carries (clone/ssh/html/api),
// each canonicalized so an https clone URL matches a ".git"-suffixed spec.
func repoMatches(specRepo string, ev pushEvent) bool {
	return repoURLsMatch(specRepo, ev.Repository.CloneURL, ev.Repository.SSHURL, ev.Repository.HTMLURL, ev.Repository.URL)
}

// repoURLsMatch reports whether specRepo names any of the given repository URL
// forms, each canonicalized so an https clone URL matches a ".git"-suffixed spec.
// Shared by the push (repoMatches) and delete (recordBranchDeleted) paths.
func repoURLsMatch(specRepo string, urls ...string) bool {
	want := core.CanonicalRepo(specRepo)
	if want == "" {
		return false
	}
	for _, u := range urls {
		if u != "" && core.CanonicalRepo(u) == want {
			return true
		}
	}
	return false
}

// branchMatches reports whether an App tracking specBranch should redeploy for a
// push to branch. An empty spec.branch tracks "main" (the CR default).
func branchMatches(specBranch, branch string) bool {
	if specBranch == "" {
		specBranch = "main"
	}
	return branch == "" || specBranch == branch
}

// rootDirMatches gates the path-scoped auto-deploy filter (App.spec.rootDir,
// monorepo support, mirroring Render's Root Directory setting): a push whose
// changed files all sit outside rootDir does not redeploy an App scoped to it.
// An empty rootDir always matches (today's whole-repo behavior, unchanged).
// When rootDir is set but the payload carries no commit path info at all
// (e.g. a host/event that omits it), this fails open and matches, since there
// is nothing to filter on.
func rootDirMatches(rootDir string, paths []string) bool {
	if rootDir == "" || len(paths) == 0 {
		return true
	}
	dir := strings.Trim(rootDir, "/")
	prefix := dir + "/"
	for _, p := range paths {
		p = strings.TrimPrefix(p, "/")
		if p == dir || strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}

// buildFilterMatches gates the glob-based auto-deploy filter (App.spec.buildFilter,
// Render's Build Filters): a push redeploys only when at least one changed file is
// a "triggering" file — one that matches an include glob (or, with no includes, any
// file) and is NOT excluded by an ignore glob. Ignored wins over included, matching
// Render ("ignored paths will not trigger an autodeploy, even if those files also
// match an included path"). A nil or all-empty filter always matches (today's
// behavior, unchanged). Globs are repository-root-relative (matching Render — a
// filter can name paths outside RootDir), so this composes independently with the
// coarse rootDirMatches prefix scoping. When the filter is set but the payload
// carries no changed-path info, this fails open and matches (as rootDirMatches
// does), since there is nothing to filter on.
func buildFilterMatches(bf *appv1alpha1.BuildFilterSpec, paths []string) bool {
	if bf == nil || (len(bf.Paths) == 0 && len(bf.IgnoredPaths) == 0) {
		return true
	}
	if len(paths) == 0 {
		return true
	}
	for _, p := range paths {
		p = strings.TrimPrefix(p, "/")
		if matchesAnyGlob(bf.IgnoredPaths, p) {
			continue // ignored files never trigger, even when they also match Paths
		}
		if len(bf.Paths) == 0 || matchesAnyGlob(bf.Paths, p) {
			return true
		}
	}
	return false
}

// matchesAnyGlob reports whether path matches any of the doublestar glob patterns
// (Render's dialect: *, **, ?, and [class] wildcards, "/"-separated). A malformed
// pattern — which the API layer rejects on write (store.ValidGlob) — simply never
// matches here, so a bad hand-applied CR can't wedge the webhook.
func matchesAnyGlob(globs []string, path string) bool {
	for _, g := range globs {
		if ok, err := doublestar.Match(strings.TrimPrefix(g, "/"), path); err == nil && ok {
			return true
		}
	}
	return false
}

// validSignature verifies a "sha256=<hex>" header against the HMAC-SHA256 of
// body under secret, in constant time. Anything malformed is a mismatch.
func validSignature(secret, header string, body []byte) bool {
	const prefix = "sha256="
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	got, err := hex.DecodeString(header[len(prefix):])
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hmac.Equal(got, mac.Sum(nil))
}
