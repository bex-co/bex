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
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The decision verb (ADR100 D5) is how a running turn pauses for a person: the
// driver opens a decision (an approval the engine asked for, or a question the
// agent asked with ask_user) on its own Pod-bound gateway URL, then awaits the
// answer on the same URL plus the decision id. The gateway authenticates the
// source Pod like the result route and forwards under a signed, kind-tagged
// envelope; bex-api re-checks the Pod against the session's current sandbox.
// No credential ever enters the sandbox, and the answer itself is given by a
// workspace member on the public API.

const (
	// DecisionProxyPath prefixes the sandbox-facing route on the gateway's agent
	// listener: POST /agent-decisions/<b64 ns>/<b64 session> opens a decision,
	// POST …/<b64 session>/<dcs-id> awaits its answer.
	DecisionProxyPath = "/agent-decisions/"
	// InternalDecisionPath lives only on bex-api's cluster-internal listener.
	InternalDecisionPath = "/v1/agent-session-decisions"

	// MaxDecisionRequestBytes bounds the sandbox's JSON request at the gateway.
	MaxDecisionRequestBytes = 16 << 10
	// maxDecisionEnvelopeBytes bounds the signed gateway to bex-api request.
	maxDecisionEnvelopeBytes = MaxDecisionRequestBytes + 4<<10
	// MaxDecisionTitleRunes and MaxDecisionDetailBytes bound what a decision
	// shows the person answering it.
	MaxDecisionTitleRunes  = 512
	MaxDecisionDetailBytes = 8 << 10

	// DecisionKind tags the envelope, and its MAC domain, as this verb's.
	DecisionKind = "agent-decision"
	// DecisionOpOpen and DecisionOpAwait are the envelope's two operations.
	DecisionOpOpen  = "open"
	DecisionOpAwait = "await"

	// DecisionAwaitHold is how long bex-api holds one await before it answers
	// "still pending": under its 30 s write timeout, so the driver polls again.
	DecisionAwaitHold = 20 * time.Second

	// AuditVerbDecision records every decision request on the gateway hop.
	AuditVerbDecision = "agentsessions.RequestDecision"
)

// Decision kinds the sandbox may open.
const (
	DecisionApproval = store.AgentDecisionApproval
	DecisionQuestion = store.AgentDecisionQuestion
)

// Decision error codes returned to the sandbox (the result verb's vocabulary
// plus not_found for an unknown decision id).
const (
	DecisionNotFound = "not_found"
)

var decisionMACDomain = []byte(DecisionKind + "\n")

func decisionMACInput(envelope []byte) []byte {
	return append(append([]byte{}, decisionMACDomain...), envelope...)
}

// NewDecisionError builds a decision-route refusal with the code's standard
// status and a hint the driver can act on. It shares ResultError's wire shape.
func NewDecisionError(code, message string) *ResultError {
	status, hint := http.StatusServiceUnavailable, "Retry later."
	switch code {
	case ResultInvalidRequest:
		status, hint = http.StatusUnprocessableEntity, `Send {"requestId","kind","title","detail"} to open a decision, or an empty object to await one.`
	case ResultTooLarge:
		status, hint = http.StatusRequestEntityTooLarge, fmt.Sprintf("A decision request is limited to %d KiB.", MaxDecisionRequestBytes>>10)
	case ResultForbidden:
		status, hint = http.StatusForbidden, "Only this session's sandbox may ask, while its turn is running."
	case ResultRateLimited:
		status, hint = http.StatusTooManyRequests, "Wait and try again."
	case DecisionNotFound:
		status, hint = http.StatusNotFound, "Open a new decision."
	}
	return &ResultError{Status: status, Code: code, Message: message, Hint: hint}
}

// DecisionURL is the sandbox's decision endpoint. It carries no credential.
func DecisionURL(baseURL, namespace, sessionID string) (string, error) {
	return sessionScopedURL(baseURL, DecisionProxyPath, namespace, sessionID)
}

var decisionIDPattern = regexp.MustCompile(`^dcs-[0-9a-v]{20}$`)

// ParseDecisionPath extracts the claimed namespace and session, and the
// decision id an await names (empty for an open); the gateway still authorizes
// both against the resolved source Pod.
func ParseDecisionPath(path string) (namespace, sessionID, decisionID string, err error) {
	namespace, sessionID, rest, hasRest, err := parseSessionScopedPath(path, DecisionProxyPath)
	if err != nil {
		return "", "", "", ErrInvalidRequest
	}
	if hasRest && !decisionIDPattern.MatchString(rest) {
		return "", "", "", ErrInvalidRequest
	}
	return namespace, sessionID, rest, nil
}

