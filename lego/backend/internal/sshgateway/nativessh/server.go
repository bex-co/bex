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

// Package nativessh is the native SSH transport of the isolated gateway: it
// authenticates registered public keys, authorizes their identities against a
// requested App, and bridges SSH session channels to the Kubernetes pods/exec
// API through the shared sshgateway kernel. Share one
// sshgateway.SessionLimiter with the sibling transports (webshell,
// sandboxsse) so the session caps bound the process, not each feature.
package nativessh

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/netip"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/agentsessions"
	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/proxyproto"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/types/netutil"
)

const (
	permissionSubject     = "bex.subject"
	permissionTarget      = "bex.target"
	permissionFingerprint = "bex.fingerprint"
	// permissionKeyAction is the action a connection whose key's owner is a
	// bex API key is held to (w9/125); absent for a person's connection.
	permissionKeyAction = "bex.key-action"
	// permissionWaking marks a login held for its agent session's sandbox
	// (w10/m597): its target names the session but no pod yet.
	permissionWaking = "bex.waking"
)

// Store is the native SSH transport's deliberately small database authority:
// authenticate a fingerprint and write content-free session audit facts. (The
// exec-ticket nonce claim lives on sshgateway.NonceStore, used only by the
// ticketed transports.)
type Store interface {
	SSHKeyByFingerprint(context.Context, string) (store.SSHKey, error)
	StartSSHSession(context.Context, store.SSHSessionAudit) error
	EndSSHSession(context.Context, string, string, time.Time) error
}

type Server struct {
	Store    Store
	Apps     sshgateway.TargetResolver
	Executor sshgateway.Executor
	Signer   ssh.Signer
	Metrics  *sshgateway.Metrics

	// Limits caps concurrent sessions. Share ONE limiter with the webshell and
	// sandboxsse transports so the caps bound the process, not each feature;
	// nil gets a private limiter with the defaults.
	Limits *sshgateway.SessionLimiter

	// ChannelLimits caps concurrent exec STREAMS (session channels), which the
	// session limiter cannot see: one multiplexed connection may hold many
	// channels, each its own pods/exec (codex round-8 #7). Share ONE across
	// replicas of this transport; nil gets a private limiter with the defaults.
	ChannelLimits *sshgateway.ChannelLimiter

	HandshakeTimeout time.Duration
	// SessionTimeout bounds an agent-session sandbox connection (ADR054 D6)
	// and a service connection that has not opened its shell yet. A service
	// shell itself has no time limit, as on Render (w6/174): it ends with its
	// client, its access, its instance or the gateway's drain bound.
	SessionTimeout time.Duration
	// KeepaliveInterval is how often the gateway asks the client whether it is
	// still there (keepalive@openssh.com, as sshd's ClientAliveInterval does);
	// a connection whose client has not answered within keepaliveMisses
	// intervals is closed, so a client that vanished without closing its
	// connection cannot hold a session slot forever (w6/174). 0 gets the
	// default (one minute); negative disables it.
	KeepaliveInterval time.Duration

	// MaxPreAuthConns bounds how many connections may be in the pre-authentication
	// phase (TCP accepted, SSH handshake + public-key database lookups in flight)
	// at once. The session limiter is acquired only AFTER the handshake, so
	// without this an unauthenticated flood spawns unbounded goroutines and store
	// lookups before any cap applies. A slot is held only across the handshake and
	// released the moment the connection authenticates (the session limiter takes
	// over) or fails. 0 gets the default (256).
	MaxPreAuthConns int

	// MaxPreAuthConnsPerSource bounds how many of the global pre-auth slots ONE
	// resolved source address may hold at once (round-11 #2). Without it a single
	// unauthenticated source can park silent connections in every global slot for
	// the full HandshakeTimeout and force the gateway to shed every other
	// tenant's handshakes. The source is the PROXY-protocol-resolved client when
	// the immediate peer is trusted, else the immediate peer itself. 0 gets the
	// default (32); negative disables per-source fairness (global-only, the
	// pre-round-11 behavior).
	MaxPreAuthConnsPerSource int

	// MaxChannelsPerConn bounds concurrent session channels on ONE connection for
	// agent-session sandbox targets (ADR054 D3), which Zed's ControlMaster remoting
	// multiplexes. 0 disables the exception: sandbox targets fall back to the
	// single-channel App path. App (srv-…) targets are always single-channel
	// regardless of this value.
	MaxChannelsPerConn int

	// RevalidateInterval is how often an ESTABLISHED exec stream re-runs the
	// fresh key + target authorization the channel-open path performs (codex
	// round-9 #6): a revocation mid-stream cancels the stream's context instead
	// of waiting for a disconnect. 0 => the platform default
	// (sshgateway.DefaultRevalidateInterval); negative disables the watchdog
	// (the pre-round-9 admission-only behavior).
	RevalidateInterval time.Duration

	// TrustedProxies lists the immediate peers (Traefik's pod network) allowed
	// to assert a PROXY protocol v1/v2 original-client address on the ssh
	// entrypoint's forwarded connection. Empty (the default) leaves every
	// connection's RemoteAddr as the immediate TCP peer, unchanged — the same
	// gap w4/029.md #10 reported (ssh_sessions.remote_address recording
	// Traefik's own pod IP). Set it once lego/operator/config/ssh/
	// ingressroutetcp.yaml's service carries proxyProtocol.
	TrustedProxies []netip.Prefix

	// EmailVerified refuses a key whose owner is a human with an unverified
	// email (ADR075 D8 revision, w2/m168): key auth never passes bex-api's
	// middleware, so authentication and every reauthorization ask. Any
	// non-nil error refuses; only agentsession.ErrEmailUnverified is the
	// definite answer, so a live session retries any other error (w6/147).
	// nil skips the check (BEX_REQUIRE_VERIFIED_EMAIL=0).
	EmailVerified func(ctx context.Context, subject string) error

	// Audit receives each service shell session's StartShellEvent and
	// EndShellEvent rows (w9/m451, sshgateway.RecordShellEvent). nil records
	// none.
	Audit core.AuditSink

	// KeyGrants asks, when a key authenticates, whether its owner is a bex
	// API key (one enrolled before enrollment was refused to machine
	// callers, ADR072 F3) and whether that key's grant covers the target;
	// such a connection asks again on every channel and revalidation tick,
	// so a narrowing ends it (w9/125). A person's connection asks once, at
	// authentication. nil skips the check (unit tests exercising transport
	// behavior — cmd/ssh-gateway always wires it).
	KeyGrants *sshgateway.KeyGrantGate

	// Wakes starts a closed agent session's sandbox for a login that may
	// start it (w10/m597, ADR054 D9): the login is held, its channels wait
	// for the sandbox, then run in it. nil keeps w10/043's banner for every
	// session with no sandbox to open, as does MaxChannelsPerConn 0.
	Wakes SessionWaker
	// WakeTimeout bounds a held login's wait; 0 gets the start's own ceiling,
	// agentsessions.SSHWakeTimeout.
	WakeTimeout time.Duration
	// wakePollInterval is how often a held login reads whether its session
	// settled; 0 gets defaultWakePollInterval (tests shorten it).
	wakePollInterval time.Duration

	// mu guards the drain state (w6/m482): the listener Serve accepts on, which
	// Shutdown closes, and the open connections Shutdown waits for.
	mu       sync.Mutex
	listener net.Listener
	shutdown bool
	sessions *sshgateway.Inflight
}

