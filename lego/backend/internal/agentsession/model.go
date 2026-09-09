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
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sessionegress"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The model-credential broker is the ADR062 counterpart to the Git smart-HTTP
// proxy above: it keeps a workspace's reusable BYO model provider key (ADR047
// D7) out of the untrusted sandbox process tree. The sandbox holds only a
// placeholder; the isolated gateway's model proxy fetches the real key from
// bex-api (which reads OpenBao), injects it on the upstream hop, and forwards to
// the session's registered vendor endpoint. Same one-trusted-hop shape as the
// Git proxy, same shared HMAC transport (Sign/Verify), so a sandbox never sees a
// durable credential.

const (
	// InternalModelMintPath lives only on bex-api's cluster-internal :8091
	// listener, alongside InternalMintPath. Domain-separated by path; both hops
	// are authenticated by the same gateway-only HMAC secret.
	InternalModelMintPath = "/v1/agent-session-model-credentials"
	// InternalModelAuthFailurePath is the sibling verb the gateway model proxy
	// calls when the vendor rejects the injected BYO key (HTTP 401/403 on the
	// gateway→vendor hop). Same :8091 listener, same gateway-only HMAC, path-
	// domain-separated; it terminalizes the session fast so a bad/expired key does
	// not ride the agent CLI's full retry/backoff (~3min observed) — w5/m80 t003.
	InternalModelAuthFailurePath = InternalModelMintPath + "/auth-failure"
	// ModelProxyPath is the gateway model proxy's listen path prefix. The agent's
	// base URL is ModelProxyURL(...) and every model API call lands under it; the
	// gateway strips the prefix and forwards the remainder to the vendor host.
	ModelProxyPath = "/model/"

	// ModelKeyField is the single map key a workspace's BYO model key is stored
	// under in OpenBao and read by the proxy mint. Keeping it here, in the
	// contract both bex-api and the gateway import, prevents key-name drift.
	ModelKeyField = "BEX_AGENT_MODEL_API_KEY"
)

// RegisteredModelEndpoint resolves a supported agent profile server-side and
// rejects caller-supplied endpoint overrides. It is shared by create/rehydrate
// and the credential minter, so legacy rows cannot bypass the registration
// boundary after a new-create check is deployed.
func RegisteredModelEndpoint(agent, requested string) (string, error) {
	profile, ok := LookupAgentProfile(agent)
	if !ok {
		return "", core.NewBadRequestError(
			"AGENT_SESSION_MODEL_ENDPOINT_INVALID",
			"agentConfig.agent has no registered model provider", nil)
	}
	expected := profile.ModelEndpoint
	requested = strings.TrimRight(strings.TrimSpace(requested), "/")
	if requested != "" && requested != expected {
		return "", core.NewBadRequestError(
			"AGENT_SESSION_MODEL_ENDPOINT_INVALID",
			"agentConfig.modelEndpoint must match the selected agent's registered provider", nil)
	}
	if _, err := sessionegress.ModelEndpointHost(expected); err != nil {
		return "", err
	}
	return expected, nil
}

// ModelKeyPlaceholder is the deliberately-fake value the sandbox's agent-native
// credential env holds when the model proxy is active (ADR062 D2). It is useless
// off-pod by construction — the proxy authenticates the source Pod, not this
// string — and its obviously-non-credential shape makes an accidental direct use
// fail loudly rather than look like a real key.
func ModelKeyPlaceholder(sessionID string) string {
	return "bex-model-proxy-placeholder-" + sessionID
}

// IsModelKeyPlaceholder is the final admission guard at the sandbox lifecycle
// seam: only the exact session-bound fake credential may enter a Pod spec.
func IsModelKeyPlaceholder(value, sessionID string) bool {
	return value != "" && value == ModelKeyPlaceholder(sessionID)
}

