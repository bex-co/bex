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

// Package kvinternalauth gives a workspace's internal Key Value connections
// the default user's ACL without asking internal clients for a password. The
// external TLS listener remains an unmodified, authenticated Valkey stream.
package kvinternalauth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// Config identifies a single store and transport. Passthrough is exclusively
// for the external TLS path: its bytes, including client authentication, are
// forwarded without interpretation.
type Config struct {
	Upstream         string
	Password         string
	Passthrough      bool
	MaxConnections   int
	HandshakeTimeout time.Duration
}

func (c Config) normalized() (Config, error) {
	if _, _, err := net.SplitHostPort(c.Upstream); err != nil {
		return c, errors.New("kv-internal-proxy: upstream must be a host:port")
	}
	if !c.Passthrough && c.Password == "" {
		return c, errors.New("kv-internal-proxy: internal password is required")
	}
	if c.MaxConnections <= 0 {
		c.MaxConnections = 1024
	}
	if c.HandshakeTimeout <= 0 {
		c.HandshakeTimeout = 5 * time.Second
	}
	return c, nil
}

// Serve owns listener and closes it and all accepted streams on cancellation.
// It sheds excess connections immediately, before allocating an upstream.
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
		mu.Lock()
		if ctx.Err() != nil || len(active) >= config.MaxConnections {
			mu.Unlock()
			_ = conn.Close()
			if ctx.Err() != nil {
				return nil
			}
			continue
		}
		active[conn] = struct{}{}
		mu.Unlock()
		handlers.Go(func() {
			defer func() {
				_ = conn.Close()
				mu.Lock()
				delete(active, conn)
				mu.Unlock()
			}()
			proxy(ctx, conn, config)
		})
	}
}

// Check verifies that the upstream accepts the same default-user credentials
// the internal front injects and can serve a command, rather than just TCP.
func Check(ctx context.Context, config Config) error {
	config, err := config.normalized()
	if err != nil {
		return err
	}
	if config.Passthrough {
		return errors.New("kv-internal-proxy: a check requires internal authentication")
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
	if !config.Passthrough {
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
		// exposing the injected reply. Every other answer fails closed.
		var reply [5]byte
		if _, err := io.ReadFull(upstream, reply[:]); err != nil || string(reply[:]) != "+OK\r\n" {
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

func proxy(ctx context.Context, client net.Conn, config Config) {
	upstream, err := connect(ctx, config)
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
