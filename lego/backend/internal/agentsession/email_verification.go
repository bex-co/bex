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
)

// InternalEmailVerificationPath is the gateway's "may this SSH key owner use
// bex" question (ADR075 D8 revision, w2/m168). Native public-key SSH trusts the
// stored ssh_keys.subject and never passes bex-api's auth middleware, so the
// gateway asks per connection and per channel. It lives only on bex-api's
// cluster-internal :8091 listener, authenticated by the same domain-separated
// sandbox-exec HMAC and single-use nonce as the credential mints. The gateway
// never holds a Kratos admin credential.
const InternalEmailVerificationPath = "/v1/identity-email-verification"

// EmailVerificationRequest names the subject a stored SSH key belongs to.
type EmailVerificationRequest struct {
	Subject string `json:"subject"`
	Nonce   string `json:"nonce"`
}

// EmailVerificationResponse reports whether the subject is a Kratos identity
// (Human) and, if so, whether its trait email is verified. A non-human subject
// (a machine API-key client that registered a key) is exempt, exactly as
// machine callers are exempt from the API gate.
type EmailVerificationResponse struct {
	Subject  string `json:"subject"`
	Human    bool   `json:"human"`
	Verified bool   `json:"verified"`
}

// Allowed is the gate's verdict for one answer.
func (r EmailVerificationResponse) Allowed() bool { return !r.Human || r.Verified }

// EmailVerificationResolver answers for one subject; an error (Kratos
// unreachable, bad response) fails the verb closed.
type EmailVerificationResolver func(ctx context.Context, subject string) (human, verified bool, err error)

// EmailVerificationHandler serves InternalEmailVerificationPath.
type EmailVerificationHandler struct {
	Secret  []byte
	Resolve EmailVerificationResolver
	Nonce   NonceClaimer
	Now     func() time.Time
}

func (h *EmailVerificationHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var answer func(context.Context, EmailVerificationRequest) (EmailVerificationResponse, error)
	if h.Resolve != nil {
		answer = func(ctx context.Context, req EmailVerificationRequest) (EmailVerificationResponse, error) {
			if req.Subject == "" {
				return EmailVerificationResponse{}, ErrInvalidRequest
			}
			human, verified, err := h.Resolve(ctx, req.Subject)
			if err != nil {
				return EmailVerificationResponse{}, err
			}
			return EmailVerificationResponse{Subject: req.Subject, Human: human, Verified: verified}, nil
		}
	}
	serveSignedMint(w, r, h.Secret, h.now(), h.Nonce, func(req EmailVerificationRequest) string { return req.Nonce }, answer)
}

// EmailVerificationClient is the gateway's caller for the verb.
type EmailVerificationClient struct {
	URL    string
	Secret []byte
	HTTP   *http.Client
	Now    func() time.Time
}

// Check returns nil when subject may use bex, ErrForbidden for an unverified
// human, and any other error when the answer could not be obtained — callers
// refuse on every non-nil error.
func (c *EmailVerificationClient) Check(ctx context.Context, subject string) error {
	req := EmailVerificationRequest{Subject: subject, Nonce: newNonce()}
	out, err := postSignedMint(ctx, c.URL, c.Secret, c.now(), c.HTTP, req,
		func(out EmailVerificationResponse) bool { return out.Subject == subject })
	if err != nil {
		return err
	}
	if !out.Allowed() {
		return ErrForbidden
	}
	return nil
}

func (h *EmailVerificationHandler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

func (c *EmailVerificationClient) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}
