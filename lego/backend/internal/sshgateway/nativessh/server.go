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
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const (
	permissionSubject = "bex.subject"
	permissionTarget  = "bex.target"
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

	HandshakeTimeout time.Duration
	SessionTimeout   time.Duration

	// MaxChannelsPerConn bounds concurrent session channels on ONE connection for
	// agent-session sandbox targets (ADR054 D3), which Zed's ControlMaster remoting
	// multiplexes. 0 disables the exception: sandbox targets fall back to the
	// single-channel App path. App (srv-…) targets are always single-channel
	// regardless of this value.
	MaxChannelsPerConn int
}

func (s *Server) defaults() {
	if s.HandshakeTimeout <= 0 {
		s.HandshakeTimeout = 10 * time.Second
	}
	if s.SessionTimeout <= 0 {
		s.SessionTimeout = 4 * time.Hour
	}
	if s.Limits == nil {
		s.Limits = sshgateway.NewSessionLimiter(0, 0)
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
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		go func() {
			if err := s.serveConn(ctx, conn, config); err != nil && ctx.Err() == nil {
				log.Printf("ssh gateway connection ended: %v", err)
			}
		}()
	}
}

func (s *Server) serveConn(ctx context.Context, raw net.Conn, config *ssh.ServerConfig) error {
	defer raw.Close()
	_ = raw.SetDeadline(time.Now().Add(s.HandshakeTimeout))
	conn, channels, requests, err := ssh.NewServerConn(raw, config)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = raw.SetDeadline(time.Time{})

	subject := conn.Permissions.Extensions[permissionSubject]
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
		result = s.serveMultiChannel(sessionCtx, channels, target)
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
		channel, channelRequests, err := newChannel.Accept()
		if err != nil {
			result = "failed"
			return err
		}
		// The connection is deliberately single-session. Drain the connection's
		// channel stream while the accepted exec is active so a client cannot
		// queue a second shell behind a long-running first one and consume server
		// resources indefinitely. Closing conn below ends this goroutine.
		go rejectAdditionalChannels(channels)
		if err := serveSession(sessionCtx, channel, channelRequests, s.Executor, target); err != nil {
			result = "failed"
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
// channel fan-out. A connection that opens no channels (Zed's `-N` master) is
// valid and simply idles until the client closes it or the session times out.
func (s *Server) serveMultiChannel(ctx context.Context, channels <-chan ssh.NewChannel, target apps.SSHInstanceTarget) string {
	sem := make(chan struct{}, s.MaxChannelsPerConn)
	var wg sync.WaitGroup
	defer wg.Wait()
	for {
		var newChannel ssh.NewChannel
		select {
		case <-ctx.Done():
			return "failed"
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
		channel, channelRequests, err := newChannel.Accept()
		if err != nil {
			<-sem
			continue
		}
		s.Metrics.ChannelOpened()
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			_ = serveSession(ctx, channel, channelRequests, s.Executor, target)
		}()
	}
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
