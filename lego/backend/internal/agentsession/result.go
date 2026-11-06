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
	"fmt"
	"net/http"
	"time"
)

// The result verb (ADR094 D5) is how the agent hands back the structured
// answer its turn's outputSchema asks for. The sandbox POSTs {result, final} to
// its own Pod-bound gateway URL; the gateway authenticates the source Pod
// exactly like the share route (session label only, so repo-less sessions can
// submit too) and forwards the result under a signed, kind-tagged envelope that
// names the verified session. bex-api re-checks the Pod against the session's
// current sandbox and validates the result against the turn's stored schema
// before it keeps it. No credential ever enters the sandbox.

const (
	// ResultProxyPath prefixes the sandbox-facing route on the gateway's agent
	// listener: POST /agent-results/<b64 ns>/<b64 session>.
	ResultProxyPath = "/agent-results/"
	// InternalResultPath lives only on bex-api's cluster-internal listener.
	InternalResultPath = "/v1/agent-session-results"

	// MaxResultBytes caps one submitted result (ADR094 D5).
	MaxResultBytes = 256 << 10
	// MaxResultRequestBytes bounds the sandbox's JSON request at the gateway:
	// the result plus the {"result":…,"final":…} wrapper.
	MaxResultRequestBytes = MaxResultBytes + 1<<10
	// maxResultEnvelopeBytes bounds the signed gateway to bex-api request.
	maxResultEnvelopeBytes = MaxResultBytes + 4<<10

	// ResultKind tags the envelope, and its MAC domain, as this verb's.
	ResultKind = "submit-result"

	// AuditVerbResult records every result decision on both hops.
	AuditVerbResult = "agentsessions.SubmitResult"
)

// resultMACDomain separates this verb's signatures from every other verb's,
// whose MAC covers the bare body or another domain.
var resultMACDomain = []byte(ResultKind + "\n")

func resultMACInput(envelope []byte) []byte {
	return append(append([]byte{}, resultMACDomain...), envelope...)
}

// Result error codes returned to the sandbox, reported to the agent verbatim
// by the in-sandbox tool.
const (
	ResultInvalidRequest = "invalid_request"
	ResultTooLarge       = "too_large"
	ResultNotExpected    = "not_expected"
	ResultSchemaMismatch = "schema_mismatch"
	ResultForbidden      = "forbidden"
	ResultRateLimited    = "rate_limited"
	ResultUnavailable    = "unavailable"
)

// ResultError is a typed refusal of the result verb, safe to relay to the
// sandbox: no upstream text, no credential.
type ResultError struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

func (e *ResultError) Error() string { return e.Code + ": " + e.Message }

// NewResultError builds a refusal with the code's standard status and hint.
func NewResultError(code, message string) *ResultError {
	status, hint := http.StatusServiceUnavailable, "Retry once later; if it keeps failing, put the answer in your final reply instead."
	switch code {
	case ResultInvalidRequest:
		status, hint = http.StatusUnprocessableEntity, `Send a JSON object with "result" (any JSON value) and "final" (a boolean).`
	case ResultTooLarge:
		status, hint = http.StatusRequestEntityTooLarge, fmt.Sprintf("A result is limited to %d KiB; submit a smaller one.", MaxResultBytes>>10)
	case ResultNotExpected:
		status, hint = http.StatusConflict, "This turn did not ask for a structured result, so there is nothing to submit; put your answer in your reply."
	case ResultSchemaMismatch:
		status, hint = http.StatusUnprocessableEntity, "Fix the result so it matches the output schema, then call this tool again."
	case ResultForbidden:
		status, hint = http.StatusForbidden, "Only this session's sandbox may submit a result, while the session is running."
	case ResultRateLimited:
		status, hint = http.StatusTooManyRequests, "Wait and call this tool again later; do not call it in a loop."
	}
	return &ResultError{Status: status, Code: code, Message: message, Hint: hint}
}

// WriteResultError writes a refusal as {"error":{code,message,hint}}.
func WriteResultError(w http.ResponseWriter, err *ResultError) {
	WriteNoStoreJSON(w, err.Status, struct {
		Error *ResultError `json:"error"`
	}{err})
}

// ResultURL is the sandbox's result endpoint. It carries no credential.
func ResultURL(baseURL, namespace, sessionID string) (string, error) {
	return sessionScopedURL(baseURL, ResultProxyPath, namespace, sessionID)
}

// ParseResultPath extracts the claimed namespace and session; the gateway
// still authorizes both against the resolved source Pod.
func ParseResultPath(path string) (namespace, sessionID string, err error) {
	namespace, sessionID, _, hasRest, err := parseSessionScopedPath(path, ResultProxyPath)
	if err != nil || hasRest {
		return "", "", ErrInvalidRequest
	}
	return namespace, sessionID, nil
}

