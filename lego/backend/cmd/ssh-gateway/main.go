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
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/agentattach"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/agentcred"
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
	// The gateway is a least-privilege CONSUMER of the control-plane store, not
	// its owner (w7/m56, docs/ADR035-ssh.md §116). Migrating and ownership-
	// checking are DDL/authority operations reserved for bex-api, which runs on
	// the full-privilege app role; the gateway connects as bex_ssh_gateway — a
	// role scoped to key lookup + its own session/nonce/audit rows (see
	// scripts/ssh-gateway-db-role.sh) — which by design cannot run them. bex-api
	// has already migrated the schema by the time any tenant SSHes in; if a table
	// is missing the gateway's first query fails and it restarts until bex-api
	// converges, the standard startup-dependency behavior.
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
	nonces := &sshgateway.NonceGuard{Store: st}
	executor := &sshgateway.KubeExecutor{Config: restConfig, Client: clientset}
	handshakeTimeout := durationEnv("BEX_SSH_HANDSHAKE_TIMEOUT", 10*time.Second)
	sessionTimeout := durationEnv("BEX_SSH_SESSION_TIMEOUT", 4*time.Hour)

	gateway := &nativessh.Server{
		Store: st, Apps: sshResolver,
		Executor:         executor,
		Signer:           signer,
		Metrics:          metrics,
		Limits:           limits,
		HandshakeTimeout: handshakeTimeout,
		SessionTimeout:   sessionTimeout,
		// Bound concurrent pre-auth handshakes so an anonymous connection flood
		// can't exhaust the gateway before the post-handshake session limiter.
		MaxPreAuthConns: intEnv("BEX_SSH_MAX_PREAUTH_CONNS", 256),
		// Zed remoting into an agent-session sandbox multiplexes many session
		// channels over one connection (ADR054 D3). This per-connection channel cap
		// bounds that fan-out for sandbox targets only; 0 disables the exception and
		// restores single-channel everywhere. srv-… App targets are always single.
		MaxChannelsPerConn: intEnvAllowZero("BEX_SSH_MAX_CHANNELS_PER_CONN", 16),
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
	}
	sandbox := &sandboxsse.Server{
		Secret:         []byte(os.Getenv("BEX_SANDBOX_EXEC_SECRET")),
		Executor:       executor,
		Metrics:        metrics,
		Limits:         limits,
		Nonces:         nonces,
		SessionTimeout: sessionTimeout,
	}
	credentials := &agentcred.Broker{Metrics: metrics}
	if secret := os.Getenv("BEX_SANDBOX_EXEC_SECRET"); secret != "" {
		apiURL := envOr("BEX_AGENT_CREDENTIAL_API_URL", "http://bex-api.bex-system.svc:8091"+agentsession.InternalMintPath)
		credentials.Pods = agentcred.KubeSessionPodResolver{Client: clientset}
		credentials.API = &agentsession.Client{URL: apiURL, Secret: []byte(secret)}
		credentials.Audit = st
	}
	// Agent-session conversation transport (ADR047 D9). Shares the web-shell
	// ticket secret (BEX_SHELL_TICKET_SECRET): bex-api mints agent-session
	// tickets under the same key. The gateway dials the in-sandbox driver's
	// stream port directly (the sandbox NetworkPolicy admits only gateway
	// ingress) and tees the UI-message stream into the durable transcript.
	attach := &agentattach.Server{
		Secret:         []byte(os.Getenv("BEX_SHELL_TICKET_SECRET")),
		Store:          st,
		Pods:           agentattach.KubePodIPResolver{Client: clientset},
		DriverPort:     intEnv("BEX_AGENT_SESSION_DRIVER_PORT", 8787),
		AllowedOrigins: splitCSV(os.Getenv("BEX_API_CORS_ORIGIN")),
		Metrics:        metrics,
		Limits:         limits,
		Nonces:         nonces,
		SessionTimeout: sessionTimeout,
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
		shellMux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
		shellServer := &http.Server{Handler: shellMux, ReadHeaderTimeout: 5 * time.Second}
		shellAddr := envOr("BEX_SHELL_WS_ADDR", ":8080")
		shellListener, err := net.Listen("tcp", shellAddr)
		if err != nil {
			log.Fatalf("ssh gateway: web shell listen %s: %v", shellAddr, err)
		}
		go func() {
			if err := shellServer.Serve(shellListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
				stop()
			}
		}()
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = shellServer.Shutdown(shutdownCtx)
		}()
		log.Printf("web shell listening on %s", shellAddr)
	}

	// Sandbox-exec SSE listener (w3/m33, `render ea sandbox exec`). Internal-only
	// (bex-api → gateway; the exec-secret HMAC is the trust) — NOT browser-facing,
	// so it is a distinct listener from the Web Shell. Started only when
	// BEX_SANDBOX_EXEC_SECRET is set. pods/exec stays confined here.
	if sandbox.Enabled() {
		execMux := http.NewServeMux()
		execMux.Handle("POST /sandbox-exec", sandbox.Handler())
		execMux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
		execServer := &http.Server{Handler: execMux, ReadHeaderTimeout: 5 * time.Second}
		execAddr := envOr("BEX_SANDBOX_EXEC_ADDR", ":8081")
		execListener, err := net.Listen("tcp", execAddr)
		if err != nil {
			log.Fatalf("ssh gateway: sandbox-exec listen %s: %v", execAddr, err)
		}
		go func() {
			if err := execServer.Serve(execListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
				stop()
			}
		}()
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = execServer.Shutdown(shutdownCtx)
		}()
		log.Printf("sandbox exec listening on %s", execAddr)
	}

	// Agent-session credential listener (ADR047 D2). Sandboxes call this one
	// directly; Cilium admits only sandbox-regime Pods, then the handler resolves
	// the TCP source IP back to a Pod and checks its immutable session bindings.
	// It has no Ingress and is distinct from browser/SSH/sandbox-exec listeners.
	if credentials.Enabled() {
		credentialMux := http.NewServeMux()
		credentialMux.Handle("POST "+agentsession.GatewayPath, credentials.Handler())
		credentialMux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
		credentialServer := &http.Server{Handler: credentialMux, ReadHeaderTimeout: 5 * time.Second}
		credentialAddr := envOr("BEX_AGENT_CREDENTIAL_ADDR", ":8082")
		credentialListener, err := net.Listen("tcp", credentialAddr)
		if err != nil {
			log.Fatalf("ssh gateway: agent credential listen %s: %v", credentialAddr, err)
		}
		go func() {
			if err := credentialServer.Serve(credentialListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
				stop()
			}
		}()
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = credentialServer.Shutdown(shutdownCtx)
		}()
		log.Printf("agent credential broker listening on %s", credentialAddr)
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
		attachMux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
		attachServer := &http.Server{Handler: attachMux, ReadHeaderTimeout: 5 * time.Second}
		attachAddr := envOr("BEX_AGENT_ATTACH_ADDR", ":8083")
		attachListener, err := net.Listen("tcp", attachAddr)
		if err != nil {
			log.Fatalf("ssh gateway: agent attach listen %s: %v", attachAddr, err)
		}
		go func() {
			if err := attachServer.Serve(attachListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
				stop()
			}
		}()
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = attachServer.Shutdown(shutdownCtx)
		}()
		log.Printf("agent session attach listening on %s", attachAddr)
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

func metricsHandler(gatherer prometheus.Gatherer) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{}))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
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
