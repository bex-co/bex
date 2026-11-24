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
	"net/netip"
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
	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/authz"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/proxyproto"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/sandboxfiles"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/agentattach"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/agentcred"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/dbrole"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/egressproxy"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/modelproxy"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/nativessh"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/sandboxbroker"
	sandboxfilegateway "github.com/bex-co/bex/lego/backend/internal/sshgateway/sandboxfiles"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/sandboxsse"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway/webshell"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/tenancy"
	"github.com/bex-co/bex/lego/types/env"
	"github.com/bex-co/bex/lego/types/netutil"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// newTwoFactorGate builds the gateway's workspace two-factor gate (docs/ADR012-auth.md
// §9a) over bex-api's internal verb, authenticated with the same BEX_SANDBOX_EXEC_SECRET HMAC as the
// email-verification verb. Without the secret there is no way to ask bex-api
// whether a workspace requires 2FA, so the gate is left unable to ask and every
// authorization it guards fails closed, as native SSH's email check does.
func newTwoFactorGate(secret, apiURL string, sessions sshgateway.TwoFactorSessions, metrics *sshgateway.Metrics) *sshgateway.TwoFactorGate {
	gate := &sshgateway.TwoFactorGate{Sessions: sessions, Metrics: metrics}
	if secret == "" {
		log.Printf("ssh gateway: BEX_SANDBOX_EXEC_SECRET is unset, so workspace two-factor requirements cannot be checked; every native SSH key and ticket redemption is refused")
		return gate
	}
	gate.Asker = &agentsession.TwoFactorClient{URL: apiURL, Secret: []byte(secret), HTTP: &http.Client{Timeout: 5 * time.Second}}
	return gate
}

// newSessionWaker builds the native SSH gateway's wake of a closed agent
// session (w10/m597) over bex-api's internal verb, under the same
// BEX_SANDBOX_EXEC_SECRET HMAC as the gateway's other verbs. Without the
// secret there is none: a login into a closed session gets the banner.
func newSessionWaker(secret, apiURL string) nativessh.SessionWaker {
	if secret == "" {
		log.Printf("ssh gateway: BEX_SANDBOX_EXEC_SECRET is unset, so an SSH login into a closed agent session cannot start its sandbox")
		return nil
	}
	return &agentsession.SessionWakeClient{URL: apiURL, Secret: []byte(secret), HTTP: &http.Client{Timeout: 5 * time.Second}}
}

// newKeyGrantGate builds the gateway's hold on a bex API key's grant (w9/125)
// over bex-api's internal verb, authenticated with the same
// BEX_SANDBOX_EXEC_SECRET HMAC as the two-factor verb. Without the secret the
// gate cannot ask, so every stream a key opened is refused, and so is every
// native SSH key, whose owner it cannot tell from a key.
func newKeyGrantGate(secret, apiURL string) *sshgateway.KeyGrantGate {
	gate := &sshgateway.KeyGrantGate{}
	if secret == "" {
		log.Printf("ssh gateway: BEX_SANDBOX_EXEC_SECRET is unset, so API key grants cannot be checked; every stream an API key opens is refused")
		return gate
	}
	gate.Asker = &agentsession.KeyGrantClient{URL: apiURL, Secret: []byte(secret), HTTP: &http.Client{Timeout: 5 * time.Second}}
	return gate
}

