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

// Package oauthgrant is the operator's cut-off for one human's OAuth grant
// (docs/ADR012-auth.md §8d, docs/runbooks/oauth-grant-cutoff.md, w21/090).
//
// A revocation made at Hydra alone is refused by every bex-api replica only
// once its positive introspection cache entry expires (introspectionTTL, 5 s),
// because Hydra has no revocation hook and a direct revocation never writes
// the shared oauth_revocations marker that evicts the entry at the next
// request. The two writes belong together, and this package makes them one
// operation: delete the (subject, client) consent session at Hydra, which
// revokes every access and refresh token of that grant, then advance the
// marker for the same pair, then record the action in the audit trail.
//
// It runs inside the bex-api pod (cmd/api's revoke-oauth-grant command, run by
// `kubectl exec`), where Hydra's admin API, the control-plane database and the
// ops workspace id are already in the environment, so the operator needs no
// credential beyond the cluster access they already hold.
package oauthgrant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// AuditCaller and AuditCallerMethod name the actor of that row: the operator
// holding cluster access, who has no bex identity. The row maps onto Render's
// "system" actor type.
const (
	AuditCaller       = "operator"
	AuditCallerMethod = "exec"
)

// defaultAuditWorkspace holds the row when no ops workspace is configured, the
// way the audit trail files the store-off and bootstrap path.
const defaultAuditWorkspace = "default"

// maxIDLen bounds a subject or client id the way the dashboard's revoke does.
const maxIDLen = 255

// Markers advances the shared invalidation marker for one (subject, client)
// consent chain. *store.PGStore is the production implementation.
type Markers interface {
	BumpOAuthRevocation(ctx context.Context, subject, clientID string) error
}

// Revoker revokes human OAuth grants at Hydra and on the shared marker.
type Revoker struct {
	// HydraAdminURL is Hydra's admin API (BEX_HYDRA_ADMIN_URL).
	HydraAdminURL string
	// Client calls Hydra; nil means a client with a 10 s timeout on the shared
	// Ory transport.
	Client *http.Client
	// Markers is the shared marker every bex-api replica reads on every
	// introspection cache hit.
	Markers Markers
	// Audit receives one row per revoked grant; nil records nothing.
	Audit core.AuditSink
	// AuditWorkspace is the workspace whose trail holds those rows, the pinned
	// ops workspace (BEX_OPS_WORKSPACE); empty means "default".
	AuditWorkspace string
	// Now is the clock of the audit rows; nil means time.Now.
	Now func() time.Time
}

// Grant is one OAuth client a subject has consented to, as Hydra reports it.
type Grant struct {
	ClientID  string     `json:"clientId"`
	Scopes    []string   `json:"scopes,omitempty"`
	HandledAt *time.Time `json:"handledAt,omitempty"`
}

// Request asks for the revocation of one subject's grant to ClientID, or to
// every client the subject has consented to. Without Apply nothing is written.
type Request struct {
	Subject    string
	ClientID   string
	AllClients bool
	Apply      bool
}

// Revocation.Hydra values.
const (
	hydraDeleted = "deleted"
	hydraNone    = "none"
)

// Revocation is what one applied revocation did.
type Revocation struct {
	ClientID string `json:"clientId"`
	// Hydra is "deleted" when Hydra held a consent session for the pair and
	// removed it (with every token of the grant), "none" when it held none.
	Hydra string `json:"hydra"`
	// Marker is true once the shared marker was advanced.
	Marker bool `json:"marker"`
	// Audited is true once the audit row was written.
	Audited bool `json:"audited"`
}

// Result is the plan of a dry run or the outcome of an applied run.
type Result struct {
	Subject string `json:"subject"`
	Applied bool   `json:"applied"`
	// Grants are the consent sessions Hydra holds for the subject right now.
	Grants []Grant `json:"grants"`
	// Targets are the clients the request revokes, applied or not.
	Targets []string `json:"targets"`
	// Revoked is what an applied run did, one entry per target reached.
	Revoked []Revocation `json:"revoked,omitempty"`
}

