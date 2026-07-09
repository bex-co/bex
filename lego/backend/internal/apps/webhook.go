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
	"io"
	"net/http"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
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

// GitWebhook is the HMAC-verified push endpoint. Secret is the shared HMAC key;
// empty disables the endpoint (503) rather than accepting unsigned pushes.
type GitWebhook struct {
	Svc    *Service
	Secret string
}

// pushEvent is the slice of a GitHub/Gitea push payload the webhook needs: which
// ref moved and which repository it belongs to (matched against App.spec.repo).
type pushEvent struct {
	Ref        string `json:"ref"` // e.g. refs/heads/main
	Repository struct {
		CloneURL string `json:"clone_url"`
		SSHURL   string `json:"ssh_url"`
		HTMLURL  string `json:"html_url"`
		URL      string `json:"url"`
	} `json:"repository"`
}

// ServeHTTP verifies the signature, resolves the pushed repo+branch to matching
// Apps, and redeploys each. An absent/mismatched signature is 401 with no action.
func (h *GitWebhook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.Secret == "" {
		core.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "git webhook not configured (BEX_WEBHOOK_SECRET unset)"})
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBody))
	if err != nil {
		core.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "cannot read body"})
		return
	}
	if !validSignature(h.Secret, r.Header.Get("X-Hub-Signature-256"), body) {
		core.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or missing signature"})
		return
	}
	var ev pushEvent
	if err := json.Unmarshal(body, &ev); err != nil {
		core.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "malformed push payload"})
		return
	}
	branch := strings.TrimPrefix(ev.Ref, "refs/heads/")
	redeployed, err := h.redeployMatching(r.Context(), ev, branch)
	if err != nil {
		core.WriteErr(w, err)
		return
	}
	core.WriteJSON(w, http.StatusOK, map[string]any{"redeployed": redeployed})
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
	redeployed := []string{}
	for i := range list.Items {
		a := &list.Items[i]
		if a.Spec.Repo == "" || !repoMatches(a.Spec.Repo, ev) || !branchMatches(a.Spec.Branch, branch) {
			continue
		}
		if _, err := h.Svc.redeploy(ctx, a.Name); err != nil {
			return nil, err
		}
		redeployed = append(redeployed, a.Name)
	}
	return redeployed, nil
}

// repoMatches reports whether an App's spec.repo names the pushed repository,
// comparing against every URL form the payload carries (clone/ssh/html/api),
// each canonicalized so an https clone URL matches a ".git"-suffixed spec.
func repoMatches(specRepo string, ev pushEvent) bool {
	want := canonicalRepo(specRepo)
	if want == "" {
		return false
	}
	for _, u := range []string{ev.Repository.CloneURL, ev.Repository.SSHURL, ev.Repository.HTMLURL, ev.Repository.URL} {
		if u != "" && canonicalRepo(u) == want {
			return true
		}
	}
	return false
}

// canonicalRepo reduces a git URL to a comparable key: lowercased, scheme and
// any user@ stripped, trailing ".git" and slashes removed — so https, ssh, and
// scp-style forms of the same repo compare equal.
func canonicalRepo(u string) string {
	s := strings.ToLower(strings.TrimSpace(u))
	for _, scheme := range []string{"https://", "http://", "ssh://", "git://"} {
		s = strings.TrimPrefix(s, scheme)
	}
	if at := strings.Index(s, "@"); at != -1 {
		s = s[at+1:] // drop user@ (git@github.com:... / user@host/...)
	}
	s = strings.ReplaceAll(s, ":", "/") // scp-style host:owner/repo -> host/owner/repo
	return strings.TrimSuffix(strings.TrimRight(s, "/"), ".git")
}

// branchMatches reports whether an App tracking specBranch should redeploy for a
// push to branch. An empty spec.branch tracks "main" (the CR default).
func branchMatches(specBranch, branch string) bool {
	if specBranch == "" {
		specBranch = "main"
	}
	return branch == "" || specBranch == branch
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