// ResultContent is the sandbox's whole say in a submission: a JSON value and
// whether it is the final one.
type ResultContent struct {
	Result json.RawMessage `json:"result"`
	Final  bool            `json:"final"`
}

// Validate bounds the submission on both hops. Whether the result matches the
// turn's schema is bex-api's call: only it holds the stored schema.
func (c ResultContent) Validate() *ResultError {
	switch {
	case len(c.Result) == 0:
		return NewResultError(ResultInvalidRequest, `"result" is required`)
	case len(c.Result) > MaxResultBytes:
		return NewResultError(ResultTooLarge, fmt.Sprintf("the result is %d bytes", len(c.Result)))
	case !json.Valid(c.Result):
		return NewResultError(ResultInvalidRequest, "the result is not valid JSON")
	case bytes.Contains(c.Result, []byte(`\u0000`)): // jsonb cannot store NUL
		return NewResultError(ResultInvalidRequest, "the result must not contain NUL characters")
	}
	return nil
}

// ResultEnvelope is the signed gateway to bex-api statement: the session and
// Pod the gateway verified, and the submission the sandbox declared.
type ResultEnvelope struct {
	Kind      string          `json:"kind"`
	Workspace string          `json:"workspace"`
	SessionID string          `json:"sessionId"`
	PodName   string          `json:"podName"`
	PodUID    string          `json:"podUid"`
	Result    json.RawMessage `json:"result"`
	Final     bool            `json:"final"`
	Nonce     string          `json:"nonce"`
}

// decodeResultEnvelope reads exactly a result envelope: its kind, no unknown
// field, and the gateway-verified session and Pod present.
func decodeResultEnvelope(raw []byte) (ResultEnvelope, bool) {
	var envelope ResultEnvelope
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil || decoder.More() {
		return ResultEnvelope{}, false
	}
	ok := envelope.Kind == ResultKind && envelope.Workspace != "" && envelope.SessionID != "" &&
		envelope.PodName != "" && envelope.PodUID != ""
	return envelope, ok
}

// DecodeResultContent reads the sandbox's request body strictly: a JSON object
// with only "result" and "final".
func DecodeResultContent(raw []byte) (ResultContent, *ResultError) {
	var content ResultContent
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&content); err != nil || decoder.More() {
		return ResultContent{}, NewResultError(ResultInvalidRequest, `the body must be a JSON object with only "result" and "final"`)
	}
	if invalid := content.Validate(); invalid != nil {
		return ResultContent{}, invalid
	}
	return content, nil
}

// ResultResponse is what the agent gets back.
type ResultResponse struct {
	Accepted bool `json:"accepted"`
	Final    bool `json:"final"`
}

// ResultSink stores a verified submission. Every error it returns should be a
// *ResultError; anything else is reported as unavailable.
type ResultSink interface {
	SubmitResult(ctx context.Context, envelope ResultEnvelope) (ResultResponse, error)
}

// ResultHandler is bex-api's internal result verb.
type ResultHandler struct {
	Secret  []byte
	Results ResultSink
	Nonce   NonceClaimer
	Now     func() time.Time
}

// resultVerb is the result verb's envelope contract on both hops.
var resultVerb = signedVerb{maxEnvelope: maxResultEnvelopeBytes, mac: resultMACInput, refuse: NewResultError, subject: "result"}

func (h *ResultHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// The nonce store is not optional: without it a captured envelope could be
	// replayed within the clock skew.
	if len(h.Secret) == 0 || h.Results == nil || h.Nonce == nil {
		WriteResultError(w, NewResultError(ResultUnavailable, "structured results are not configured"))
		return
	}
	now := time.Now()
	if h.Now != nil {
		now = h.Now()
	}
	serveSignedVerb(w, r, resultVerb, h.Secret, now, h.Nonce, decodeResultEnvelope,
		func(e ResultEnvelope) string { return e.Nonce },
		func(ctx context.Context, envelope ResultEnvelope) (any, error) {
			if invalid := (ResultContent{Result: envelope.Result, Final: envelope.Final}).Validate(); invalid != nil {
				return nil, invalid
			}
			return h.Results.SubmitResult(ctx, envelope)
		})
}

// ResultClient is the gateway's HMAC-authenticated caller for the internal
// result verb.
type ResultClient struct {
	URL    string
	Secret []byte
	HTTP   *http.Client
	Now    func() time.Time
}

func (c *ResultClient) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// Submit forwards a Pod-verified submission. Every failure is a *ResultError
// safe to relay to the sandbox.
func (c *ResultClient) Submit(ctx context.Context, envelope ResultEnvelope) (ResultResponse, error) {
	envelope.Kind = ResultKind
	envelope.Nonce = newNonce()
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return forwardSignedVerb(ctx, resultVerb, c.URL, c.Secret, c.now(), client, envelope,
		func(out ResultResponse) bool { return out.Accepted })
}
