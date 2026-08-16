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
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/agentcred"
)

// credentialTTL bounds how long a minted model credential is reused from the
// gateway's process memory (ADR062 D2/D3). It collapses the per-request bex-api
// mint round-trip (and its audit event) to at most one per session per window,
// so a chatty turn does not fan out a mint per HTTP call. The backing TTLCache
// drops an expired entry on its next lookup and sweeps at CacheMax, so a dead
// session's credential does not linger unbounded.
const credentialTTL = 60 * time.Second

// Broker is the model proxy. It is disabled (Enabled()==false) unless both the
// source-pod resolver and the bex-api model-mint client are wired, so an
// unconfigured deployment simply never starts the listener.
type Broker struct {
	Pods    agentcred.SessionPodResolver
	API     *agentsession.ModelClient
	Metrics *sshgateway.Metrics
	Now     func() time.Time
	// HTTP is a test seam; production leaves it nil and the streaming client below
	// is built once (no total timeout, so a long SSE model response is never
	// truncated mid-stream, and the connection pool survives across requests).
	HTTP *http.Client

	clientOnce sync.Once
	client     *http.Client
	credsOnce  sync.Once
	creds      *core.TTLCache[agentsession.ModelMintResponse]
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
	upstream, err := upstreamURL(credential.EndpointHost, upstreamPath, r.URL.RawQuery, credential.Scheme)
	if err != nil {
		http.Error(w, "model proxy unavailable", http.StatusBadGateway)
		return
	}
	request, err := http.NewRequestWithContext(r.Context(), r.Method, upstream, r.Body)
	if err != nil {
		http.Error(w, "model proxy unavailable", http.StatusBadGateway)
		return
	}
	request.ContentLength = r.ContentLength
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
	flushingCopy(w, response.Body)
	b.Metrics.Authentication("accepted")
}

// mint returns the session's model credential, served from the short-TTL memory
// cache when fresh, otherwise from a fresh bex-api mint. Caching bounds the
// gateway→bex-api round-trip, the mint's audit-event volume, and (via TTLCache's
// expiry-drop + CacheMax sweep) the in-memory residency of the credential.
func (b *Broker) mint(ctx context.Context, namespace string, req agentsession.ModelMintRequest) (agentsession.ModelMintResponse, error) {
	// The cache key is workspace-scoped: namespace encodes the workspace, and thus
	// the OpenBao path, so no two sessions can alias each other's credential.
	key := namespace + "\x00" + req.SessionID
	cache := b.credCache()
	if hit, ok := cache.Get(key); ok {
		return hit, nil
	}
	resp, err := b.API.Mint(ctx, req)
	if err != nil {
		return agentsession.ModelMintResponse{}, err
	}
	cache.Put(key, resp, b.now().Add(credentialTTL))
	return resp, nil
}

func (b *Broker) credCache() *core.TTLCache[agentsession.ModelMintResponse] {
	b.credsOnce.Do(func() { b.creds = core.NewTTLCache[agentsession.ModelMintResponse]() })
	return b.creds
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

func copyResponseHeaders(dst, src http.Header) {
	for _, name := range []string{"Content-Type", "Cache-Control", "Anthropic-Version", "X-Request-Id", "Retry-After"} {
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

func (b *Broker) httpClient() *http.Client {
	if b.HTTP != nil {
		return b.HTTP
	}
	// Built once so the vendor connection pool (IdleConnTimeout) survives across
	// requests — a per-turn model path re-dials otherwise. Like the Git proxy, it
	// bounds dial/TLS/response-header against a hung upstream but NOT the total
	// exchange: a model SSE response streams for the length of a turn and a total
	// timeout would truncate it. The request context still bounds the lifetime.
	b.clientOnce.Do(func() {
		b.client = &http.Client{
			Transport: &http.Transport{
				DialContext:           (&net.Dialer{Timeout: 15 * time.Second}).DialContext,
				TLSHandshakeTimeout:   15 * time.Second,
				ResponseHeaderTimeout: 60 * time.Second,
				IdleConnTimeout:       90 * time.Second,
			},
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}
	})
	return b.client
}

func (b *Broker) now() time.Time {
	if b.Now != nil {
		return b.Now().UTC()
	}
	return time.Now().UTC()
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
