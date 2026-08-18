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

// Package modelproxy is the Pod-bound credential-injecting model proxy of the
// isolated SSH gateway (ADR062). A sandbox's agent reaches its model provider
// only through this listener; the reusable BYO model key (ADR047 D7) stays
// entirely on the trusted gateway→vendor hop and never enters the sandbox
// process tree. It mirrors the agentcred Git proxy: verify the direct source
// Pod, mint through bex-api (which reads OpenBao), inject on the upstream hop.
package modelproxy

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/agentcred"
)

const (
	// Model requests are JSON prompts, not file uploads. Reading the bounded body
	// before opening the credential-bearing upstream request ensures an oversized
	// client cannot stream a prefix to the provider and fail only after the cap.
	maxModelRequestBodyBytes = 4 << 20
	// Provider streams are copied, not accumulated; this byte ceiling still
	// prevents one exchange from consuming unbounded gateway bandwidth.
	maxModelResponseBodyBytes = 64 << 20
	// Long agent turns remain supported, but every stream has a finite lifetime.
	defaultModelRequestDuration = 2 * time.Hour
)

// Broker is the model proxy. It is disabled (Enabled()==false) unless both the
// source-pod resolver and the bex-api model-mint client are wired, so an
// unconfigured deployment simply never starts the listener.
type Broker struct {
	Pods    agentcred.SessionPodResolver
	API     *agentsession.ModelClient
	Metrics *sshgateway.Metrics
	// HTTP is a test seam; production leaves it nil and the streaming client below
	// is built once; the request context supplies the finite stream lifetime while
	// the connection pool survives across requests.
	HTTP *http.Client
	// Limits bounds concurrent exchanges globally and per source sandbox Pod IP.
	// Production wires configurable values; nil gets secure defaults (32/2).
	Limits *sshgateway.SessionLimiter
	// MaxDuration is a test/config seam for the total request + response lifetime.
	// Zero uses defaultModelRequestDuration.
	MaxDuration time.Duration
	// MaxRequestsPerSession bounds the cumulative provider exchanges ONE agent
	// session may push through the proxy (round-13 #8): every per-exchange bound
	// (concurrency, bytes, duration) resets on completion, so without a cumulative
	// cap malicious repository code running in a live sandbox could loop billable
	// inference for the session's whole lifetime. Process-local and atomic; a
	// gateway restart resets it (accepted — it bounds runaway loops, not spend
	// accounting). <= 0 disables the dimension.
	MaxRequestsPerSession int
	// MaxRequestsPerWorkspace is the same cumulative bound across every session
	// of one workspace (all `<ws>-sandbox` pods share the delegated credential).
	// <= 0 disables the dimension.
	MaxRequestsPerWorkspace int

	clientOnce sync.Once
	client     *http.Client
	limitsOnce sync.Once
	limits     *sshgateway.SessionLimiter
	budgetOnce sync.Once
	budget     *requestBudget
}

func (b *Broker) Enabled() bool         { return b != nil && b.Pods != nil && b.API != nil }
func (b *Broker) Handler() http.Handler { return http.HandlerFunc(b.serveModel) }