// ModelKeySecretPath is the OpenBao KV v2 path a workspace's BYO agent-session
// model key is stored at (ADR047 D7). workspaceID (tea-<xid>) is opaque and
// unguessable, so folding it into the path achieves per-workspace isolation
// without the store's tenant-context mechanism (the agentsessions feature and
// the ModelMinter both read through this one definition).
func ModelKeySecretPath(workspaceID string) string {
	return "agent-sessions/" + workspaceID + "/model-key"
}

// Injection schemes the ModelMinter selects and the gateway proxy applies. The
// gateway holds the credential only at request time, so the scheme (which the
// mint computes from the endpoint host + credential shape) travels in the mint
// response and the proxy switches on it. Keeping the vocabulary here holds both
// sides to one enum.
const (
	// AuthSchemeAnthropicKey injects `x-api-key: <cred>` (Anthropic API keys).
	AuthSchemeAnthropicKey = "anthropic-key"
	// AuthSchemeAnthropicOAuth injects `Authorization: Bearer <cred>` for an
	// Anthropic OAuth token (`sk-ant-oat…`); claude-code already sends the
	// matching `anthropic-beta: oauth-…` header, which the proxy passes through.
	AuthSchemeAnthropicOAuth = "anthropic-oauth"
	// AuthSchemeBearer injects `Authorization: Bearer <cred>` (OpenAI-compatible).
	AuthSchemeBearer = "bearer"
	// AuthSchemeGoogleKey injects `x-goog-api-key: <cred>` (Gemini) and strips any
	// client-supplied `key` query parameter.
	AuthSchemeGoogleKey = "google-key"
)

// AuditVerbMintModelCredential records credential issuance. The gateway mints
// once per provider exchange so every request revalidates the session lifecycle.
const AuditVerbMintModelCredential = "agentsessions.MintModelCredential"

// ModelProxyURL binds an agent's model base URL to the exact session identity
// the gateway verifies against the direct source Pod. It carries no credential;
// the per-provider path suffix (e.g. OpenAI's `/v1`) is added by the driver,
// which owns per-agent base-URL routing (ADR062 D5).
func ModelProxyURL(baseURL, namespace, sessionID string) (string, error) {
	if strings.TrimSpace(baseURL) == "" || namespace == "" || sessionID == "" {
		return "", ErrInvalidRequest
	}
	encode := func(value string) string { return base64.RawURLEncoding.EncodeToString([]byte(value)) }
	return strings.TrimRight(baseURL, "/") + ModelProxyPath + encode(namespace) + "/" + encode(sessionID), nil
}

// ParseModelProxyPath extracts the claimed namespace, session id, and the vendor
// subpath (everything after the session segment, leading slash preserved) from a
// model-proxy request path. The namespace/session are claims only: the gateway
// still authorizes them against the resolved source Pod before minting.
func ParseModelProxyPath(path string) (namespace, sessionID, upstreamPath string, err error) {
	if !strings.HasPrefix(path, ModelProxyPath) {
		return "", "", "", ErrInvalidRequest
	}
	parts := strings.SplitN(strings.TrimPrefix(path, ModelProxyPath), "/", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", "", ErrInvalidRequest
	}
	decode := func(value string) (string, error) {
		raw, decodeErr := base64.RawURLEncoding.DecodeString(value)
		return string(raw), decodeErr
	}
	if namespace, err = decode(parts[0]); err != nil {
		return "", "", "", ErrInvalidRequest
	}
	if sessionID, err = decode(parts[1]); err != nil {
		return "", "", "", ErrInvalidRequest
	}
	upstreamPath = "/"
	if len(parts) == 3 {
		upstreamPath = "/" + parts[2]
	}
	return namespace, sessionID, upstreamPath, nil
}

// ModelMintRequest carries only the verified session identity — no repository or
// branch (unlike the Git MintRequest); the model key is not repo-scoped. It
// contains no credential. Nonce is the per-request single-use replay guard (see
// MintRequest.Nonce), bound by the HMAC because it rides inside the signed body.
type ModelMintRequest struct {
	SessionID string `json:"sessionId"`
	Workspace string `json:"workspace"`
	PodName   string `json:"podName"`
	PodUID    string `json:"podUid"`
	Nonce     string `json:"nonce,omitempty"`
}