// DecisionContent is what the sandbox asks: its own request id (a retry
// returns the decision it already opened), the kind, a one-line title, and a
// small JSON object of detail ({toolKind, command, paths, rule} for an
// approval, {question, options} for a question).
type DecisionContent struct {
	RequestID string          `json:"requestId"`
	Kind      string          `json:"kind"`
	Title     string          `json:"title"`
	Detail    json.RawMessage `json:"detail,omitempty"`
}

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,128}$`)

// Validate bounds an open on both hops.
func (c DecisionContent) Validate() *ResultError {
	invalid := func(message string) *ResultError { return NewDecisionError(ResultInvalidRequest, message) }
	switch {
	case !requestIDPattern.MatchString(c.RequestID):
		return invalid("requestId must be 1-128 of A-Z a-z 0-9 _ . : -")
	case c.Kind != DecisionApproval && c.Kind != DecisionQuestion:
		return invalid("kind must be approval or question")
	case strings.TrimSpace(c.Title) == "" || !utf8.ValidString(c.Title):
		return invalid("title is required")
	case utf8.RuneCountInString(c.Title) > MaxDecisionTitleRunes:
		return invalid(fmt.Sprintf("title must be at most %d characters", MaxDecisionTitleRunes))
	case strings.ContainsFunc(c.Title, unicode.IsControl):
		return invalid("title must be one line")
	case len(c.Detail) > MaxDecisionDetailBytes:
		return NewDecisionError(ResultTooLarge, fmt.Sprintf("detail is %d bytes", len(c.Detail)))
	}
	if len(c.Detail) > 0 {
		var object map[string]any
		if json.Unmarshal(c.Detail, &object) != nil || object == nil {
			return invalid("detail must be a JSON object")
		}
		if bytes.Contains(c.Detail, []byte(`\u0000`)) { // jsonb cannot store NUL
			return invalid("detail must not contain NUL characters")
		}
	}
	return nil
}

// DecodeDecisionContent reads an open's body strictly.
func DecodeDecisionContent(raw []byte) (DecisionContent, *ResultError) {
	var content DecisionContent
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&content); err != nil || decoder.More() {
		return DecisionContent{}, NewDecisionError(ResultInvalidRequest, "the body must be a JSON object with requestId, kind, title and detail")
	}
	if invalid := content.Validate(); invalid != nil {
		return DecisionContent{}, invalid
	}
	return content, nil
}

// DecisionEnvelope is the signed gateway to bex-api statement: the session and
// Pod the gateway verified, the operation, and what the sandbox declared.
type DecisionEnvelope struct {
	Kind       string          `json:"kind"`
	Op         string          `json:"op"`
	Workspace  string          `json:"workspace"`
	SessionID  string          `json:"sessionId"`
	PodName    string          `json:"podName"`
	PodUID     string          `json:"podUid"`
	DecisionID string          `json:"decisionId,omitempty"`
	Content    DecisionContent `json:"content"`
	Nonce      string          `json:"nonce"`
}

func decodeDecisionEnvelope(raw []byte) (DecisionEnvelope, bool) {
	var envelope DecisionEnvelope
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil || decoder.More() {
		return DecisionEnvelope{}, false
	}
	ok := envelope.Kind == DecisionKind && envelope.Workspace != "" && envelope.SessionID != "" &&
		envelope.PodName != "" && envelope.PodUID != ""
	switch envelope.Op {
	case DecisionOpOpen:
		ok = ok && envelope.DecisionID == ""
	case DecisionOpAwait:
		ok = ok && decisionIDPattern.MatchString(envelope.DecisionID)
	default:
		ok = false
	}
	return envelope, ok
}

// DecisionResponse is what the sandbox gets back: the decision's id and state
// and, once final, its answer ({allow, scope} or {text}).
type DecisionResponse struct {
	ID     string          `json:"id"`
	State  string          `json:"state"`
	Answer json.RawMessage `json:"answer,omitempty"`
}

// DecisionSink opens and awaits verified decisions. Every error it returns
// should be a *ResultError; anything else is reported as unavailable.
type DecisionSink interface {
	OpenDecision(ctx context.Context, envelope DecisionEnvelope) (DecisionResponse, error)
	AwaitDecision(ctx context.Context, envelope DecisionEnvelope) (DecisionResponse, error)
}

// DecisionHandler is bex-api's internal decision verb.
type DecisionHandler struct {
	Secret    []byte
	Decisions DecisionSink
	Nonce     NonceClaimer
	Now       func() time.Time
}

// decisionVerb is the decision verb's envelope contract on both hops.
var decisionVerb = signedVerb{maxEnvelope: maxDecisionEnvelopeBytes, mac: decisionMACInput, refuse: NewDecisionError, subject: "decision"}

func (h *DecisionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if len(h.Secret) == 0 || h.Decisions == nil || h.Nonce == nil {
		WriteResultError(w, NewDecisionError(ResultUnavailable, "decisions are not configured"))
		return
	}
	now := time.Now()
	if h.Now != nil {
		now = h.Now()
	}
	serveSignedVerb(w, r, decisionVerb, h.Secret, now, h.Nonce, decodeDecisionEnvelope,
		func(e DecisionEnvelope) string { return e.Nonce },
		func(ctx context.Context, envelope DecisionEnvelope) (any, error) {
			if envelope.Op == DecisionOpAwait {
				return h.Decisions.AwaitDecision(ctx, envelope)
			}
			if invalid := envelope.Content.Validate(); invalid != nil {
				return nil, invalid
			}
			return h.Decisions.OpenDecision(ctx, envelope)
		})
}

// DecisionClient is the gateway's HMAC-authenticated caller for the internal
// decision verb.
type DecisionClient struct {
	URL    string
	Secret []byte
	HTTP   *http.Client
	Now    func() time.Time
}

func (c *DecisionClient) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// Forward relays a Pod-verified open or await. Every failure is a
// *ResultError safe to relay to the sandbox.
func (c *DecisionClient) Forward(ctx context.Context, envelope DecisionEnvelope) (DecisionResponse, error) {
	envelope.Kind = DecisionKind
	envelope.Nonce = newNonce()
	client := c.HTTP
	if client == nil {
		// An await is held up to DecisionAwaitHold before bex-api answers.
		client = &http.Client{Timeout: DecisionAwaitHold + 15*time.Second}
	}
	return forwardSignedVerb(ctx, decisionVerb, c.URL, c.Secret, c.now(), client, envelope,
		func(out DecisionResponse) bool { return out.ID != "" && out.State != "" })
}