var errUnverifiedOwner = errors.New("public key rejected: verify your bex account email first")

// The two-factor refusal (w22/m402, docs/ADR012-auth.md §9a) is a cause the
// gateway does tell the client, because it only arises after the member was
// authorized: the funnel asks the 2FA gate once OpenFGA has said yes, so the
// message reveals nothing to a key that is not a member with access. The SSH
// client prints the banner while authenticating; a channel refused later
// carries the reason in its open-failure description.
//
// The Google-login refusal (w7/m465, §9b) is told the same way: a key that was
// not added while its owner was signed in with Google, or an owner whose
// Google account no longer matches their email, gets its own banner.
//
// When the workspace's org's own setting decided the refusal (§9c), both name
// the org, as its refusal does on every other surface (w7/090), so the member
// looks for the setting on the org's page, not the workspace's.
//
// An agent session with no sandbox to open is told at login too (w10/043,
// sessionRefusal); a channel refused later keeps the generic reason.
const reauthorizeFailed = "authorization no longer valid"

// loginRefusal is how the client is told about a workspace login-requirement
// refusal, however a resolver wrapped it. ok is false for every other error.
func loginRefusal(err error) (r refusal, ok bool) {
	lr, ok := core.LoginRefusalOf(err)
	if !ok {
		return refusal{}, false
	}
	who, scope := "this workspace", ""
	if lr.OrganizationName != "" {
		who, scope = "the organization "+lr.OrganizationName, " in all of its workspaces"
	}
	requirement, fix, metric := "two-factor authentication", "Enable it in your account settings, then connect again.", "rejected_two_factor"
	if lr.Code == core.CodeWorkspaceGoogleLoginRequired {
		requirement, fix, metric = "signing in with Google", "Sign in to bex with Google, add this SSH key again, then connect again.", "rejected_google_login"
	}
	reason := who + " requires " + requirement + scope
	return refusal{banner: "bex: " + reason + ". " + fix + "\n", reason: reason, metric: metric}, true
}

// refusal is one refusal the SSH client is told about, as it sees it: the
// banner while authenticating, the reason a refused channel carries, and the
// authentications_total result it is counted under.
type refusal struct{ banner, reason, metric string }

// sessionRefusal is how the client is told that an agent session it may open
// has no sandbox now (w10/043): its turn ended and the sandbox closed, it is
// still starting, or it was canceled or archived. The agent-session resolver
// refuses so only after can_view_sensitive passed, and its message says what
// brings the sandbox back. Every other target refusal stays a bare rejection
// (ADR035 § Fail-closed behavior). ok is false for every other error.
func sessionRefusal(err error) (r refusal, ok bool) {
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != core.CodeAgentSessionNotAttachable {
		return refusal{}, false
	}
	return refusal{banner: "bex: " + coded.Error() + ".\n", reason: coded.Error(), metric: "rejected_no_sandbox"}, true
}

// channelRefusal is the description a refused channel open carries.
func channelRefusal(err error) string {
	if r, ok := loginRefusal(err); ok {
		return r.reason
	}
	return reauthorizeFailed
}

// keyIdentity is the identity a registered key authenticates as: its owner,
// with the sign-in the key was added under, which a workspace that requires
// Google login reads (w7/m465).
func keyIdentity(key store.SSHKey) core.Identity {
	return core.Identity{Subject: key.Subject, Method: core.MethodSSH, SignIn: key.CreatedSignIn}
}

// verifiedOwner applies EmailVerified under the handshake deadline.
func (s *Server) verifiedOwner(ctx context.Context, subject string) error {
	if s.EmailVerified == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, s.HandshakeTimeout)
	defer cancel()
	return s.EmailVerified(ctx, subject)
}

func (s *Server) defaults() {
	if s.HandshakeTimeout <= 0 {
		s.HandshakeTimeout = 10 * time.Second
	}
	if s.SessionTimeout <= 0 {
		s.SessionTimeout = 4 * time.Hour
	}
	if s.KeepaliveInterval == 0 {
		s.KeepaliveInterval = defaultKeepaliveInterval
	}
	if s.MaxPreAuthConns <= 0 {
		s.MaxPreAuthConns = 256
	}
	if s.MaxPreAuthConnsPerSource == 0 {
		s.MaxPreAuthConnsPerSource = 32
	}
	if s.RevalidateInterval == 0 {
		s.RevalidateInterval = sshgateway.DefaultRevalidateInterval
	}
	if s.Limits == nil {
		s.Limits = sshgateway.NewSessionLimiter(0, 0)
	}
	if s.ChannelLimits == nil {
		s.ChannelLimits = sshgateway.NewChannelLimiter(0, 0)
	}
	if s.WakeTimeout <= 0 {
		s.WakeTimeout = agentsessions.SSHWakeTimeout
	}
	if s.wakePollInterval <= 0 {
		s.wakePollInterval = defaultWakePollInterval
	}
}

