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
	"strconv"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/id"
)

// The spawn verbs (ADR102 D9) are how a running agent starts a child session
// and reads its own descendants. The sandbox calls its own Pod-bound gateway
// URL; the gateway authenticates the source Pod exactly like the result route
// (session label only) and forwards a signed, kind-tagged envelope that names
// the verified session and Pod. The envelope's session IS the parent: the
// sandbox's body can name only the child's own fields, and an unknown field
// (a parentSessionId, say) is refused. No bex API credential and no user token
// ever enters the sandbox.
//
//	POST /agent-children/<b64 ns>/<b64 session>              start one child
//	GET  /agent-children/<b64 ns>/<b64 session>/<child id>   read a descendant

const (
	// SpawnProxyPath prefixes the sandbox-facing routes on the gateway's agent
	// listener.
	SpawnProxyPath = "/agent-children/"
	// InternalSpawnPath lives only on bex-api's cluster-internal listener.
	InternalSpawnPath = "/v1/agent-session-children"

	// SpawnKind tags the envelope, and its MAC domain, as these verbs'.
	SpawnKind = "spawn-session"
	// SpawnOpCreate and SpawnOpRead are the envelope's two operations; the MAC
	// covers the whole envelope, so one cannot be replayed as the other.
	SpawnOpCreate = "create"
	SpawnOpRead   = "read"

	// MaxSpawnTaskBytes mirrors the create limit on agentConfig.task.
	MaxSpawnTaskBytes = 100_000
	// MaxSpawnRequestBytes bounds the sandbox's JSON request at the gateway.
	MaxSpawnRequestBytes = MaxSpawnTaskBytes + 16<<10
	// MaxSpawnKeyBytes bounds the caller's idempotency key so the derived
	// create key (spawn:<session>:<key>) stays within the create limit of 128.
	MaxSpawnKeyBytes = 64
	// maxSpawnEnvelopeBytes bounds the signed gateway to bex-api request.
	maxSpawnEnvelopeBytes = MaxSpawnRequestBytes + 4<<10
	// maxSpawnResponseBytes bounds what the gateway reads back.
	maxSpawnResponseBytes = 1 << 20

	// AuditVerbSpawn and AuditVerbReadChild record every decision on both hops.
	AuditVerbSpawn     = "agentsessions.SpawnChild"
	AuditVerbReadChild = "agentsessions.ReadChild"
)

var spawnMACDomain = []byte(SpawnKind + "\n")

func spawnMACInput(envelope []byte) []byte {
	return append(append([]byte{}, spawnMACDomain...), envelope...)
}

// Spawn error codes owned by this verb. Refusals of the create path itself
// (AGENT_SESSION_DEPTH_LIMIT, AGENT_SESSION_CHILD_LIMIT, the spend and secret
// codes, ...) pass through under their own names and messages.
const (
	SpawnInvalidRequest = "invalid_request"
	SpawnTooLarge       = "too_large"
	SpawnForbidden      = "forbidden"
	SpawnNotFound       = "not_found"
	SpawnRateLimited    = "rate_limited"
	SpawnUnavailable    = "unavailable"
)

// SpawnError is a typed refusal safe to relay to the sandbox: no upstream
// internals, no credential. Code is one of the constants above or a named
// AGENT_SESSION_* code from the create path.
type SpawnError struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

func (e *SpawnError) Error() string { return e.Code + ": " + e.Message }

// NewSpawnError builds a refusal with the code's standard status and hint. A
// named create-path code is a definite refusal: retrying the same call fails
// the same way.
func NewSpawnError(code, message string) *SpawnError {
	status, hint := http.StatusConflict, "This is a limit of your session tree, not a transient error. Do not retry the same call: change the request, wait for a child to finish, or do the work yourself."
	switch code {
	case SpawnInvalidRequest:
		status, hint = http.StatusUnprocessableEntity, "Fix the arguments named in the message and call this tool again."
	case SpawnTooLarge:
		status, hint = http.StatusRequestEntityTooLarge, "Send a smaller request."
	case SpawnForbidden:
		status, hint = http.StatusForbidden, "Only this session's sandbox may start children, while the session is running."
	case SpawnNotFound:
		status, hint = http.StatusNotFound, "Only sessions this session started, directly or through its children, can be read."
	case SpawnRateLimited:
		status, hint = http.StatusTooManyRequests, "Wait and call this tool again later; do not call it in a loop."
	case SpawnUnavailable:
		status, hint = http.StatusServiceUnavailable, "Retry once later; if it keeps failing, report the error in your reply."
	}
	return &SpawnError{Status: status, Code: code, Message: message, Hint: hint}
}

// WriteSpawnError writes a refusal as {"error":{code,message,hint}}.
func WriteSpawnError(w http.ResponseWriter, err *SpawnError) {
	WriteNoStoreJSON(w, err.Status, struct {
		Error *SpawnError `json:"error"`
	}{err})
}

// SpawnURL is the sandbox's spawn endpoint. It carries no credential.
func SpawnURL(baseURL, namespace, sessionID string) (string, error) {
	return sessionScopedURL(baseURL, SpawnProxyPath, namespace, sessionID)
}

