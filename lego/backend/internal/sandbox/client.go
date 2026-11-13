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
	"log"
	"net"
	"net/http"
	"net/url"
	"syscall"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// tenantKeyHeader is OpenSandbox's multi-tenant API-key header (OSEP-0014): the
// server forwards it to the HTTP tenant-lookup provider, which resolves it to a
// `<ws>-sandbox` namespace and scopes every lifecycle op there (ADR042 D4, m32
// t006). bex-api mints one key per workspace and sends it per request.
const tenantKeyHeader = "OPEN-SANDBOX-API-KEY"

var errOpenSandboxNotFound = errors.New("opensandbox sandbox not found")

// errOpenSandboxConflict is the server's 409 for a pause or resume that the
// sandbox's current phase refuses (already Pausing/Paused/Resuming). Idle
// suspension and wake-on-exec race each other across replicas by design, so
// callers treat it as "someone else is already moving the sandbox".
var errOpenSandboxConflict = errors.New("opensandbox lifecycle conflict")

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

// CodeSandboxRuntimeUnavailable is the 503 a sandbox verb answers while the
// OpenSandbox lifecycle server cannot be reached or is not answering (w8/248).
// The server is one replica that is recreated on a rollout, so every restart
// has a window with no server; until this code that window read as a bare
// `500 internal error` on list, get and stop in every workspace. It differs
// from core.ErrSandboxesUnavailable, which says the feature is not configured.
const CodeSandboxRuntimeUnavailable = "SANDBOX_RUNTIME_UNAVAILABLE"

// runtimeRetryAfterSeconds is the Retry-After a runtime-unavailable answer
// carries: the lifecycle server is back within seconds of its pod starting.
const runtimeRetryAfterSeconds = 5

var errRuntimeUnavailable = core.NewUnavailableError(CodeSandboxRuntimeUnavailable,
	"the sandbox runtime is not answering right now; retry in a few seconds",
	map[string]any{"retryAfterSeconds": runtimeRetryAfterSeconds})

// defaultReadRetryBackoff spaces the repeats of a read the lifecycle server
// failed to answer. A read changes nothing, so repeating it is safe, and about
// a second rides out an endpoint switch without holding the request for long.
var defaultReadRetryBackoff = []time.Duration{300 * time.Millisecond, 900 * time.Millisecond}

// readRetryBudget stops the repeats of a read whose failures are slow: a
// connection that is dropped instead of refused fails only at runtimeDialTimeout,
// and repeating that would hold a list for the sum of them.
const readRetryBudget = 3 * time.Second

// runtimeDialTimeout bounds connecting to the lifecycle server, an in-cluster
// Service that connects in milliseconds when it is there. The default dialer's
// 30s would hold a request that long when the dataplane drops the connection.
const runtimeDialTimeout = 5 * time.Second