func (s *Server) config(ctx context.Context) (*ssh.ServerConfig, error) {
	if s.Store == nil || s.Apps == nil || s.Executor == nil || s.Signer == nil {
		return nil, errors.New("ssh gateway is missing a required dependency")
	}
	s.defaults()
	config := &ssh.ServerConfig{
		ServerVersion: "SSH-2.0-bex",
		MaxAuthTries:  3,
		Config: ssh.Config{
			KeyExchanges: []string{"curve25519-sha256", "curve25519-sha256@libssh.org", "ecdh-sha2-nistp256"},
			Ciphers:      []string{"chacha20-poly1305@openssh.com", "aes256-gcm@openssh.com", "aes128-gcm@openssh.com", "aes256-ctr", "aes128-ctr"},
			MACs:         []string{"hmac-sha2-512-etm@openssh.com", "hmac-sha2-256-etm@openssh.com", "hmac-sha2-512", "hmac-sha2-256"},
		},
		PublicKeyAuthAlgorithms: []string{
			ssh.KeyAlgoED25519, ssh.KeyAlgoSKED25519, ssh.KeyAlgoECDSA256,
			ssh.KeyAlgoECDSA384, ssh.KeyAlgoECDSA521, ssh.KeyAlgoSKECDSA256,
			ssh.KeyAlgoRSASHA256, ssh.KeyAlgoRSASHA512,
		},
	}
	config.PublicKeyCallback = func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
		lookupCtx, cancel := context.WithTimeout(ctx, s.HandshakeTimeout)
		defer cancel()
		registered, err := s.Store.SSHKeyByFingerprint(lookupCtx, ssh.FingerprintSHA256(key))
		if err != nil || registered.Subject == "" {
			s.Metrics.Authentication("rejected_key")
			return nil, errors.New("public key rejected")
		}
		return &ssh.Permissions{Extensions: map[string]string{permissionSubject: registered.Subject}}, nil
	}
	// Target authorization runs only after the client has proved possession of
	// the accepted key; PublicKeyCallback may be invoked as a key query.
	config.VerifiedPublicKeyCallback = func(conn ssh.ConnMetadata, key ssh.PublicKey, permissions *ssh.Permissions, _ string) (*ssh.Permissions, error) {
		subject := permissions.Extensions[permissionSubject]
		// PublicKeyCallback can run before the client proves possession. Re-read
		// after signature verification so deleting a key while that handshake is
		// in flight revokes it before target authorization or channel creation.
		lookupCtx, lookupCancel := context.WithTimeout(ctx, s.HandshakeTimeout)
		registered, lookupErr := s.Store.SSHKeyByFingerprint(lookupCtx, ssh.FingerprintSHA256(key))
		lookupCancel()
		if lookupErr != nil || registered.Subject == "" || registered.Subject != subject {
			s.Metrics.Authentication("rejected_key")
			return nil, errors.New("public key rejected")
		}
		if err := s.verifiedOwner(ctx, subject); err != nil {
			s.Metrics.Authentication("rejected_unverified")
			return nil, errUnverifiedOwner
		}
		// The origin rides the authorization, so a refusal's denied row (the
		// audit log's StartShellEvent error) says where the caller connected from.
		authCtx := core.WithIdentity(core.WithRequestOrigin(ctx, core.RequestOrigin{SourceIP: sourceIP(conn.RemoteAddr())}), keyIdentity(registered))
		authCtx, cancel := context.WithTimeout(authCtx, s.HandshakeTimeout)
		defer cancel()
		tell := func(r refusal) (*ssh.Permissions, error) {
			s.Metrics.Authentication(r.metric)
			if pre, ok := conn.(ssh.ServerPreAuthConn); ok {
				_ = pre.SendAuthBanner(r.banner)
			}
			return nil, errors.New("public key rejected: " + r.reason)
		}
		target, resolveErr := s.Apps.ResolveSSHSession(authCtx, conn.User())
		// A session whose sandbox closed or is starting may still be
		// reached: the login is held for it once it passed every check an
		// admitted login passes (w10/m597).
		wake := s.wakeOf(conn.User(), resolveErr)
		if err := resolveErr; err != nil && wake.state == "" {
			if r, ok := s.toldRefusal(authCtx, subject, conn.User(), err); ok {
				return tell(r)
			}
			s.Metrics.Authentication("rejected_target")
			return nil, errors.New("public key rejected")
		}
		keyAction, err := s.keyOwnerAction(authCtx, subject, conn.User())
		if err != nil {
			s.Metrics.Authentication("rejected_target")
			return nil, errors.New("public key rejected")
		}
		if keyAction != "" {
			permissions.Extensions[permissionKeyAction] = keyAction
		}
		if wake.state != "" {
			var told *refusal
			if target, told = s.wakeSession(authCtx, conn, registered, wake, resolveErr); told != nil {
				return tell(*told)
			}
			permissions.Extensions[permissionWaking] = "1"
		}
		s.Metrics.Authentication("accepted")
		// The fingerprint rides the transport so every later channel can re-read
		// the key (deleting it mid-connection must stop new channels, codex
		// round-8 #5).
		permissions.Extensions[permissionFingerprint] = ssh.FingerprintSHA256(key)
		permissions.Extensions[permissionTarget] = encodeTarget(target)
		return permissions, nil
	}
	config.AddHostKey(s.Signer)
	return config, nil
}

