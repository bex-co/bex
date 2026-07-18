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

// Package sshgateway authenticates registered public keys, authorizes their
// identities against a requested App, and bridges SSH session channels to the
// Kubernetes pods/exec API. It is deployed separately from bex-api so only
// this process and ServiceAccount receive pods/exec permission.
package sshgateway

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
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const (
	permissionSubject = "bex.subject"
	permissionTarget  = "bex.target"
)

// Store is the gateway's deliberately small database authority: authenticate
// a fingerprint and write content-free session audit facts.
type Store interface {
	SSHKeyByFingerprint(context.Context, string) (store.SSHKey, error)
	StartSSHSession(context.Context, store.SSHSessionAudit) error
	EndSSHSession(context.Context, string, string, time.Time) error
}

type TargetResolver interface {
	ResolveSSHSession(context.Context, string) (apps.SSHInstanceTarget, error)
}

type Server struct {
	Store    Store
	Apps     TargetResolver
	Executor Executor
	Signer   ssh.Signer
	Metrics  *Metrics

	// TicketSecret enables the Browser Web Shell WebSocket path (websocket.go,
	// docs/ADR035-ssh.md § Browser Web Shell). It must equal bex-api's
	// BEX_SHELL_TICKET_SECRET so the gateway can verify the exec tickets bex-api
	// mints. Empty => the WebSocket listener is not started (native SSH only).
	TicketSecret []byte

	HandshakeTimeout time.Duration
	SessionTimeout   time.Duration
	MaxSessions      int
	MaxPerIdentity   int

	mu          sync.Mutex
	global      int
	perIdentity map[string]int

	// usedNonces enforces best-effort single-use of exec tickets on this replica:
	// a ticket's nonce is recorded until the ticket's own expiry, so a replay
	// within the short TTL is refused. Cross-replica reuse is bounded by the TTL.
	usedMu     sync.Mutex
	usedNonces map[string]time.Time
}

func (s *Server) defaults() {
	if s.HandshakeTimeout <= 0 {
		s.HandshakeTimeout = 10 * time.Second
	}
	if s.SessionTimeout <= 0 {
		s.SessionTimeout = 4 * time.Hour
	}
	if s.MaxSessions <= 0 {
		s.MaxSessions = 100
	}
	if s.MaxPerIdentity <= 0 {
		s.MaxPerIdentity = 5
	}
	if s.perIdentity == nil {
		s.perIdentity = map[string]int{}
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
			s.Metrics.authentication("rejected_key")
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
			s.Metrics.authentication("rejected_key")
			return nil, errors.New("public key rejected")
		}
		authCtx := core.WithIdentity(ctx, core.Identity{Subject: subject, Method: "ssh"})
		authCtx, cancel := context.WithTimeout(authCtx, s.HandshakeTimeout)
		defer cancel()
		target, err := s.Apps.ResolveSSHSession(authCtx, conn.User())
		if err != nil {
			s.Metrics.authentication("rejected_target")
			return nil, errors.New("public key rejected")
		}
		s.Metrics.authentication("accepted")
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
	acquired, limitScope := s.acquire(subject)
	if !acquired {
		s.Metrics.limitRejected(limitScope)
		return errors.New("session limit reached")
	}
	defer s.release(subject)

	sessionID := ids.New(ids.SSHSession)
	started := time.Now().UTC()
	s.Metrics.sessionStarted()
	result := "closed"
	defer func() { s.Metrics.sessionEnded(result, time.Since(started)) }()
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

func rejectAdditionalChannels(channels <-chan ssh.NewChannel) {
	for channel := range channels {
		_ = channel.Reject(ssh.Prohibited, "only one session channel is allowed")
	}
}

func (s *Server) acquire(subject string) (bool, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.global >= s.MaxSessions {
		return false, "global"
	}
	if s.perIdentity[subject] >= s.MaxPerIdentity {
		return false, "identity"
	}
	s.global++
	s.perIdentity[subject]++
	return true, ""
}

func (s *Server) release(subject string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.global--
	s.perIdentity[subject]--
	if s.perIdentity[subject] == 0 {
		delete(s.perIdentity, subject)
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
