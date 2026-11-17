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

// Package kvinternalauth fronts one managed Key Value store. It holds the
// store's plan limit across its listeners (w6/m456) and, unless the store
// requires internal authentication (w5/m268, w6/m463), gives its internal
// connections the default user's ACL without asking for a password. The
// external TLS listener remains an unmodified, authenticated Valkey stream.
package kvinternalauth

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// maxClientsReply is Valkey's refusal of a client past its maxclients, byte
// for byte, which a client at the store's limit reads from this proxy too.
const maxClientsReply = "-ERR max number of clients reached\r\n"

// maxRefusing bounds the refusals one listener answers at once. Past it a
// refused client is closed, so a flood past the limit costs no more than this.
const maxRefusing = 16

// errUpstreamFull is the upstream's own maxclients refusal.
var errUpstreamFull = errors.New("kv-internal-proxy: upstream refused: max number of clients reached")

// Budget is the client connections the listeners in front of one store hold
// together: the store's plan limit, shared by its plain and TLS listeners
// (w6/m456).
type Budget struct{ slots chan struct{} }

// NewBudget holds at most n connections; n <= 0 holds 1,024.
func NewBudget(n int) *Budget {
	if n <= 0 {
		n = 1024
	}
	return &Budget{slots: make(chan struct{}, n)}
}

func (b *Budget) acquire() bool {
	select {
	case b.slots <- struct{}{}:
		return true
	default:
		return false
	}
}

func (b *Budget) release() { <-b.slots }

// Mode is what a listener does with its clients' bytes.
type Mode int

const (
	// InjectAuth authenticates each upstream connection as the default user
	// before forwarding the client's bytes, so internal clients need no
	// password (w8/204).
	InjectAuth Mode = iota
	// PlainPassthrough forwards a plain client's bytes unchanged, so the
	// client's own AUTH decides its access: the internal listener of a store
	// with internal authentication (w5/m268, w6/m463).
	PlainPassthrough
	// TLSPassthrough forwards a TLS client's bytes unchanged, including its
	// authentication: TLS ends in Valkey (the external path).
	TLSPassthrough
)

// Config identifies a single store and transport.
type Config struct {
	Upstream string
	// Password is the default user's, which only InjectAuth writes upstream.
	Password string
	Mode     Mode
	// Budget is shared by every listener in front of the store; nil gives
	// this listener a budget of its own (NewBudget(0)).
	Budget *Budget
	// TLSCertFile and TLSKeyFile are the store's own certificate. A
	// TLSPassthrough listener completes the handshake with it only to answer a
	// client past the budget with Valkey's refusal, which Valkey itself cannot
	// do before a handshake. They are read at each refusal, so a renewed
	// certificate applies at once; without them the client is closed.
	TLSCertFile      string
	TLSKeyFile       string
	HandshakeTimeout time.Duration
}

func (c Config) normalized() (Config, error) {
	if _, _, err := net.SplitHostPort(c.Upstream); err != nil {
		return c, errors.New("kv-internal-proxy: upstream must be a host:port")
	}
	switch c.Mode {
	case InjectAuth:
		if c.Password == "" {
			return c, errors.New("kv-internal-proxy: internal password is required")
		}
	case PlainPassthrough, TLSPassthrough:
	default:
		return c, errors.New("kv-internal-proxy: unknown listener mode")
	}
	if c.Budget == nil {
		c.Budget = NewBudget(0)
	}
	if c.HandshakeTimeout <= 0 {
		c.HandshakeTimeout = 5 * time.Second
	}
	return c, nil
}

// Serve owns listener and closes it and all accepted streams on cancellation.
// A client past the budget is refused before an upstream is dialed, with the
// reply Valkey gives at its own maxclients.
// Only dial and the injected AUTH have deadlines: Redis blocking commands,
// pubsub and idle connection pools may keep a stream open indefinitely.
func Serve(ctx context.Context, listener net.Listener, config Config) error {
	defer func() { _ = listener.Close() }()
	config, err := config.normalized()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var mu sync.Mutex
	active := make(map[net.Conn]struct{})
	var handlers sync.WaitGroup
	refusing := make(chan struct{}, maxRefusing)
	shutdownDone := make(chan struct{})
	context.AfterFunc(ctx, func() {
		defer close(shutdownDone)
		_ = listener.Close()
		mu.Lock()
		defer mu.Unlock()
		for conn := range active {
			_ = conn.Close()
		}
	})
	defer func() { cancel(); <-shutdownDone; handlers.Wait() }()
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		if ctx.Err() != nil {
			_ = conn.Close()
			return nil
		}
		if !config.Budget.acquire() {
			select {
			case refusing <- struct{}{}:
				handlers.Go(func() {
					defer func() { <-refusing }()
					refuse(ctx, conn, config)
				})
			default:
				_ = conn.Close()
			}
			continue
		}
		mu.Lock()
		if ctx.Err() != nil {
			mu.Unlock()
			config.Budget.release()
			_ = conn.Close()
			return nil
		}
		active[conn] = struct{}{}
		mu.Unlock()
		handlers.Go(func() {
			defer func() {
				_ = conn.Close()
				mu.Lock()
				delete(active, conn)
				mu.Unlock()
				config.Budget.release()
			}()
			proxy(ctx, conn, config)
		})
	}
}