// Serve accepts until ctx is cancelled or Shutdown is called. Every
// connection runs under ctx, so cancelling it ends them all at once; Shutdown
// instead stops accepting and leaves them running (w6/m482).
func (s *Server) Serve(ctx context.Context, listener net.Listener) error {
	config, err := s.config(ctx)
	if err != nil {
		return err
	}
	if !s.track(listener) {
		return listener.Close()
	}
	stop := context.AfterFunc(ctx, func() { _ = listener.Close() })
	defer stop()
	// Pre-authentication admission. Bounds concurrent handshakes-in-flight before
	// ssh.NewServerConn and its key-lookup DB round trips run, so an anonymous
	// connection flood cannot exhaust goroutines, descriptors, and store QPS ahead
	// of the post-handshake session limiter. A full buffer sheds immediately.
	preAuth := make(chan struct{}, s.MaxPreAuthConns)
	// Per-source fairness inside that pool (round-11 #2): the global cap alone
	// lets one silent source occupy every slot for the full HandshakeTimeout.
	// serveConn acquires a source slot once the PROXY header resolves the real
	// client, and holds it exactly as long as the global slot.
	preAuthSources := sshgateway.NewSourceLimiter(s.MaxPreAuthConnsPerSource)
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil || s.draining() {
				return nil
			}
			return err
		}
		select {
		case preAuth <- struct{}{}:
		default:
			s.Metrics.LimitRejected("preauth")
			_ = conn.Close()
			continue
		}
		if !s.admit() {
			// Accepted just as Shutdown closed the listener.
			<-preAuth
			_ = conn.Close()
			return nil
		}
		go func() {
			defer s.sessions.End()
			// Idempotent release: serveConn frees the slot as soon as the handshake
			// resolves; this is the safety net if it returns without doing so.
			released := false
			release := func() {
				if !released {
					released = true
					<-preAuth
				}
			}
			if err := s.serveConn(ctx, conn, config, release, preAuthSources); err != nil && ctx.Err() == nil {
				log.Printf("ssh gateway connection ended: %v", err)
			}
			release()
		}()
	}
}

// Shutdown stops accepting and waits for the open connections to end, or
// returns ctx's error once ctx is done; they keep running under Serve's
// context, whose cancellation is what ends them (w6/m482). Calling it again
// waits again.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	s.shutdown = true
	listener := s.listener
	sessions := s.inflightLocked()
	s.mu.Unlock()
	if listener != nil {
		_ = listener.Close()
	}
	return sessions.Wait(ctx)
}

// track records the listener Shutdown closes; false once Shutdown has run.
func (s *Server) track(listener net.Listener) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inflightLocked()
	s.listener = listener
	return !s.shutdown
}

// admit counts one accepted connection in; false once Shutdown has run, so
// every connection it counts is one Shutdown's wait sees.
func (s *Server) admit() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.shutdown {
		return false
	}
	s.sessions.Begin()
	return true
}

func (s *Server) draining() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.shutdown
}

func (s *Server) inflightLocked() *sshgateway.Inflight {
	if s.sessions == nil {
		s.sessions = &sshgateway.Inflight{Name: "native SSH"}
	}
	return s.sessions
}

