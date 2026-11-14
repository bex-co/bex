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
	"net/http"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// InternalTwoFactorPath is the gateway's "may this principal act in this
// workspace, given its two-factor requirement" question (docs/ADR012-auth.md §9a). The SSH gateway has no Kratos client and its
// Postgres role cannot read the workspace setting, so native public-key SSH and
// every ticket it redeems or revalidates ask bex-api, which runs the real
// twofactor.Gate. Like InternalEmailVerificationPath it lives only on bex-api's
// cluster-internal :8091 listener, authenticated by the same sandbox-exec HMAC
// and single-use nonce, and the gateway never holds a Kratos admin credential.
const InternalTwoFactorPath = "/v1/workspace-two-factor"

// twoFactorMethods is the closed set of identities the gateway authenticates
// itself and may therefore ask about: an SSH key's owner, and the subject of a
// web-shell, agent-attach or sandbox-exec ticket. An API key ("oauth2", which
// would make the gate read its creator's factor) or a background "session" is
// never the gateway's to assert, so they are refused rather than forwarded.
var twoFactorMethods = map[string]bool{
	core.MethodSSH:         true,
	core.MethodShell:       true,
	core.MethodAgentAttach: true,
	core.MethodSandboxExec: true,
}

// TwoFactorRequest is the question: may the Kratos subject (authenticated by
// the gateway as Method) act in WorkspaceID. It carries no other identity
// field, so the gateway cannot name an API key or a client.
type TwoFactorRequest struct {
	WorkspaceID string `json:"workspaceId"`
	Subject     string `json:"subject"`
	Method      string `json:"method"`
	Nonce       string `json:"nonce"`
}

// TwoFactorResponse is the answer. Required=false means the workspace has no
// requirement: the answer holds for every member of it, so the gateway can
// cache it per workspace instead of per member. Required=true answers for the
// one subject asked about. A refusal is a 200 with Allowed=false, never an HTTP
// error, so the gateway can tell "this member has no second factor" from "the
// answer could not be had" (which it must fail closed on).
type TwoFactorResponse struct {
	WorkspaceID string `json:"workspaceId"`
	Subject     string `json:"subject"`
	Required    bool   `json:"required"`
	Allowed     bool   `json:"allowed"`
}

// TwoFactorPolicy is the decision the verb serves: *twofactor.Gate. Required
// reads the workspace's setting; Check is core.TwoFactorGate. Either returns
// an error when the answer cannot be established, which fails the verb closed.
type TwoFactorPolicy interface {
	Required(ctx context.Context, workspaceID string) (bool, error)
	core.TwoFactorGate
}

// TwoFactorHandler serves InternalTwoFactorPath.
type TwoFactorHandler struct {
	Secret []byte
	Policy TwoFactorPolicy
	Nonce  NonceClaimer
	Now    func() time.Time
}

func (h *TwoFactorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var answer func(context.Context, TwoFactorRequest) (TwoFactorResponse, error)
	if h.Policy != nil {
		answer = h.answer
	}
	serveSignedMint(w, r, h.Secret, h.now(), h.Nonce, func(req TwoFactorRequest) string { return req.Nonce }, answer)
}

func (h *TwoFactorHandler) answer(ctx context.Context, req TwoFactorRequest) (TwoFactorResponse, error) {
	if req.WorkspaceID == "" || req.Subject == "" || !twoFactorMethods[req.Method] {
		return TwoFactorResponse{}, ErrInvalidRequest
	}
	required, err := h.Policy.Required(ctx, req.WorkspaceID)
	if err != nil {
		return TwoFactorResponse{}, err
	}
	out := TwoFactorResponse{WorkspaceID: req.WorkspaceID, Subject: req.Subject, Required: required, Allowed: true}
	if !required {
		return out, nil
	}
	switch err := h.Policy.Check(ctx, req.WorkspaceID, core.Identity{Subject: req.Subject, Method: req.Method}); {
	case err == nil:
	case errors.Is(err, core.ErrForbidden):
		out.Allowed = false
	default:
		return TwoFactorResponse{}, err
	}
	return out, nil
}

// TwoFactorClient is the gateway's caller for the verb.
type TwoFactorClient struct {
	URL    string
	Secret []byte
	HTTP   *http.Client
	Now    func() time.Time
}

// Ask returns the verb's answer for subject, authenticated by the gateway as
// method. Any error means the answer could not be obtained, and callers refuse
// on it: a transport failure, a non-200, a bad signature and a malformed or
// mismatched body are all the same "cannot tell".
func (c *TwoFactorClient) Ask(ctx context.Context, workspaceID, subject, method string) (required, allowed bool, err error) {
	req := TwoFactorRequest{WorkspaceID: workspaceID, Subject: subject, Method: method, Nonce: newNonce()}
	out, err := postSignedMint(ctx, c.URL, c.Secret, c.now(), c.HTTP, req, func(out TwoFactorResponse) bool {
		// Not required implies allowed: a body saying otherwise is not an answer.
		return out.WorkspaceID == workspaceID && out.Subject == subject && (out.Required || out.Allowed)
	})
	if err != nil {
		return false, false, err
	}
	return out.Required, out.Allowed, nil
}

func (h *TwoFactorHandler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

func (c *TwoFactorClient) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}