func (b *Broker) serveModel(w http.ResponseWriter, r *http.Request) {
	if !b.Enabled() {
		http.Error(w, "model proxy unavailable", http.StatusServiceUnavailable)
		return
	}
	namespace, sessionID, upstreamPath, err := agentsession.ParseModelProxyPath(r.URL.Path)
	if err != nil {
		http.Error(w, "invalid model request", http.StatusBadRequest)
		return
	}
	sourceIP, err := agentcred.RemoteIP(r.RemoteAddr)
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if ok, scope := b.limiter().Acquire(sourceIP); !ok {
		b.Metrics.LimitRejected(scope)
		http.Error(w, "too many concurrent model requests from this sandbox", http.StatusTooManyRequests)
		return
	}
	defer b.limiter().Release(sourceIP)
	ctx, cancel := context.WithTimeout(r.Context(), b.maxDuration())
	defer cancel()
	r = r.WithContext(ctx)
	if r.ContentLength > maxModelRequestBodyBytes {
		http.Error(w, "model request body too large", http.StatusRequestEntityTooLarge)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxModelRequestBodyBytes+1))
	if err != nil {
		http.Error(w, "invalid model request body", http.StatusBadRequest)
		return
	}
	if len(body) > maxModelRequestBodyBytes {
		http.Error(w, "model request body too large", http.StatusRequestEntityTooLarge)
		return
	}
	pod, err := b.Pods.ResolveSessionPod(r.Context(), namespace, sourceIP)
	if err != nil {
		b.Metrics.Authentication("rejected_key")
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	verified, err := agentsession.AuthorizeSessionPod(namespace, pod.Name, pod.UID, pod.Labels, sessionID)
	if err != nil {
		b.Metrics.Authentication("rejected_target")
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	// Cumulative budget (round-13 #8), charged before the mint so an exhausted
	// session is refused before any credential is attached or the provider hit.
	// The counter is atomic under the process-wide budget; a request later
	// refused by the operation allowlist still consumes its charge (conservative
	// by design — refunds would let a loop probe forever).
	if !b.budgets().chargeSession(namespace, sessionID) {
		b.Metrics.LimitRejected("model_session_budget")
		http.Error(w, "model request budget exhausted for this session", http.StatusTooManyRequests)
		return
	}
	if !b.budgets().chargeWorkspace(namespace) {
		b.Metrics.LimitRejected("model_workspace_budget")
		http.Error(w, "model request budget exhausted for this workspace", http.StatusTooManyRequests)
		return
	}
	credential, err := b.mint(r.Context(), namespace, verified)
	if err != nil {
		if errors.Is(err, agentsession.ErrForbidden) {
			b.Metrics.Authentication("rejected_target")
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		http.Error(w, "model proxy unavailable", http.StatusBadGateway)
		return
	}
	if !allowedProviderOperation(credential.EndpointHost, r.Method, upstreamPath, r.URL.RawQuery, r.Header.Get("Content-Type")) {
		http.Error(w, "model operation is not allowed", http.StatusForbidden)
		return
	}
	upstream, err := upstreamURL(credential.EndpointHost, upstreamPath, r.URL.RawQuery, credential.Scheme)
	if err != nil {
		http.Error(w, "model proxy unavailable", http.StatusBadGateway)
		return
	}
	request, err := http.NewRequestWithContext(r.Context(), r.Method, upstream, bytes.NewReader(body))
	if err != nil {
		http.Error(w, "model proxy unavailable", http.StatusBadGateway)
		return
	}
	request.ContentLength = int64(len(body))
	copyForwardableHeaders(request.Header, r.Header)
	// Strip any client-supplied auth (the sandbox holds only a placeholder), then
	// inject the real credential — so the placeholder never reaches the vendor and
	// the injected key is the sole authenticator on the upstream hop.
	injectAuth(request.Header, credential.Scheme, credential.Credential)
	request.Header.Set("User-Agent", "bex-agent-model-proxy/1")

	response, err := b.httpClient().Do(request)
	if err != nil {
		http.Error(w, "model upstream unavailable", http.StatusBadGateway)
		return
	}
	defer response.Body.Close()
	copyResponseHeaders(w.Header(), response.Header)
	w.WriteHeader(response.StatusCode)
	flushingCopy(w, io.LimitReader(response.Body, maxModelResponseBodyBytes))
	b.Metrics.Authentication("accepted")
}

// mint always revalidates the current session lifecycle through bex-api. Model
// requests are turn-scale, so one internal check per provider exchange is a
// better boundary than allowing a canceled session to ride a credential cache.
func (b *Broker) mint(ctx context.Context, namespace string, req agentsession.ModelMintRequest) (agentsession.ModelMintResponse, error) {
	_ = namespace // retained in the signature to keep the verified call shape explicit.
	return b.API.Mint(ctx, req)
}

func (b *Broker) limiter() *sshgateway.SessionLimiter {
	if b.Limits != nil {
		return b.Limits
	}
	b.limitsOnce.Do(func() { b.limits = sshgateway.NewSessionLimiter(32, 2) })
	return b.limits
}

// budgetSweepInterval / budgetIdleTTL keep the budget's key set bounded: agent
// sessions are short-lived (minutes to hours), so entries idle for a day are
// gone sessions and are pruned. The janitor stops with the process.
const (
	budgetSweepInterval = 30 * time.Minute
	budgetIdleTTL       = 24 * time.Hour
)

// requestBudget is the process-wide cumulative exchange counter for the model
// proxy (round-13 #8): per-session and per-workspace atomic counts. Sessions
// are keyed by namespace+session id (the verified pair), workspaces by the
// `<ws>` the `<ws>-sandbox` namespace derives from.
type requestBudget struct {
	perSession   int
	perWorkspace int
	mu           sync.Mutex
	charged      map[string]int
	lastUsed     map[string]time.Time
	janitorOnce  sync.Once
}

func (b *requestBudget) charge(key string, max int) bool {
	if max <= 0 {
		return true // dimension disabled
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.charged == nil {
		b.charged = make(map[string]int)
		b.lastUsed = make(map[string]time.Time)
		b.janitorOnce.Do(func() {
			go func() {
				ticker := time.NewTicker(budgetSweepInterval)
				defer ticker.Stop()
				for range ticker.C {
					b.sweep(time.Now().Add(-budgetIdleTTL))
				}
			}()
		})
	}
	if b.charged[key] >= max {
		return false
	}
	b.charged[key]++
	b.lastUsed[key] = time.Now()
	return true
}

// sweep drops keys idle past the horizon so finished sessions do not accumulate
// forever (the budget map must not become its own unbounded state).
func (b *requestBudget) sweep(horizon time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for key, at := range b.lastUsed {
		if at.Before(horizon) {
			delete(b.charged, key)
			delete(b.lastUsed, key)
		}
	}
}

func (b *requestBudget) chargeSession(namespace, sessionID string) bool {
	return b.charge("session|"+namespace+"|"+sessionID, b.perSession)
}

func (b *requestBudget) chargeWorkspace(namespace string) bool {
	return b.charge("workspace|"+strings.TrimSuffix(namespace, "-sandbox"), b.perWorkspace)
}

func (b *Broker) budgets() *requestBudget {
	b.budgetOnce.Do(func() {
		b.budget = &requestBudget{perSession: b.MaxRequestsPerSession, perWorkspace: b.MaxRequestsPerWorkspace}
	})
	return b.budget
}

func (b *Broker) maxDuration() time.Duration {
	if b.MaxDuration > 0 {
		return b.MaxDuration
	}
	return defaultModelRequestDuration
}

// forwardableHeaders are the request headers the proxy relays verbatim to the
// vendor. Auth headers are deliberately absent (injectAuth sets them); hop-by-hop
// and host headers are dropped so a client cannot smuggle routing or a stale
// Authorization onto the credential-bearing hop.
var forwardableHeaders = []string{
	"Accept", "Accept-Encoding", "Content-Type",
	"Anthropic-Version", "Anthropic-Beta", "OpenAI-Beta", "X-Stainless-Lang",
}

func copyForwardableHeaders(dst, src http.Header) {
	for _, name := range forwardableHeaders {
		if value := src.Get(name); value != "" {
			dst.Set(name, value)
		}
	}
}

// Content-Encoding must travel with the body: the request hop forwards the
// agent's own Accept-Encoding (so Go's transparent gzip handling is disabled
// and the vendor's compressed bytes stream through verbatim), which means a
// gzip response relayed WITHOUT its Content-Encoding header hands the agent's
// JSON parser raw gzip bytes — every Claude turn died on
// `Unexpected token '\x1f'` (the gzip magic number) until it was forwarded.
func copyResponseHeaders(dst, src http.Header) {
	for _, name := range []string{"Content-Type", "Content-Encoding", "Cache-Control", "Anthropic-Version", "X-Request-Id", "Retry-After"} {
		if value := src.Get(name); value != "" {
			dst.Set(name, value)
		}
	}
}

// anthropicOAuthBeta is the beta flag the Anthropic API requires for an OAuth
// (sk-ant-oat) token. The proxy adds it itself so OAuth works regardless of
// whether the in-sandbox agent runs in API-key or OAuth mode — the sandbox only
// holds a placeholder and cannot be relied on to send the right beta header.
const anthropicOAuthBeta = "oauth-2025-04-20"

// injectAuth applies the mint's scheme by first clearing every credential-bearing
// header a client could have set, then setting only the injected one. It is the
// single place the proxy trusts pod identity over the placeholder string.
func injectAuth(header http.Header, scheme, credential string) {
	header.Del("Authorization")
	header.Del("X-Api-Key")
	header.Del("X-Goog-Api-Key")
	switch scheme {
	case agentsession.AuthSchemeAnthropicKey:
		header.Set("X-Api-Key", credential)
	case agentsession.AuthSchemeGoogleKey:
		header.Set("X-Goog-Api-Key", credential)
	case agentsession.AuthSchemeAnthropicOAuth:
		header.Set("Authorization", "Bearer "+credential)
		header.Set("Anthropic-Beta", mergeBeta(header.Get("Anthropic-Beta"), anthropicOAuthBeta))
	default: // AuthSchemeBearer (OpenAI-compatible)
		header.Set("Authorization", "Bearer "+credential)
	}
}

// mergeBeta appends flag to a comma-separated anthropic-beta value without
// duplicating it, preserving any flags the agent already requested.
func mergeBeta(existing, flag string) string {
	if existing == "" {
		return flag
	}
	for _, part := range strings.Split(existing, ",") {
		if strings.TrimSpace(part) == flag {
			return existing
		}
	}
	return existing + "," + flag
}

var googleInferencePath = regexp.MustCompile(`^/v1beta/models/[A-Za-z0-9._:-]+:(generateContent|streamGenerateContent|countTokens)$`)

// allowedProviderOperation is the confused-deputy boundary: the sandbox can
// invoke only inference operations needed by the three installed adapters, not
// account, file, batch, fine-tune, key-management, or deletion APIs that happen
// to share the credential's origin.
func allowedProviderOperation(host, method, path, rawQuery, contentType string) bool {
	if method != http.MethodPost || !jsonContentType(contentType) {
		return false
	}
	switch strings.ToLower(host) {
	case "api.openai.com":
		return rawQuery == "" && (path == "/v1/responses" || path == "/v1/responses/compact" || path == "/v1/chat/completions")
	case "api.anthropic.com":
		if path != "/v1/messages" && path != "/v1/messages/count_tokens" {
			return false
		}
		// The Anthropic SDK's beta namespace — the one Claude Code actually uses —
		// addresses the SAME inference operations as `POST /v1/messages?beta=true`
		// (and the count_tokens sibling). Requiring an empty query here therefore
		// refused every real Claude turn with "403 model operation is not allowed",
		// so the flag is admitted explicitly rather than by widening the paths.
		return allowedQuery(rawQuery, map[string]string{"beta": "true"})
	case "generativelanguage.googleapis.com":
		if !googleInferencePath.MatchString(path) {
			return false
		}
		if rawQuery == "" {
			return true
		}
		query, err := url.ParseQuery(rawQuery)
		if err != nil {
			return false
		}
		// Gemini SDKs commonly carry the placeholder in ?key=. It is never
		// forwarded: upstreamURL strips it before the real key is injected.
		query.Del("key")
		return allowedQueryValues(query, map[string]string{"alt": "sse"})
	default:
		return false
	}
}

// allowedQuery admits a query string only when every parameter it carries is
// named in allowed and holds exactly that one value. An empty query always
// passes; an unparseable one never does.
func allowedQuery(rawQuery string, allowed map[string]string) bool {
	if rawQuery == "" {
		return true
	}
	query, err := url.ParseQuery(rawQuery)
	if err != nil {
		return false
	}
	return allowedQueryValues(query, allowed)
}

func allowedQueryValues(query url.Values, allowed map[string]string) bool {
	for name, values := range query {
		want, ok := allowed[name]
		if !ok || len(values) != 1 || values[0] != want {
			return false
		}
	}
	return true
}

func jsonContentType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && mediaType == "application/json"
}

// upstreamURL composes the vendor request from the mint's host and the exact
// subpath the agent produced. Only the host comes from the trusted mint; the
// path/query come from the sandbox, so a compromised sandbox can vary the path
// but can never redirect to a different host than its session's registered one.
// The `key` query parameter is stripped for the Google scheme (the credential is
// injected as a header instead).
func upstreamURL(host, path, rawQuery, scheme string) (string, error) {
	if host == "" {
		return "", errors.New("empty model endpoint host")
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if scheme == agentsession.AuthSchemeGoogleKey && rawQuery != "" {
		values, err := url.ParseQuery(rawQuery)
		if err == nil {
			values.Del("key")
			rawQuery = values.Encode()
		}
	}
	u := url.URL{Scheme: "https", Host: host, Opaque: "", Path: path, RawQuery: rawQuery}
	// url.URL.String uses Path (already sandbox-supplied, percent-decoded by the
	// server); re-encode via EscapedPath by round-tripping through Parse so a
	// crafted path cannot inject a host or scheme.
	parsed, err := url.Parse(u.String())
	if err != nil || parsed.Host != host || parsed.Scheme != "https" {
		return "", errors.New("invalid upstream url")
	}
	return parsed.String(), nil
}

// modelResponseHeaderTimeout bounds time-to-first-response-header for the
// upstream model hop; the exchange context and response byte cap bound the SSE
// body after headers arrive.
const modelResponseHeaderTimeout = 60 * time.Second

func (b *Broker) httpClient() *http.Client {
	if b.HTTP != nil {
		return b.HTTP
	}
	// Built once so the vendor connection pool survives across requests — a
	// per-turn model path re-dials otherwise.
	b.clientOnce.Do(func() {
		b.client = sshgateway.NewUpstreamClient(modelResponseHeaderTimeout)
	})
	return b.client
}

// flushingCopy streams the upstream body to the client, flushing after each
// chunk so SSE tokens reach the agent as they arrive rather than being buffered.
func flushingCopy(w http.ResponseWriter, src io.Reader) {
	flusher, _ := w.(http.Flusher)
	buf := make([]byte, 32*1024)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
		if err != nil {
			return
		}
	}
}