// ModelMintResponse is the minimal model-credential response. It is always
// emitted with Cache-Control: no-store and must never be logged. EndpointHost is
// the validated vendor host the proxy forwards to (host only — the vendor path
// comes from the agent's own request); Scheme names how to inject Credential.
type ModelMintResponse struct {
	Credential   string `json:"credential"`
	EndpointHost string `json:"endpointHost"`
	Scheme       string `json:"scheme"`
}

// ModelAuthFailureResponse acknowledges a reported vendor auth rejection. The
// gateway needs only to know the session was terminalized (or was already
// terminal); it carries no credential and is safe to log.
type ModelAuthFailureResponse struct {
	Acknowledged bool `json:"acknowledged"`
}

// ModelAuthFailureReason is the terminal failureReason recorded when a session
// cannot authenticate to the model provider. It is actionable — the fix is to
// set a working key, not to retry — so the dashboard renders a next-step message
// rather than a blank failure card (w5/m80 t003/t005).
//
// It covers BOTH causes deliberately (w1/m136): the vendor rejecting a key that
// was sent, and no key existing to send. The remedy is identical — provision a
// valid key and start a new session — so one constant keeps the report a plain
// ModelMintRequest instead of carrying a cause through the signed envelope. The
// wording therefore names both causes rather than asserting the provider
// rejected a key that may never have been sent.
const ModelAuthFailureReason = "this workspace's model API key is missing or was rejected by the model provider (authentication failed); set a valid model key and start a new session"

// modelFailedPhase is the terminal phase the auth-failure verb CASes a live
// session into. It matches agentsessions.PhaseFailed and the store's literal;
// this package cannot import agentsessions (that would cycle), so the value is
// pinned here with the FinalizeAgentSession CAS as the single writer.
const modelFailedPhase = "failed"

// AuthorizeSessionPod binds a model-credential request to the source Pod the
// gateway resolved by TCP source IP. It is the session-scoped subset of
// AuthorizePod: namespace/workspace and the session label must match, but there
// is no repository/branch to check. A sibling sandbox knows the public session
// id but its immutable session label differs.
func AuthorizeSessionPod(namespace, podName, podUID string, labels map[string]string, sessionID string) (ModelMintRequest, error) {
	if strings.TrimSpace(sessionID) == "" {
		return ModelMintRequest{}, ErrForbidden
	}
	workspace := labels[LabelWorkspace]
	if workspace == "" || namespace != workspace+"-sandbox" || labels[LabelRegime] != RegimeSandbox ||
		labels[LabelSession] == "" || labels[LabelSession] != sessionID {
		return ModelMintRequest{}, ErrForbidden
	}
	return ModelMintRequest{SessionID: sessionID, Workspace: workspace, PodName: podName, PodUID: podUID}, nil
}

// ModelMinter is bex-api's internal-only model-credential verb. It trusts only a
// request authenticated by ModelHandler's HMAC and re-checks the session's
// current lifecycle before releasing the workspace's BYO key from OpenBao — the
// same fail-closed gate as the Git minter (a terminal/canceling session, whose
// sandbox may survive the ADR054 grace window, gets nothing).
type ModelMinter struct {
	Keys     core.SecretKV
	Sessions SessionStore
	Audit    core.AuditSink
	Now      func() time.Time
}

