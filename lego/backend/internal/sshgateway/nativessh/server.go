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

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/proxyproto"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const (
	permissionSubject     = "bex.subject"
	permissionTarget      = "bex.target"
	permissionFingerprint = "bex.fingerprint"
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
	SessionTimeout   time.Duration

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
	// of waiting for a disconnect or the 4h session cap. 0 => the platform
	// default (sshgateway.DefaultRevalidateInterval); negative disables the
	// watchdog (the pre-round-9 admission-only behavior).
	RevalidateInterval time.Duration

	// TrustedProxies lists the immediate peers (Traefik's pod network) allowed
	// to assert a PROXY protocol v1/v2 original-client address on the ssh
	// entrypoint's forwarded connection. Empty (the default) leaves every
	// connection's RemoteAddr as the immediate TCP peer, unchanged — the same
	// gap w4/029.md #10 reported (ssh_sessions.remote_address recording
	// Traefik's own pod IP). Set it once lego/operator/config/ssh/
	// ingressroutetcp.yaml's service carries proxyProtocol.
	TrustedProxies []netip.Prefix
}

func (s *Server) defaults() {
	if s.HandshakeTimeout <= 0 {
		s.HandshakeTimeout = 10 * time.Second
	}
	if s.SessionTimeout <= 0 {
		s.SessionTimeout = 4 * time.Hour
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
		authCtx := core.WithIdentity(ctx, core.Identity{Subject: subject, Method: "ssh"})
		authCtx, cancel := context.WithTimeout(authCtx, s.HandshakeTimeout)
		defer cancel()
		target, err := s.Apps.ResolveSSHSession(authCtx, conn.User())
		if err != nil {
			s.Metrics.Authentication("rejected_target")
			return nil, errors.New("public key rejected")
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

// Serve accepts until ctx is cancelled. Individual connections inherit that
// cancellation and therefore cannot outlive a terminating gateway process.
func (s *Server) Serve(ctx context.Context, listener net.Listener) error {
	config, err := s.config(ctx)
	if err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()
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
			if ctx.Err() != nil {
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
		go func() {
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

func (s *Server) serveConn(ctx context.Context, raw net.Conn, config *ssh.ServerConfig, releasePreAuth func(), preAuthSources *sshgateway.SourceLimiter) error {
	defer raw.Close()
	_ = raw.SetDeadline(time.Now().Add(s.HandshakeTimeout))
	transport, err := proxyproto.Wrap(raw, s.TrustedProxies)
	if err != nil {
		releasePreAuth()
		return err
	}
	// Round-11 #2: per-source fairness inside the global pre-auth pool. The
	// wrap has resolved the real client (Traefik-asserted for trusted peers, the
	// immediate peer otherwise); a source at its cap is shed here, before the
	// handshake reads anything.
	source := ""
	if addr, err := proxyproto.RemoteIP(transport.RemoteAddr()); err == nil {
		source = addr.String()
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
		releasePreAuth()
		releaseSource()
		return err
	}
	defer conn.Close()
	_ = raw.SetDeadline(time.Time{})
	// Handshake done: the authenticated session limiter now bounds this
	// connection, so free the pre-auth slots for the next incoming handshake.
	releasePreAuth()
	releaseSource()

	subject := conn.Permissions.Extensions[permissionSubject]
	fingerprint := conn.Permissions.Extensions[permissionFingerprint]
	connUser := conn.User()
	target, err := decodeTarget(conn.Permissions.Extensions[permissionTarget])
	if err != nil {
		return err
	}
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
	workspaceID := target.OwnerID
	if workspaceID == "" {
		workspaceID = core.DefaultTenant
	}
	auditCtx, cancelAudit := context.WithTimeout(context.Background(), 2*time.Second)
	err = s.Store.StartSSHSession(auditCtx, store.SSHSessionAudit{
		ID: sessionID, Subject: subject, WorkspaceID: workspaceID,
		ServiceID: target.ServiceID, InstanceID: target.ID,
		RemoteAddress: conn.RemoteAddr().String(), StartedAt: started,
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

	go ssh.DiscardRequests(requests)
	sessionCtx, cancel := context.WithTimeout(ctx, s.SessionTimeout)
	defer cancel()

	// An agent-session sandbox target (ADR054 D3) accepts Zed's multiplexed
	// remoting: many concurrent session channels over one connection, each its own
	// pods/exec stream, bounded per connection. Every other target — and a sandbox
	// target when the cap is disabled (0) — keeps the single-channel contract.
	if target.Sandbox && s.MaxChannelsPerConn > 0 {
		result = s.serveMultiChannel(sessionCtx, channels, subject, fingerprint, connUser)
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
		// hours (BEX_SSH_SESSION_TIMEOUT); re-read the key and re-resolve +
		// re-authorize (uncached) the target immediately before accepting the
		// channel. A revocation after transport auth ends the transport, not just
		// this channel — an already-open channel is a stream in flight, but no
		// NEW exec may start against a decision that is hours stale.
		resolved, err := s.reauthorize(ctx, subject, fingerprint, connUser)
		if err != nil {
			_ = newChannel.Reject(ssh.Prohibited, "authorization no longer valid")
			result = "revoked"
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
		// resources indefinitely. Closing conn below ends this goroutine.
		go rejectAdditionalChannels(channels)
		// codex round-9 #6: keep re-running the channel-open reauthorization
		// while the exec is LIVE, not only at admission — a revocation cancels
		// the stream's context from below. sessionCtx still bounds the lifetime.
		streamCtx, cancelStream := sshgateway.WithRevalidation(sessionCtx, s.RevalidateInterval, func(c context.Context) error {
			_, err := s.reauthorize(c, subject, fingerprint, connUser)
			return err
		})
		err = serveSession(streamCtx, channel, channelRequests, s.Executor, resolved)
		cancelStream()
		s.ChannelLimits.ReleaseChannel(subject)
		s.Metrics.ChannelClosed()
		if err != nil {
			result = "failed"
			if sessionCtx.Err() == nil && streamCtx.Err() != nil {
				result = "revoked" // the watchdog, not the client, ended the stream
			}
			return err
		}
		// One SSH connection maps to one exec stream. This bounds resource use and
		// rejects attempts to multiplex extra shells over an authorized channel.
		result = "completed"
		return nil
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
func (s *Server) serveMultiChannel(ctx context.Context, channels <-chan ssh.NewChannel, subject, fingerprint, connUser string) string {
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
	mctx, mcancel := sshgateway.WithRevalidation(ctx, s.RevalidateInterval, func(c context.Context) error {
		_, err := s.reauthorize(c, subject, fingerprint, connUser)
		return err
	})
	defer mcancel()
	sem := make(chan struct{}, s.MaxChannelsPerConn)
	for {
		var newChannel ssh.NewChannel
		select {
		case <-mctx.Done():
			if ctx.Err() != nil {
				return "failed"
			}
			return "revoked" // the watchdog, not the client, ended the transport
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
		resolved, err := s.reauthorize(mctx, subject, fingerprint, connUser)
		if err != nil {
			<-sem
			_ = newChannel.Reject(ssh.Prohibited, "authorization no longer valid")
			return "revoked"
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
			_ = serveSession(mctx, channel, channelRequests, s.Executor, resolved)
		}()
	}
}

// reauthorize re-runs, immediately before a channel is accepted, the two checks
// transport authentication made: the fingerprint's key still exists and still
// belongs to the subject, and the subject is still authorized for the requested
// target — the resolvers assert their relation UNCACHED (round-7 #7), so this
// is an authoritative decision, not a cached one (codex round-8 #5). It returns
// the CURRENT target so each channel execs against the live pod, never the
// transport-auth-time snapshot. Any failure — key gone, membership revoked,
// target gone, or the store unreachable (fail closed, like every fresh check) —
// ends the transport.
func (s *Server) reauthorize(ctx context.Context, subject, fingerprint, connUser string) (apps.SSHInstanceTarget, error) {
	lookupCtx, cancel := context.WithTimeout(ctx, s.HandshakeTimeout)
	defer cancel()
	registered, err := s.Store.SSHKeyByFingerprint(lookupCtx, fingerprint)
	if err != nil || registered.Subject == "" || registered.Subject != subject {
		s.Metrics.Reauthorization("rejected")
		return apps.SSHInstanceTarget{}, errors.New("public key rejected")
	}
	authCtx := core.WithIdentity(lookupCtx, core.Identity{Subject: subject, Method: "ssh"})
	target, err := s.Apps.ResolveSSHSession(authCtx, connUser)
	if err != nil {
		s.Metrics.Reauthorization("rejected")
		return apps.SSHInstanceTarget{}, err
	}
	s.Metrics.Reauthorization("accepted")
	return target, nil
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

func decodeTarget(raw string) (apps.SSHInstanceTarget, error) {
	var target apps.SSHInstanceTarget
	if err := json.Unmarshal([]byte(raw), &target); err != nil || target.PodName == "" || target.ID == "" {
		return apps.SSHInstanceTarget{}, errors.New("invalid authorized target")
	}
	return target, nil
}
