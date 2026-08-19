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

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/crypto/ssh"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/agentsessions"
	"github.com/bex-co/bex/lego/backend/internal/api"
	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/authz"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/proxyproto"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/agentattach"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/agentcred"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/dbrole"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/modelproxy"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/nativessh"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/sandboxsse"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/webshell"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	dbURI := requiredEnv("BEX_CP_DB_URI")
	fgaURL := requiredEnv("BEX_OPENFGA_URL")
	hostKeyPath := requiredEnv("BEX_SSH_HOST_KEY_PATH")

	pool, err := pgxpool.New(ctx, dbURI)
	if err != nil {
		log.Fatalf("ssh gateway: database config: %v", err)
	}
	defer pool.Close()
	if err := waitForDB(ctx, pool); err != nil {
		log.Fatalf("ssh gateway: database unreachable: %v", err)
	}
	if err := dbrole.CheckRequiredPrivileges(ctx, pool); err != nil {
		log.Fatalf("ssh gateway: database privilege preflight: %v", err)
	}
	// The gateway is a least-privilege CONSUMER of the control-plane store, not
	// its owner (w7/m56, docs/ADR035-ssh.md §116). Migrating and ownership-
	// checking are DDL/authority operations reserved for bex-api, which runs on
	// the full-privilege app role; the gateway connects as bex_ssh_gateway — a
	// role scoped to key lookup + its own session/nonce/audit rows (see
	// scripts/ssh-gateway-db-role.sh) — which by design cannot run them. bex-api
	// has already migrated the schema by the time any tenant SSHes in. The
	// privilege preflight above verifies the DEPLOYED role against every current
	// method before listeners start, while deploy.yml applies dbrole.sql after the
	// bex-api rollout; a missing table/grant is therefore a visible readiness
	// failure rather than a user's first broken conversation stream.
	st := store.NewPGStore(pool)

	keyPEM, err := os.ReadFile(hostKeyPath)
	if err != nil {
		log.Fatalf("ssh gateway: read host key: %v", err)
	}
	signer, err := parseHostSigner(keyPEM)
	if err != nil {
		log.Fatalf("ssh gateway: parse host key: %v", err)
	}
	keyPEM = nil

	scheme := clientgoscheme.Scheme
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		log.Fatalf("ssh gateway: register App scheme: %v", err)
	}
	restConfig := ctrl.GetConfigOrDie()
	kubeClient, err := client.New(restConfig, client.Options{Scheme: scheme})
	if err != nil {
		log.Fatalf("ssh gateway: Kubernetes client: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		log.Fatalf("ssh gateway: Kubernetes exec client: %v", err)
	}

	base := &core.Base{
		Client: kubeClient, Namespace: envOr("BEX_API_NAMESPACE", "default"),
		Authz: authz.NewOpenFGAChecker(fgaURL, os.Getenv("BEX_OPENFGA_TOKEN")),
		Audit: st,
	}
	tenantSvc := api.NewTenantService(st, nil)
	base.Workspace = tenantSvc
	appService := &apps.Service{Base: base}
	// Compose the SSH target resolver (ADR054 D1): srv-… App instances stay on
	// apps.Service byte-identically; ags-… agent-session usernames route to the
	// sandbox resolver, which authorizes can_view_sensitive on the agent_session
	// object and derives the sandbox pod from the (SELECT-only) session row.
	agentResolver := &agentsessions.SSHResolver{Base: base, Store: st}
	sshResolver := sshgateway.CompositeResolver{Apps: appService, AgentSessions: agentResolver}

	registry := prometheus.NewRegistry()
	// The metrics, session limiter, and nonce guard are each constructed ONCE
	// and shared by every transport: the caps bound the whole process (not each
	// feature), and an exec-ticket nonce cannot be redeemed once per feature.
	metrics := sshgateway.NewMetrics(registry)
	limits := sshgateway.NewSessionLimiter(
		intEnv("BEX_SSH_MAX_SESSIONS", 100),
		intEnv("BEX_SSH_MAX_SESSIONS_PER_IDENTITY", 5),
	)
	// Exec-stream (channel) caps: the session limiter counts transports, but a
	// multiplexed agent-session connection holds many pods/exec streams (codex
	// round-8 #7). Native SSH acquires one slot per accepted session channel.
	channelLimits := sshgateway.NewChannelLimiter(
		intEnv("BEX_SSH_MAX_CHANNELS", 512),
		intEnv("BEX_SSH_MAX_CHANNELS_PER_IDENTITY", 32),
	)
	nonces := &sshgateway.NonceGuard{Store: st}
	executor := &sshgateway.KubeExecutor{Config: restConfig, Client: clientset}
	handshakeTimeout := durationEnv("BEX_SSH_HANDSHAKE_TIMEOUT", 10*time.Second)
	sessionTimeout := durationEnv("BEX_SSH_SESSION_TIMEOUT", 4*time.Hour)
	// codex round-9 #6: every established stream (native-SSH exec, web shell,
	// agent attach) re-runs its fresh authorization on this interval, so a
	// revocation ends LIVE streams too — not just the next admission. Negative
	// disables the watchdog (admission-only, the pre-round-9 behavior).
	revalidateInterval := durationEnv("BEX_SSH_REVALIDATE_INTERVAL", sshgateway.DefaultRevalidateInterval)
	trustedProxies, err := proxyproto.ParseTrustedProxyCIDRs(os.Getenv("BEX_SSH_PROXY_PROTOCOL_TRUSTED_CIDRS"))
	if err != nil {
		log.Fatalf("ssh gateway: invalid BEX_SSH_PROXY_PROTOCOL_TRUSTED_CIDRS: %v", err)
	}

	gateway := &nativessh.Server{
		Store: st, Apps: sshResolver,
		Executor:         executor,
		Signer:           signer,
		Metrics:          metrics,
		Limits:           limits,
		ChannelLimits:    channelLimits,
		HandshakeTimeout: handshakeTimeout,
		SessionTimeout:   sessionTimeout,
		// Bound concurrent pre-auth handshakes so an anonymous connection flood
		// can't exhaust the gateway before the post-handshake session limiter.
		MaxPreAuthConns: intEnv("BEX_SSH_MAX_PREAUTH_CONNS", 256),
		// …and bound how many of those slots ONE source may hold (round-11 #2):
		// silent connections from a single address park slots for the full
		// handshake deadline, so per-source fairness keeps everyone else's
		// handshakes admissible. Negative disables (global-only, pre-round-11).
		MaxPreAuthConnsPerSource: intEnvAllowNegative("BEX_SSH_MAX_PREAUTH_CONNS_PER_SOURCE", 32),
		// Zed remoting into an agent-session sandbox multiplexes many session
		// channels over one connection (ADR054 D3). This per-connection channel cap
		// bounds that fan-out for sandbox targets only; 0 disables the exception and
		// restores single-channel everywhere. srv-… App targets are always single.
		MaxChannelsPerConn: intEnvAllowZero("BEX_SSH_MAX_CHANNELS_PER_CONN", 16),
		// Live-stream revalidation cadence for every accepted channel (round-9 #6).
		RevalidateInterval: revalidateInterval,
		// Traefik's pod network, so ssh_sessions.remote_address records the real
		// client instead of Traefik's own pod IP (w4/029.md #10); empty (the
		// default) leaves every connection's RemoteAddr as the immediate peer,
		// unchanged.
		TrustedProxies: trustedProxies,
	}
	shell := &webshell.Server{
		TicketSecret:     []byte(os.Getenv("BEX_SHELL_TICKET_SECRET")),
		Store:            st,
		Apps:             appService,
		Executor:         executor,
		Metrics:          metrics,
		Limits:           limits,
		Nonces:           nonces,
		HandshakeTimeout: handshakeTimeout,
		SessionTimeout:   sessionTimeout,
		// Live-shell revalidation cadence for the browser bridge (round-9 #6).
		RevalidateInterval: revalidateInterval,
	}
	sandbox := &sandboxsse.Server{
		Secret:   []byte(os.Getenv("BEX_SANDBOX_EXEC_SECRET")),
		Executor: executor,
		Metrics:  metrics,
		Limits:   limits,
		Nonces:   nonces,
		// Redemption-time reauthorization + live-stream revalidation for caller
		// exec tickets (round-13 #3): fresh can_operate on the ticket's workspace
		// and, for agent-session sandboxes, can_view_sensitive on the session
		// object (round-13 #1, defense in depth behind bex-api's mint gate).
		Revalidator:        &sandboxsse.ExecRevalidator{Base: base},
		SessionTimeout:     sessionTimeout,
		RevalidateInterval: revalidateInterval,
	}
	credentials := &agentcred.Broker{Metrics: metrics}
	// The ADR062 model proxy shares the Git proxy's trust model: the same
	// gateway-only HMAC secret, the same source-pod resolver, an internal-only
	// listener. It keeps the BYO model key on the gateway→vendor hop.
	model := &modelproxy.Broker{Metrics: metrics}
	if secret := os.Getenv("BEX_SANDBOX_EXEC_SECRET"); secret != "" {
		apiURL := envOr("BEX_AGENT_CREDENTIAL_API_URL", "http://bex-api.bex-system.svc:8091"+agentsession.InternalMintPath)
		credentials.Pods = agentcred.KubeSessionPodResolver{Client: clientset}
		credentials.API = &agentsession.Client{URL: apiURL, Secret: []byte(secret)}
		credentials.Audit = st
		// codex round-9 #4: bound concurrent git-proxy exchanges per source Pod
		// and process-wide, acquired before the Pod lookup and credential mint.
		credentials.Limits = sshgateway.NewSessionLimiter(
			intEnv("BEX_AGENT_GIT_MAX_CONNS", 64),
			intEnv("BEX_AGENT_GIT_MAX_CONNS_PER_POD", 4),
		)

		modelAPIURL := envOr("BEX_AGENT_MODEL_CREDENTIAL_API_URL", "http://bex-api.bex-system.svc:8091"+agentsession.InternalModelMintPath)
		model.Pods = agentcred.KubeSessionPodResolver{Client: clientset}
		model.API = &agentsession.ModelClient{URL: modelAPIURL, Secret: []byte(secret)}
		model.Limits = sshgateway.NewSessionLimiter(
			intEnv("BEX_AGENT_MODEL_MAX_CONNS", 32),
			intEnv("BEX_AGENT_MODEL_MAX_CONNS_PER_POD", 2),
		)
		model.MaxDuration = durationEnv("BEX_AGENT_MODEL_MAX_DURATION", 2*time.Hour)
		// Cumulative exchange budgets (round-13 #8): every per-exchange bound
		// resets on completion, so without these a live sandbox's tenant code
		// could loop billable inference for the session's lifetime. 0 disables a
		// dimension.
		model.MaxRequestsPerSession = intEnv("BEX_AGENT_MODEL_MAX_REQUESTS_PER_SESSION", 1000)
		model.MaxRequestsPerWorkspace = intEnv("BEX_AGENT_MODEL_MAX_REQUESTS_PER_WORKSPACE", 5000)
	}
	// Agent-session conversation transport (ADR047 D9). Shares the web-shell
	// ticket secret (BEX_SHELL_TICKET_SECRET): bex-api mints agent-session
	// tickets under the same key. The gateway dials the in-sandbox driver's
	// stream port directly (the sandbox NetworkPolicy admits only gateway
	// ingress) and tees the UI-message stream into the durable transcript.
	attach := &agentattach.Server{
		Secret: []byte(os.Getenv("BEX_SHELL_TICKET_SECRET")),
		Store:  st,
		Pods:   agentattach.KubePodIPResolver{Client: clientset},
		// Redemption-time re-check (codex-security round-6 #11): the verified
		// ticket froze bex-api's mint-time decision; re-run the same relation
		// (fresh) + turn-phase gate here so revocation/cancellation inside the
		// ticket's TTL window is effective at the gateway too.
		Revalidator:    &agentsessions.AttachRevalidator{Base: base, Store: st},
		DriverPort:     intEnv("BEX_AGENT_SESSION_DRIVER_PORT", 8787),
		AllowedOrigins: splitCSV(os.Getenv("BEX_API_CORS_ORIGIN")),
		Metrics:        metrics,
		Limits:         limits,
		Nonces:         nonces,
		SessionTimeout: sessionTimeout,
		// Live-stream revalidation cadence for read/turn streams (round-9 #6).
		RevalidateInterval: revalidateInterval,
	}
	addr := envOr("BEX_SSH_ADDR", ":2222")
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("ssh gateway: listen %s: %v", addr, err)
	}
	metricsServer := &http.Server{Handler: metricsHandler(registry), ReadHeaderTimeout: 5 * time.Second}
	metricsAddr := envOr("BEX_SSH_METRICS_ADDR", ":9090")
	metricsListener, err := net.Listen("tcp", metricsAddr)
	if err != nil {
		log.Fatalf("ssh gateway: metrics listen %s: %v", metricsAddr, err)
	}
	metricsErr := make(chan error, 1)
	go func() {
		err := metricsServer.Serve(metricsListener)
		metricsErr <- err
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			stop()
		}
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = metricsServer.Shutdown(shutdownCtx)
	}()

	// Browser Web Shell WebSocket listener (docs/ADR035-ssh.md § Browser Web
	// Shell). Started only when BEX_SHELL_TICKET_SECRET is set; Traefik
	// terminates TLS onto this plain-HTTP listener. The handler shares the
	// gateway's session caps and content-free audit with native SSH.
	if shell.Enabled() {
		shellMux := http.NewServeMux()
		shellMux.Handle("GET /shell", shell.Handler())
		defer startAuxListener("web shell", "web shell", envOr("BEX_SHELL_WS_ADDR", ":8080"), shellMux, stop, metrics)()
	}

	// Sandbox-exec SSE listener (w3/m33, `render ea sandbox exec`). Internal-only
	// (bex-api → gateway; the exec-secret HMAC is the trust) — NOT browser-facing,
	// so it is a distinct listener from the Web Shell. Started only when
	// BEX_SANDBOX_EXEC_SECRET is set. pods/exec stays confined here.
	if sandbox.Enabled() {
		execMux := http.NewServeMux()
		execMux.Handle("POST /sandbox-exec", sandbox.Handler())
		defer startAuxListener("sandbox-exec", "sandbox exec", envOr("BEX_SANDBOX_EXEC_ADDR", ":8081"), execMux, stop, metrics)()
	}

	// Agent-session Git smart-HTTP proxy (ADR047 D2). Sandboxes can clone/push
	// through this listener but never receive the GitHub installation token. The
	// gateway verifies the direct source Pod and confines receive-pack to the
	// session's exact branch before injecting credentials on the upstream hop.
	// The listener additionally carries a whole-request read deadline (codex
	// round-9 #4): the pkt-line validator's blocking reads may otherwise be
	// dripped indefinitely by a hostile Pod, and a per-request body budget
	// without a duration budget still pins a goroutine per drip. Ten minutes is
	// orders of magnitude past any in-cluster pack exchange; response streaming
	// (the clone direction) is a write and stays unbounded.
	if credentials.Enabled() {
		credentialMux := http.NewServeMux()
		credentialMux.Handle("GET "+agentsession.GitProxyPath, credentials.Handler())
		credentialMux.Handle("POST "+agentsession.GitProxyPath, credentials.Handler())
		defer startAuxListener("agent git proxy", "agent git proxy", envOr("BEX_AGENT_CREDENTIAL_ADDR", ":8082"), credentialMux, stop, metrics,
			func(server *http.Server) {
				server.ReadTimeout = durationEnv("BEX_AGENT_GIT_READ_TIMEOUT", 10*time.Minute)
				server.IdleTimeout = 2 * time.Minute
			})()
	}
	// Agent-session model proxy (ADR062). Internal-only (sandbox → gateway → vendor
	// on :8084 by default); the sandbox's agent base URL points here so the BYO
	// model key is injected on the upstream hop and never enters the sandbox.
	// Started only when BEX_SANDBOX_EXEC_SECRET is set (same gate as the Git proxy).
	if model.Enabled() {
		modelMux := http.NewServeMux()
		modelMux.Handle("POST "+agentsession.ModelProxyPath, model.Handler())
		defer startAuxListener("agent model proxy", "agent model proxy", envOr("BEX_AGENT_MODEL_PROXY_ADDR", ":8084"), modelMux, stop, metrics,
			func(server *http.Server) {
				server.ReadTimeout = durationEnv("BEX_AGENT_MODEL_READ_TIMEOUT", 2*time.Minute)
				server.IdleTimeout = 2 * time.Minute
			})()
	}
	// Agent-session conversation listener (ADR047 D9, w3/m43). Browser-facing via
	// the platform edge, which path-routes api.bex.co/v1/agent-sessions/{id}/stream
	// to this listener (t006). Started only when BEX_SHELL_TICKET_SECRET is set.
	// Both GET (attach: replay + live) and POST (live prompt turn) mount here.
	if attach.Enabled() {
		attachMux := http.NewServeMux()
		attachMux.Handle("GET /v1/agent-sessions/{id}/stream", attach.Handler())
		attachMux.Handle("POST /v1/agent-sessions/{id}/stream", attach.Handler())
		// OPTIONS reaches the same handler so the CORS preflight is answered (the
		// cross-origin dashboard sends one before the ticketed GET/POST).
		attachMux.Handle("OPTIONS /v1/agent-sessions/{id}/stream", attach.Handler())
		defer startAuxListener("agent attach", "agent session attach", envOr("BEX_AGENT_ATTACH_ADDR", ":8083"), attachMux, stop, metrics)()
	}
	log.Printf("ssh gateway listening on %s", addr)
	if err := gateway.Serve(ctx, listener); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("ssh gateway: %v", err)
	}
	select {
	case err := <-metricsErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("ssh gateway: metrics: %v", err)
		}
	default:
	}
}

