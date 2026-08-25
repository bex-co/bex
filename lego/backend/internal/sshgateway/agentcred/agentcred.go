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

// Package agentcred is the Pod-bound Git smart-HTTP proxy of the isolated SSH
// gateway (ADR047 D2). Sandboxes can clone and push only their bound repository
// and bex-agent branch, while the raw GitHub installation token stays entirely
// on the trusted gateway->GitHub hop.
package agentcred

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
)

const (
	maxPacketBytes       = 65520
	maxShallowBoundaries = 1024
	maxResponseBodyBytes = 512 << 20
	defaultMaxDuration   = 10 * time.Minute
	budgetSweepInterval  = 30 * time.Minute
	budgetIdleTTL        = 24 * time.Hour

	// maxReceivePackPrefixBytes is the total-bytes budget for the receive-pack
	// preamble validateReceivePack replays upstream (codex-security 2026-08 F3).
	// The per-packet and per-kind caps above are each individually reasonable
	// but their product is not: 1024 shallow lines of up-to-64 KiB packets is
	// ~64 MiB of resident buffer in the shared gateway. A legitimate preamble
	// (a handful of shallow boundaries plus ≤8 update commands) is kilobytes;
	// anything past this budget is padding, and padding is the attack.
	maxReceivePackPrefixBytes = 1 << 20
)

// Admission bounds used when no limiter is injected; cmd/ssh-gateway supplies
// the configured ones.
const (
	defaultMaxConns       = 64
	defaultMaxConnsPerPod = 4
)

// maxRequestBodyBytes caps one git smart-HTTP request body (a pack upload). A
// legitimate session exchange is source-code sized; the cap exists so a
// hostile Pod cannot drip an unbounded body at the shared gateway (codex
// round-9 #4) — the server read deadline bounds its duration, this bounds its
// bytes.
const maxRequestBodyBytes = 256 << 20

type SessionPod struct {
	Name   string
	UID    string
	Labels map[string]string
}

type SessionPodResolver interface {
	ResolveSessionPod(context.Context, string, string) (SessionPod, error)
}

type KubeSessionPodResolver struct{ Client kubernetes.Interface }

func (r KubeSessionPodResolver) ResolveSessionPod(ctx context.Context, namespace, sourceIP string) (SessionPod, error) {
	if r.Client == nil || namespace == "" || net.ParseIP(sourceIP) == nil {
		return SessionPod{}, agentsession.ErrForbidden
	}
	pods, err := r.Client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("status.podIP", sourceIP).String(),
		Limit:         2,
	})
	if err != nil || len(pods.Items) != 1 {
		return SessionPod{}, agentsession.ErrForbidden
	}
	pod := pods.Items[0]
	if pod.Status.PodIP != sourceIP || pod.DeletionTimestamp != nil {
		return SessionPod{}, agentsession.ErrForbidden
	}
	return SessionPod{Name: pod.Name, UID: string(pod.UID), Labels: pod.Labels}, nil
}

type Broker struct {
	Pods    SessionPodResolver
	API     *agentsession.Client
	Audit   core.AuditSink
	Metrics *sshgateway.Metrics
	Now     func() time.Time
	// Limits bounds concurrent in-flight proxy requests, globally and per
	// source Pod IP (codex round-9 #4): the slot is acquired BEFORE the Pod
	// lookup and credential mint and held for the whole exchange, so a hostile
	// session Pod dripping request bodies cannot stack unbounded goroutines,
	// file descriptors, and Kubernetes API lookups on the shared gateway. nil
	// gets a private limiter with the defaults (64 global, 4 per source).
	Limits *sshgateway.SessionLimiter
	// UpstreamOrigin and HTTP are test seams. Production leaves the origin empty,
	// which fixes every credential-bearing request to HTTPS github.com.
	UpstreamOrigin          string
	HTTP                    *http.Client
	MaxDuration             time.Duration
	MaxRequestsPerSession   int
	MaxRequestsPerWorkspace int

	// clientOnce/client memoize the default upstream client so its connection
	// pool is reused across requests (see httpClient).
	clientOnce sync.Once
	client     *http.Client

	// limitsOnce/limits memoize the default limiter (see limiter).
	limitsOnce sync.Once
	limits     *sshgateway.SessionLimiter
	budgetOnce sync.Once
	budget     *requestBudget
}

