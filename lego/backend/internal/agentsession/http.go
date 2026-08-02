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
	"fmt"
	"io"
	"net/http"
	"time"
)

const maxCredentialBody = 16 << 10

// Handler authenticates the gateway→bex-api internal mint request. It is
// mounted only on :8091 and never under the public bex-api router.
type Handler struct {
	Secret []byte
	Minter *Minter
	Now    func() time.Time
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if len(h.Secret) == 0 || h.Minter == nil {
		http.Error(w, "agent credentials unavailable", http.StatusServiceUnavailable)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxCredentialBody+1))
	if err != nil || len(body) > maxCredentialBody {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if err := Verify(h.Secret, body, r.Header.Get(TimestampHeader), r.Header.Get(SignatureHeader), h.now(), 30*time.Second); err != nil {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}
	var req MintRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	response, err := h.Minter.Mint(r.Context(), req)
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
	body, err := json.Marshal(req)
	if err != nil {
		return MintResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(body))
	if err != nil {
		return MintResponse{}, err
	}
	timestamp, signature := Sign(c.Secret, body, c.now())
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set(TimestampHeader, timestamp)
	httpReq.Header.Set(SignatureHeader, signature)
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return MintResponse{}, fmt.Errorf("agent credential broker unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
			return MintResponse{}, ErrForbidden
		}
		return MintResponse{}, fmt.Errorf("agent credential broker returned status %d", resp.StatusCode)
	}
	var out MintResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxCredentialBody)).Decode(&out); err != nil || out.Token == "" {
		return MintResponse{}, fmt.Errorf("agent credential broker returned an invalid response")
	}
	return out, nil
}

func (c *Client) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}