// startAuxListener runs one auxiliary HTTP listener (health-checked mux, fatal
// on bind failure, stop() on serve failure) and returns the shutdown closure
// the caller defers, keeping the process-exit LIFO order at the call sites.
// Each opt may still shape the server (e.g. a request-read deadline for a
// body-carrying proxy listener); the defaults suit streaming listeners.
func startAuxListener(name, logName, addr string, mux *http.ServeMux, stop context.CancelFunc, metrics *sshgateway.Metrics, opts ...func(*http.Server)) func() {
	mux.HandleFunc("GET /healthz", healthzOK)
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	for _, opt := range opts {
		opt(server)
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("ssh gateway: %s listen %s: %v", name, addr, err)
	}
	// Browser-facing listeners sit behind one edge peer and internal exec sits
	// behind bex-api, so their immediate-source cap equals the global cap. The
	// sandbox-direct proxies override this with a real per-Pod admission budget.
	global, perSource := 128, 128
	switch name {
	case "agent git proxy":
		global = intEnv("BEX_AGENT_GIT_MAX_PREAUTH_CONNS", global)
		perSource = intEnv("BEX_AGENT_GIT_MAX_PREAUTH_CONNS_PER_SOURCE", perSource)
	case "agent model proxy":
		global = intEnv("BEX_AGENT_MODEL_MAX_PREAUTH_CONNS", global)
		perSource = intEnv("BEX_AGENT_MODEL_MAX_PREAUTH_CONNS_PER_SOURCE", perSource)
	}
	listener = newAdmissionListener(listener, global, perSource, metrics)
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			stop()
		}
	}()
	log.Printf("%s listening on %s", logName, addr)
	return func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}
}

