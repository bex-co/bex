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
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// InternalSessionWakePath is the SSH gateway's "start this closed session's
// sandbox for the key that just logged in" verb (w10/m597, ADR054 D9), as
// SSH into a sleeping Devin session wakes it. It is the gateway's first verb
// that changes something: the gateway's database role only reads
// agent_sessions, so it asks bex-api to run the same Service.Resume the
// dashboard's resume calls, as the key's owner. Like the two-factor and
// key-grant verbs it lives only on bex-api's cluster-internal :8091
// listener, under the same sandbox-exec HMAC, behind its own MAC domain
// (SessionWakeKind) and a single-use nonce.
const InternalSessionWakePath = "/v1/agent-session-wake"

// SessionWakeKind tags the envelope, and its MAC domain, as the wake's, so a
// signature made for any other gateway verb never verifies here.
const SessionWakeKind = "wake-session"

// maxSessionWakeBody bounds the signed request: a few ids and a nonce.
const maxSessionWakeBody = 4 << 10

// The refusals the verb answers with, in SessionWakeResponse.Refusal. A
// refusal is a 200, never an HTTP error, so the gateway can tell "no" from
// "no answer" (an old bex-api's 404 included), which falls back to the
// banner of w10/043.
const (
	// SessionWakeForbidden: the key's owner may not start the session: no
	// can_operate, no fresh can_create, a workspace login requirement, the
	// key gone or someone else's, or a bex API key whose grant does not
	// cover managing agent sessions.
	SessionWakeForbidden = "forbidden"
	// SessionWakeRefused: the start itself was refused, with Code and
	// Message as every other surface says it: the workspace's admission
	// (payment method, billing, the live-sandbox cap), or the session's own
	// state since the gateway read it (archived, canceled).
	SessionWakeRefused = "refused"
)

// SessionWakeRequest names what the gateway verified: the registered key's
// fingerprint and its owner, the session the login named, and the client's
// address for the audit row. bex-api re-reads the key by fingerprint and
// acts only as its owner, so the gateway can name no other subject.
type SessionWakeRequest struct {
	Kind        string `json:"kind"`
	SessionID   string `json:"sessionId"`
	Subject     string `json:"subject"`
	Fingerprint string `json:"fingerprint"`
	SourceIP    string `json:"sourceIp,omitempty"`
	Nonce       string `json:"nonce"`
}

// SessionWakeResponse is the answer: the session's phase once resume took
// it (resuming while its sandbox starts, its own phase when one is up or a
// turn runs), or the refusal.
type SessionWakeResponse struct {
	SessionID string `json:"sessionId"`
	Phase     string `json:"phase,omitempty"`
	Refusal   string `json:"refusal,omitempty"`
	Code      string `json:"code,omitempty"`
	Message   string `json:"message,omitempty"`
}

// SessionResumer starts a session's sandbox as the caller in ctx and
// returns its phase: agentsessions.Service.ResumePhase.
type SessionResumer interface {
	ResumePhase(ctx context.Context, sessionID string) (string, error)
}

// SSHKeyReader reads a registered SSH key by its fingerprint.
// *store.PGStore satisfies it.
type SSHKeyReader interface {
	SSHKeyByFingerprint(ctx context.Context, fingerprint string) (store.SSHKey, error)
}

// SessionWakeHandler serves InternalSessionWakePath. A missing secret,
// resumer, key reader, grant reader or nonce store answers 503, so the
// gateway falls back to the banner.
type SessionWakeHandler struct {
	Secret   []byte
	Sessions SessionResumer
	Keys     SSHKeyReader
	Grants   KeyGrantReader
	Nonce    NonceClaimer
	Now      func() time.Time
}

func (r SessionWakeRequest) verbKind() string  { return r.Kind }
func (r SessionWakeRequest) verbNonce() string { return r.Nonce }

func (h *SessionWakeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if len(h.Secret) == 0 || h.Sessions == nil || h.Keys == nil || h.Grants == nil || h.Nonce == nil {
		http.Error(w, "session wake unavailable", http.StatusServiceUnavailable)
		return
	}
	req, ok := readKindedRequest[SessionWakeRequest](w, r, h.Secret, h.Nonce, h.Now, SessionWakeKind, maxSessionWakeBody)
	if !ok {
		return
	}
	out, err := h.answer(r.Context(), req)
	switch {
	case errors.Is(err, ErrInvalidRequest):
		http.Error(w, "invalid request", http.StatusBadRequest)
	case err != nil:
		http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
	default:
		WriteNoStoreJSON(w, http.StatusOK, out)
	}
}

