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
	// Installations resolves a GitHub App installation id to the workspace that
	// owns it (the unique installation→workspace binding, w1/m65 F2). When set, a
	// GitHub-App-signed delivery is CONFINED to that workspace's Apps (codex #7): a
	// forged app-signed event can then reach only the installation's own
	// workspace, not another tenant's. Nil ⇒ app-signed deliveries fail closed
	// in multitenant operation (round-6 #9) and stay global single-tenant; a
	// manual-key delivery is always a global match — the manual secret is a
	// single operator-configured key with no per-repo binding (per-workspace
	// manual secrets are the follow-up).
	Installations InstallationResolver
	// Multitenant, when true, rejects the shared manual secret (BEX_WEBHOOK_SECRET)
	// because it carries no per-workspace binding and would authorize cross-tenant
	// deployment mutations (codex #4). The GitHub App key is unaffected — its
	// deliveries are confined via Installations. Set when the control-plane store
	// is active (BEX_CP_DB_URI set), i.e. genuine multitenant operation.
	Multitenant bool
	// Replays durably claims each processed delivery body so the exact signed
	// bytes can never be replayed (codex round-8 #9) — the HMAC authenticates
	// bytes, not freshness, and X-GitHub-Delivery is unsigned. Nil (store-less
	// single-tenant operation) keeps the prior replayable behavior: there is
	// nothing durable to claim with.
	Replays WebhookReplayGuard
}

// WebhookReplayGuard is the durable replay ledger the mutating git-webhook
// deliveries claim before acting. Implemented by *store.PGStore.
type WebhookReplayGuard interface {
	ClaimGitWebhookDelivery(ctx context.Context, digest string) (bool, error)
	ReleaseGitWebhookDelivery(ctx context.Context, digest string) error
}

// InstallationResolver maps a GitHub App installation id to the workspace that
// owns it. Implemented in the composition root over the git-connection store.
type InstallationResolver interface {
	WorkspaceForInstallation(ctx context.Context, installationID int64) (workspaceID string, ok bool, err error)
}

// configured reports whether at least one HMAC key is set.
func (h *GitWebhook) configured() bool { return h.Secret != "" || h.GitHubSecret != "" }

// verifiedKey identifies which configured HMAC key accepted a delivery.
type verifiedKey int

const (
	keyNone      verifiedKey = iota // no configured key matched
	keyManual                       // the shared manual secret (BEX_WEBHOOK_SECRET)
	keyGitHubApp                    // the GitHub App's app-wide secret (BEX_GITHUB_WEBHOOK_SECRET)
)

// verifyKey reports which configured key accepted the signature (keyNone if
// neither). Each validSignature call is constant-time (hmac.Equal) and both
// configured keys are always evaluated with no early return, so a remote timing
// analysis can't distinguish WHICH key matched — only that at least one did
// (w6/004). The identity is used server-side to CONFINE a GitHub App delivery to
// its installation's workspace (codex #7); it is never exposed to the caller.
func (h *GitWebhook) verifyKey(sig string, body []byte) verifiedKey {
	manual := h.Secret != "" && validSignature(h.Secret, sig, body)
	ghApp := h.GitHubSecret != "" && validSignature(h.GitHubSecret, sig, body)
	switch {
	case manual:
		return keyManual
	case ghApp:
		return keyGitHubApp
	default:
		return keyNone
	}
}

