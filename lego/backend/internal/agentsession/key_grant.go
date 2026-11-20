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
	"net/http"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// InternalKeyGrantPath is the gateway's "does this API key's grant still
// cover this stream" question (w9/125, docs/ADR118-security-review-scoped-api-keys.md).
// A bex API key's grant lives on its machine row, which the gateway's
// Postgres role cannot read, so a web shell, sandbox exec or file transfer,
// or agent attach a key opened is re-asked here at redemption and on every
// revalidation tick, and a native SSH key's owner once per connection. Like
// InternalTwoFactorPath it lives only on bex-api's cluster-internal :8091
// listener, authenticated by the same sandbox-exec HMAC and single-use nonce.
const InternalKeyGrantPath = "/v1/api-key-grant"

// KeyGrantRequest is the question: does Subject's grant cover Action, one of
// core.KeyActions. Subject comes from what the gateway verified (a ticket's
// signed subject, a registered SSH key's owner), never from a client.
type KeyGrantRequest struct {
	Subject string `json:"subject"`
	Action  string `json:"action"`
	Nonce   string `json:"nonce"`
}

// KeyGrantResponse is the answer, exactly what bex-api's own dispatch gate
// would decide on the key's next request (core.Base.KeyGrant): Allowed is the
// grant read uncached covering Action. Key reports whether Subject is a bound
// bex API key at all; a subject that is not one (a person) has no grant to
// narrow and is always Allowed.
type KeyGrantResponse struct {
	Subject string `json:"subject"`
	Action  string `json:"action"`
	Key     bool   `json:"key"`
	Allowed bool   `json:"allowed"`
}

// KeyGrantReader reads a key's grant uncached, and whether clientID is a
// bound key at all (it has a machine row). An error means the store could not
// answer, which fails the verb closed. *store.PGStore satisfies it.
type KeyGrantReader interface {
	BoundAPIKeyGrant(ctx context.Context, clientID string) (grant core.KeyGrant, bound bool, err error)
}

// KeyGrantHandler serves InternalKeyGrantPath. A nil Grants (no store) leaves
// the verb answering 503, so the gateway refuses every stream a key opened.
type KeyGrantHandler struct {
	Secret []byte
	Grants KeyGrantReader
	Nonce  NonceClaimer
	Now    func() time.Time
}

func (h *KeyGrantHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var answer func(context.Context, KeyGrantRequest) (KeyGrantResponse, error)
	if h.Grants != nil {
		answer = h.answer
	}
	serveSignedMint(w, r, h.Secret, h.now(), h.Nonce, func(req KeyGrantRequest) string { return req.Nonce }, answer)
}

func (h *KeyGrantHandler) answer(ctx context.Context, req KeyGrantRequest) (KeyGrantResponse, error) {
	action := core.KeyAction(req.Action)
	if req.Subject == "" || !core.KnownKeyAction(action) {
		return KeyGrantResponse{}, ErrInvalidRequest
	}
	grant, bound, err := h.Grants.BoundAPIKeyGrant(ctx, req.Subject)
	if err != nil {
		return KeyGrantResponse{}, err
	}
	return KeyGrantResponse{Subject: req.Subject, Action: req.Action, Key: bound, Allowed: !bound || grant.Allows(action)}, nil
}

// KeyGrantClient is the gateway's caller for the verb.
type KeyGrantClient struct {
	URL    string
	Secret []byte
	HTTP   *http.Client
	Now    func() time.Time
}

// Ask returns whether subject is a bound bex API key and whether its grant
// covers action. Any error means the answer could not be obtained (a
// transport failure, a non-200, a bad signature, a malformed or mismatched
// body), and the caller refuses on it.
func (c *KeyGrantClient) Ask(ctx context.Context, subject, action string) (key, allowed bool, err error) {
	req := KeyGrantRequest{Subject: subject, Action: action, Nonce: newNonce()}
	out, err := postSignedMint(ctx, c.URL, c.Secret, c.now(), c.HTTP, req, func(out KeyGrantResponse) bool {
		// Not a key implies allowed: a body saying otherwise is not an answer.
		return out.Subject == subject && out.Action == action && (out.Key || out.Allowed)
	})
	if err != nil {
		return false, false, err
	}
	return out.Key, out.Allowed, nil
}

func (h *KeyGrantHandler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

func (c *KeyGrantClient) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}
