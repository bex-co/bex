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
	"fmt"
	"io"
	"net/http"
	"time"
)

// tenantKeyHeader is OpenSandbox's multi-tenant API-key header (OSEP-0014): the
// server forwards it to the HTTP tenant-lookup provider, which resolves it to a
// `<ws>-sandbox` namespace and scopes every lifecycle op there (ADR042 D4, m32
// t006). bex-api mints one key per workspace and sends it per request.
const tenantKeyHeader = "OPEN-SANDBOX-API-KEY"

// Client is bex-api's in-package OpenSandbox lifecycle client. It is separate
// from the operator's runtime/opensandbox client (unimportable across the
// operator→types←backend boundary) and adds per-workspace tenant-key scoping.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// NewClient builds a Client against an OpenSandbox server base URL.
func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL: baseURL,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
}

// osSandbox is the OpenSandbox server's sandbox representation (a subset).
type osSandbox struct {
	ID     string `json:"id"`
	Status struct {
		Phase string `json:"phase"`
	} `json:"status"`
}

// createRequest is the OpenSandbox create body. bex resolves image+port from a
// template (never an arbitrary caller image), so this is server-facing only.
type createRequest struct {
	Image string            `json:"image"`
	Port  int               `json:"port,omitempty"`
	Env   map[string]string `json:"env,omitempty"`
	Name  string            `json:"name,omitempty"`
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
func (c *Client) Create(ctx context.Context, tenantKey, image string, port int, env map[string]string, name string) (string, Status, error) {
	data, code, err := c.do(ctx, http.MethodPost, "/sandboxes", tenantKey, createRequest{Image: image, Port: port, Env: env, Name: name})
	if err != nil {
		return "", "", err
	}
	if code != http.StatusOK && code != http.StatusCreated {
		return "", "", fmt.Errorf("opensandbox create: status %d: %s", code, truncate(data))
	}
	var s osSandbox
	if err := json.Unmarshal(data, &s); err != nil || s.ID == "" {
		return "", "", fmt.Errorf("opensandbox create: bad response: %s", truncate(data))
	}
	return s.ID, mapOpenSandboxStatus(s.Status.Phase), nil
}

// Get returns a sandbox's mapped status.
func (c *Client) Get(ctx context.Context, tenantKey, id string) (Status, error) {
	data, code, err := c.do(ctx, http.MethodGet, "/sandboxes/"+id, tenantKey, nil)
	if err != nil {
		return "", err
	}
	if code == http.StatusNotFound {
		return StatusTerminated, nil
	}
	if code != http.StatusOK {
		return "", fmt.Errorf("opensandbox get: status %d", code)
	}
	var s osSandbox
	if err := json.Unmarshal(data, &s); err != nil {
		return "", fmt.Errorf("opensandbox get: bad response")
	}
	return mapOpenSandboxStatus(s.Status.Phase), nil
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
	return c.expectOK(c.do(ctx, http.MethodPost, "/sandboxes/"+id+"/pause", tenantKey, nil))
}

// Resume wakes a suspended sandbox.
func (c *Client) Resume(ctx context.Context, tenantKey, id string) error {
	return c.expectOK(c.do(ctx, http.MethodPost, "/sandboxes/"+id+"/resume", tenantKey, nil))
}

// Terminate deletes a sandbox.
func (c *Client) Terminate(ctx context.Context, tenantKey, id string) error {
	_, code, err := c.do(ctx, http.MethodDelete, "/sandboxes/"+id, tenantKey, nil)
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