func main() {
	// Three lifetimes (w6/m482, w6/152, docs/ADR035-ssh.md § Sessions across a
	// gateway release). readyCtx is readiness': SIGTERM ends it at once, so the
	// pod leaves every route. ctx is the listeners': it ends BEX_SSH_ACCEPT_WINDOW
	// later, so a connection the edge still routes here meanwhile is served.
	// sessionCtx is the open sessions': they keep running until they end on
	// their own or the drain bound passes, and a second signal ends them at once.
	ctx, stopListening := context.WithCancel(context.Background())
	defer stopListening()
	readyCtx, stopReady := context.WithCancel(ctx)
	defer stopReady()
	sessionCtx, endSessions := context.WithCancelCause(context.Background())
	defer endSessions(nil)
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	go drainOnSignal(signals, durationEnv("BEX_SSH_ACCEPT_WINDOW", 10*time.Second), stopReady, stopListening, endSessions)
	// A listener that fails ends the process at once, as before: a pod that
	// stops accepting while Kubernetes still routes to it must not linger.
	stop := func() {
		stopListening()
		endSessions(errListenerFailed)
	}
	shutdownTimeout := durationEnv("BEX_SSH_SHUTDOWN_TIMEOUT", time.Hour)

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
		Client: kubeClient, Namespace: env.Or("BEX_API_NAMESPACE", "default"),
		Authz: authz.NewOpenFGAChecker(fgaURL, os.Getenv("BEX_OPENFGA_TOKEN")),
		Audit: st,
	}
	tenantSvc := tenancy.NewTenantService(st, nil)
	base.Workspace = tenantSvc
	appService := &apps.Service{Base: base}
	// Compose the SSH target resolver (ADR054 D1): srv-… App instances stay on
	// apps.Service byte-identically; ags-… agent-session usernames route to the
	// sandbox resolver, which authorizes can_view_sensitive on the agent_session
	// object and derives the sandbox pod from the (SELECT-only) session row.
	agentResolver := &agentsessions.SSHResolver{Base: base, Store: st}
	sshResolver := sshgateway.CompositeResolver{Apps: appService, AgentSessions: agentResolver}

	registry := prometheus.NewRegistry()
	// The metrics, the limiters, and the nonce guard are each constructed ONCE
	// and shared by the transports: the caps bound the whole process (not each
	// feature), and an exec-ticket nonce cannot be redeemed once per feature.
	metrics := sshgateway.NewMetrics(registry)
	// The gateway's own Base carries the gate, so the one funnel hook covers
	// native SSH (admission, every channel, every revalidation tick), the web
	// shell, agent attach and sandbox exec/files, whose tickets bex-api mints
	// through its own gated Base.
	base.TwoFactor = newTwoFactorGate(os.Getenv("BEX_SANDBOX_EXEC_SECRET"),
		env.Or("BEX_WORKSPACE_TWO_FACTOR_API_URL", "http://bex-api.bex-system.svc:8091"+agentsession.InternalTwoFactorPath), st, metrics)
	// One hold on API key grants for every transport: a stream a key opened
	// is re-asked at redemption and on every revalidation tick (w9/125).
	keyGrants := newKeyGrantGate(os.Getenv("BEX_SANDBOX_EXEC_SECRET"),
		env.Or("BEX_API_KEY_GRANT_API_URL", "http://bex-api.bex-system.svc:8091"+agentsession.InternalKeyGrantPath))
	limits, sandboxLimits := gatewayLimits()
	// Exec-stream (channel) caps: the session limiter counts transports, but a
	// multiplexed agent-session connection holds many pods/exec streams (codex
	// round-8 #7). Native SSH acquires one slot per accepted session channel.
	channelLimits := sshgateway.NewChannelLimiter(
		intEnv("BEX_SSH_MAX_CHANNELS", 512),
		intEnv("BEX_SSH_MAX_CHANNELS_PER_IDENTITY", 32),
	)
	nonces := &sshgateway.NonceGuard{Store: st}
	// Exec output is read from kube-apiserver at full speed into a bounded
	// buffer, so a slow SSH client no longer loses its tail (w6/099).
	executor := sshgateway.DrainingExecutor{
		Executor: &sshgateway.KubeExecutor{Config: restConfig, Client: clientset},
		Budget: sshgateway.NewDrainBudget(
			intEnv("BEX_SSH_DRAIN_BUFFER_PER_CHANNEL", 64<<20),
			intEnv("BEX_SSH_DRAIN_BUFFER_TOTAL", 128<<20),
		),
	}
	if executor.Budget != nil {
		executor.Budget.Metrics = metrics
	}
	handshakeTimeout := durationEnv("BEX_SSH_HANDSHAKE_TIMEOUT", 10*time.Second)
	// The cap on an agent-session connection (ADR054 D6), an agent attach
	// stream and a sandbox exec, and on a service connection that has not
	// opened its shell yet. A service shell, native SSH or the Web Shell, has
	// no time limit, as on Render (w6/174, ADR035 § How long a session lasts).
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
		// Each service shell session's StartShellEvent/EndShellEvent rows
		// (w9/m451), through the role's INSERT-only audit_events grant.
		Audit:     st,
		KeyGrants: keyGrants,
		// An SSH login into an agent session whose sandbox closed starts it
		// through bex-api as the key's owner and waits for it (w10/m597,
		// ADR054 D9); unwired without the secret, so such a login keeps
		// w10/043's banner.
		Wakes: newSessionWaker(os.Getenv("BEX_SANDBOX_EXEC_SECRET"), env.Or("BEX_AGENT_SESSION_WAKE_API_URL", "http://bex-api.bex-system.svc:8091"+agentsession.InternalSessionWakePath)),
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
		// Live-shell revalidation cadence for the browser bridge (round-9 #6).
		RevalidateInterval: revalidateInterval,
		// The WebSocket listener is HTTP behind the same Traefik pod network as
		// native SSH. Trust forwarding headers only from those configured peers.
		TrustedProxies: core.TrustedProxies(trustedProxies),
		Audit:          st,
		KeyGrants:      keyGrants,
	}
	sandbox := &sandboxsse.Server{
		Secret:   []byte(os.Getenv("BEX_SANDBOX_EXEC_SECRET")),
		Executor: executor,
		Metrics:  metrics,
		Limits:   sandboxLimits,
		Nonces:   nonces,
		// Redemption-time reauthorization + live-stream revalidation for caller
		// exec tickets (round-13 #3): fresh can_operate on the ticket's workspace
		// and, for agent-session sandboxes, can_view_sensitive on the session
		// object (round-13 #1, defense in depth behind bex-api's mint gate).
		Revalidator:        &sshgateway.ExecRevalidator{Base: base, KeyGrants: keyGrants},
		SessionTimeout:     sessionTimeout,
		RevalidateInterval: revalidateInterval,
	}
	// w2/m168 (ADR075 D8 revision): a native SSH key owner must have a verified
	// email, because key auth never passes bex-api's middleware. On unless
	// BEX_REQUIRE_VERIFIED_EMAIL=0. Without the exec secret the question can't
	// be asked, so every key is refused rather than admitted.
	if v := os.Getenv("BEX_REQUIRE_VERIFIED_EMAIL"); v != "0" && !strings.EqualFold(v, "false") {
		if secret := os.Getenv("BEX_SANDBOX_EXEC_SECRET"); secret != "" {
			gateway.EmailVerified = (&agentsession.EmailVerificationClient{
				URL:    env.Or("BEX_IDENTITY_VERIFICATION_API_URL", "http://bex-api.bex-system.svc:8091"+agentsession.InternalEmailVerificationPath),
				Secret: []byte(secret),
				HTTP:   &http.Client{Timeout: 5 * time.Second},
			}).Check
		} else {
			log.Printf("ssh gateway: BEX_REQUIRE_VERIFIED_EMAIL is on but BEX_SANDBOX_EXEC_SECRET is unset; every native SSH key is refused")
			gateway.EmailVerified = func(context.Context, string) error { return errors.New("email verification unavailable") }
		}
	}
	credentials := &agentcred.Broker{Metrics: metrics}
	// The ADR062 model proxy shares the Git proxy's trust model: the same
	// gateway-only HMAC secret, the same source-pod resolver, an internal-only
	// listener. It keeps the BYO model key on the gateway→vendor hop.
	model := &modelproxy.Broker{Metrics: metrics}
	// w10/m19: the egress proxy for full-tier sandboxes. Same trust model
	// again (source-Pod binding, gateway-only HMAC to bex-api for the tier
	// grant); it reaches the public internet on the allowed ports only and
	// refuses the model providers and private ranges itself.
	egress := &egressproxy.Broker{Metrics: metrics}
	if secret := os.Getenv("BEX_SANDBOX_EXEC_SECRET"); secret != "" {
		apiURL := env.Or("BEX_AGENT_CREDENTIAL_API_URL", "http://bex-api.bex-system.svc:8091"+agentsession.InternalMintPath)
		credentials.Pods = sshgateway.KubeSessionPodResolver{Client: clientset}
		credentials.API = &agentsession.Client{URL: apiURL, Secret: []byte(secret)}
		credentials.Audit = st
		// codex round-9 #4: bound concurrent git-proxy exchanges per source Pod
		// and process-wide, acquired before the Pod lookup and credential mint.
		credentials.Limits = sshgateway.NewSessionLimiter(
			intEnv("BEX_AGENT_GIT_MAX_CONNS", 64),
			intEnv("BEX_AGENT_GIT_MAX_CONNS_PER_POD", 4),
		)
		credentials.MaxDuration = durationEnv("BEX_AGENT_GIT_MAX_DURATION", 10*time.Minute)
		credentials.MaxRequestsPerSession = intEnv("BEX_AGENT_GIT_MAX_REQUESTS_PER_SESSION", 1000)
		credentials.MaxRequestsPerWorkspace = intEnv("BEX_AGENT_GIT_MAX_REQUESTS_PER_WORKSPACE", 5000)
		credentials.MaxResponseBytes = int64(intEnv("BEX_AGENT_GIT_MAX_RESPONSE_BYTES", 4<<30))
		// w10/m513: the read-only GitHub API route `gh` uses, under the
		// `github-api` tail of the same URL. bex-api mints a read-only token
		// of the session's repository per call; the route has its own budget.
		credentials.GitHubRead = &agentsession.Client{
			URL:    env.Or("BEX_AGENT_GITHUB_READ_API_URL", "http://bex-api.bex-system.svc:8091"+agentsession.InternalGitHubReadMintPath),
			Secret: []byte(secret),
		}
		// w10/m522: the route's admitted writes get their own per-call token,
		// which adds pull requests, issues and Actions write to the reads.
		credentials.GitHubWrite = &agentsession.Client{
			URL:    env.Or("BEX_AGENT_GITHUB_WRITE_API_URL", "http://bex-api.bex-system.svc:8091"+agentsession.InternalGitHubWriteMintPath),
			Secret: []byte(secret),
		}
		credentials.MaxGitHubAPIRequestsPerSession = intEnv("BEX_AGENT_GITHUB_API_MAX_REQUESTS_PER_SESSION", 1000)
		credentials.MaxGitHubAPIRequestsPerWorkspace = intEnv("BEX_AGENT_GITHUB_API_MAX_REQUESTS_PER_WORKSPACE", 10000)
		// ADR090: the agent opens/updates its own PR through the fifth
		// `pull-request` segment of its Git proxy URL; bex-api resolves every
		// target from the durable session and holds the GitHub token.
		credentials.PullRequests = &agentsession.PullRequestClient{
			URL:    env.Or("BEX_AGENT_PULL_REQUEST_API_URL", "http://bex-api.bex-system.svc:8091"+agentsession.InternalPullRequestPath),
			Secret: []byte(secret),
		}
		// ADR047 D11: the agent replies on its own PR through the
		// `pull-request/comment` tail of the same URL; bex-api fixes the
		// repository and PR number from the durable session.
		credentials.PullRequestComments = &agentsession.PullRequestCommentClient{
			URL:    env.Or("BEX_AGENT_PULL_REQUEST_COMMENT_API_URL", "http://bex-api.bex-system.svc:8091"+agentsession.InternalPullRequestCommentPath),
			Secret: []byte(secret),
		}
		// w10/m538: the agent marks its own PR ready and merges it through
		// the `pull-request/ready` and `pull-request/merge` tails; bex-api
		// fixes the PR number and the pinned head from the durable session.
		credentials.PullRequestReady = &agentsession.PullRequestReadyClient{
			URL:    env.Or("BEX_AGENT_PULL_REQUEST_READY_API_URL", "http://bex-api.bex-system.svc:8091"+agentsession.InternalPullRequestReadyPath),
			Secret: []byte(secret),
		}
		credentials.PullRequestMerges = &agentsession.PullRequestMergeClient{
			URL:    env.Or("BEX_AGENT_PULL_REQUEST_MERGE_API_URL", "http://bex-api.bex-system.svc:8091"+agentsession.InternalPullRequestMergePath),
			Secret: []byte(secret),
		}
		// ADR095 D6: the agent shares output files through its Pod-bound share
		// URL; bex-api stores them as the session's output attachments.
		credentials.Files = &agentsession.ShareFileClient{
			URL:    env.Or("BEX_AGENT_SHARE_FILE_API_URL", "http://bex-api.bex-system.svc:8091"+agentsession.InternalShareFilePath),
			Secret: []byte(secret),
		}

		// ADR094 D5: the agent submits a turn's structured result through its
		// Pod-bound result URL; bex-api validates it against the turn's schema.
		credentials.Results = &agentsession.ResultClient{
			URL:    env.Or("BEX_AGENT_RESULT_API_URL", "http://bex-api.bex-system.svc:8091"+agentsession.InternalResultPath),
			Secret: []byte(secret),
		}

		// ADR102 D9: a running agent starts and reads child sessions through its
		// Pod-bound spawn URL; the sandbox holds no bex credential.
		credentials.Spawns = &agentsession.SpawnClient{
			URL:    env.Or("BEX_AGENT_SPAWN_API_URL", "http://bex-api.bex-system.svc:8091"+agentsession.InternalSpawnPath),
			Secret: []byte(secret),
		}

		modelAPIURL := env.Or("BEX_AGENT_MODEL_CREDENTIAL_API_URL", "http://bex-api.bex-system.svc:8091"+agentsession.InternalModelMintPath)
		model.Pods = sshgateway.KubeSessionPodResolver{Client: clientset}
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

		egress.Pods = sshgateway.KubeSessionPodResolver{Client: clientset}
		egress.API = &agentsession.EgressGrantClient{
			URL:    env.Or("BEX_AGENT_EGRESS_GRANT_API_URL", "http://bex-api.bex-system.svc:8091"+agentsession.InternalEgressGrantPath),
			Secret: []byte(secret),
			HTTP:   &http.Client{Timeout: 10 * time.Second},
		}
		egress.Audit = st
		egress.Limits = sshgateway.NewSessionLimiter(
			intEnv("BEX_AGENT_EGRESS_MAX_CONNS", 256),
			intEnv("BEX_AGENT_EGRESS_MAX_CONNS_PER_POD", 32),
		)
		egress.Ports = portsEnv("BEX_AGENT_EGRESS_PORTS", egressproxy.DefaultPorts)
		egress.DenyPrefixes = prefixesEnv("BEX_AGENT_EGRESS_DENY_CIDRS")
		egress.IdleTimeout = durationEnv("BEX_AGENT_EGRESS_IDLE_TIMEOUT", 5*time.Minute)
		egress.MaxDuration = durationEnv("BEX_AGENT_EGRESS_MAX_DURATION", time.Hour)
		// Cumulative budgets: they bound a runaway loop in one session or
		// workspace; 0 disables a dimension.
		egress.MaxConnectionsPerSession = intEnvAllowZero("BEX_AGENT_EGRESS_MAX_CONNECTIONS_PER_SESSION", 20000)
		egress.MaxConnectionsPerWorkspace = intEnvAllowZero("BEX_AGENT_EGRESS_MAX_CONNECTIONS_PER_WORKSPACE", 100000)
		egress.MaxBytesPerSession = int64(intEnvAllowZero("BEX_AGENT_EGRESS_MAX_BYTES_PER_SESSION", 20<<30))
	}
	// Sandbox credential broker (w2/m534, ADR122 D6). Started only when
	// configured; partial configuration fails closed at startup.
	sandboxBroker, sandboxBrokerAddr, err := buildSandboxBroker(
		os.Getenv,
		os.Getenv("BEX_SANDBOX_EXEC_SECRET"),
		clientset,
		st,
		metrics,
	)
	if err != nil {
		log.Fatalf("ssh gateway: sandbox credential broker: %v", err)
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
		KeyGrants:      keyGrants,
		DriverPort:     intEnv("BEX_AGENT_SESSION_DRIVER_PORT", 8787),
		AllowedOrigins: splitCSV(os.Getenv("BEX_API_CORS_ORIGIN")),
		// Same trusted peers the web shell uses: this endpoint is HTTP behind
		// the same Traefik pod network, so without it every attach session's
		// audit row records Traefik's pod IP instead of the client (w1/107).
		TrustedProxies: core.TrustedProxies(trustedProxies),
		Metrics:        metrics,
		Limits:         limits,
		Nonces:         nonces,
		SessionTimeout: sessionTimeout,
		// Live-stream revalidation cadence for read/turn streams (round-9 #6).
		RevalidateInterval: revalidateInterval,
	}
	addr := env.Or("BEX_SSH_ADDR", ":2222")
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("ssh gateway: listen %s: %v", addr, err)
	}
	// /readyz turns 503 at SIGTERM, so a terminating pod leaves every Service
	// endpoint (serving=false), not only the ready set, while it still accepts.
	metricsServer := &http.Server{Handler: metricsHandler(registry, func() bool { return readyCtx.Err() == nil }), ReadHeaderTimeout: 5 * time.Second}
	metricsAddr := env.Or("BEX_SSH_METRICS_ADDR", ":9090")
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

	// Each auxiliary listener's drain: stop accepting, then wait for its open
	// sessions.
	var drainers []func(context.Context) error

	// Browser Web Shell WebSocket listener (docs/ADR035-ssh.md § Browser Web
	// Shell). Started only when BEX_SHELL_TICKET_SECRET is set; Traefik
	// terminates TLS onto this plain-HTTP listener. The handler shares the
	// gateway's session caps and content-free audit with native SSH.
	if shell.Enabled() {
		shellMux := http.NewServeMux()
		shellMux.Handle("GET /shell", shell.Handler())
		drainers = append(drainers, startAuxListener("web shell", "web shell", env.Or("BEX_SHELL_WS_ADDR", ":8080"), shellMux, sessionCtx, stop, metrics))
	}

	// Sandbox-exec SSE listener (w3/m33, `render ea sandbox exec`). Internal-only
	// (bex-api → gateway; the exec-secret HMAC is the trust) — NOT browser-facing,
	// so it is a distinct listener from the Web Shell. Started only when
	// BEX_SANDBOX_EXEC_SECRET is set. pods/exec stays confined here.
	if sandbox.Enabled() {
		execMux := http.NewServeMux()
		execMux.Handle("POST /sandbox-exec", sandbox.Handler())
		files := &sandboxfilegateway.Server{
			Secret:             sandbox.Secret,
			Executor:           executor,
			Metrics:            metrics,
			Limits:             sandboxLimits,
			Nonces:             nonces,
			Revalidator:        &sandboxfilegateway.ExecRevalidator{Base: base, KeyGrants: keyGrants},
			RevalidateInterval: revalidateInterval,
		}
		fileHandler := files.Handler()
		execMux.Handle("PUT "+sandboxfiles.GatewayPath, fileHandler)
		execMux.Handle("GET "+sandboxfiles.GatewayPath, fileHandler)
		// Public-only Sandbox Broker CA certificate for bex-api (w2/m535).
		// Exposed solely on this :8081 internal listener (Cilium
		// sandbox-exec-gateway-ingress restricts :8081 to bex-system/bex-api)
		// so bex-api never mounts or reads the bex-sandbox-broker-ca private
		// key Secret.
		execMux.Handle("GET "+sandboxBrokerCACertPath, sandboxBrokerCACertHandler(sandboxBroker))
		drainers = append(drainers, startAuxListener("sandbox-exec", "sandbox exec", env.Or("BEX_SANDBOX_EXEC_ADDR", ":8081"), execMux, sessionCtx, stop, metrics))
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
		credentialMux := newCredentialMux(credentials)
		drainers = append(drainers, startAuxListener("agent git proxy", "agent git proxy", env.Or("BEX_AGENT_CREDENTIAL_ADDR", ":8082"), credentialMux, sessionCtx, stop, metrics,
			func(server *http.Server) {
				server.ReadTimeout = durationEnv("BEX_AGENT_GIT_READ_TIMEOUT", 10*time.Minute)
				server.IdleTimeout = 2 * time.Minute
			}))
	}
	// Agent-session model proxy (ADR062). Internal-only (sandbox → gateway → vendor
	// on :8084 by default); the sandbox's agent base URL points here so the BYO
	// model key is injected on the upstream hop and never enters the sandbox.
	// Started only when BEX_SANDBOX_EXEC_SECRET is set (same gate as the Git proxy).
	if model.Enabled() {
		modelMux := http.NewServeMux()
		modelMux.Handle("POST "+agentsession.ModelProxyPath, model.Handler())
		drainers = append(drainers, startAuxListener("agent model proxy", "agent model proxy", env.Or("BEX_AGENT_MODEL_PROXY_ADDR", ":8084"), modelMux, sessionCtx, stop, metrics,
			func(server *http.Server) {
				server.ReadTimeout = durationEnv("BEX_AGENT_MODEL_READ_TIMEOUT", 2*time.Minute)
				server.IdleTimeout = 2 * time.Minute
			}))
	}
	// Agent-session egress proxy (w10/m19). Internal-only (sandbox → gateway →
	// internet on :8085 by default); only full-tier session policies admit
	// this hop. Serves CONNECT and absolute-form HTTP, so it takes the broker
	// handler directly rather than a path mux. Same start gate as the others.
	if egress.Enabled() {
		drainers = append(drainers, startAuxListener("agent egress proxy", "agent egress proxy", env.Or("BEX_AGENT_EGRESS_PROXY_ADDR", ":8085"), egress.Handler(), sessionCtx, stop, metrics,
			func(server *http.Server) {
				// Bounds a plain-HTTP request body; CONNECT tunnels clear the
				// deadline after the hijack and run under their own idle and
				// total timeouts.
				server.ReadTimeout = durationEnv("BEX_AGENT_EGRESS_READ_TIMEOUT", 2*time.Minute)
				server.IdleTimeout = 2 * time.Minute
			}))
	}
	// Sandbox credential broker (w2/m534, ADR122 D6). Internal-only (sandbox
	// Pod → gateway on :8086 by default → internet:443). Authenticates callers
	// strictly by source Pod IP + immutable sandbox labels, passes uncredentialed
	// TLS through untouched, and terminates TLS + injects group credentials only
	// for credentialed domains.
	if sandboxBroker != nil && sandboxBroker.Enabled() {
		drainers = append(drainers, startAuxListener("sandbox credential broker", "sandbox credential broker", sandboxBrokerAddr, sandboxBroker.Handler(), sessionCtx, stop, metrics,
			func(server *http.Server) {
				server.ReadTimeout = durationEnv("BEX_SANDBOX_BROKER_READ_TIMEOUT", 2*time.Minute)
				server.IdleTimeout = 2 * time.Minute
			}))
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
		drainers = append(drainers, startAuxListener("agent attach", "agent session attach", env.Or("BEX_AGENT_ATTACH_ADDR", ":8083"), attachMux, sessionCtx, stop, metrics))
	}
	log.Printf("ssh gateway listening on %s", addr)
	if err := serve(ctx, sessionCtx, endSessions, gateway, listener, shutdownTimeout, drainers); err != nil {
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

// errListenerFailed ends the open sessions when a listener fails.
var errListenerFailed = errors.New("a gateway listener failed")

// drainGrace is how long the sessions ended at the drain bound get to send
// their end reason and write their audit row before the process exits.
const drainGrace = 10 * time.Second

// drainOnSignal runs a release's steps (w6/m482, w6/152). The first SIGTERM or
// SIGINT turns readiness off at once and leaves the listeners accepting for
// window: Traefik and the Services' load balancing keep sending a terminating
// pod new connections until they have seen its endpoint leave, which takes a
// few seconds, and a closed listener there fails them before the SSH banner.
// Then it closes the listeners, and the open sessions keep running. A second
// signal, during the window or after it, ends every open session at once.
func drainOnSignal(signals <-chan os.Signal, window time.Duration, stopReady, stopListening context.CancelFunc, endSessions context.CancelCauseFunc) {
	first := <-signals
	stopReady()
	log.Printf("ssh gateway: %v: not ready; accepting new connections for %s while the endpoint leaves the routes", first, window)
	select {
	case <-time.After(window):
		log.Printf("ssh gateway: closing the listeners; open sessions keep running")
		stopListening()
	case second := <-signals:
		log.Printf("ssh gateway: second %v: closing the listeners and ending every open session", second)
		stopListening()
		endSessions(sshgateway.ErrGatewayShutdown)
		return
	}
	log.Printf("ssh gateway: second %v: ending every open session", <-signals)
	endSessions(sshgateway.ErrGatewayShutdown)
}

// serve runs native SSH under sessionCtx until ctx, the listeners' lifetime,
// ends, then drains it with every auxiliary listener (w6/m482). It returns
// Serve's error when native SSH stops accepting on its own.
func serve(ctx, sessionCtx context.Context, endSessions context.CancelCauseFunc, gateway *nativessh.Server, listener net.Listener, bound time.Duration, drainers []func(context.Context) error) error {
	served := make(chan error, 1)
	go func() { served <- gateway.Serve(sessionCtx, listener) }()
	select {
	case err := <-served:
		if err != nil {
			return err
		}
	case <-ctx.Done():
	}
	drain(sessionCtx, endSessions, bound, drainGrace, append(drainers, gateway.Shutdown))
	return nil
}

// drain runs once the listeners are closing (w6/m482): it waits, up to bound,
// for every listener's open sessions to end on their own, then ends the rest
// with sshgateway.ErrGatewayShutdown and gives them grace to finish. Ending
// sessionCtx early (a second signal, a failed listener) ends the wait too.
// Sessions that ignore their context (a CONNECT tunnel) die with the process.
func drain(sessionCtx context.Context, endSessions context.CancelCauseFunc, bound, grace time.Duration, drainers []func(context.Context) error) {
	log.Printf("ssh gateway: draining for up to %s", bound)
	waitCtx, cancel := context.WithTimeout(sessionCtx, bound)
	drainAll(waitCtx, drainers)
	cancel()
	if errors.Is(waitCtx.Err(), context.DeadlineExceeded) {
		log.Printf("ssh gateway: the %s drain bound passed; ending the open sessions", bound)
	}
	endSessions(sshgateway.ErrGatewayShutdown)
	graceCtx, cancelGrace := context.WithTimeout(context.Background(), grace)
	defer cancelGrace()
	drainAll(graceCtx, drainers)
	log.Printf("ssh gateway: drained")
}

// drainAll runs every drainer at once and returns when all have returned.
func drainAll(ctx context.Context, drainers []func(context.Context) error) {
	var wg sync.WaitGroup
	for _, d := range drainers {
		wg.Go(func() { _ = d(ctx) })
	}
	wg.Wait()
}

// startAuxListener runs one auxiliary HTTP listener (health-checked mux, fatal
// on bind failure, stop() on serve failure) and returns its drain: close the
// listener, then wait for the requests in flight, hijacked ones included,
// which run under sessionCtx and so outlive the listener (w6/m482). Each opt
// may still shape the server (e.g. a request-read deadline for a
// body-carrying proxy listener); the defaults suit streaming listeners.
func startAuxListener(name, logName, addr string, handler http.Handler, sessionCtx context.Context, stop func(), metrics *sshgateway.Metrics, opts ...func(*http.Server)) func(context.Context) error {
	// A path mux gets the shared health route; a raw handler (the egress
	// proxy, which must see CONNECT) answers /healthz itself.
	if mux, ok := handler.(*http.ServeMux); ok {
		mux.HandleFunc("GET /healthz", healthzOK)
	}
	sessions := &sshgateway.Inflight{Name: logName}
	server := &http.Server{
		Handler:           sessions.Handler(handler),
		ReadHeaderTimeout: 5 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return sessionCtx },
	}
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
	case "agent egress proxy":
		// Held for a tunnel's whole life, so the global cap sits above the
		// broker's concurrent-tunnel limiter rather than below it.
		global = intEnv("BEX_AGENT_EGRESS_MAX_PREAUTH_CONNS", 512)
		perSource = intEnv("BEX_AGENT_EGRESS_MAX_PREAUTH_CONNS_PER_SOURCE", 64)
	case "sandbox credential broker":
		global = intEnv("BEX_SANDBOX_BROKER_MAX_PREAUTH_CONNS", 512)
		perSource = intEnv("BEX_SANDBOX_BROKER_MAX_PREAUTH_CONNS_PER_SOURCE", 64)
	}
	listener = newAdmissionListener(listener, global, perSource, metrics)
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			stop()
		}
	}()
	log.Printf("%s listening on %s", logName, addr)
	return func(ctx context.Context) error {
		// Shutdown first: a request it still counts as active may yet hijack
		// its connection, and is in sessions by then.
		err := server.Shutdown(ctx)
		return errors.Join(err, sessions.Wait(ctx))
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

// connectionSource is the client a listener's per-source cap counts: the peer
// host, with an IPv6 address collapsed to its /64 (netutil.SourceKey).
func connectionSource(addr net.Addr) string {
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		host = addr.String()
	}
	return netutil.SourceKeyString(host)
}