func (s *Server) serveConn(ctx context.Context, raw net.Conn, config *ssh.ServerConfig, releasePreAuth func(), preAuthSources *sshgateway.SourceLimiter) error {
	defer raw.Close()
	_ = raw.SetDeadline(time.Now().Add(s.HandshakeTimeout))
	transport, err := proxyproto.Wrap(raw, s.TrustedProxies)
	if err != nil {
		// A PROXY-header parse failure is a transport fault before the SSH
		// handshake — count it as a failed handshake so a misconfigured edge is
		// loud, then close (no in-band SSH channel exists to say more pre-KEXINIT).
		s.Metrics.Handshake("failed")
		releasePreAuth()
		return err
	}
	// Round-11 #2: per-source fairness inside the global pre-auth pool. The
	// wrap has resolved the real client (Traefik-asserted for trusted peers, the
	// immediate peer otherwise); a source at its cap is shed here, before the
	// handshake reads anything.
	source := ""
	if addr, err := proxyproto.RemoteIP(transport.RemoteAddr()); err == nil {
		source = netutil.SourceKey(addr).String() // an IPv6 client counts under its /64
	}
	if !preAuthSources.Acquire(source) {
		s.Metrics.LimitRejected("preauth_source")
		releasePreAuth()
		return errors.New("pre-auth per-source connection limit reached")
	}
	sourceReleased := false
	releaseSource := func() {
		if !sourceReleased {
			sourceReleased = true
			preAuthSources.Release(source)
		}
	}
	conn, channels, requests, err := ssh.NewServerConn(transport, config)
	if err != nil {
		// The version exchange or key exchange did not complete: the client never
		// received a usable KEXINIT (an un-stripped PROXY header, a malformed
		// client, or a hang-up). This is the pre-authentication counterpart to the
		// auth-stage results, and — unlike those — it is not a security boundary
		// (ADR035:106), so it may be observed distinctly. w6/m132.
		s.Metrics.Handshake("failed")
		releasePreAuth()
		releaseSource()
		return err
	}
	s.Metrics.Handshake("established")
	defer conn.Close()
	_ = raw.SetDeadline(time.Time{})
	// Handshake done: the authenticated session limiter now bounds this
	// connection, so free the pre-auth slots for the next incoming handshake.
	releasePreAuth()
	releaseSource()

	subject := conn.Permissions.Extensions[permissionSubject]
	fingerprint := conn.Permissions.Extensions[permissionFingerprint]
	clientIP := sourceIP(conn.RemoteAddr())
	// Every re-authorization below (each channel, each revalidation tick)
	// records a refusal with the client's address too.
	ctx = core.WithRequestOrigin(ctx, core.RequestOrigin{SourceIP: clientIP})
	connUser := conn.User()
	waking := conn.Permissions.Extensions[permissionWaking] != ""
	target, err := decodeTarget(conn.Permissions.Extensions[permissionTarget], waking)
	if err != nil {
		return err
	}
	sticky := &connectionTarget{target: target, keyAction: conn.Permissions.Extensions[permissionKeyAction]}
	acquired, limitScope := s.Limits.Acquire(subject)
	if !acquired {
		s.Metrics.LimitRejected(limitScope)
		return errors.New("session limit reached")
	}
	defer s.Limits.Release(subject)

	sessionID := ids.New(ids.SSHSession)
	started := time.Now().UTC()
	s.Metrics.SessionStarted()
	result := "closed"
	defer func() { s.Metrics.SessionEnded(result, time.Since(started)) }()
	auditTarget := sticky.Get()
	workspaceID := auditTarget.OwnerID
	if workspaceID == "" {
		workspaceID = core.DefaultTenant
	}
	auditCtx, cancelAudit := context.WithTimeout(context.Background(), 2*time.Second)
	err = s.Store.StartSSHSession(auditCtx, store.SSHSessionAudit{
		ID: sessionID, Subject: subject, WorkspaceID: workspaceID,
		ServiceID: auditTarget.ServiceID, InstanceID: auditTarget.ID,
		RemoteAddress: conn.RemoteAddr().String(), StartedAt: started,
		Transport: store.SSHTransportSSH,
	})
	cancelAudit()
	if err != nil {
		result = "failed"
		return fmt.Errorf("record SSH session start: %w", err)
	}
	defer func() {
		endCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := s.Store.EndSSHSession(endCtx, sessionID, result, time.Now().UTC()); err != nil {
			log.Printf("ssh gateway session audit end failed: %v", err)
		}
	}()
	// The workspace audit log's shell rows (w9/m451). Native SSH carries no
	// user agent; the end row repeats the start row's metadata.
	shell := sshgateway.ShellSession{ID: sessionID, Subject: subject, Method: core.MethodSSH, Target: auditTarget, SourceIP: clientIP}
	sshgateway.RecordShellEvent(s.Audit, core.AuditVerbStartShell, shell, started)
	defer func() { sshgateway.RecordShellEvent(s.Audit, core.AuditVerbEndShell, shell, time.Now().UTC()) }()

	go ssh.DiscardRequests(requests)
	keepaliveCtx, stopKeepalive := context.WithCancel(ctx)
	defer stopKeepalive()
	go keepalive(keepaliveCtx, conn, s.KeepaliveInterval)
	sessionCtx, cancel := context.WithTimeout(ctx, s.SessionTimeout)
	defer cancel()
	// A service shell lasts as long as its client, its access and its
	// instance, with no time limit, as on Render (w6/174, ADR035 § How long a
	// session lasts); sessionCtx bounds only the wait for it. An agent-session
	// sandbox connection keeps the cap whole (ADR054 D6).
	lifetime := ctx
	if target.Sandbox {
		lifetime = sessionCtx
	}

	// An agent-session sandbox target (ADR054 D3) accepts Zed's multiplexed
	// remoting: many concurrent session channels over one connection, each its own
	// pods/exec stream, bounded per connection. Every other target — and a sandbox
	// target when the cap is disabled (0) — keeps the single-channel contract.
	if target.Sandbox && s.MaxChannelsPerConn > 0 {
		var wait *sandboxWait
		if waking {
			wait = s.awaitSandbox(sessionCtx, subject, fingerprint, connUser)
		}
		result = s.serveMultiChannel(sessionCtx, channels, subject, fingerprint, connUser, sticky, wait)
		return nil
	}

	for {
		var newChannel ssh.NewChannel
		select {
		case <-sessionCtx.Done():
			result = "failed"
			return sessionCtx.Err()
		case channel, ok := <-channels:
			if !ok {
				return nil
			}
			newChannel = channel
		}
		if newChannel.ChannelType() != "session" {
			_ = newChannel.Reject(ssh.UnknownChannelType, "unsupported channel type")
			continue
		}
		// codex round-8 #5: the transport may outlive the auth-time decision by
		// hours; re-read the key and re-resolve + re-authorize (uncached) the
		// target immediately before accepting the channel. A revocation after
		// transport auth ends the transport, not just this channel — an
		// already-open channel is a stream in flight, but no NEW exec may start
		// against a decision that is hours stale.
		resolved, err := s.reauthorize(ctx, subject, fingerprint, connUser, sticky)
		if err != nil {
			_ = newChannel.Reject(ssh.Prohibited, channelRefusal(err))
			result = "failed" // no answer refuses the channel too, but revoked nothing (w6/147)
			if definiteRefusal(err) {
				result = "revoked"
			}
			return nil
		}
		if ok, scope := s.ChannelLimits.AcquireChannel(subject); !ok {
			s.Metrics.LimitRejected(scope)
			_ = newChannel.Reject(ssh.ResourceShortage, "channel limit reached")
			continue
		}
		channel, channelRequests, err := newChannel.Accept()
		if err != nil {
			s.ChannelLimits.ReleaseChannel(subject)
			result = "failed"
			return err
		}
		s.Metrics.ChannelOpened()
		// The connection is deliberately single-session. Drain the connection's
		// channel stream while the accepted exec is active so a client cannot
		// queue a second shell behind a long-running first one and consume server
		// resources indefinitely. Closing conn below ends this goroutine; so does
		// the client closing it, which clientClosed reports.
		clientClosed := make(chan struct{})
		go func() {
			rejectAdditionalChannels(channels)
			close(clientClosed)
		}()
		// codex round-9 #6: keep re-running the channel-open reauthorization
		// while the exec is LIVE, not only at admission — a revocation cancels
		// the stream's context from below. lifetime still bounds it.
		streamCtx, cancelStream := sshgateway.WithRevalidation(lifetime, s.RevalidateInterval, s.revalidator(subject, fingerprint, connUser, sticky, nil))
		err = serveSession(streamCtx, channel, channelRequests, s.Executor, resolved, nil)
		cancelStream()
		s.ChannelLimits.ReleaseChannel(subject)
		s.Metrics.ChannelClosed()
		if err != nil {
			result = "failed"
			if lifetime.Err() == nil && streamCtx.Err() != nil {
				result = "revoked" // the watchdog, not the client, ended the stream
			}
			return err
		}
		// One SSH connection maps to one exec stream. This bounds resource use and
		// rejects attempts to multiplex extra shells over an authorized channel.
		result = "completed"
		// Let the client close the transport, as OpenSSH's sshd does (w6/096).
		// Output the exec wrote may still be in flight, in this socket's send
		// buffer or the edge proxy's; closing first while the client's window
		// adjustments sit unread resets the connection and drops that tail, so
		// `ssh host 'head -c 1M /dev/urandom'` delivered a fraction of it.
		select {
		case <-clientClosed:
		case <-lifetime.Done():
		case <-time.After(clientCloseGrace):
		}
		return nil
	}
}