func (m *ModelMinter) Mint(ctx context.Context, req ModelMintRequest) (response ModelMintResponse, err error) {
	// Audit issuance and refusal. The event schema carries the session id and the
	// non-secret vendor host, never the credential.
	auditHost := ""
	defer func() {
		outcome := core.AuditAllowed
		if err != nil {
			outcome = core.AuditDenied
			log.Printf("agent-session model credential mint denied (session=%s): %v", req.SessionID, err)
		}
		core.RecordAuditEvent(ctx, m.Audit, core.AuditEvent{
			Caller: req.PodName, CallerMethod: "sandbox", Verb: AuditVerbMintModelCredential,
			Resource: core.WorkspaceObject(req.Workspace), Target: "agent-session:" + req.SessionID,
			TargetName: auditHost, Outcome: outcome, At: m.now(),
		})
	}()
	if m.Keys == nil || m.Sessions == nil {
		return ModelMintResponse{}, fmt.Errorf("agent model credentials unavailable")
	}
	if req.Workspace == "" || req.SessionID == "" || req.PodName == "" || req.PodUID == "" {
		return ModelMintResponse{}, fmt.Errorf("%w: incomplete verified identity", ErrInvalidRequest)
	}
	session, sessErr := m.Sessions.GetAgentSession(ctx, req.SessionID)
	if sessErr != nil {
		if errors.Is(sessErr, store.ErrNotFound) {
			return ModelMintResponse{}, ErrForbidden
		}
		return ModelMintResponse{}, sessErr
	}
	// Same live-lifecycle gate as the Git minter (codex F12 / round-5 finding 10):
	// bind to the session's CURRENT phase, not just the pod's immutable identity,
	// so a retained or compromised sandbox in the grace window cannot pull a fresh
	// model credential after the task is done/failed/canceled/canceling.
	if !currentSandboxCaller(session, req.Workspace, req.PodName, req.PodUID) {
		return ModelMintResponse{}, ErrForbidden
	}
	host, err := modelEndpointHostOf(session.AgentConfig)
	if err != nil {
		return ModelMintResponse{}, err
	}
	auditHost = host
	data, err := m.Keys.Get(ctx, ModelKeySecretPath(req.Workspace))
	if err != nil {
		return ModelMintResponse{}, fmt.Errorf("%w: read agent session model key: %v", core.ErrSecretsUnavailable, err)
	}
	credential := data[ModelKeyField]
	if credential == "" {
		// No key provisioned for this workspace: the proxy has nothing to inject, so
		// the agent cannot authenticate. Refuse (the same end state as a keyless
		// session today) rather than forward an unauthenticated request.
		//
		// The distinct sentinel (it still satisfies errors.Is(_, ErrForbidden)) is
		// what lets the model proxy report an auth failure for THIS cause only, so
		// the session terminalizes with ModelAuthFailureReason instead of the agent
		// CLI retrying into a raw JSON-RPC error (w1/m136).
		return ModelMintResponse{}, ErrModelKeyMissing
	}
	return ModelMintResponse{Credential: credential, EndpointHost: host, Scheme: ResolveAuthScheme(host, credential)}, nil
}

func (m *ModelMinter) now() time.Time {
	if m.Now != nil {
		return m.Now().UTC()
	}
	return time.Now().UTC()
}

// ModelSessionFailer is the store surface the auth-failure verb needs: resolve
// the reported session and CAS a live one to failed. *store.PGStore satisfies it.
type ModelSessionFailer interface {
	GetAgentSession(context.Context, string) (store.AgentSession, error)
	FinalizeAgentSession(ctx context.Context, id, phase, headSHA, prURL string, prNumber int, evidence json.RawMessage, failureReason string) (store.AgentSession, store.TerminalTurnFact, error)
}

// ModelAuthFailer terminalizes a live session whose workspace BYO model key the
// vendor rejected (HTTP 401/403 on the gateway→vendor hop). The gateway reports
// the rejection over the same HMAC channel as the mint; failing the session here
// turns the agent CLI's full retry/backoff (~3min observed on a bad key) into a
// seconds-fast terminal failure, and the Completer reaps the now-terminal
// sandbox on its next tick (w5/m80 t003/t004). It re-checks the same
// current-sandbox-caller binding the minter enforces, so a stale or cross
// sandbox — or a replayed report — can never fail another session.
type ModelAuthFailer struct {
	Sessions ModelSessionFailer
	Audit    core.AuditSink
	Now      func() time.Time
	// ObserveTerminal, when set, records a successful vendor-auth-reject CAS
	// (w5/m88). Wired from bex-api's shared CompletionMetrics; nil in unit tests.
	ObserveTerminal func(fact store.TerminalTurnFact)
}