// limiter returns the admission limiter, memoizing the default so acquire and
// release always land on the SAME instance — a per-call instance makes the cap
// count nothing.
func (b *Broker) limiter() *sshgateway.SessionLimiter {
	if b.Limits != nil {
		return b.Limits
	}
	// Explicit rather than NewSessionLimiter's zero-value defaults, which are
	// tuned for SSH sessions (100/5) and would silently redefine this proxy's
	// documented bound.
	b.limitsOnce.Do(func() { b.limits = sshgateway.NewSessionLimiter(defaultMaxConns, defaultMaxConnsPerPod) })
	return b.limits
}

func (b *Broker) Enabled() bool         { return b != nil && b.Pods != nil && b.API != nil }
func (b *Broker) Handler() http.Handler { return http.HandlerFunc(b.serveGit) }

func (b *Broker) serveGit(w http.ResponseWriter, r *http.Request) {
	if !b.Enabled() {
		http.Error(w, "git proxy unavailable", http.StatusServiceUnavailable)
		return
	}
	namespace, mintRequest, service, err := parseProxyPath(r.URL.Path)
	if err != nil || !validGitRequest(r, service) {
		http.Error(w, "invalid git request", http.StatusBadRequest)
		return
	}
	sourceIP, err := RemoteIP(r.RemoteAddr)
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	// Admission before any stateful work (codex round-9 #4): the concurrency
	// slot is held across the Pod lookup, credential mint, pkt-line
	// validation, and the upstream exchange, and the body is byte-capped, so
	// slow or stacked requests cost at most one bounded slot each.
	limiter := b.limiter()
	if ok, scope := limiter.Acquire(sourceIP); !ok {
		b.Metrics.LimitRejected(scope)
		http.Error(w, "too many concurrent git requests from this sandbox", http.StatusTooManyRequests)
		return
	}
	defer limiter.Release(sourceIP)
	ctx, cancel := context.WithTimeout(r.Context(), b.maxDuration())
	defer cancel()
	r = r.WithContext(ctx)
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	pod, err := b.Pods.ResolveSessionPod(r.Context(), namespace, sourceIP)
	if err != nil {
		b.Metrics.Authentication("rejected_key")
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	verified, err := agentsession.AuthorizePod(namespace, pod.Name, pod.UID, pod.Labels, mintRequest)
	if err != nil {
		b.Metrics.Authentication("rejected_target")
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if !b.budgets().chargeSession(namespace, verified.SessionID) {
		b.Metrics.LimitRejected("git_session_budget")
		http.Error(w, "git request budget exhausted for this session", http.StatusTooManyRequests)
		return
	}
	if !b.budgets().chargeWorkspace(namespace) {
		b.Metrics.LimitRejected("git_workspace_budget")
		http.Error(w, "git request budget exhausted for this workspace", http.StatusTooManyRequests)
		return
	}
	body := io.Reader(r.Body)
	if service == "git-receive-pack" && r.Method == http.MethodPost {
		body, err = validateReceivePack(r.Body, verified.Branch)
		if err != nil {
			b.Metrics.Authentication("rejected_target")
			http.Error(w, "push is not confined to the session branch", http.StatusForbidden)
			return
		}
	}
	credential, err := b.API.Mint(r.Context(), verified)
	if err != nil {
		if errors.Is(err, agentsession.ErrForbidden) {
			b.Metrics.Authentication("rejected_target")
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		http.Error(w, "git proxy unavailable", http.StatusBadGateway)
		return
	}
	upstream, err := b.upstreamURL(verified.Repository, service, r.URL.RawQuery)
	if err != nil {
		http.Error(w, "git proxy unavailable", http.StatusBadGateway)
		return
	}
	request, err := http.NewRequestWithContext(r.Context(), r.Method, upstream, body)
	if err != nil {
		http.Error(w, "git proxy unavailable", http.StatusBadGateway)
		return
	}
	request.ContentLength = r.ContentLength
	request.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(credential.Username+":"+credential.Token)))
	request.Header.Set("User-Agent", "bex-agent-git-proxy/1")
	for _, name := range []string{"Accept", "Content-Type", "Git-Protocol"} {
		if value := r.Header.Get(name); value != "" {
			request.Header.Set(name, value)
		}
	}
	response, err := b.httpClient().Do(request)
	if err != nil {
		http.Error(w, "git upstream unavailable", http.StatusBadGateway)
		return
	}
	defer response.Body.Close()
	// GitHub smart HTTP succeeds with 200. Do not reflect error bodies or auth
	// challenges from the credential-bearing hop into the untrusted sandbox.
	if response.StatusCode != http.StatusOK {
		http.Error(w, "git upstream refused request", http.StatusBadGateway)
		return
	}
	for _, name := range []string{"Content-Type", "Cache-Control"} {
		if value := response.Header.Get(name); value != "" {
			w.Header().Set(name, value)
		}
	}
	w.WriteHeader(http.StatusOK)
	if deadline, ok := r.Context().Deadline(); ok {
		_ = http.NewResponseController(w).SetWriteDeadline(deadline)
	}
	_, _ = io.Copy(w, io.LimitReader(response.Body, maxResponseBodyBytes))
	b.Metrics.Authentication("accepted")
	core.RecordAuditEvent(r.Context(), b.Audit, core.AuditEvent{
		Caller: pod.Name, CallerMethod: "sandbox", Verb: agentsession.AuditVerbProxyCredential,
		Resource: core.WorkspaceObject(verified.Workspace), Target: "agent-session:" + verified.SessionID,
		TargetName: verified.Repository, Outcome: core.AuditAllowed, At: b.now(),
	})
}

type requestBudget struct {
	perSession, perWorkspace int
	mu                       sync.Mutex
	charged                  map[string]int
	lastUsed                 map[string]time.Time
	janitorOnce              sync.Once
}

func (b *requestBudget) charge(key string, max int) bool {
	if max <= 0 {
		return true
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
	return defaultMaxDuration
}

func parseProxyPath(path string) (string, agentsession.MintRequest, string, error) {
	if !strings.HasPrefix(path, agentsession.GitProxyPath) {
		return "", agentsession.MintRequest{}, "", agentsession.ErrInvalidRequest
	}
	parts := strings.SplitN(strings.TrimPrefix(path, agentsession.GitProxyPath), "/", 5)
	if len(parts) != 5 {
		return "", agentsession.MintRequest{}, "", agentsession.ErrInvalidRequest
	}
	decode := func(value string) (string, error) {
		raw, err := base64.RawURLEncoding.DecodeString(value)
		return string(raw), err
	}
	namespace, err := decode(parts[0])
	if err != nil {
		return "", agentsession.MintRequest{}, "", err
	}
	sessionID, err := decode(parts[1])
	if err != nil {
		return "", agentsession.MintRequest{}, "", err
	}
	repository, err := decode(parts[2])
	if err != nil {
		return "", agentsession.MintRequest{}, "", err
	}
	branch, err := decode(parts[3])
	if err != nil {
		return "", agentsession.MintRequest{}, "", err
	}
	service := ""
	switch parts[4] {
	case "info/refs":
		service = "info/refs"
	case "git-upload-pack", "git-receive-pack":
		service = parts[4]
	default:
		return "", agentsession.MintRequest{}, "", agentsession.ErrInvalidRequest
	}
	return namespace, agentsession.MintRequest{SessionID: sessionID, Repository: repository, Branch: branch}, service, nil
}

func validGitRequest(r *http.Request, service string) bool {
	if service == "info/refs" {
		return r.Method == http.MethodGet && (r.URL.Query().Get("service") == "git-upload-pack" || r.URL.Query().Get("service") == "git-receive-pack")
	}
	return r.Method == http.MethodPost && r.URL.RawQuery == "" && strings.EqualFold(r.Header.Get("Content-Type"), "application/x-"+service+"-request")
}

func validateReceivePack(body io.Reader, branch string) (io.Reader, error) {
	reader := bufio.NewReader(body)
	var prefix bytes.Buffer
	commands := 0
	shallows := 0
	for {
		header := make([]byte, 4)
		if _, err := io.ReadFull(reader, header); err != nil {
			return nil, err
		}
		prefix.Write(header)
		if string(header) == "0000" {
			break
		}
		length, err := strconv.ParseUint(string(header), 16, 16)
		if err != nil || length < 4 || length > maxPacketBytes {
			return nil, agentsession.ErrForbidden
		}
		packet := make([]byte, int(length)-4)
		if _, err := io.ReadFull(reader, packet); err != nil {
			return nil, err
		}
		prefix.Write(packet)
		// The product of the per-packet cap and the per-kind counts bounds
		// nothing by itself; the accumulated replay buffer is what the process
		// pays for, so it gets its own explicit budget (codex-security 2026-08
		// F3). Fail closed on the padding that would otherwise reach it.
		if prefix.Len() > maxReceivePackPrefixBytes {
			return nil, agentsession.ErrForbidden
		}
		line := strings.TrimSuffix(string(packet), "\n")
		fields := strings.Fields(line)
		// receive-pack permits zero or more `shallow <oid>` declarations
		// before the command list. They describe clone history boundaries and
		// do not name a ref, so preserve them while still validating and
		// bounding their shape. A shallow declaration after a command, or one
		// carrying a capability suffix, fails closed. The shape test is the
		// exact single-space git wire form, NOT strings.Fields tolerance:
		// whitespace padding must not reach the per-packet maximum (2026-08
		// F3) — "shallow" + N spaces + oid is malformed here even though a
		// Fields-based two-field check would accept it.
		if commands == 0 && !strings.ContainsRune(line, 0) && strings.HasPrefix(line, "shallow ") {
			oid := line[len("shallow "):]
			if strings.ContainsAny(oid, " \t") {
				return nil, agentsession.ErrForbidden
			}
			shallows++
			if shallows > maxShallowBoundaries || !validObjectID(oid) {
				return nil, agentsession.ErrForbidden
			}
			continue
		}
		if nul := strings.IndexByte(line, 0); nul >= 0 {
			// Only the first update command may advertise capabilities.
			if commands != 0 {
				return nil, agentsession.ErrForbidden
			}
			line = line[:nul]
		}
		fields = strings.Fields(line)
		if len(fields) != 3 || fields[2] != "refs/heads/"+branch || !validObjectID(fields[0]) || !validObjectID(fields[1]) || allZero(fields[1]) {
			return nil, agentsession.ErrForbidden
		}
		commands++
		if commands > 8 {
			return nil, agentsession.ErrForbidden
		}
	}
	if commands == 0 {
		return nil, agentsession.ErrForbidden
	}
	return io.MultiReader(bytes.NewReader(prefix.Bytes()), reader), nil
}

func validObjectID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for _, r := range value {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}

func allZero(value string) bool { return strings.Trim(value, "0") == "" }

func (b *Broker) upstreamURL(repository, service, query string) (string, error) {
	origin := b.UpstreamOrigin
	if origin == "" {
		origin = "https://github.com"
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("invalid git upstream")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/" + repository + ".git/" + service
	parsed.RawQuery = query
	return parsed.String(), nil
}

// gitResponseHeaderTimeout bounds time-to-first-response-header for the
// upstream Git hop; the pack body itself is deliberately unbounded (see
// sshgateway.NewUpstreamClient).
const gitResponseHeaderTimeout = 30 * time.Second

func (b *Broker) httpClient() *http.Client {
	// An injected client gets the no-follow policy forced (sshgateway.NoFollow
	// carries the rationale).
	if b.HTTP != nil {
		return sshgateway.NoFollow(b.HTTP)
	}
	// Built once so the connection pool survives across requests: a per-request
	// client re-dials and re-handshakes TLS to the forge on every clone/push.
	b.clientOnce.Do(func() {
		b.client = sshgateway.NewUpstreamClient(gitResponseHeaderTimeout)
	})
	return b.client
}

func (b *Broker) now() time.Time {
	if b.Now != nil {
		return b.Now().UTC()
	}
	return time.Now().UTC()
}

// RemoteIP extracts the validated source IP from a request RemoteAddr. Shared
// with the model proxy (ADR062), which resolves the same source Pod.
func RemoteIP(remoteAddr string) (string, error) {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(strings.TrimSpace(host))
	if ip == nil {
		return "", agentsession.ErrForbidden
	}
	return ip.String(), nil
}