// Run lists the subject's grants, picks the targets and, with Apply, revokes
// each. On an error it still returns what it did so far, so the operator sees
// which write a half-finished revocation is missing.
func (r *Revoker) Run(ctx context.Context, req Request) (Result, error) {
	res := Result{Subject: req.Subject, Grants: []Grant{}, Targets: []string{}}
	if err := validate(req); err != nil {
		return res, err
	}
	grants, err := r.Grants(ctx, req.Subject)
	if err != nil {
		return res, err
	}
	res.Grants = grants
	if req.AllClients {
		for _, g := range grants {
			res.Targets = append(res.Targets, g.ClientID)
		}
	} else {
		res.Targets = append(res.Targets, req.ClientID)
	}
	if !req.Apply {
		return res, nil
	}
	res.Applied = true
	for _, client := range res.Targets {
		rev, err := r.revoke(ctx, req.Subject, client)
		res.Revoked = append(res.Revoked, rev)
		if err != nil {
			return res, fmt.Errorf("revoking %s: %w", client, err)
		}
	}
	return res, nil
}

func validate(req Request) error {
	switch {
	case req.Subject == "" || len(req.Subject) > maxIDLen:
		return errors.New("a subject (the Kratos identity id) is required")
	case req.AllClients == (req.ClientID != ""):
		return errors.New("name one client id, or pass --all-clients")
	case len(req.ClientID) > maxIDLen:
		return errors.New("the client id is too long")
	case req.ClientID == req.Subject:
		// An API key's token has the client as its own subject and no consent
		// session; its revocation is the key's (dashboard or REST), which also
		// drops its workspace binding.
		return errors.New("subject and client are the same id: that is a machine credential, revoke the API key instead")
	}
	return nil
}

// revoke removes one grant at Hydra, then advances its marker, then audits.
//
// The order is the point. Hydra goes first so that a replica which re-fetches
// the token after the marker moves hears "inactive". A marker written first
// would let a replica evict the entry, ask Hydra, hear "active" for the grant
// Hydra has not removed yet and cache that answer newer than the marker, where
// nothing would evict it again for the cache window. If Hydra fails, nothing
// else is written: a marker without the Hydra revocation only costs a refetch.
func (r *Revoker) revoke(ctx context.Context, subject, client string) (Revocation, error) {
	rev := Revocation{ClientID: client}
	held, err := r.hydra().deleteConsentSessions(ctx, subject, client)
	if err != nil {
		return rev, err
	}
	rev.Hydra = hydraNone
	if held {
		rev.Hydra = hydraDeleted
	}
	if r.Markers == nil {
		return rev, errors.New("no revocation marker store is configured")
	}
	if err := r.Markers.BumpOAuthRevocation(ctx, subject, client); err != nil {
		return rev, fmt.Errorf("the grant is revoked at Hydra but the shared marker was not written (replicas refuse the token within the 5 s cache window; run the command again): %w", err)
	}
	rev.Marker = true
	if r.Audit != nil {
		if err := r.Audit.Record(ctx, r.auditEvent(subject, client)); err != nil {
			return rev, fmt.Errorf("the grant is revoked everywhere but its audit row was not written: %w", err)
		}
		rev.Audited = true
	}
	return rev, nil
}

func (r *Revoker) auditEvent(subject, client string) core.AuditEvent {
	workspace := r.AuditWorkspace
	if workspace == "" {
		workspace = defaultAuditWorkspace
	}
	now := time.Now
	if r.Now != nil {
		now = r.Now
	}
	return core.AuditEvent{
		Caller:       AuditCaller,
		CallerMethod: AuditCallerMethod,
		// Render's RevokeOAuthAccessEvent, the verb a member's own revocation
		// records too (w4/m454). It names itself: an operator-run command has
		// no service method for the verb to be derived from.
		Verb:     core.AuditVerbRevokeOAuthAccess,
		Resource: core.WorkspaceObject(workspace),
		// The grant's client is the target; the subject it belongs to is the
		// target's name, an opaque Kratos identity id and no secret.
		Target:     core.OAuthGrantTarget(client),
		TargetName: subject,
		Outcome:    core.AuditAllowed,
		At:         now(),
	}
}

func (r *Revoker) hydra() hydraAdmin { return newHydraAdmin(r.HydraAdminURL, r.Client) }

// hydraAdmin is the slice of Hydra's admin API this package calls: a subject's
// consent sessions and a client's registration.
type hydraAdmin struct {
	url    string
	client *http.Client
}

// newHydraAdmin is Hydra's admin API at adminURL; a nil client means one with
// a 10 s timeout on the shared Ory transport.
func newHydraAdmin(adminURL string, client *http.Client) hydraAdmin {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second, Transport: core.OryTransport}
	}
	return hydraAdmin{url: strings.TrimSuffix(adminURL, "/"), client: client}
}

// consentSessionsURL is Hydra's admin consent-session endpoint for a subject.
func (h hydraAdmin) consentSessionsURL(query url.Values) string {
	return h.url + "/admin/oauth2/auth/sessions/consent?" + query.Encode()
}