// clientCloseGrace bounds how long a finished single-session connection waits
// for its client to disconnect, so a client that never closes cannot hold a
// session slot.
const clientCloseGrace = 30 * time.Second

// keepaliveRequest is OpenSSH's keepalive global request. Clients answer a
// request they do not know with a failure, which is an answer all the same.
const keepaliveRequest = "keepalive@openssh.com"

// defaultKeepaliveInterval and keepaliveMisses close a connection whose client
// has not answered for about three minutes, sshd's ClientAliveInterval 60 with
// ClientAliveCountMax 3.
const (
	defaultKeepaliveInterval = time.Minute
	keepaliveMisses          = 3
)

// keepalive asks the client every interval whether it is still there and
// closes the connection when an answer has not come within keepaliveMisses
// intervals (w6/174). Without it, a client that vanished without closing its
// connection (a laptop that slept on another network) would hold its session
// slot until the gateway's next release, since a service shell has no time
// limit. It returns when ctx ends or the connection does; a negative interval
// disables it.
func keepalive(ctx context.Context, conn ssh.Conn, interval time.Duration) {
	if interval <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		// SendRequest waits for the client's answer; closing the connection
		// releases it.
		answered := make(chan error, 1)
		go func() {
			_, _, err := conn.SendRequest(keepaliveRequest, true, nil)
			answered <- err
		}()
		select {
		case <-ctx.Done():
			return
		case err := <-answered:
			if err != nil {
				return // the connection is gone
			}
		case <-time.After(keepaliveMisses * interval):
			log.Printf("ssh gateway: closing a connection whose client stopped answering keepalives")
			_ = conn.Close()
			return
		}
	}
}

// serveMultiChannel serves an agent-session sandbox connection that may open
// several concurrent session channels (Zed's ControlMaster protocol, its
// terminals, and its tasks). Each channel runs the self-contained serveSession
// against its own pods/exec stream; the connection holds one session-limiter
// slot (acquired by the caller) while a per-connection semaphore bounds the
// channel fan-out and the shared ChannelLimiter bounds exec streams per
// identity and process-wide (codex round-8 #7). Every channel re-authorizes
// (codex round-8 #5): a key deletion, membership revocation, or session
// teardown that lands after transport auth tears the whole transport down —
// no new exec starts on a decision that may be hours stale. A connection that
// opens no channels (Zed's `-N` master) is valid and simply idles until the
// client closes it or the session times out.
//
// A held login (wait non-nil, w10/m597) accepts the channels opened before
// its session's sandbox is up without a target: each waits for it, then
// re-authorizes as any channel does before its command runs. A wait that
// ends without a sandbox ends the connection once they told the client why.
func (s *Server) serveMultiChannel(ctx context.Context, channels <-chan ssh.NewChannel, subject, fingerprint, connUser string, sticky *connectionTarget, wait *sandboxWait) string {
	// Cancellable view of the session ctx for the channel goroutines: a
	// revocation must end ACTIVE channels too, not just stop new ones. Defer
	// order is load-bearing (LIFO): mcancel fires BEFORE wg.Wait, so the wait
	// never blocks on channels that only end with their context.
	var wg sync.WaitGroup
	defer wg.Wait()
	// codex round-9 #6: the transport-level watchdog re-runs the channel-open
	// reauthorization on the interval, so a revocation ends EVERY live channel
	// at once (each new channel still rechecks on open, round-8 #5). One
	// watchdog per connection, not per channel.
	mctx, mcancel := sshgateway.WithRevalidation(ctx, s.RevalidateInterval, s.revalidator(subject, fingerprint, connUser, sticky, wait))
	defer mcancel()
	sem := make(chan struct{}, s.MaxChannelsPerConn)
	var waited <-chan struct{}
	if wait != nil {
		waited = wait.done
	}
	held := 0 // channels accepted while the login was held
	for {
		var newChannel ssh.NewChannel
		select {
		case <-mctx.Done():
			if ctx.Err() != nil {
				return "failed"
			}
			return "revoked" // the watchdog, not the client, ended the transport
		case <-waited:
			waited = nil
			var failure *wakeFailure
			if errors.As(wait.err, &failure) && held > 0 {
				endWithoutSandbox(mctx, channels, failure.reason)
			}
			if wait.err != nil {
				return "failed"
			}
			continue
		case channel, ok := <-channels:
			if !ok {
				return "completed"
			}
			newChannel = channel
		}
		if newChannel.ChannelType() != "session" {
			_ = newChannel.Reject(ssh.UnknownChannelType, "unsupported channel type")
			continue
		}
		select {
		case sem <- struct{}{}:
		default:
			// Over the per-connection channel budget: shed this channel without
			// tearing down the connection or its active channels.
			_ = newChannel.Reject(ssh.ResourceShortage, "channel limit reached")
			continue
		}
		// A channel opened while the login is held waits for its target.
		var resolved apps.SSHInstanceTarget
		var pending pendingTarget
		if wait.pending() {
			held++
			resolved, pending = sticky.Get(), s.pendingTarget(wait, subject, fingerprint, connUser, sticky)
		} else {
			var err error
			if resolved, err = s.reauthorize(mctx, subject, fingerprint, connUser, sticky); err != nil {
				<-sem
				_ = newChannel.Reject(ssh.Prohibited, channelRefusal(err))
				if definiteRefusal(err) {
					return "revoked"
				}
				// No answer refuses this channel only: the live ones stay under
				// the watchdog, which retries the same checks (w6/147).
				continue
			}
		}
		if ok, scope := s.ChannelLimits.AcquireChannel(subject); !ok {
			<-sem
			s.Metrics.LimitRejected(scope)
			_ = newChannel.Reject(ssh.ResourceShortage, "channel limit reached")
			continue
		}
		channel, channelRequests, err := newChannel.Accept()
		if err != nil {
			<-sem
			s.ChannelLimits.ReleaseChannel(subject)
			continue
		}
		s.Metrics.ChannelOpened()
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer s.Metrics.ChannelClosed()
			defer s.ChannelLimits.ReleaseChannel(subject)
			defer func() { <-sem }()
			_ = serveSession(mctx, channel, channelRequests, s.Executor, resolved, pending)
		}()
	}
}