// Check verifies that the upstream accepts the default user's credentials and
// can serve a command, rather than just TCP: an authenticated PING must read
// +PONG, so a loading server (-LOADING) is not ready (w1/m166). Every mode
// checks this way, so a proxy whose clients authenticate themselves still
// holds the password, for this check only (w6/m463): Valkey answers an
// unauthenticated PING with -NOAUTH before it looks at loading, so a check
// without it could not tell a loading server from a ready one.
func Check(ctx context.Context, config Config) error {
	config.Mode = InjectAuth
	config, err := config.normalized()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, config.HandshakeTimeout)
	defer cancel()
	upstream, err := connect(ctx, config)
	if err != nil {
		return err
	}
	defer func() { _ = upstream.Close() }()
	stop := context.AfterFunc(ctx, func() { _ = upstream.Close() })
	defer stop()
	deadline, _ := ctx.Deadline()
	if err := upstream.SetDeadline(deadline); err != nil {
		return errors.New("kv-internal-proxy: upstream check failed")
	}
	if _, err := io.WriteString(upstream, "*1\r\n$4\r\nPING\r\n"); err != nil {
		return errors.New("kv-internal-proxy: upstream check failed")
	}
	var reply [7]byte
	if _, err := io.ReadFull(upstream, reply[:]); err != nil || string(reply[:]) != "+PONG\r\n" {
		return errors.New("kv-internal-proxy: upstream check failed")
	}
	return nil
}

func connect(ctx context.Context, config Config) (net.Conn, error) {
	handshake, cancel := context.WithTimeout(ctx, config.HandshakeTimeout)
	defer cancel()
	upstream, err := (&net.Dialer{}).DialContext(handshake, "tcp", config.Upstream)
	if err != nil {
		return nil, errors.New("kv-internal-proxy: upstream connection failed")
	}
	stop := context.AfterFunc(handshake, func() { _ = upstream.Close() })
	defer stop()
	success := false
	defer func() {
		if !success {
			_ = upstream.Close()
		}
	}()
	if config.Mode == InjectAuth {
		deadline, _ := handshake.Deadline()
		if err := upstream.SetDeadline(deadline); err != nil {
			return nil, errors.New("kv-internal-proxy: upstream authentication failed")
		}
		// Bulk lengths preserve every password byte. Never use the platform user:
		// tenant commands must retain the default user's command restrictions.
		auth := fmt.Sprintf("*3\r\n$4\r\nAUTH\r\n$7\r\ndefault\r\n$%d\r\n%s\r\n", len(config.Password), config.Password)
		if _, err := io.WriteString(upstream, auth); err != nil {
			return nil, errors.New("kv-internal-proxy: upstream authentication failed")
		}
		// Read exactly +OK, without buffering any following stream bytes or
		// exposing the injected reply. Valkey's maxclients refusal, written
		// before it reads the AUTH, is reported for the client; every other
		// answer fails closed.
		var reply [5]byte
		if _, err := io.ReadFull(upstream, reply[:]); err != nil || string(reply[:]) != "+OK\r\n" {
			if err == nil && upstreamFull(upstream, reply[:]) {
				return nil, errUpstreamFull
			}
			return nil, errors.New("kv-internal-proxy: upstream authentication failed")
		}
		if err := upstream.SetDeadline(time.Time{}); err != nil {
			return nil, errors.New("kv-internal-proxy: upstream authentication failed")
		}
	}
	if handshake.Err() != nil {
		return nil, errors.New("kv-internal-proxy: upstream connection failed")
	}
	success = true
	return upstream, nil
}

// upstreamFull reports whether the upstream's reply, of which head has been
// read, is Valkey's maxclients refusal.
func upstreamFull(upstream net.Conn, head []byte) bool {
	if string(head) != maxClientsReply[:len(head)] {
		return false
	}
	rest := make([]byte, len(maxClientsReply)-len(head))
	_, err := io.ReadFull(upstream, rest)
	return err == nil && string(rest) == maxClientsReply[len(head):]
}

// refuse answers a client past the budget as Valkey answers one past its
// maxclients, then closes it: in plain text on a plain listener, whether the
// proxy or the client authenticates, and inside TLS, which needs the store's
// certificate, on a TLS one.
func refuse(ctx context.Context, conn net.Conn, config Config) {
	defer func() { _ = conn.Close() }()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	if err := conn.SetDeadline(time.Now().Add(config.HandshakeTimeout)); err != nil {
		return
	}
	reply := conn
	if config.Mode == TLSPassthrough {
		if config.TLSCertFile == "" {
			return
		}
		cert, err := tls.LoadX509KeyPair(config.TLSCertFile, config.TLSKeyFile)
		if err != nil {
			return
		}
		server := tls.Server(conn, &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12})
		if server.Handshake() != nil {
			return
		}
		defer func() { _ = server.Close() }()
		reply = server
	}
	_, _ = io.WriteString(reply, maxClientsReply)
}

func proxy(ctx context.Context, client net.Conn, config Config) {
	upstream, err := connect(ctx, config)
	if errors.Is(err, errUpstreamFull) {
		_ = client.SetWriteDeadline(time.Now().Add(config.HandshakeTimeout))
		_, _ = io.WriteString(client, maxClientsReply)
	}
	if err != nil {
		return
	}
	defer func() { _ = upstream.Close() }()
	stop := context.AfterFunc(ctx, func() { _ = upstream.Close() })
	defer stop()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, err := io.Copy(upstream, client)
		if err != nil {
			_ = upstream.Close()
			return
		}
		// A client may half-close after a pipeline and still read its replies.
		if tcp, ok := upstream.(*net.TCPConn); ok {
			_ = tcp.CloseWrite()
		} else {
			_ = upstream.Close()
		}
	}()
	_, _ = io.Copy(client, upstream)
	_ = client.Close()
	_ = upstream.Close()
	<-done
}