// Client is bex-api's in-package OpenSandbox lifecycle client. It is separate
// from the operator's runtime/opensandbox client (unimportable across the
// operator→types←backend boundary) and adds per-workspace tenant-key scoping.
type Client struct {
	BaseURL string
	HTTP    *http.Client
	// readRetryBackoff is the waits between repeats of an unanswered GET; none
	// means a GET is sent once.
	readRetryBackoff []time.Duration
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
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{Timeout: runtimeDialTimeout, KeepAlive: 30 * time.Second}).DialContext
	return &Client{
		BaseURL:          baseURL,
		readRetryBackoff: defaultReadRetryBackoff,
		HTTP: &http.Client{
			Timeout:   330 * time.Second,
			Transport: transport,
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
	// EphemeralStorage caps the container's writable layer when set; without
	// it the namespace LimitRange's default (10Gi) applies.
	EphemeralStorage string `json:"ephemeral-storage,omitempty"`
}

// withoutReadRetry is c with a read sent once, for a caller that holds
// something other requests wait on while it reads.
func (c *Client) withoutReadRetry() *Client {
	once := *c
	once.readRetryBackoff = nil
	return &once
}

// do issues a JSON request to the OpenSandbox server, attaching the workspace's
// tenant key when non-empty. Returns the raw body and HTTP status.
//
// A server that cannot be reached, or a status that says it could not serve
// the request, answers errRuntimeUnavailable instead of the transport error or
// the status, which would reach the caller as a bare 500 (w8/248). A read is
// repeated first. The caller's own cancellation or deadline is returned as it
// is.
func (c *Client) do(ctx context.Context, method, path, tenantKey string, body any) ([]byte, int, error) {
	var raw []byte
	if body != nil {
		var err error
		if raw, err = json.Marshal(body); err != nil {
			return nil, 0, err
		}
	}
	var backoff []time.Duration
	if method == http.MethodGet {
		backoff = c.readRetryBackoff
	}
	start := time.Now()
	for attempt := 0; ; attempt++ {
		data, code, err := c.once(ctx, method, path, tenantKey, raw)
		if ctx.Err() != nil {
			return data, code, err
		}
		down := runtimeUnreachable(method, err) || (err == nil && runtimeNotAnswering(method, code, data))
		if !down {
			return data, code, err
		}
		if attempt >= len(backoff) || time.Since(start) > readRetryBudget {
			// The coded answer is not logged where it is written, so the cause
			// is logged here.
			if err != nil {
				log.Printf("sandbox runtime unreachable (%s %s): %v", method, path, err)
			} else {
				log.Printf("sandbox runtime not answering (%s %s): status %d: %s", method, path, code, truncate(data))
			}
			return nil, 0, errRuntimeUnavailable
		}
		select {
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		case <-time.After(backoff[attempt]):
		}
	}
}

// once is one attempt of do.
func (c *Client) once(ctx context.Context, method, path, tenantKey string, raw []byte) ([]byte, int, error) {
	var rdr io.Reader
	if raw != nil {
		rdr = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, rdr)
	if err != nil {
		return nil, 0, err
	}
	if raw != nil {
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

// runtimeUnreachable reports a transport failure that means no lifecycle
// server took the request: the connection was never made (refused, no route,
// no endpoint behind the Service, a dial that timed out). For a read it also
// covers a connection that ended before an answer, which is what a pod being
// replaced does to a request in flight; a write cut off that way may have been
// applied, so it stays the error it is.
func runtimeUnreachable(method string, err error) bool {
	if err == nil {
		return false
	}
	var op *net.OpError
	if errors.As(err, &op) && op.Op == "dial" {
		return true
	}
	return method == http.MethodGet &&
		(errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, syscall.ECONNRESET))
}

// runtimeNotAnswering reports a status that says the server could not serve
// the request. On a read that is any 5xx, where the server has nothing of the
// caller's to refuse. On a write it is 503 alone, the one status that says the
// request was not taken: after a 502 or 504 the write may have been applied,
// as after a connection that ended, so those stay the errors they are. A 5xx
// the lifecycle server sent with its own error code is the server answering
// (its pod-ready timeout is a 504), so it is never this.
func runtimeNotAnswering(method string, code int, body []byte) bool {
	if code < http.StatusInternalServerError || serverErrorCode(body) != "" {
		return false
	}
	return method == http.MethodGet || code == http.StatusServiceUnavailable
}

// serverErrorCode is the `code` of an OpenSandbox error body
// ("KUBERNETES::POD_READY_TIMEOUT"), "" for any other body.
func serverErrorCode(body []byte) string {
	var envelope struct {
		Code string `json:"code"`
	}
	if json.Unmarshal(body, &envelope) != nil {
		return ""
	}
	return envelope.Code
}

// Create starts a sandbox from a resolved template image and returns its id and
// mapped status. tenantKey scopes it to the caller's `<ws>-sandbox` namespace.
func (c *Client) Create(ctx context.Context, tenantKey, image string, entrypoint []string, limits osResourceLimits, timeout int, env, metadata map[string]string) (osSandbox, error) {
	// Overcommit: request a fraction of the limit so bursty, mostly-idle sandboxes
	// pack the node instead of each reserving its full limit at schedule time
	// (which capped a node at ~2 sandboxes and produced "Insufficient cpu/memory"
	// scheduling failures at low real load). The limit still caps a burst.
	reqCPU, reqMem := requestFor(limits.CPU, limits.Memory)
	data, code, err := c.do(ctx, http.MethodPost, "/sandboxes", tenantKey, createRequest{
		Image:            osImageSpec{URI: image},
		Entrypoint:       entrypoint,
		ResourceLimits:   limits,
		ResourceRequests: osResourceLimits{CPU: reqCPU, Memory: reqMem, EphemeralStorage: ephemeralRequestFor(limits.EphemeralStorage)},
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

// Suspend pauses a sandbox (OpenSandbox rootfs snapshot, ADR042 D5). The
// server answers 202 and moves the sandbox through Pausing to Paused.
func (c *Client) Suspend(ctx context.Context, tenantKey, id string) error {
	return c.expectOK(c.do(ctx, http.MethodPost, "/sandboxes/"+url.PathEscape(id)+"/pause", tenantKey, nil))
}

// Resume wakes a suspended sandbox: 202, then Resuming until Running.
func (c *Client) Resume(ctx context.Context, tenantKey, id string) error {
	return c.expectOK(c.do(ctx, http.MethodPost, "/sandboxes/"+url.PathEscape(id)+"/resume", tenantKey, nil))
}

// PatchMetadata merges values into a sandbox's metadata (OpenSandbox's
// RFC 7396 merge patch; absent keys are kept). It does not restart the
// sandbox.
func (c *Client) PatchMetadata(ctx context.Context, tenantKey, id string, values map[string]string) error {
	return c.expectOK(c.do(ctx, http.MethodPatch, "/sandboxes/"+url.PathEscape(id)+"/metadata", tenantKey, values))
}

// RenewExpiration moves a sandbox's absolute expiry (BatchSandbox
// spec.expireTime) to at, which must be in the future; it may be earlier
// than the current expiry. A sandbox created without a timeout has no expiry
// to move: the server answers 409 (errOpenSandboxConflict).
func (c *Client) RenewExpiration(ctx context.Context, tenantKey, id string, at time.Time) error {
	return c.expectOK(c.do(ctx, http.MethodPost, "/sandboxes/"+url.PathEscape(id)+"/renew-expiration", tenantKey,
		map[string]string{"expiresAt": at.UTC().Format(time.RFC3339)}))
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
	switch code {
	case http.StatusOK, http.StatusNoContent, http.StatusAccepted:
		return nil
	case http.StatusConflict:
		return errOpenSandboxConflict
	case http.StatusNotFound:
		return errOpenSandboxNotFound
	}
	return fmt.Errorf("opensandbox: unexpected status %d", code)
}

func truncate(b []byte) string {
	const max = 200
	if len(b) > max {
		return string(b[:max])
	}
	return string(b)
}