// deleteConsentSessions revokes the (subject, client) consent sessions, which
// Hydra answers by revoking every access and refresh token issued under them.
// It reports false for a pair Hydra holds no session for (404); the marker is
// still advanced.
func (h hydraAdmin) deleteConsentSessions(ctx context.Context, subject, client string) (bool, error) {
	err := core.DoJSON(ctx, h.client, http.MethodDelete,
		h.consentSessionsURL(url.Values{"subject": {subject}, "client": {client}}), "", nil, http.StatusNoContent, nil)
	var status *core.HTTPStatusError
	if errors.As(err, &status) && status.Code == http.StatusNotFound {
		return false, nil
	}
	return err == nil, err
}

// consentSession is one consent a subject gave a client, as Hydra lists it.
// Hydra lists only the consents a human decided (or a trusted client was
// auto-granted), never one it skipped because it remembered an earlier one.
type consentSession struct {
	GrantScope     []string   `json:"grant_scope"`
	HandledAt      *time.Time `json:"handled_at"`
	ConsentRequest struct {
		Client struct {
			ClientID   string `json:"client_id"`
			ClientName string `json:"client_name"`
			// SkipConsent marks one of the platform's own clients (the bex CLI,
			// the desktop and mobile apps, the ops portals): only the operator's
			// admin API can set it, never a self-registered agent.
			SkipConsent bool `json:"skip_consent"`
		} `json:"client"`
	} `json:"consent_request"`
}

// errMoreThanOnePage refuses a consent-session listing Hydra pages.
var errMoreThanOnePage = errors.New("hydra lists more consent sessions for this subject than one page")

// consentSessions lists every consent session Hydra holds for subject. It
// refuses a listing Hydra pages, so a caller never silently reads only the
// first page.
func (h hydraAdmin) consentSessions(ctx context.Context, subject string) ([]consentSession, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		h.consentSessionsURL(url.Values{"subject": {subject}, "page_size": {"500"}}), nil)
	if err != nil {
		return nil, err
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer core.DrainClose(resp)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hydra list consent sessions returned %s", resp.Status)
	}
	if strings.Contains(resp.Header.Get("Link"), `rel="next"`) {
		return nil, errMoreThanOnePage
	}
	var sessions []consentSession
	if err := json.NewDecoder(resp.Body).Decode(&sessions); err != nil {
		return nil, fmt.Errorf("hydra list consent sessions: %w", err)
	}
	return sessions, nil
}

// clientName is a client's registered client_name; known is false when Hydra
// holds no such client.
func (h hydraAdmin) clientName(ctx context.Context, clientID string) (name string, known bool, err error) {
	var client struct {
		ClientName string `json:"client_name"`
	}
	err = core.DoJSON(ctx, h.client, http.MethodGet, h.url+"/admin/clients/"+url.PathEscape(clientID), "", nil, http.StatusOK, &client)
	var status *core.HTTPStatusError
	if errors.As(err, &status) && status.Code == http.StatusNotFound {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return client.ClientName, true, nil
}

// Grants lists the clients a subject has consented to, one entry per client
// with the union of the scopes granted and the latest grant time. It refuses a
// listing Hydra pages, so a revocation of "every client" never silently stops
// at the first page.
func (r *Revoker) Grants(ctx context.Context, subject string) ([]Grant, error) {
	sessions, err := r.hydra().consentSessions(ctx, subject)
	if errors.Is(err, errMoreThanOnePage) {
		return nil, fmt.Errorf("%w; name the client instead of --all-clients", err)
	}
	if err != nil {
		return nil, err
	}
	index := map[string]int{} // client id -> position in grants
	grants := []Grant{}
	for _, s := range sessions {
		id := s.ConsentRequest.Client.ClientID
		if id == "" {
			continue
		}
		i, seen := index[id]
		if !seen {
			i = len(grants)
			index[id] = i
			grants = append(grants, Grant{ClientID: id})
		}
		g := &grants[i]
		for _, scope := range s.GrantScope {
			if !slices.Contains(g.Scopes, scope) {
				g.Scopes = append(g.Scopes, scope)
			}
		}
		if s.HandledAt != nil && (g.HandledAt == nil || s.HandledAt.After(*g.HandledAt)) {
			g.HandledAt = s.HandledAt
		}
	}
	slices.SortFunc(grants, func(a, b Grant) int { return strings.Compare(a.ClientID, b.ClientID) })
	return grants, nil
}