func (f *ModelAuthFailer) Fail(ctx context.Context, req ModelMintRequest) (response ModelAuthFailureResponse, err error) {
	defer func() {
		outcome := core.AuditAllowed
		if err != nil {
			outcome = core.AuditDenied
			log.Printf("agent-session model auth-failure report denied (session=%s): %v", req.SessionID, err)
		}
		core.RecordAuditEvent(ctx, f.Audit, core.AuditEvent{
			Caller: req.PodName, CallerMethod: "sandbox", Verb: AuditVerbMintModelCredential,
			Resource: core.WorkspaceObject(req.Workspace), Target: "agent-session:" + req.SessionID,
			TargetName: "vendor-auth-rejected", Outcome: outcome, At: f.now(),
		})
	}()
	if f.Sessions == nil {
		return ModelAuthFailureResponse{}, fmt.Errorf("agent model credentials unavailable")
	}
	if req.Workspace == "" || req.SessionID == "" || req.PodName == "" || req.PodUID == "" {
		return ModelAuthFailureResponse{}, fmt.Errorf("%w: incomplete verified identity", ErrInvalidRequest)
	}
	session, sessErr := f.Sessions.GetAgentSession(ctx, req.SessionID)
	if sessErr != nil {
		if errors.Is(sessErr, store.ErrNotFound) {
			return ModelAuthFailureResponse{}, ErrForbidden
		}
		return ModelAuthFailureResponse{}, sessErr
	}
	if !currentSandboxCaller(session, req.Workspace, req.PodName, req.PodUID) {
		// A stale/cross sandbox — or a session already terminal (its sandbox reaped,
		// so the pod triple no longer matches) — is refused here. A repeated report
		// after the session has failed lands here and 403s harmlessly (the gateway's
		// report is best-effort and just logs).
		return ModelAuthFailureResponse{}, ErrForbidden
	}
	// CAS the live session to failed. The phase guard handles the concurrent race
	// where two reports both read `running`: the loser's CAS matches no live row and
	// returns ErrNotFound, which we acknowledge rather than surface — the desired end
	// state (failed) already holds.
	if _, fact, finErr := f.Sessions.FinalizeAgentSession(ctx, req.SessionID, modelFailedPhase, "", "", 0, nil, ModelAuthFailureReason); finErr != nil {
		if !errors.Is(finErr, store.ErrNotFound) {
			return ModelAuthFailureResponse{}, finErr
		}
	} else if f.ObserveTerminal != nil {
		f.ObserveTerminal(fact)
	}
	return ModelAuthFailureResponse{Acknowledged: true}, nil
}

func (f *ModelAuthFailer) now() time.Time {
	if f.Now != nil {
		return f.Now().UTC()
	}
	return time.Now().UTC()
}

// modelEndpointHostOf extracts and re-validates the vendor host from a session's
// stored AgentConfig. Create/Steer/resume always persist the resolved endpoint
// (never empty), so a minimal decode suffices; ModelEndpointHost re-applies the
// SSRF guards (no IP literals, no private/cluster-local names) as defense in
// depth at credential-release time.
func modelEndpointHostOf(config json.RawMessage) (string, error) {
	var decoded struct {
		Agent         string `json:"agent"`
		ModelEndpoint string `json:"modelEndpoint"`
	}
	if len(config) > 0 {
		if err := json.Unmarshal(config, &decoded); err != nil {
			return "", fmt.Errorf("%w: decode agent config", ErrInvalidRequest)
		}
	}
	endpoint, err := RegisteredModelEndpoint(decoded.Agent, decoded.ModelEndpoint)
	if err != nil {
		return "", err
	}
	host, err := sessionegress.ModelEndpointHost(endpoint)
	if err != nil {
		return "", err
	}
	return host, nil
}