// ParseSpawnPath extracts the claimed namespace and session and, for the read
// route, the child id. The gateway still authorizes the claim against the
// resolved source Pod.
func ParseSpawnPath(path string) (namespace, sessionID, childID string, err error) {
	namespace, sessionID, rest, hasRest, err := parseSessionScopedPath(path, SpawnProxyPath)
	if err != nil {
		return "", "", "", ErrInvalidRequest
	}
	if hasRest {
		if kind, ok := id.KindOf(rest); !ok || kind != id.AgentSession {
			return "", "", "", ErrInvalidRequest
		}
	}
	return namespace, sessionID, rest, nil
}

// SpawnContent is the sandbox's whole say in a spawn: the child's own fields.
// SecretNames distinguishes absent (inherit every parent secret) from an empty
// list (none).
type SpawnContent struct {
	Task           string    `json:"task"`
	Repo           string    `json:"repo,omitempty"`
	Branch         string    `json:"branch,omitempty"`
	Title          string    `json:"title,omitempty"`
	Tags           []string  `json:"tags,omitempty"`
	MaxSpendUSD    *float64  `json:"maxSpendUSD,omitempty"`
	SecretNames    *[]string `json:"secretNames,omitempty"`
	IdempotencyKey string    `json:"idempotencyKey,omitempty"`
}

// Validate bounds the spawn on both hops. The deeper rules (limits, spend,
// secrets, repository) are bex-api's: only it can check them.
func (c SpawnContent) Validate() *SpawnError {
	switch {
	case strings.TrimSpace(c.Task) == "":
		return NewSpawnError(SpawnInvalidRequest, `"task" is required`)
	case len(c.Task) > MaxSpawnTaskBytes:
		return NewSpawnError(SpawnTooLarge, fmt.Sprintf("the task is %d bytes; the limit is %d", len(c.Task), MaxSpawnTaskBytes))
	case len(c.IdempotencyKey) > MaxSpawnKeyBytes || !printableASCII(c.IdempotencyKey):
		return NewSpawnError(SpawnInvalidRequest, fmt.Sprintf(`"idempotencyKey" must be at most %d printable ASCII characters`, MaxSpawnKeyBytes))
	}
	return nil
}

func printableASCII(s string) bool {
	for _, r := range s {
		if r < 0x20 || r > 0x7e {
			return false
		}
	}
	return true
}

// DecodeSpawnContent reads the sandbox's request body strictly: a JSON object
// with only SpawnContent's fields. A parentSessionId or any other field is an
// error, never ignored.
func DecodeSpawnContent(raw []byte) (SpawnContent, *SpawnError) {
	var content SpawnContent
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&content); err != nil || decoder.More() {
		return SpawnContent{}, NewSpawnError(SpawnInvalidRequest, `the body must be a JSON object with only task, repo, branch, title, tags, maxSpendUSD, secretNames and idempotencyKey`)
	}
	if invalid := content.Validate(); invalid != nil {
		return SpawnContent{}, invalid
	}
	return content, nil
}

// SpawnEnvelope is the signed gateway to bex-api statement: the session and
// Pod the gateway verified (the parent), the operation, and what the sandbox
// declared for it.
type SpawnEnvelope struct {
	Kind      string        `json:"kind"`
	Op        string        `json:"op"`
	Workspace string        `json:"workspace"`
	SessionID string        `json:"sessionId"`
	PodName   string        `json:"podName"`
	PodUID    string        `json:"podUid"`
	Spawn     *SpawnContent `json:"spawn,omitempty"`
	ChildID   string        `json:"childId,omitempty"`
	Nonce     string        `json:"nonce"`
}

// valid reports a well-formed envelope: the verified identity present, and
// exactly the payload its operation takes.
func (e SpawnEnvelope) valid() bool {
	if e.Kind != SpawnKind || e.Workspace == "" || e.SessionID == "" || e.PodName == "" || e.PodUID == "" {
		return false
	}
	switch e.Op {
	case SpawnOpCreate:
		return e.Spawn != nil && e.ChildID == ""
	case SpawnOpRead:
		return e.Spawn == nil && e.ChildID != ""
	}
	return false
}

func decodeSpawnEnvelope(raw []byte) (SpawnEnvelope, bool) {
	var envelope SpawnEnvelope
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil || decoder.More() {
		return SpawnEnvelope{}, false
	}
	return envelope, envelope.valid()
}

// SpawnResponse is what the agent gets back from a spawn.
type SpawnResponse struct {
	SessionID string `json:"sessionId"`
	State     string `json:"state"`
	Created   bool   `json:"created"`
}

// ChildView is what the agent reads about a descendant.
type ChildView struct {
	SessionID       string          `json:"sessionId"`
	ParentSessionID string          `json:"parentSessionId"`
	State           string          `json:"state"`
	Title           string          `json:"title,omitempty"`
	Repo            string          `json:"repo,omitempty"`
	Branch          string          `json:"branch,omitempty"`
	Depth           int             `json:"depth"`
	Outcome         json.RawMessage `json:"outcome,omitempty"`
}

