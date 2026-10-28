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

package agentsession

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sessionegress"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The egress grant (w10/m19) is the third gateway→bex-api verb beside the Git
// and model mints. The gateway egress proxy authenticates the source Pod
// itself (the same resolver and label binding as the Git/model proxies), then
// asks bex-api whether that live session's workspace runs the `full` network
// tier. The gateway's least-privilege DB role cannot read the tier, and the
// answer carries no credential, so it is safe to cache briefly.

const (
	// InternalEgressGrantPath lives only on bex-api's cluster-internal :8091
	// listener, path-domain-separated from the mints under the same HMAC.
	InternalEgressGrantPath = "/v1/agent-session-egress-grants"
	// AuditVerbGrantEgress records each grant decision (allowed or denied).
	AuditVerbGrantEgress = "agentsessions.GrantEgress"
	// AuditVerbEgressUsage records the gateway's content-free per-session
	// egress counters (connections and bytes, never hosts or URLs).
	AuditVerbEgressUsage = "agentsessions.EgressUsage"
)

// EgressProxyURL renders the proxy URL a full-tier sandbox's HTTPS_PROXY and
// HTTP_PROXY carry. The userinfo is the session claim (base64url namespace and
// session id), not a credential: every HTTP client sends it as Basic
// Proxy-Authorization, and the gateway authorizes it against the resolved
// source Pod exactly like the Git/model proxy path claims.
func EgressProxyURL(baseURL, namespace, sessionID string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || u.Scheme != "http" || u.Host == "" || namespace == "" || sessionID == "" {
		return "", ErrInvalidRequest
	}
	encode := func(value string) string { return base64.RawURLEncoding.EncodeToString([]byte(value)) }
	u.User = url.UserPassword(encode(namespace), encode(sessionID))
	u.Path, u.RawQuery, u.Fragment = "", "", ""
	return u.String(), nil
}

// ParseEgressProxyAuthorization extracts the claimed namespace and session id
// from a Proxy-Authorization header EgressProxyURL's userinfo produced. They
// are claims only; the gateway still binds them to the source Pod.
func ParseEgressProxyAuthorization(header string) (namespace, sessionID string, err error) {
	scheme, encoded, ok := strings.Cut(strings.TrimSpace(header), " ")
	if !ok || !strings.EqualFold(scheme, "Basic") {
		return "", "", ErrInvalidRequest
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return "", "", ErrInvalidRequest
	}
	user, password, ok := strings.Cut(string(raw), ":")
	if !ok {
		return "", "", ErrInvalidRequest
	}
	decode := func(value string) (string, error) {
		decoded, decodeErr := base64.RawURLEncoding.DecodeString(value)
		return string(decoded), decodeErr
	}
	if namespace, err = decode(user); err != nil || namespace == "" {
		return "", "", ErrInvalidRequest
	}
	if sessionID, err = decode(password); err != nil || sessionID == "" {
		return "", "", ErrInvalidRequest
	}
	return namespace, sessionID, nil
}

// EgressGrantResponse answers a grant. Allowed is always true on a 200; a
// non-full tier or a stale sandbox is a 403, never a 200 with false.
type EgressGrantResponse struct {
	Allowed   bool   `json:"allowed"`
	SessionID string `json:"sessionId"`
}

// EgressTierStore reads the workspace's agent environment row.
// *store.PGStore satisfies it.
type EgressTierStore interface {
	GetAgentEnvironment(ctx context.Context, workspaceID string) (store.AgentEnvironment, error)
}

// EgressGranter is bex-api's half of the grant: it re-checks the session's
// current lifecycle and sandbox binding (the same gate as the credential
// mints, so a terminal session's surviving Pod gets nothing) and the
// workspace's CURRENT tier, so switching a workspace away from `full` stops
// its live sandboxes' proxy use within the gateway's cache TTL.
type EgressGranter struct {
	Sessions     SessionStore
	Environments EgressTierStore
	Audit        core.AuditSink
	Now          func() time.Time
}

func (g *EgressGranter) Grant(ctx context.Context, req ModelMintRequest) (response EgressGrantResponse, err error) {
	defer func() {
		outcome := core.AuditAllowed
		if err != nil {
			outcome = core.AuditDenied
		}
		core.RecordAuditEvent(ctx, g.Audit, core.AuditEvent{
			Caller: req.PodName, CallerMethod: "sandbox", Verb: AuditVerbGrantEgress,
			Resource: core.WorkspaceObject(req.Workspace), Target: "agent-session:" + req.SessionID,
			TargetName: "egress-proxy", Outcome: outcome, At: g.now(),
		})
	}()
	if g.Sessions == nil || g.Environments == nil {
		return EgressGrantResponse{}, fmt.Errorf("agent egress grants unavailable")
	}
	if req.Workspace == "" || req.SessionID == "" || req.PodName == "" || req.PodUID == "" {
		return EgressGrantResponse{}, fmt.Errorf("%w: incomplete verified identity", ErrInvalidRequest)
	}
	session, err := g.Sessions.GetAgentSession(ctx, req.SessionID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return EgressGrantResponse{}, ErrForbidden
		}
		return EgressGrantResponse{}, err
	}
	if !currentSandboxCaller(session, req.Workspace, req.PodName, req.PodUID) {
		return EgressGrantResponse{}, ErrForbidden
	}
	env, err := g.Environments.GetAgentEnvironment(ctx, req.Workspace)
	if errors.Is(err, store.ErrNotFound) {
		// No row is the default `registries` tier.
		return EgressGrantResponse{}, ErrForbidden
	}
	if err != nil {
		log.Printf("agent egress grant: environment read failed (workspace=%s): %v", req.Workspace, err)
		return EgressGrantResponse{}, fmt.Errorf("agent egress grants unavailable")
	}
	if env.NetworkTier != sessionegress.NetworkTierFull {
		return EgressGrantResponse{}, ErrForbidden
	}
	return EgressGrantResponse{Allowed: true, SessionID: req.SessionID}, nil
}

func (g *EgressGranter) now() time.Time {
	if g.Now != nil {
		return g.Now().UTC()
	}
	return time.Now().UTC()
}

// EgressGrantHandler authenticates the gateway→bex-api grant request. Same
// signed envelope, nonce guard and :8091-only mount as the mints.
type EgressGrantHandler struct {
	Secret  []byte
	Granter *EgressGranter
	Nonce   NonceClaimer
	Now     func() time.Time
}

func (h *EgressGrantHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var grant func(context.Context, ModelMintRequest) (EgressGrantResponse, error)
	if h.Granter != nil {
		grant = h.Granter.Grant
	}
	now := time.Now()
	if h.Now != nil {
		now = h.Now()
	}
	serveSignedMint(w, r, h.Secret, now, h.Nonce, func(req ModelMintRequest) string { return req.Nonce }, grant)
}

// EgressGrantClient is the gateway's HMAC-authenticated caller for the grant.
type EgressGrantClient struct {
	URL    string
	Secret []byte
	HTTP   *http.Client
	Now    func() time.Time
}

func (c *EgressGrantClient) Grant(ctx context.Context, req ModelMintRequest) (EgressGrantResponse, error) {
	req.Nonce = newNonce()
	now := time.Now()
	if c.Now != nil {
		now = c.Now()
	}
	return postSignedMint(ctx, c.URL, c.Secret, now, c.HTTP, req,
		func(out EgressGrantResponse) bool { return out.Allowed && out.SessionID == req.SessionID })
}
