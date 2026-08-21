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
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const maxCredentialBody = 16 << 10

// mintClockSkew bounds how far a signed mint request's timestamp may be from the
// server clock. The nonce's durable claim is retained for twice this (the full
// accept-then-replay window around issuance) so the single-use record always
// outlives any request the signature would still accept.
const mintClockSkew = 30 * time.Second

// NonceClaimer is the durable single-use guard for internal mint requests. Its
// production wiring is *store.PGStore.ClaimShellNonce — the same authoritative
// INSERT…ON CONFLICT single-use table the gateway's ticket NonceGuard uses, so a
// replayed mint request (identical timestamp+signature+body captured on the
// internal hop) is rejected exactly once across every bex-api replica. A nil
// claimer disables the check (store-off/dev; byte-identical to before the nonce
// existed). The claim is domain-prefixed ("agent-mint:") so a mint nonce and a
// gateway ticket nonce never collide in the shared table.
type NonceClaimer interface {
	ClaimShellNonce(ctx context.Context, nonce string, expiresAt time.Time) (bool, error)
}

// newNonce mints a 256-bit random single-use token for one mint request.
func newNonce() string {
	var b [32]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// serveSignedMint is the shared HMAC-authenticated mint HTTP envelope for every
// gateway→bex-api credential verb (the Git and model flavors). It verifies the
// signed request, unmarshals Req, runs the flavor's mint, and maps the domain
// error to a status — so the skew window, body cap, and status mapping live in
// exactly one place (backend/CLAUDE.md's anti-drift rule). A nil mint (an unwired
// Minter) reports the feature unavailable.
func serveSignedMint[Req, Resp any](w http.ResponseWriter, r *http.Request, secret []byte, now time.Time, claimer NonceClaimer, nonceOf func(Req) string, mint func(context.Context, Req) (Resp, error)) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if len(secret) == 0 || mint == nil {
		http.Error(w, "agent credentials unavailable", http.StatusServiceUnavailable)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxCredentialBody+1))
	if err != nil || len(body) > maxCredentialBody {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if err := Verify(secret, body, r.Header.Get(TimestampHeader), r.Header.Get(SignatureHeader), now, mintClockSkew); err != nil {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}
	var req Req
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	// Single-use claim closes the replay window: a captured (timestamp,
	// signature, body) presented again within ±skew carries the same body-bound
	// nonce, so the second claim loses. Fail closed — an absent nonce (a
	// pre-nonce caller) or a store error is rejected, never let through, so an
	// on-path attacker cannot bypass by stripping the nonce. Only enforced when a
	// claimer is wired (store present); nil is byte-identical to before.
	if claimer != nil {
		nonce := nonceOf(req)
		if nonce == "" {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
		claimCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		claimed, claimErr := claimer.ClaimShellNonce(claimCtx, "agent-mint:"+nonce, now.Add(2*mintClockSkew))
		cancel()
		if claimErr != nil || !claimed {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
	}
	response, err := mint(r.Context(), req)
	if err != nil {
		status := http.StatusBadGateway
		switch {
		case errors.Is(err, ErrForbidden):
			status = http.StatusForbidden
		case errors.Is(err, ErrInvalidRequest):
			status = http.StatusBadRequest
		}
		http.Error(w, http.StatusText(status), status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(response)
}

// postSignedMint is the shared client half: sign the request, POST it, and
// decode a validated response. valid rejects a well-formed-but-empty response so
// a broker bug can't hand back a credential-less body.
func postSignedMint[Req, Resp any](ctx context.Context, url string, secret []byte, now time.Time, httpClient *http.Client, req Req, valid func(Resp) bool) (Resp, error) {
	var zero Resp
	body, err := json.Marshal(req)
	if err != nil {
		return zero, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return zero, err
	}
	timestamp, signature := Sign(secret, body, now)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set(TimestampHeader, timestamp)
	httpReq.Header.Set(SignatureHeader, signature)
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return zero, fmt.Errorf("agent credential broker unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
			return zero, ErrForbidden
		}
		return zero, fmt.Errorf("agent credential broker returned status %d", resp.StatusCode)
	}
	var out Resp
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxCredentialBody)).Decode(&out); err != nil || !valid(out) {
		return zero, fmt.Errorf("agent credential broker returned an invalid response")
	}
	return out, nil
}

// Handler authenticates the gateway→bex-api internal mint request. It is
// mounted only on :8091 and never under the public bex-api router.
type Handler struct {
	Secret []byte
	Minter *Minter
	// Nonce is the single-use replay guard (durable, cross-replica). Wired to the
	// control-plane store in production; nil disables the check (dev/store-off).
	Nonce NonceClaimer
	Now   func() time.Time
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var mint func(context.Context, MintRequest) (MintResponse, error)
	if h.Minter != nil {
		mint = h.Minter.Mint
	}
	serveSignedMint(w, r, h.Secret, h.now(), h.Nonce, func(req MintRequest) string { return req.Nonce }, mint)
}

func (h *Handler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

// Client is the gateway's HMAC-authenticated caller for bex-api's internal mint
// verb. It never logs or persists the returned token.
type Client struct {
	URL    string
	Secret []byte
	HTTP   *http.Client
	Now    func() time.Time
}

func (c *Client) Mint(ctx context.Context, req MintRequest) (MintResponse, error) {
	req.Nonce = newNonce()
	return postSignedMint(ctx, c.URL, c.Secret, c.now(), c.HTTP, req, func(out MintResponse) bool { return out.Token != "" })
}

func (c *Client) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}
