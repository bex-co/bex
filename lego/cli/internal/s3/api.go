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

package s3

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/render-oss/cli/pkg/client"
)

const (
	// maxResponseBytes bounds one API answer; the largest is a key list.
	maxResponseBytes = 4 << 20
	// requestTimeout bounds one exchange: the upstream client sets none, and a
	// stalled mint must not hold `bex s3 ls` forever.
	requestTimeout = time.Minute
)

// api is bex-api as the upstream client reaches it: the same base URL, the
// same auth and control-plane headers (request editors) and the same token
// refresh, so `bex s3` authenticates exactly like every other command.
type api struct {
	server string // the API base, ending in /v1/
	http   *http.Client
	edit   []client.RequestEditorFn
}

func defaultAPI() (*api, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, err
	}
	raw, ok := c.ClientInterface.(*client.Client)
	if !ok {
		return nil, errors.New("unexpected API client")
	}
	httpDoer, ok := raw.Client.(*http.Client)
	if !ok {
		httpDoer = &http.Client{Transport: doerTransport{raw.Client}}
	}
	return &api{server: raw.Server, http: httpDoer, edit: raw.RequestEditors}, nil
}

// doerTransport adapts the upstream HttpRequestDoer to a RoundTripper.
type doerTransport struct{ doer client.HttpRequestDoer }

func (t doerTransport) RoundTrip(r *http.Request) (*http.Response, error) { return t.doer.Do(r) }

// apiError is a non-2xx answer in bex's error envelope.
type apiError struct {
	Status  int
	Code    string
	Message string
}

func (e *apiError) Error() string {
	text := e.Message
	if text == "" {
		text = http.StatusText(e.Status)
	}
	if e.Code != "" {
		return fmt.Sprintf("%d %s: %s", e.Status, e.Code, text)
	}
	return fmt.Sprintf("%d %s", e.Status, text)
}

func decodeAPIError(status int, body []byte) *apiError {
	var envelope struct {
		Message string `json:"message"`
		Error   string `json:"error"`
		Code    string `json:"code"`
	}
	e := &apiError{Status: status}
	if json.Unmarshal(body, &envelope) == nil {
		e.Code = envelope.Code
		e.Message = strings.TrimSpace(envelope.Message)
		if e.Message == "" {
			e.Message = strings.TrimSpace(envelope.Error)
		}
	}
	return e
}

// do sends one JSON exchange and returns the answer's body. path elements are
// escaped, so a caller-chosen name can never climb out of its route.
func (a *api) do(ctx context.Context, method string, payload any, path ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	escaped := make([]string, len(path))
	for i, element := range path {
		escaped[i] = url.PathEscape(element)
	}
	endpoint, err := url.JoinPath(a.server, escaped...)
	if err != nil {
		return nil, err
	}
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, edit := range a.edit {
		if err := edit(ctx, req); err != nil {
			return nil, err
		}
	}
	resp, err := a.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxResponseBytes {
		return nil, fmt.Errorf("the API answer is larger than %d MiB", maxResponseBytes>>20)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, decodeAPIError(resp.StatusCode, raw)
	}
	return raw, nil
}

// The REST routes of ADR112 D3 and D4, all under one workspace.

func keysPath(owner string, more ...string) []string {
	return append([]string{"object-storage", owner, "access-keys"}, more...)
}

func bucketPath(owner, bucket string, more ...string) []string {
	return append([]string{"object-storage", owner, "buckets", bucket}, more...)
}

// accessKey is a key as the API shows it. Secret is set in the create answer
// alone.
type accessKey struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Permission string  `json:"permission"`
	CreatedBy  string  `json:"createdBy"`
	CreatedAt  string  `json:"createdAt"`
	ExpiresAt  *string `json:"expiresAt"`
	LastUsedAt *string `json:"lastUsedAt"`
	Status     string  `json:"status"`
	Secret     string  `json:"secret"`
}

type createKeyRequest struct {
	Name             string `json:"name"`
	Permission       string `json:"permission"`
	ExpiresInSeconds int64  `json:"expiresInSeconds,omitempty"`
}

// createKey mints a key and returns it with the API's raw answer.
func (a *api) createKey(ctx context.Context, owner string, req createKeyRequest) (accessKey, []byte, error) {
	raw, err := a.do(ctx, http.MethodPost, req, keysPath(owner)...)
	if err != nil {
		return accessKey{}, nil, err
	}
	var key accessKey
	if err := json.Unmarshal(raw, &key); err != nil {
		return accessKey{}, nil, fmt.Errorf("decode the new access key: %w", err)
	}
	return key, raw, nil
}

type bucketInfo struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Region     string  `json:"region"`
	System     bool    `json:"system"`
	Public     bool    `json:"public"`
	PublicHost string  `json:"publicHost"`
	Objects    int64   `json:"objects"`
	Bytes      int64   `json:"bytes"`
	ObservedAt *string `json:"observedAt"`
	CreatedAt  string  `json:"createdAt"`
}

type bucketPublic struct {
	Public bool   `json:"public"`
	Host   string `json:"host"`
}
