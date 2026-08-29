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

package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// tenantKeyHeader is OpenSandbox's multi-tenant API-key header (OSEP-0014): the
// server forwards it to the HTTP tenant-lookup provider, which resolves it to a
// `<ws>-sandbox` namespace and scopes every lifecycle op there (ADR042 D4, m32
// t006). bex-api mints one key per workspace and sends it per request.
const tenantKeyHeader = "OPEN-SANDBOX-API-KEY"

var errOpenSandboxNotFound = errors.New("opensandbox sandbox not found")

// errPodReadyTimeout marks the one create failure whose cause bex can honestly
// name: the OpenSandbox server answered with its pod-ready-timeout error code,
// meaning the sandbox pod never became Running within the server's synchronous
// create wait (sandbox_create_timeout_seconds, deploy/opensandbox/
// sandbox-cluster.toml). The dominant live cause is the workspace's
// `<ws>-sandbox` ResourceQuota refusing another pod (.pm/w3/011.md); the
// service maps it to a typed capacity refusal instead of a generic 500.
var errPodReadyTimeout = errors.New("opensandbox pod-ready timeout")

// podReadyTimeoutCode is the OpenSandbox server's own error code for an
// elapsed pod-ready wait, matched as a body substring. The server is an
// external component, so this signature is the only signal bex-api gets.
const podReadyTimeoutCode = "KUBERNETES::POD_READY_TIMEOUT"

// Client is bex-api's in-package OpenSandbox lifecycle client. It is separate
// from the operator's runtime/opensandbox client (unimportable across the
// operator→types←backend boundary) and adds per-workspace tenant-key scoping.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// NewClient builds a Client against an OpenSandbox server base URL.
//
// The timeout must exceed the server's own pod-ready wait
// (informer_watch_timeout_seconds in sandbox-cluster.toml): OpenSandbox's
// create is SYNCHRONOUS — it holds the HTTP request open until the sandbox pod
// is Running with an IP, which on a cold agent-image pull (a fresh digest after
// every deploy, ~minutes for the large image) legitimately takes longer than a
// short client budget. A 30s timeout here gave up before the server responded,
// surfacing as "sandbox create failed: context deadline exceeded" and a failed
// session even though the pod was coming up fine. 330s comfortably covers the
// 300s pod-ready wait; list/stop/status still return in milliseconds, so this is
// only a higher ceiling for the one synchronous create path.
func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL: baseURL,
		HTTP: &http.Client{
			Timeout: 330 * time.Second,
			// The workspace tenant key is a capability for one OpenSandbox
			// namespace. Never forward it through an upstream-controlled redirect.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// osSandbox is the OpenSandbox server's sandbox representation (a subset).
// The server's status carries `state` (Running/Suspended/…), validated on prod.
type osSandbox struct {
	ID       string            `json:"id"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Created  *time.Time        `json:"createdAt,omitempty"`
	Image    osImageSpec       `json:"image,omitempty"`
	Status   struct {
		State string `json:"state"`
	} `json:"status"`
}

// createRequest is the OpenSandbox server's create body (validated against the
// live API, 2026-07-28): image is an object with a `uri`, `entrypoint` is
// required when an image is given, and `resourceLimits` is required when no
// pool is referenced. bex resolves all of these from a template (never an
// arbitrary caller image), so this is server-facing only.
type createRequest struct {
	Image            osImageSpec       `json:"image"`
	Entrypoint       []string          `json:"entrypoint"`
	ResourceLimits   osResourceLimits  `json:"resourceLimits"`
	ResourceRequests osResourceLimits  `json:"resourceRequests,omitempty"`
	Env              map[string]string `json:"env,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	Timeout          int               `json:"timeout,omitempty"`
}

type osImageSpec struct {
	URI string `json:"uri"`
}

type osResourceLimits struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
}

// do issues a JSON request to the OpenSandbox server, attaching the workspace's
// tenant key when non-empty. Returns the raw body and HTTP status.
func (c *Client) do(ctx context.Context, method, path, tenantKey string, body any) ([]byte, int, error) {
	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		rdr = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, rdr)
	if err != nil {
		return nil, 0, err
	}
	if body != nil {
		req.Header.Set("content-type", "application/json")
	}
	if tenantKey != "" {
		req.Header.Set(tenantKeyHeader, tenantKey)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return data, resp.StatusCode, nil
}