// scopeFor returns the workspace an accepted delivery is confined to ("" = an
// unconfined/global match) and whether to act at all. Only the GitHub App key is
// confined: its payload carries a verifiable installation id bound to exactly one
// workspace, so a forged app-signed event can reach only that workspace's Apps.
// proceed=false means the delivery is validly signed but its installation maps to
// no workspace (or the lookup failed) — act on nothing rather than fall back to a
// global match (codex #7). In multitenant operation the same fail-closed rule
// covers a missing resolver or missing installation id (round-6 #9).
func (h *GitWebhook) scopeFor(ctx context.Context, key verifiedKey, installationID int64) (scope string, proceed bool) {
	if key != keyGitHubApp {
		return "", true // manual key → global (multitenant already rejects it at verify)
	}
	if h.Installations == nil || installationID == 0 {
		// Resolver unwired or the payload carries no installation id. Global
		// matching is only safe when a single workspace exists: in multitenant
		// operation the empty scope would scan and mutate every workspace's
		// Apps, so a partial configuration (GitHub webhook secret + store but
		// no GitHub service) must fail closed, not fall back to the manual
		// key's intentionally global scope (codex-security round-6 #9).
		if h.Multitenant {
			return "", false
		}
		return "", true
	}
	ws, ok, err := h.Installations.WorkspaceForInstallation(ctx, installationID)
	if err != nil || !ok || ws == "" {
		return "", false // fail closed: an unknown/unbound installation acts on nothing
	}
	return ws, true
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
	Deleted     bool   `json:"deleted"`  // true when this push deleted the branch (git push --delete)
	RefType     string `json:"ref_type"` // push payloads never carry one; its presence marks a delete/create-shaped body
	DeliveryKey string `json:"-"`
	// Installation.ID is the GitHub App installation that delivered this event —
	// bound to exactly one workspace (w1/m65 F2), which confines an app-signed
	// delivery to that workspace's Apps (codex #7). Absent (0) for a manual/Gitea
	// push webhook.
	Installation struct {
		ID int64 `json:"id"`
	} `json:"installation"`
	Repository struct {
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
	Ref     string `json:"ref"`
	RefType string `json:"ref_type"` // "branch" | "tag"; only branch deletions matter
	// Installation.ID confines an app-signed branch-delete to its workspace, the
	// same as pushEvent (codex #7).
	Installation struct {
		ID int64 `json:"id"`
	} `json:"installation"`
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
	if !validGitObjectID(ev.After) || isZeroSHA(ev.After) {
		return store.CommitInfo{}
	}
	info := store.CommitInfo{Hash: ev.After, Message: ev.HeadCommit.Message}
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
	key := h.verifyKey(r.Header.Get("X-Hub-Signature-256"), body)
	if key == keyNone {
		core.WriteErrStatus(w, http.StatusUnauthorized, "invalid or missing signature")
		return
	}
	// codex #4: in multitenant mode the shared manual secret (BEX_WEBHOOK_SECRET)
	// has no per-workspace binding — any holder can name another tenant's repo and
	// trigger deployments across all workspaces. Fail closed: only the GitHub App
	// key (confined via Installations) is accepted when Multitenant is true.
	if h.Multitenant && key == keyManual {
		core.WriteErrStatus(w, http.StatusForbidden, "manual webhook key disabled in multitenant mode; use the GitHub App webhook")
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
		h.serveDelete(w, r, body, key)
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
	// codex-security round 12, finding 10: the signature authenticates the body
	// but NOT the X-GitHub-Event header, so a captured signed delete/create/
	// ping body replayed with the header stripped (or set to "push") used to
	// reach this parser, where a delete payload unmarshals into a plausible
	// pushEvent (short ref, empty after) and could redeploy — while consuming
	// the replay claim the legitimate event would need. Bind the body to one
	// grammar: a real push (GitHub and Gitea alike) carries a full refs/…
	// ref and never a ref_type; anything else is rejected BEFORE any scope
	// resolution or claim.
	if !isPushShapedBody(&ev) {
		core.WriteErrStatus(w, http.StatusBadRequest, "payload shape does not match a push event")
		return
	}
	if !validGitObjectID(ev.After) {
		core.WriteErrStatus(w, http.StatusBadRequest, "push payload has an invalid after object id")
		return
	}
	ev.DeliveryKey = deliveryKey(r, body)
	branch := strings.TrimPrefix(ev.Ref, "refs/heads/")
	// codex #7: confine a GitHub-App-signed delivery to its installation's
	// workspace. proceed=false ⇒ signed but bound to no workspace ⇒ act on nothing.
	scope, proceed := h.scopeFor(r.Context(), key, ev.Installation.ID)
	if !proceed {
		core.WriteJSON(w, http.StatusOK, map[string]any{"redeployed": []string{}})
		return
	}
	// codex round-8 #9: claim the exact signed bytes before either mutation
	// branch. Everything below may mutate Apps (redeploy or branch-delete
	// handling); without the claim a captured delivery replays.
	w, finishClaim, ok := h.claimReplay(r.Context(), w, key, body)
	if !ok {
		return
	}
	defer finishClaim()
	// A branch-delete push (git push --delete: deleted=true, or an all-zero
	// `after`) carries no commit to build — record branch_deleted and disable
	// auto-deploy for services tracking it rather than attempting a redeploy.
	if ev.Deleted || isZeroSHA(ev.After) {
		urls := []string{ev.Repository.CloneURL, ev.Repository.SSHURL, ev.Repository.HTMLURL, ev.Repository.URL}
		h.writeBranchDeleted(r.Context(), w, urls, branch, ev.DeliveryKey, scope)
		return
	}
	redeployed, tenants, err := h.redeployMatching(r.Context(), ev, branch, scope)
	if err != nil {
		core.WriteErr(w, err)
		return
	}
	// Trigger blueprint auto-sync for workspaces whose apps share this repo.
	// Runs in a goroutine so the webhook response is not blocked.
	if h.Svc.Blueprints != nil && len(tenants) > 0 {
		repo := ev.Repository.CloneURL
		if repo == "" {
			repo = ev.Repository.HTMLURL
		}
		go func() {
			for tenantID := range tenants {
				// Preserve tenant context for background sync to maintain isolation
				bgCtx := core.WithWorkspace(context.Background(), tenantID)
				h.Svc.triggerBlueprintSync(bgCtx, tenantID, repo, branch)
			}
		}()
	}
	core.WriteJSON(w, http.StatusOK, map[string]any{"redeployed": redeployed})
}

// serveDelete handles GitHub's `delete` event — a UI/API branch (or tag)
// deletion. Only branch deletions produce a branch_deleted event; a tag deletion
// is a signed no-op.
func (h *GitWebhook) serveDelete(w http.ResponseWriter, r *http.Request, body []byte, key verifiedKey) {
	var ev deleteEvent
	if err := json.Unmarshal(body, &ev); err != nil {
		core.WriteErrStatus(w, http.StatusBadRequest, "malformed delete payload")
		return
	}
	if ev.RefType != "branch" || strings.HasPrefix(ev.Ref, "refs/") {
		// ref_type must name a branch, and GitHub's delete payload carries the
		// bare branch name — a refs/-prefixed ref is a push-shaped body replayed
		// under the delete header (round 12, finding 10), a signed no-op.
		core.WriteJSON(w, http.StatusOK, map[string]string{"ignored": "delete " + ev.RefType})
		return
	}
	// codex #7: confine an app-signed delete to its installation's workspace.
	scope, proceed := h.scopeFor(r.Context(), key, ev.Installation.ID)
	if !proceed {
		core.WriteJSON(w, http.StatusOK, map[string]any{"branchDeleted": []string{}})
		return
	}
	// codex round-8 #9: branch-delete handling mutates Apps (facts +
	// autoDeploy-off patches), so it claims the delivery like the push path.
	w, finishClaim, ok := h.claimReplay(r.Context(), w, key, body)
	if !ok {
		return
	}
	defer finishClaim()
	branch := strings.TrimPrefix(ev.Ref, "refs/heads/")
	urls := []string{ev.Repository.CloneURL, ev.Repository.SSHURL, ev.Repository.HTMLURL, ev.Repository.URL}
	h.writeBranchDeleted(r.Context(), w, urls, branch, deliveryKey(r, body), scope)
}

// writeBranchDeleted records branch_deleted for every matching service and
// writes the shared `{branchDeleted: [...]}` response — the one tail both the
// `delete` event and the push-with-deleted path funnel through.
func (h *GitWebhook) writeBranchDeleted(ctx context.Context, w http.ResponseWriter, urls []string, branch, deliveryKey, scope string) {
	deleted, err := h.recordBranchDeleted(ctx, urls, branch, deliveryKey, scope)
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

// replayDigest is a mutating delivery's durable identity: the verified key class
// plus the exact signed bytes. The delivery HEADER is deliberately excluded —
// it is unsigned, so keying on it would let a replay change headers and slip
// past (codex round-8 #9). Two installations pushing byte-identical bodies
// cannot collide in practice (the payload embeds installation.id and repo
// URLs), and the key byte keeps a manual-key and an app-key acceptance of the
// same bytes distinct.
func replayDigest(key verifiedKey, body []byte) string {
	h := sha256.New()
	_, _ = h.Write([]byte{byte(key)})
	_, _ = h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

// claimReplay durably claims the signed body before a mutation branch runs
// (codex round-8 #9). ok=false means the response is already written (the claim
// errored, or the body was already processed — a replay — answered 200 so the
// git host stops retrying); the caller returns. On ok=true the caller writes
// through the returned recorder and MUST call finish after the branch: finish
// releases the claim when the branch answered 5xx (the host will redeliver, and
// that retry must not be swallowed by a claim whose work never happened). A
// delivery that completed — even with per-app failures, which this handler
// deliberately 200-swallows — keeps its claim: that IS the processed state.
func (h *GitWebhook) claimReplay(ctx context.Context, w http.ResponseWriter, key verifiedKey, body []byte) (http.ResponseWriter, func(), bool) {
	if h.Replays == nil {
		return w, func() {}, true
	}
	digest := replayDigest(key, body)
	fresh, err := h.Replays.ClaimGitWebhookDelivery(ctx, digest)
	if err != nil {
		core.WriteErr(w, err)
		return w, func() {}, false
	}
	if !fresh {
		core.WriteJSON(w, http.StatusOK, map[string]any{"redeployed": []string{}, "replayed": true})
		return w, func() {}, false
	}
	rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	return rec, func() {
		if rec.status >= http.StatusInternalServerError {
			releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
			defer cancel()
			_ = h.Replays.ReleaseGitWebhookDelivery(releaseCtx, digest)
		}
	}, true
}

// statusRecorder observes the status a mutation branch wrote so claimReplay can
// release on hard failure. A plain success (2xx) or the handler's deliberate
// 200-with-partial-list answers all leave the claim in place.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// isZeroSHA reports whether sha is git's all-zero object id, which a push
// carries in `after` when it deletes the branch. Empty is not a delete.
func isZeroSHA(sha string) bool {
	return validGitObjectID(sha) && strings.Trim(sha, "0") == ""
}

func validGitObjectID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for _, c := range value {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	return true
}

// isPushShapedBody reports whether the decoded body carries the mutually
// exclusive invariants of a push payload: a full refs/… ref (GitHub and Gitea
// both send refs/heads/<branch>, never the bare name) and no ref_type (the
// field that marks delete/create/tag payloads). This is what prevents an
// unsigned event-header swap from reinterpreting one signed grammar as
// another (codex-security round 12, finding 10).
func isPushShapedBody(ev *pushEvent) bool {
	return ev.RefType == "" && strings.HasPrefix(ev.Ref, "refs/")
}

// recordBranchDeleted records a branch_deleted event for every App tracking the
// deleted branch on the pushed repository, and disables its auto-deploy — the
// branch it built from is gone, so Render (and bex) stop auto-deploying it. It
// reads/writes through the raw client like redeployMatching: the HMAC signature
// already authorized this call. Returns the affected service names.
func (h *GitWebhook) recordBranchDeleted(ctx context.Context, urls []string, branch, deliveryKey, scope string) ([]string, error) {
	if branch == "" {
		return []string{}, nil
	}
	var list appv1alpha1.AppList
	if err := h.Svc.Client.List(ctx, &list); err != nil {
		return nil, err
	}
	affected := []string{}
	for i := range list.Items {
		a := &list.Items[i]
		// codex #7: an app-signed delivery is confined to its installation's
		// workspace; skip Apps outside it. scope=="" (manual key) matches all.
		if scope != "" && a.Labels[core.LabelTenant] != scope {
			continue
		}
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
// signature already authorized this call. It also returns a set of tenant IDs
// whose Apps share this repo (used for blueprint auto-sync post-response).
func (h *GitWebhook) redeployMatching(ctx context.Context, ev pushEvent, branch, scope string) (redeployed []string, tenants map[string]struct{}, err error) {
	var list appv1alpha1.AppList
	if err := h.Svc.Client.List(ctx, &list); err != nil {
		return nil, nil, err
	}
	paths := ev.changedPaths()
	redeployed = []string{}
	tenants = map[string]struct{}{}
	for i := range list.Items {
		a := &list.Items[i]
		// codex #7: an app-signed delivery is confined to its installation's
		// workspace; skip Apps outside it. scope=="" (manual key) matches all.
		if scope != "" && a.Labels[core.LabelTenant] != scope {
			continue
		}
		if a.Spec.Repo == "" || !repoMatches(a.Spec.Repo, ev) {
			continue
		}
		// Collect tenant IDs for all apps sharing this repo (for blueprint auto-sync).
		if t := a.Labels[core.LabelTenant]; t != "" {
			tenants[t] = struct{}{}
		}
		// autoDeploy gates push-triggered redeploys (Render's autoDeploy: no):
		// an App that opted out is never bumped by a push, only by an explicit deploy.
		if !a.Spec.AutoDeploy {
			continue
		}
		if !branchMatches(a.Spec.Branch, branch) {
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
		// codex #6: enforce the billing lifecycle gate on push-triggered
		// redeploys — a delinquent workspace must not keep triggering builds via
		// git push. Best-effort like other per-app failures: log and skip rather
		// than 5xx (which would cause retries and generation churn).
		if err := h.Svc.RequireBillingMutation(ctx, a.Labels[core.LabelTenant]); err != nil {
			log.Printf("webhook: skip redeploy %s: billing enforced: %v", a.Name, err)
			continue
		}
		if _, err := h.Svc.redeployFetched(ctx, a, ev.commitInfo()); err != nil {
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
	return redeployed, tenants, nil
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
		specBranch = appv1alpha1.DefaultBranch
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
