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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
)

// The kind-tagged PR-side verbs (the comment verb of ADR047 D11, and the
// mark-ready and merge verbs of w10/m538) share one shape on the signed
// gateway→bex-api hop: the envelope names its verb in "kind", and the HMAC
// covers it behind that kind's MAC domain, so a signature made for one verb,
// or for a bare-body verb, never verifies as another.

// kindMACInput is the MAC input of a kind-tagged envelope.
func kindMACInput(kind string, envelope []byte) []byte {
	return append([]byte(kind+"\n"), envelope...)
}

// kindedRequest is a kind-tagged envelope: its kind, and the single-use
// nonce its embedded MintRequest carries.
type kindedRequest interface {
	verbKind() string
	verbNonce() string
}

func (m MintRequest) verbNonce() string { return m.Nonce }

// serveKindedVerb is the :8091 half every kind-tagged PR-side verb shares: a
// POST, the HMAC under the verb's MAC domain, exactly the verb's envelope (its
// kind, no unknown field, one JSON value), its nonce claimed once, then run.
// A refusal goes out in the verb's vocabulary (refuse). No secret, no run or
// no nonce store answers unavailable: none of these verbs may be replayed.
func serveKindedVerb[Req kindedRequest, Resp any](w http.ResponseWriter, r *http.Request, secret []byte, nonce NonceClaimer, now func() time.Time, kind string, refuse func(code, message string) *PullRequestError, run func(context.Context, Req) (Resp, error)) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if len(secret) == 0 || run == nil || nonce == nil {
		WritePullRequestError(w, refuse(PullRequestUnavailable, kind+" is not configured"))
		return
	}
	req, ok := readKindedRequest[Req](w, r, secret, nonce, now, kind, maxPullRequestInternalBody)
	if !ok {
		return
	}
	response, err := run(r.Context(), req)
	if err != nil {
		var typed *PullRequestError
		if !errors.As(err, &typed) {
			typed = refuse(PullRequestUpstreamError, "the request could not be completed")
		}
		WritePullRequestError(w, typed)
		return
	}
	WriteNoStoreJSON(w, http.StatusOK, response)
}

// readKindedRequest is the verification half of a kind-tagged verb: at most
// maxBody bytes, the HMAC under the kind's MAC domain within the clock skew,
// exactly the verb's envelope (its kind, no unknown field, one JSON value),
// and its nonce claimed once. On any failure it writes the response itself
// and reports false.
func readKindedRequest[Req kindedRequest](w http.ResponseWriter, r *http.Request, secret []byte, nonce NonceClaimer, now func() time.Time, kind string, maxBody int) (Req, bool) {
	var req Req
	at := time.Now()
	if now != nil {
		at = now()
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, int64(maxBody)+1))
	if err != nil || len(raw) == 0 || len(raw) > maxBody {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return req, false
	}
	if err := Verify(secret, kindMACInput(kind, raw), r.Header.Get(TimestampHeader), r.Header.Get(SignatureHeader), at, mintClockSkew); err != nil {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return req, false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil || decoder.More() || req.verbKind() != kind {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return req, false
	}
	if !claimNonce(r.Context(), nonce, req.verbNonce(), at) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return req, false
	}
	return req, true
}

// postKindedVerb is the gateway half: sign req (already tagged with kind and
// a fresh nonce) under the kind's MAC domain, POST it, and decode the answer
// into out, which valid checks, or the verb's typed refusal. Every failure is
// a *PullRequestError safe to relay to the sandbox.
func postKindedVerb(ctx context.Context, url string, secret []byte, now func() time.Time, client *http.Client, kind string, req any, refuse func(code, message string) *PullRequestError, out any, valid func() bool) *PullRequestError {
	at := time.Now()
	if now != nil {
		at = now()
	}
	resp, err := signedPostDomain(ctx, url, secret, kind, at, client, req)
	if err != nil {
		return refuse(PullRequestUnavailable, "the pull-request service is unreachable")
	}
	return readPullRequestReply(resp, refuse, out, valid)
}