// answer runs the wake as the key's owner: the identity a native SSH login
// authorizes as (its owner, method ssh, the sign-in the key was added
// under), held to a bex API key owner's grant, through every check the
// owner's own resume makes.
func (h *SessionWakeHandler) answer(ctx context.Context, req SessionWakeRequest) (SessionWakeResponse, error) {
	if !isSessionID(req.SessionID) || req.Subject == "" || req.Fingerprint == "" {
		return SessionWakeResponse{}, ErrInvalidRequest
	}
	var origin core.RequestOrigin
	if req.SourceIP != "" {
		addr, err := netip.ParseAddr(req.SourceIP)
		if err != nil {
			return SessionWakeResponse{}, ErrInvalidRequest
		}
		origin.SourceIP = addr.String()
	}
	out := SessionWakeResponse{SessionID: req.SessionID}
	key, err := h.Keys.SSHKeyByFingerprint(ctx, req.Fingerprint)
	switch {
	case errors.Is(err, store.ErrNotFound):
		out.Refusal = SessionWakeForbidden
		return out, nil
	case err != nil:
		return SessionWakeResponse{}, fmt.Errorf("read the SSH key: %w", err)
	case key.Subject == "" || key.Subject != req.Subject:
		out.Refusal = SessionWakeForbidden
		return out, nil
	}
	// A key enrolled by a bex API key (before ADR072 F3 refused that) is held
	// to the key's grant, as its own REST resume would be (agents.manage),
	// besides the sandbox execution its login needed (w9/125).
	grant, bound, err := h.Grants.BoundAPIKeyGrant(ctx, key.Subject)
	if err != nil {
		return SessionWakeResponse{}, fmt.Errorf("read the API key grant: %w", err)
	}
	if bound && !grant.Allows(core.KeyAgentsManage, core.KeySandboxesExec) {
		out.Refusal = SessionWakeForbidden
		return out, nil
	}
	ctx = core.WithIdentity(core.WithRequestOrigin(ctx, origin), core.Identity{Subject: key.Subject, Method: core.MethodSSH, SignIn: key.CreatedSignIn})
	phase, err := h.Sessions.ResumePhase(ctx, req.SessionID)
	var coded *core.CodedError
	switch {
	case err == nil:
		out.Phase = phase
	case errors.Is(err, core.ErrForbidden):
		out.Refusal = SessionWakeForbidden
	case errors.Is(err, core.ErrUnavailable):
		return SessionWakeResponse{}, err
	case errors.As(err, &coded):
		out.Refusal, out.Code, out.Message = SessionWakeRefused, coded.Code, coded.Error()
	default:
		return SessionWakeResponse{}, err
	}
	return out, nil
}

// SessionWakeClient is the gateway's caller for the verb.
type SessionWakeClient struct {
	URL    string
	Secret []byte
	HTTP   *http.Client
	Now    func() time.Time
}

// Wake asks bex-api to start req's session as the owner of its key. Any
// error means there was no answer (a transport failure, a non-200 such as
// an older bex-api's 404, a malformed or mismatched body), and the gateway
// falls back to the banner.
func (c *SessionWakeClient) Wake(ctx context.Context, req SessionWakeRequest) (SessionWakeResponse, error) {
	now := time.Now()
	if c.Now != nil {
		now = c.Now()
	}
	req.Kind, req.Nonce = SessionWakeKind, newNonce()
	return postSignedVerb(ctx, c.URL, c.Secret, SessionWakeKind, now, c.HTTP, req, func(out SessionWakeResponse) bool {
		switch out.Refusal {
		case "":
			return out.SessionID == req.SessionID && out.Phase != ""
		case SessionWakeForbidden:
			return out.SessionID == req.SessionID
		case SessionWakeRefused:
			return out.SessionID == req.SessionID && out.Message != ""
		}
		return false
	})
}