type admissionListener struct {
	net.Listener
	mu        sync.Mutex
	active    int
	bySource  map[string]int
	max       int
	perSource int
	metrics   *sshgateway.Metrics
}

func newAdmissionListener(inner net.Listener, max, perSource int, metrics *sshgateway.Metrics) net.Listener {
	return &admissionListener{
		Listener: inner, bySource: make(map[string]int), max: max, perSource: perSource,
		metrics: metrics,
	}
}

func (l *admissionListener) Accept() (net.Conn, error) {
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		source := connectionSource(conn.RemoteAddr())
		l.mu.Lock()
		shedScope := ""
		switch {
		case l.active >= l.max:
			shedScope = "listener_global"
		case l.bySource[source] >= l.perSource:
			shedScope = "listener_source"
		default:
			l.active++
			l.bySource[source]++
		}
		l.mu.Unlock()
		if shedScope == "" {
			return &admittedConn{Conn: conn, release: func() { l.release(source) }}, nil
		}
		// A shed on an aux listener is observable like every other shed path in
		// the fleet (w1/m76/t004): the metric is recorded before the close so a
		// client observing EOF can never race ahead of the counter.
		l.metrics.LimitRejected(shedScope)
		_ = conn.Close()
	}
}

func (l *admissionListener) release(source string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.active--
	l.bySource[source]--
	if l.bySource[source] == 0 {
		delete(l.bySource, source)
	}
}