// maxUnansweredRevalidations is how many revalidation ticks in a row may fail
// to get an answer before a live session ends anyway (w6/147): an outage of
// the key store, bex-api or OpenFGA cannot keep sessions open without checks
// for longer than this many intervals (3 minutes at the default 1 m).
const maxUnansweredRevalidations = 3

// reauthFailure is one failed reauthorization with its bounded cause (w6/147).
// A definite failure is an authoritative no: the key is gone or bound to
// another subject, the owner's email is unverified, the authorization denies,
// or the target is gone or refused. Any other failure means the gateway could
// not get an answer: a store, bex-api, OpenFGA or Kubernetes error or timeout.
// A channel open refuses on either; a live session's watchdog retries the
// second kind (revalidator).
type reauthFailure struct {
	cause    string
	definite bool
	err      error
}

func (f *reauthFailure) Error() string { return f.err.Error() }
func (f *reauthFailure) Unwrap() error { return f.err }

// definiteRefusal reports whether err is a reauthorization's authoritative no.
func definiteRefusal(err error) bool {
	var f *reauthFailure
	return errors.As(err, &f) && f.definite
}

// The bounded causes a reauthorization fails with, logged without the subject
// or the key (w6/147). A target refusal is logged under its own
// apps.ShellRefusal reason (suspended, ineligible, instance_gone).
const (
	causeKeyGone         = "key_gone"
	causeKeyLookup       = "key_lookup_failed"
	causeEmailUnverified = "email_unverified"
	causeEmailCheck      = "email_check_failed"
	causeDenied          = "denied"
	causeTargetNotFound  = "not_found"
	causeResolveFailed   = "resolve_failed"
	causeKeyGrantDenied  = "key_grant_denied"
	causeKeyGrantCheck   = "key_grant_check_failed"
)

// maxLoggedErrorLength bounds the dependency error an unanswered
// reauthorization logs.
const maxLoggedErrorLength = 200

// reauthorize re-runs, immediately before a channel is accepted, the two checks
// transport authentication made: the fingerprint's key still exists and still
// belongs to the subject, and the subject is still authorized for the requested
// target — the resolvers assert their relation UNCACHED (round-7 #7), so this
// is an authoritative decision, not a cached one (codex round-8 #5). It first
// resolves the connection's sticky instance ID, so a bare service username
// cannot randomly select a different Ready replica between the audit row and
// pods/exec. Only an exact-instance not-found result permits selecting a new
// Ready target; authorization and store failures remain fail closed. Every
// failure is a *reauthFailure, counted and logged with its cause (w6/147).
func (s *Server) reauthorize(ctx context.Context, subject, fingerprint, connUser string, sticky *connectionTarget) (apps.SSHInstanceTarget, error) {
	target, failure := s.reassert(ctx, subject, fingerprint, connUser, sticky)
	switch {
	case failure == nil:
		sticky.Set(target)
		s.Metrics.Reauthorization("accepted")
		return target, nil
	case failure.definite:
		s.Metrics.Reauthorization("rejected")
		log.Printf("ssh gateway: reauthorization rejected: %s", failure.cause)
	default:
		s.Metrics.Reauthorization("unavailable")
		if ctx.Err() == nil { // not the session itself ending mid-check
			log.Printf("ssh gateway: reauthorization unanswered: %s: %s", failure.cause, boundedError(failure.err))
		}
	}
	return apps.SSHInstanceTarget{}, failure
}

// reassert runs reauthorize's checks and classifies the first that fails.
func (s *Server) reassert(ctx context.Context, subject, fingerprint, connUser string, sticky *connectionTarget) (apps.SSHInstanceTarget, *reauthFailure) {
	lookupCtx, cancel := context.WithTimeout(ctx, s.HandshakeTimeout)
	defer cancel()
	registered, failure := s.rereadKey(lookupCtx, subject, fingerprint)
	if failure != nil {
		return apps.SSHInstanceTarget{}, failure
	}
	if err := s.verifiedOwner(ctx, subject); err != nil {
		if errors.Is(err, agentsession.ErrEmailUnverified) {
			return apps.SSHInstanceTarget{}, &reauthFailure{cause: causeEmailUnverified, definite: true, err: errUnverifiedOwner}
		}
		return apps.SSHInstanceTarget{}, &reauthFailure{cause: causeEmailCheck, err: fmt.Errorf("email verification: %w", err)}
	}
	authCtx := core.WithIdentity(lookupCtx, keyIdentity(registered))
	target, err := s.Apps.ResolveSSHSession(authCtx, sticky.Get().ID)
	if errors.Is(err, core.ErrNotFound) {
		target, err = s.Apps.ResolveSSHSession(authCtx, connUser)
	}
	if err != nil {
		return apps.SSHInstanceTarget{}, resolveFailure(err)
	}
	if sticky.keyAction != "" {
		if err := s.KeyGrants.RequireTicket(lookupCtx, subject, sticky.keyAction); err != nil {
			if errors.Is(err, core.ErrForbidden) {
				return apps.SSHInstanceTarget{}, &reauthFailure{cause: causeKeyGrantDenied, definite: true, err: err}
			}
			return apps.SSHInstanceTarget{}, &reauthFailure{cause: causeKeyGrantCheck, err: err}
		}
	}
	return target, nil
}