// ResolveAuthScheme picks the injection scheme from the credential shape and the
// vendor host. It is deterministic and needs no agent name: an Anthropic OAuth
// token is unmistakable by prefix, and the remaining providers are keyed by host.
func ResolveAuthScheme(host, credential string) string {
	switch {
	case strings.HasPrefix(credential, "sk-ant-oat"):
		return AuthSchemeAnthropicOAuth
	case strings.HasSuffix(host, "anthropic.com"):
		return AuthSchemeAnthropicKey
	case strings.Contains(host, "googleapis.com"):
		return AuthSchemeGoogleKey
	default:
		return AuthSchemeBearer
	}
}

// ModelHandler authenticates the gateway→bex-api internal model-mint request. It
// is mounted only on :8091 and never under the public bex-api router.
type ModelHandler struct {
	Secret []byte
	Minter *ModelMinter
	// Nonce is the single-use replay guard (durable, cross-replica). Wired to the
	// control-plane store in production; nil disables the check (dev/store-off).
	Nonce NonceClaimer
	Now   func() time.Time
}

func (h *ModelHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var mint func(context.Context, ModelMintRequest) (ModelMintResponse, error)
	if h.Minter != nil {
		mint = h.Minter.Mint
	}
	serveSignedMint(w, r, h.Secret, h.now(), h.Nonce, func(req ModelMintRequest) string { return req.Nonce }, mint)
}

func (h *ModelHandler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

// ModelAuthFailureHandler authenticates the gateway→bex-api report that the
// vendor rejected a session's BYO model key. Same signed envelope, nonce guard,
// and :8091-only mount as ModelHandler; an unwired Failer reports unavailable.
type ModelAuthFailureHandler struct {
	Secret []byte
	Failer *ModelAuthFailer
	Nonce  NonceClaimer
	Now    func() time.Time
}

func (h *ModelAuthFailureHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var fail func(context.Context, ModelMintRequest) (ModelAuthFailureResponse, error)
	if h.Failer != nil {
		fail = h.Failer.Fail
	}
	serveSignedMint(w, r, h.Secret, h.now(), h.Nonce, func(req ModelMintRequest) string { return req.Nonce }, fail)
}

func (h *ModelAuthFailureHandler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

// ModelClient is the gateway's HMAC-authenticated caller for bex-api's internal
// model-mint verb. It never logs or persists the returned credential.
type ModelClient struct {
	URL    string
	Secret []byte
	HTTP   *http.Client
	Now    func() time.Time
}

func (c *ModelClient) Mint(ctx context.Context, req ModelMintRequest) (ModelMintResponse, error) {
	req.Nonce = newNonce()
	return postSignedMint(ctx, c.URL, c.Secret, c.now(), c.HTTP, req,
		func(out ModelMintResponse) bool { return out.Credential != "" && out.EndpointHost != "" })
}

// ReportAuthFailure tells bex-api the vendor rejected this session's BYO key so
// the session is terminalized fast (w5/m80 t003). It posts to the auth-failure
// sibling of the mint URL with the same signed envelope. Best-effort by contract:
// the caller relays the vendor's 401/403 to the sandbox regardless of this result.
func (c *ModelClient) ReportAuthFailure(ctx context.Context, req ModelMintRequest) error {
	req.Nonce = newNonce()
	url := strings.TrimRight(c.URL, "/") + "/auth-failure"
	_, err := postSignedMint(ctx, url, c.Secret, c.now(), c.HTTP, req,
		func(out ModelAuthFailureResponse) bool { return out.Acknowledged })
	return err
}

func (c *ModelClient) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}