type admittedConn struct {
	net.Conn
	once    sync.Once
	release func()
}

func (c *admittedConn) Close() error {
	err := c.Conn.Close()
	c.once.Do(c.release)
	return err
}

func connectionSource(addr net.Addr) string {
	host, _, err := net.SplitHostPort(addr.String())
	if err == nil {
		return host
	}
	return addr.String()
}

func healthzOK(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }

func metricsHandler(gatherer prometheus.Gatherer) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{}))
	mux.HandleFunc("GET /healthz", healthzOK)
	return mux
}

func parseHostSigner(keyPEM []byte) (ssh.Signer, error) {
	signer, err := ssh.ParsePrivateKey(keyPEM)
	if err != nil {
		return nil, err
	}
	if signer.PublicKey().Type() != ssh.KeyAlgoED25519 {
		return nil, fmt.Errorf("host key must be Ed25519")
	}
	return signer, nil
}

func requiredEnv(name string) string {
	value := os.Getenv(name)
	if value == "" {
		log.Fatalf("ssh gateway: %s must be set", name)
	}
	return value
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func durationEnv(name string, fallback time.Duration) time.Duration {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		log.Fatalf("ssh gateway: %s must be a positive duration", name)
	}
	return duration
}

// splitCSV parses a comma-separated env value (e.g. BEX_API_CORS_ORIGIN) into a
// trimmed, non-empty list. Empty input => nil (no CORS origins).
func splitCSV(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func intEnv(name string, fallback int) int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		log.Fatalf("ssh gateway: %s must be a positive integer", name)
	}
	return n
}

// intEnvAllowZero is intEnv for a knob whose 0 is a meaningful "disable", not an
// error (BEX_SSH_MAX_CHANNELS_PER_CONN=0 restores single-channel).
func intEnvAllowZero(name string, fallback int) int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		log.Fatalf("ssh gateway: %s must be a non-negative integer", name)
	}
	return n
}

// intEnvAllowNegative is intEnv for a knob whose negative value is a meaningful
// "disable" (BEX_SSH_MAX_PREAUTH_CONNS_PER_SOURCE=-1 restores global-only
// pre-auth admission).
func intEnvAllowNegative(name string, fallback int) int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		log.Fatalf("ssh gateway: %s must be an integer", name)
	}
	return n
}

func waitForDB(ctx context.Context, pool *pgxpool.Pool) error {
	var last error
	for attempt := 0; attempt < 30; attempt++ {
		if err := pool.Ping(ctx); err == nil {
			return nil
		} else {
			last = err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return fmt.Errorf("after 30 attempts: %w", last)
}