func healthzOK(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }

// metricsHandler serves Prometheus, liveness (/healthz, always OK) and
// readiness (/readyz, 503 once ready reports false: SIGTERM arrived).
func metricsHandler(gatherer prometheus.Gatherer, ready func() bool) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{}))
	mux.HandleFunc("GET /healthz", healthzOK)
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		if !ready() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
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

// portsEnv parses a comma-separated TCP port list (BEX_AGENT_EGRESS_PORTS);
// unset keeps fallback, and any malformed entry refuses startup.
func portsEnv(name string, fallback []int) []int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	var ports []int
	for _, part := range splitCSV(value) {
		n, err := strconv.Atoi(part)
		if err != nil || n <= 0 || n > 65535 {
			log.Fatalf("ssh gateway: %s must be a comma-separated list of TCP ports", name)
		}
		ports = append(ports, n)
	}
	if len(ports) == 0 {
		log.Fatalf("ssh gateway: %s must name at least one port", name)
	}
	return ports
}

// prefixesEnv parses a comma-separated CIDR list (BEX_AGENT_EGRESS_DENY_CIDRS);
// any malformed entry refuses startup.
func prefixesEnv(name string) []netip.Prefix {
	var prefixes []netip.Prefix
	for _, part := range splitCSV(os.Getenv(name)) {
		prefix, err := netip.ParsePrefix(part)
		if err != nil {
			log.Fatalf("ssh gateway: %s entry %q is not a CIDR", name, part)
		}
		prefixes = append(prefixes, prefix.Masked())
	}
	return prefixes
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

// gatewayLimits builds the session limiter that native SSH, the web shell and
// agent attach share, and the sandbox exec and file streams' own pools
// (w22/m623): Render documents no concurrency limit on sandbox commands, so
// one user's commands must neither meet the 5 per identity nor take native
// SSH's slots. ADR104 D7 records how the defaults were sized.
func gatewayLimits() (*sshgateway.SessionLimiter, *sshgateway.SandboxStreamLimits) {
	sandboxStreams := intEnv("BEX_SANDBOX_MAX_STREAMS", 100)
	return sshgateway.NewSessionLimiter(
			intEnv("BEX_SSH_MAX_SESSIONS", 100),
			intEnv("BEX_SSH_MAX_SESSIONS_PER_IDENTITY", 5),
		), sshgateway.NewSandboxStreamLimits(
			sandboxStreams,
			intEnv("BEX_SANDBOX_MAX_STREAMS_PER_IDENTITY", sandboxStreams),
			intEnv("BEX_SANDBOX_MAX_PLATFORM_STREAMS", 32),
			intEnv("BEX_SANDBOX_MAX_PLATFORM_STREAMS_PER_KEY", 5),
		)
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

// newCredentialMux routes the agent listener: the Git proxy (with the PR
// verbs under it), shared files, structured results and the child-session
// verbs (spawn, read, message, cancel, wait).
func newCredentialMux(credentials *agentcred.Broker) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("GET "+agentsession.GitProxyPath, credentials.Handler())
	mux.Handle("POST "+agentsession.GitProxyPath, credentials.Handler())
	mux.Handle("POST "+agentsession.ShareFileProxyPath, credentials.ShareFileHandler())
	mux.Handle("POST "+agentsession.ResultProxyPath, credentials.ResultHandler())
	for _, method := range []string{http.MethodPost, http.MethodGet, http.MethodDelete} {
		mux.Handle(method+" "+agentsession.SpawnProxyPath, credentials.SpawnHandler())
	}
	return mux
}

type tenantLister interface {
	ListTenants(ctx context.Context) ([]store.Tenant, error)
}

// buildSandboxBroker configures the Pod-bound outbound HTTPS credential broker
// (w2/m534, ADR122 D6). Returns (nil, "", nil) when completely unconfigured so
// deployments without the Broker CA skip the listener cleanly. Any partial
// configuration fails closed with an error at startup.
func buildSandboxBroker(getenv func(string) string, execSecret string, clientset kubernetes.Interface, tenants tenantLister, metrics *sshgateway.Metrics) (*sandboxbroker.Broker, string, error) {
	addr := strings.TrimSpace(getenv("BEX_SANDBOX_BROKER_ADDR"))
	certPath := strings.TrimSpace(getenv("BEX_SANDBOX_BROKER_CA_CERT_PATH"))
	keyPath := strings.TrimSpace(getenv("BEX_SANDBOX_BROKER_CA_KEY_PATH"))
	apiURL := strings.TrimSpace(getenv("BEX_SANDBOX_BROKER_API_URL"))

	if addr == "" && certPath == "" && keyPath == "" && apiURL == "" {
		return nil, "", nil
	}
	if certPath == "" || keyPath == "" {
		return nil, "", errors.New("partial config: both BEX_SANDBOX_BROKER_CA_CERT_PATH and BEX_SANDBOX_BROKER_CA_KEY_PATH are required")
	}
	if strings.TrimSpace(execSecret) == "" {
		return nil, "", errors.New("partial config: BEX_SANDBOX_EXEC_SECRET is required to authenticate broker lookups")
	}
	if clientset == nil {
		return nil, "", errors.New("kubernetes client is required for sandbox Pod IP binding")
	}
	if tenants == nil {
		return nil, "", errors.New("control-plane store is required to resolve sandbox namespaces")
	}
	ca, err := sandboxbroker.LoadCA(certPath, keyPath, sandboxbroker.CAOptions{})
	if err != nil {
		return nil, "", err
	}
	if addr == "" {
		addr = ":8086"
	}
	if apiURL == "" {
		apiURL = "http://bex-api.bex-system.svc:8091" + sandbox.InternalBrokerLookupPath
	}

	cacheTTL := sandbox.BrokerLookupMaxCacheTTL
	if raw := strings.TrimSpace(getenv("BEX_SANDBOX_BROKER_CACHE_TTL")); raw != "" {
		d, parseErr := time.ParseDuration(raw)
		if parseErr != nil || d <= 0 {
			return nil, "", fmt.Errorf("BEX_SANDBOX_BROKER_CACHE_TTL must be a positive duration")
		}
		cacheTTL = d
	}

	lookupClient := &sandbox.BrokerLookupClient{
		URL:              apiURL,
		Secret:           []byte(execSecret),
		HTTP:             &http.Client{Timeout: 5 * time.Second},
		CacheTTL:         cacheTTL,
		OnCacheEvent:     metrics.SandboxBrokerCache,
		OnLookupDuration: metrics.SandboxBrokerLookupDuration,
	}
	podResolver := &sandboxbroker.KubeSandboxPodResolver{
		Client: clientset,
		Namespaces: func(ctx context.Context) ([]string, error) {
			list, listErr := tenants.ListTenants(ctx)
			if listErr != nil {
				return nil, listErr
			}
			out := make([]string, 0, len(list))
			for _, t := range list {
				if t.ID != "" {
					out = append(out, store.SandboxNamespace(t.ID))
				}
			}
			return out, nil
		},
	}

	broker := &sandboxbroker.Broker{
		Pods:    podResolver,
		API:     lookupClient,
		CA:      ca,
		Metrics: metrics,
		Limits: sshgateway.NewSessionLimiter(
			intEnv("BEX_SANDBOX_BROKER_MAX_CONNS", 256),
			intEnv("BEX_SANDBOX_BROKER_MAX_CONNS_PER_POD", 32),
		),
		DenyPrefixes: prefixesEnv("BEX_SANDBOX_BROKER_DENY_CIDRS"),
		IdleTimeout:  durationEnv("BEX_SANDBOX_BROKER_IDLE_TIMEOUT", 5*time.Minute),
		MaxDuration:  durationEnv("BEX_SANDBOX_BROKER_MAX_DURATION", time.Hour),
	}
	return broker, addr, nil
}

// sandboxBrokerCACertPath aliases sandbox.GatewayBrokerCACertPath at package
// scope because main() has a local variable named sandbox.
const sandboxBrokerCACertPath = sandbox.GatewayBrokerCACertPath

// sandboxBrokerCACertHandler serves ONLY the public PEM certificate of the
// Sandbox Broker CA on the internal :8081 (bex-api-only) listener. Returns 404
// when the broker is not enabled.
func sandboxBrokerCACertHandler(broker *sandboxbroker.Broker) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if broker == nil || !broker.Enabled() || broker.CA == nil {
			http.NotFound(w, r)
			return
		}
		certPEM := broker.CA.CertPEM()
		if len(certPEM) == 0 {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/x-pem-file")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(certPEM)
	})
}