// SpawnSink creates and reads a session's descendants for a verified Pod.
// Every error it returns should be a *SpawnError; anything else is reported
// as unavailable.
type SpawnSink interface {
	SpawnSession(ctx context.Context, envelope SpawnEnvelope) (SpawnResponse, error)
	ReadChildSession(ctx context.Context, envelope SpawnEnvelope) (ChildView, error)
}

// SpawnHandler is bex-api's internal spawn verb.
type SpawnHandler struct {
	Secret []byte
	Spawns SpawnSink
	Nonce  NonceClaimer
	Now    func() time.Time
}

func (h *SpawnHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if len(h.Secret) == 0 || h.Spawns == nil || h.Nonce == nil {
		WriteSpawnError(w, NewSpawnError(SpawnUnavailable, "starting child sessions is not configured"))
		return
	}
	now := time.Now()
	if h.Now != nil {
		now = h.Now()
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxSpawnEnvelopeBytes+1))
	if err != nil || len(raw) == 0 || len(raw) > maxSpawnEnvelopeBytes {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if err := Verify(h.Secret, spawnMACInput(raw), r.Header.Get(TimestampHeader), r.Header.Get(SignatureHeader), now, mintClockSkew); err != nil {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}
	envelope, ok := decodeSpawnEnvelope(raw)
	if !ok {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if !claimNonce(r.Context(), h.Nonce, envelope.Nonce, now) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}
	var response any
	if envelope.Op == SpawnOpCreate {
		if invalid := envelope.Spawn.Validate(); invalid != nil {
			WriteSpawnError(w, invalid)
			return
		}
		response, err = h.Spawns.SpawnSession(r.Context(), envelope)
	} else {
		response, err = h.Spawns.ReadChildSession(r.Context(), envelope)
	}
	if err != nil {
		var typed *SpawnError
		if !errors.As(err, &typed) {
			typed = NewSpawnError(SpawnUnavailable, "the request could not be completed")
		}
		WriteSpawnError(w, typed)
		return
	}
	WriteNoStoreJSON(w, http.StatusOK, response)
}

// SpawnClient is the gateway's HMAC-authenticated caller for the internal
// spawn verb.
type SpawnClient struct {
	URL    string
	Secret []byte
	HTTP   *http.Client
	Now    func() time.Time
}

func (c *SpawnClient) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// Spawn forwards a Pod-verified spawn.
func (c *SpawnClient) Spawn(ctx context.Context, envelope SpawnEnvelope) (SpawnResponse, error) {
	envelope.Op = SpawnOpCreate
	var out SpawnResponse
	if err := c.call(ctx, envelope, &out); err != nil {
		return SpawnResponse{}, err
	}
	if out.SessionID == "" {
		return SpawnResponse{}, NewSpawnError(SpawnUnavailable, "the spawn service returned an invalid response")
	}
	return out, nil
}

// ReadChild forwards a Pod-verified read of one descendant.
func (c *SpawnClient) ReadChild(ctx context.Context, envelope SpawnEnvelope) (ChildView, error) {
	envelope.Op = SpawnOpRead
	var out ChildView
	if err := c.call(ctx, envelope, &out); err != nil {
		return ChildView{}, err
	}
	if out.SessionID == "" {
		return ChildView{}, NewSpawnError(SpawnUnavailable, "the spawn service returned an invalid response")
	}
	return out, nil
}

func (c *SpawnClient) call(ctx context.Context, envelope SpawnEnvelope, out any) error {
	envelope.Kind = SpawnKind
	envelope.Nonce = newNonce()
	raw, err := json.Marshal(envelope)
	if err != nil {
		return NewSpawnError(SpawnUnavailable, "the request could not be built")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(raw))
	if err != nil {
		return NewSpawnError(SpawnUnavailable, "the request could not be built")
	}
	req.Header.Set("Content-Type", "application/json")
	signHeaders(req.Header, c.Secret, spawnMACInput(raw), c.now())
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return NewSpawnError(SpawnUnavailable, "the spawn service is unreachable")
	}
	defer func() { _ = resp.Body.Close() }()
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, maxSpawnResponseBytes))
	if resp.StatusCode == http.StatusOK {
		if err := json.Unmarshal(payload, out); err != nil {
			return NewSpawnError(SpawnUnavailable, "the spawn service returned an invalid response")
		}
		return nil
	}
	var refusal struct {
		Error *SpawnError `json:"error"`
	}
	if json.Unmarshal(payload, &refusal) == nil && refusal.Error != nil && refusal.Error.Code != "" {
		// Re-derive status and hint from the code: the gateway relays only its
		// own vocabulary, with the create path's message verbatim.
		return NewSpawnError(refusal.Error.Code, refusal.Error.Message)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return NewSpawnError(SpawnForbidden, "the request was not authorized")
	}
	return NewSpawnError(SpawnUnavailable, "the spawn service returned status "+strconv.Itoa(resp.StatusCode))
}