// rereadKey reads the connection's key again: still registered, and still
// its subject's.
func (s *Server) rereadKey(ctx context.Context, subject, fingerprint string) (store.SSHKey, *reauthFailure) {
	registered, err := s.Store.SSHKeyByFingerprint(ctx, fingerprint)
	switch {
	case err != nil && !errors.Is(err, store.ErrNotFound):
		return store.SSHKey{}, &reauthFailure{cause: causeKeyLookup, err: fmt.Errorf("public key lookup: %w", err)}
	case err != nil || registered.Subject == "" || registered.Subject != subject:
		return store.SSHKey{}, &reauthFailure{cause: causeKeyGone, definite: true, err: errors.New("public key rejected")}
	}
	return registered, nil
}

// keyOwnerAction asks, at authentication, whether the key's owner is a bex
// API key whose grant covers a shell on the target (w9/125): Services Read &
// write's one-off jobs on a service, as its shell ticket needs, and Sandbox
// execution on an agent session's sandbox, as every other command run in it
// needs. It returns the action a key owner's connection is held to, "" for a
// person or when no gate is wired.
func (s *Server) keyOwnerAction(ctx context.Context, subject, connUser string) (string, error) {
	if s.KeyGrants == nil {
		return "", nil
	}
	action := string(apps.ShellKeyAction)
	if sshgateway.AgentSessionUsername(connUser) {
		action = string(core.KeySandboxesExec)
	}
	key, err := s.KeyGrants.RequireSubject(ctx, subject, action)
	if err != nil || !key {
		return "", err
	}
	return action, nil
}

// toldRefusal is the refusal of resolving connUser that the client is told
// about, if any: a login requirement, or an agent session with no sandbox to
// open. The second is told to a key owner only when its grant covers the
// target, the check an admitted connection passes next (keyOwnerAction).
func (s *Server) toldRefusal(ctx context.Context, subject, connUser string, err error) (refusal, bool) {
	if r, ok := loginRefusal(err); ok {
		return r, true
	}
	r, ok := sessionRefusal(err)
	if !ok || !sshgateway.AgentSessionUsername(connUser) {
		return refusal{}, false
	}
	if _, err := s.keyOwnerAction(ctx, subject, connUser); err != nil {
		return refusal{}, false
	}
	return r, true
}

// resolveFailure classifies a ResolveSSHSession error the way the web shell's
// watchdogEndReason does: a runtime-gate refusal, a deny (a login requirement
// and a protected environment included) and a missing target are answers;
// anything else is the resolver failing closed on a dependency it could not
// reach.
func resolveFailure(err error) *reauthFailure {
	var refusal *apps.ShellRefusal
	switch {
	case errors.As(err, &refusal):
		return &reauthFailure{cause: refusal.Reason, definite: true, err: err}
	case errors.Is(err, core.ErrForbidden):
		return &reauthFailure{cause: causeDenied, definite: true, err: err}
	case errors.Is(err, core.ErrNotFound):
		return &reauthFailure{cause: causeTargetNotFound, definite: true, err: err}
	default:
		return &reauthFailure{cause: causeResolveFailed, err: err}
	}
}

// boundedError is err's text cut to maxLoggedErrorLength bytes.
func boundedError(err error) string {
	msg := err.Error()
	if len(msg) > maxLoggedErrorLength {
		msg = msg[:maxLoggedErrorLength] + "…"
	}
	return msg
}

// revalidator is a live session's watchdog check (codex round-9 #6). A
// definite refusal ends the session at once; a reauthorization that got no
// answer is retried on the next tick, until maxUnansweredRevalidations in a
// row end the session too (w6/147). The watchdog calls it from one goroutine.
// While a held login waits (w10/m597) it skips: the wait re-reads the key and
// the authorization itself, and nothing runs until a channel re-authorized.
func (s *Server) revalidator(subject, fingerprint, connUser string, sticky *connectionTarget, wait *sandboxWait) func(context.Context) error {
	unanswered := 0
	return func(ctx context.Context) error {
		if wait.pending() {
			return nil
		}
		_, err := s.reauthorize(ctx, subject, fingerprint, connUser, sticky)
		switch {
		case err == nil:
			unanswered = 0
			return nil
		case definiteRefusal(err):
			return err
		}
		unanswered++
		if unanswered < maxUnansweredRevalidations {
			return nil
		}
		log.Printf("ssh gateway: ending a session after %d revalidations in a row without an answer", unanswered)
		return err
	}
}

// connectionTarget is the instance selected at transport authentication. It is
// shared by all channels on a multiplexed connection and changes only when the
// exact sticky instance is no longer Ready and the resolver selects a fallback.
type connectionTarget struct {
	mu     sync.RWMutex
	target apps.SSHInstanceTarget
	// keyAction is fixed at authentication: the action a connection whose
	// key's owner is a bex API key is held to on every reauthorization
	// (w9/125), "" for a person's.
	keyAction string
}

func (t *connectionTarget) Get() apps.SSHInstanceTarget {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.target
}

func (t *connectionTarget) Set(target apps.SSHInstanceTarget) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.target = target
}

func rejectAdditionalChannels(channels <-chan ssh.NewChannel) {
	for channel := range channels {
		_ = channel.Reject(ssh.Prohibited, "only one session channel is allowed")
	}
}

func encodeTarget(target apps.SSHInstanceTarget) string {
	b, _ := json.Marshal(target)
	return string(b)
}

// decodeTarget reads the target the login was authorized for. A held login's
// (waking) names its session but no pod yet.
func decodeTarget(raw string, waking bool) (apps.SSHInstanceTarget, error) {
	var target apps.SSHInstanceTarget
	if err := json.Unmarshal([]byte(raw), &target); err != nil || target.ID == "" ||
		(target.PodName == "" && !(waking && target.Sandbox)) {
		return apps.SSHInstanceTarget{}, errors.New("invalid authorized target")
	}
	return target, nil
}

// sourceIP is addr's IP, the client's (PROXY-protocol-resolved for a trusted
// peer), as the audit log records it; "" when addr has none.
func sourceIP(addr net.Addr) string {
	ip, err := proxyproto.RemoteIP(addr)
	if err != nil {
		return ""
	}
	return ip.String()
}