// Create starts a sandbox from a resolved template image and returns its id and
// mapped status. tenantKey scopes it to the caller's `<ws>-sandbox` namespace.
func (c *Client) Create(ctx context.Context, tenantKey, image string, entrypoint []string, cpu, memory string, timeout int, env, metadata map[string]string) (osSandbox, error) {
	// Overcommit: request a fraction of the limit so bursty, mostly-idle sandboxes
	// pack the node instead of each reserving its full limit at schedule time
	// (which capped a node at ~2 sandboxes and produced "Insufficient cpu/memory"
	// scheduling failures at low real load). The limit still caps a burst.
	reqCPU, reqMem := requestFor(cpu, memory)
	data, code, err := c.do(ctx, http.MethodPost, "/sandboxes", tenantKey, createRequest{
		Image:            osImageSpec{URI: image},
		Entrypoint:       entrypoint,
		ResourceLimits:   osResourceLimits{CPU: cpu, Memory: memory},
		ResourceRequests: osResourceLimits{CPU: reqCPU, Memory: reqMem},
		Env:              env,
		Metadata:         metadata,
		Timeout:          timeout,
	})
	if err != nil {
		return osSandbox{}, err
	}
	if code != http.StatusOK && code != http.StatusCreated && code != http.StatusAccepted {
		if bytes.Contains(data, []byte(podReadyTimeoutCode)) {
			return osSandbox{}, fmt.Errorf("opensandbox create: status %d: %w", code, errPodReadyTimeout)
		}
		return osSandbox{}, fmt.Errorf("opensandbox create: status %d: %s", code, truncate(data))
	}
	var s osSandbox
	if err := json.Unmarshal(data, &s); err != nil || s.ID == "" {
		return osSandbox{}, fmt.Errorf("opensandbox create: bad response: %s", truncate(data))
	}
	return s, nil
}

// Get returns a sandbox's durable OpenSandbox representation. A missing object
// remains distinguishable so the service can map it to one non-enumerating 404.
func (c *Client) Get(ctx context.Context, tenantKey, id string) (osSandbox, error) {
	data, code, err := c.do(ctx, http.MethodGet, "/sandboxes/"+url.PathEscape(id), tenantKey, nil)
	if err != nil {
		return osSandbox{}, err
	}
	if code == http.StatusNotFound {
		return osSandbox{}, errOpenSandboxNotFound
	}
	if code != http.StatusOK {
		return osSandbox{}, fmt.Errorf("opensandbox get: status %d", code)
	}
	var s osSandbox
	if err := json.Unmarshal(data, &s); err != nil || s.ID == "" {
		return osSandbox{}, fmt.Errorf("opensandbox get: bad response")
	}
	return s, nil
}

// List returns every sandbox visible to the tenant key (the server scopes to
// the resolved namespace), each with its mapped status.
func (c *Client) List(ctx context.Context, tenantKey string) ([]osSandbox, error) {
	data, code, err := c.do(ctx, http.MethodGet, "/sandboxes", tenantKey, nil)
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		return nil, fmt.Errorf("opensandbox list: status %d", code)
	}
	// The server may return either a bare array or {items:[...]}.
	var arr []osSandbox
	if err := json.Unmarshal(data, &arr); err == nil {
		return arr, nil
	}
	var wrap struct {
		Items []osSandbox `json:"items"`
	}
	if err := json.Unmarshal(data, &wrap); err != nil {
		return nil, fmt.Errorf("opensandbox list: bad response")
	}
	return wrap.Items, nil
}

// Suspend pauses a sandbox (OpenSandbox rootfs snapshot, ADR042 D5).
func (c *Client) Suspend(ctx context.Context, tenantKey, id string) error {
	return c.expectOK(c.do(ctx, http.MethodPost, "/sandboxes/"+url.PathEscape(id)+"/pause", tenantKey, nil))
}

// Resume wakes a suspended sandbox.
func (c *Client) Resume(ctx context.Context, tenantKey, id string) error {
	return c.expectOK(c.do(ctx, http.MethodPost, "/sandboxes/"+url.PathEscape(id)+"/resume", tenantKey, nil))
}

// Terminate deletes a sandbox.
func (c *Client) Terminate(ctx context.Context, tenantKey, id string) error {
	_, code, err := c.do(ctx, http.MethodDelete, "/sandboxes/"+url.PathEscape(id), tenantKey, nil)
	if err != nil {
		return err
	}
	if code != http.StatusOK && code != http.StatusNoContent && code != http.StatusNotFound {
		return fmt.Errorf("opensandbox terminate: status %d", code)
	}
	return nil
}

func (c *Client) expectOK(_ []byte, code int, err error) error {
	if err != nil {
		return err
	}
	if code != http.StatusOK && code != http.StatusNoContent && code != http.StatusAccepted {
		return fmt.Errorf("opensandbox: unexpected status %d", code)
	}
	return nil
}

func truncate(b []byte) string {
	const max = 200
	if len(b) > max {
		return string(b[:max])
	}
	return string(b)
}
