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
// external TLS listener remains an unmodified, authenticated Valkey stream,
// and an external client the store's allow list refuses is answered by a
// listener of its own that never reaches Valkey (w8/m562).
package kvinternalauth

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
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
// (w6/m456). It knows each stream it admitted, so a draining proxy can close
// some of them to keep the store's platform reserve free (KeepReserve).
type Budget struct {
	limit   int
	mu      sync.Mutex
	streams map[net.Conn]struct{}
}

// NewBudget holds at most n connections; n <= 0 holds 1,024.
func NewBudget(n int) *Budget {
	if n <= 0 {
		n = 1024
	}
	return &Budget{limit: n, streams: make(map[net.Conn]struct{})}
}

func (b *Budget) admit(conn net.Conn) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.streams) >= b.limit {
		return false
	}
	b.streams[conn] = struct{}{}
	return true
}

func (b *Budget) release(conn net.Conn) {
	b.mu.Lock()
	delete(b.streams, conn)
	b.mu.Unlock()
}

// shed closes up to n of the budget's streams and gives their slots back.
func (b *Budget) shed(n int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for conn := range b.streams {
		if n <= 0 {
			return
		}
		_ = conn.Close()
		delete(b.streams, conn)
		n--
	}
}

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
	// AllowListRefusal serves the external clients the store's allow list
	// refuses (w8/m562): it ends TLS with the store's certificate and answers
	// every command with Render's refusal (allowListRefusalReply). It has no
	// upstream and holds no password, so nothing a refused client sends can
	// reach Valkey, and it draws nothing from the budget.
	AllowListRefusal
)

// allowListRefusalReply is what a client the store's allow list refuses reads
// for each command it sends. valkey-cli prints a refused opening AUTH as
// "AUTH failed: <text>", Render's documented "AUTH failed: Client IP address
// is not in the allowlist." (render.com/docs/key-value § Enabling external
// connections, read 2026-11-22), and any other command's as "(error) <text>".
const allowListRefusalReply = "-Client IP address is not in the allowlist.\r\n"

// A refused client's commands are read only to answer each one, within
// Valkey's bounds for a client that has not authenticated: at most 10
// arguments of at most 16 KiB each (networking.c, "unauthenticated multibulk
// length" and "unauthenticated bulk length"), or an inline line of at most 16
// KiB. Past them, or past maxRefusedCommands, the client is closed.
const (
	maxRefusedCommands = 64
	maxRefusedArgs     = 10
	maxRefusedArgBytes = 16 << 10
)

var errRefusedCommand = errors.New("kv-internal-proxy: malformed or oversized command")

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
	// do before a handshake, and an AllowListRefusal listener to answer every
	// client. They are read at each refusal, so a renewed certificate applies
	// at once; without them the client is closed.
	TLSCertFile      string
	TLSKeyFile       string
	HandshakeTimeout time.Duration
	// Drain, once closed, starts this listener's drain (w6/m512): it stops
	// accepting, and every open stream keeps relaying until its client or the
	// upstream closes it. Streams still open DrainTimeout later close as on
	// cancellation, so a zero DrainTimeout closes them at once. Nil never
	// drains.
	Drain        <-chan struct{}
	DrainTimeout time.Duration
}

func (c Config) normalized() (Config, error) {
	switch c.Mode {
	case AllowListRefusal:
		// It refuses every client, so it has nothing to dial and no password
		// to hold: a configuration naming either is a mistake.
		if c.Upstream != "" || c.Password != "" {
			return c, errors.New("kv-internal-proxy: an allow-list refusal listener has no upstream or password")
		}
		if c.TLSCertFile == "" || c.TLSKeyFile == "" {
			return c, errors.New("kv-internal-proxy: an allow-list refusal listener requires the store's certificate")
		}
	case InjectAuth, PlainPassthrough, TLSPassthrough:
		if _, _, err := net.SplitHostPort(c.Upstream); err != nil {
			return c, errors.New("kv-internal-proxy: upstream must be a host:port")
		}
		if c.Mode == InjectAuth && c.Password == "" {
			return c, errors.New("kv-internal-proxy: internal password is required")
		}
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
// reply Valkey gives at its own maxclients. An AllowListRefusal listener
// refuses every client, outside the budget.
// Only dial and the injected AUTH have deadlines: Redis blocking commands,
// pubsub and idle connection pools may keep a stream open indefinitely.
// Once config.Drain closes, the listener closes and Serve returns as soon as
// its last stream ends, or at the drain's deadline, when it closes the rest.
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
	go func() {
		select {
		case <-config.Drain:
		case <-ctx.Done():
			return
		}
		_ = listener.Close()
		deadline := time.AfterFunc(config.DrainTimeout, cancel)
		<-ctx.Done()
		deadline.Stop()
	}()
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if draining(config.Drain) {
				handlers.Wait()
				return nil
			}
			return err
		}
		if ctx.Err() != nil {
			_ = conn.Close()
			return nil
		}
		if config.Mode == AllowListRefusal || !config.Budget.admit(conn) {
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
			config.Budget.release(conn)
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
				config.Budget.release(conn)
			}()
			proxy(ctx, conn, config)
		})
	}
}

// draining reports whether drain is closed; a nil drain never is.
func draining(drain <-chan struct{}) bool {
	select {
	case <-drain:
		return true
	default:
		return false
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

// reserveInterval is how often a draining proxy reads its store's client
// count (KeepReserve).
const reserveInterval = 100 * time.Millisecond

// KeepReserve keeps the store's platform reserve free while this proxy drains
// (w6/m512, ADR021 § 9). Valkey admits the plan's limit plus a reserve of 10
// for the readiness probes, the exporter and the operator. A draining pod
// accepts nothing new, but a new pod admits up to the limit on a budget of
// its own, so tenants who dial while the old pod still holds their streams
// could take the reserve. Until ctx ends, KeepReserve reads the store's
// connected clients (INFO clients, as the default user, whatever the
// listeners' mode) and, while there are more than the budget's limit besides
// its own connection, closes that many of the streams the budget still holds.
// Those would close at the drain's deadline anyway.
func KeepReserve(ctx context.Context, config Config, budget *Budget) {
	config.Mode = InjectAuth
	config, err := config.normalized()
	if err != nil {
		return
	}
	var upstream net.Conn
	var reader *bufio.Reader
	defer func() {
		if upstream != nil {
			_ = upstream.Close()
		}
	}()
	tick := time.NewTicker(reserveInterval)
	defer tick.Stop()
	for ; ; <-tick.C {
		if ctx.Err() != nil {
			return
		}
		if upstream == nil {
			conn, err := connect(ctx, config)
			if errors.Is(err, errUpstreamFull) {
				// The store is past its limit already: the reserve is taken.
				budget.shed(1)
				continue
			}
			if err != nil {
				continue
			}
			upstream, reader = conn, bufio.NewReader(conn)
		}
		connected, err := connectedClients(upstream, reader, config.HandshakeTimeout)
		if err != nil {
			_ = upstream.Close()
			upstream = nil
			continue
		}
		if over := connected - 1 - budget.limit; over > 0 {
			budget.shed(over)
		}
	}
}

// connectedClients asks upstream for INFO clients and reads connected_clients.
func connectedClients(upstream net.Conn, reader *bufio.Reader, timeout time.Duration) (int, error) {
	if err := upstream.SetDeadline(time.Now().Add(timeout)); err != nil {
		return 0, err
	}
	if _, err := io.WriteString(upstream, "*2\r\n$4\r\nINFO\r\n$7\r\nclients\r\n"); err != nil {
		return 0, err
	}
	head, err := reader.ReadString('\n')
	if err != nil {
		return 0, err
	}
	size, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(head, "$"), "\r\n"))
	if !strings.HasPrefix(head, "$") || err != nil || size < 0 || size > 64<<10 {
		return 0, errors.New("kv-internal-proxy: unexpected INFO reply")
	}
	body := make([]byte, size+2)
	if _, err := io.ReadFull(reader, body); err != nil {
		return 0, err
	}
	for line := range strings.SplitSeq(string(body), "\r\n") {
		if value, ok := strings.CutPrefix(line, "connected_clients:"); ok {
			return strconv.Atoi(value)
		}
	}
	return 0, errors.New("kv-internal-proxy: INFO reply has no connected_clients")
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
// certificate, on a TLS one. An AllowListRefusal listener's client reads
// Render's refusal inside TLS for each command (answerAllowListRefusals).
// Either way the connection's whole life is config.HandshakeTimeout.
func refuse(ctx context.Context, conn net.Conn, config Config) {
	defer func() { _ = conn.Close() }()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	if err := conn.SetDeadline(time.Now().Add(config.HandshakeTimeout)); err != nil {
		return
	}
	reply := conn
	if config.Mode == TLSPassthrough || config.Mode == AllowListRefusal {
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
	if config.Mode == AllowListRefusal {
		answerAllowListRefusals(reply)
		return
	}
	_, _ = io.WriteString(reply, maxClientsReply)
}

// answerAllowListRefusals answers each command conn sends with Render's
// allow-list refusal, without regard to what it asks: an AUTH's password is
// read past, never checked, so the reply says nothing about it. It returns at
// the client's close or the deadline, on a malformed or oversized command, or
// after maxRefusedCommands commands. valkey-cli keeps its connection after a
// refused AUTH and sends its command on it, so answering only the first would
// print "Server closed the connection" after Render's line.
func answerAllowListRefusals(conn net.Conn) {
	reader := bufio.NewReaderSize(conn, maxRefusedArgBytes+2)
	for range maxRefusedCommands {
		empty, err := readRefusedCommand(reader)
		if err != nil {
			return
		}
		// Valkey answers no empty command, and neither does the refusal.
		if empty {
			continue
		}
		if _, err := io.WriteString(conn, allowListRefusalReply); err != nil {
			return
		}
	}
}

// readRefusedCommand reads past one command, multibulk or inline, within the
// refusal's bounds, and reports whether it was empty.
func readRefusedCommand(reader *bufio.Reader) (empty bool, err error) {
	line, err := readRefusedLine(reader)
	if err != nil {
		return false, err
	}
	if len(line) == 0 || line[0] != '*' {
		return len(bytes.TrimSpace(line)) == 0, nil
	}
	args, err := strconv.Atoi(string(line[1:]))
	if err != nil || args > maxRefusedArgs {
		return false, errRefusedCommand
	}
	for range args {
		head, err := readRefusedLine(reader)
		if err != nil {
			return false, err
		}
		length, bulk := bytes.CutPrefix(head, []byte("$"))
		size, err := strconv.Atoi(string(length))
		if !bulk || err != nil || size < 0 || size > maxRefusedArgBytes {
			return false, errRefusedCommand
		}
		if _, err := reader.Discard(size); err != nil {
			return false, err
		}
		var end [2]byte
		if _, err := io.ReadFull(reader, end[:]); err != nil {
			return false, err
		}
		if string(end[:]) != "\r\n" {
			return false, errRefusedCommand
		}
	}
	return args <= 0, nil
}

// readRefusedLine reads one line of at most maxRefusedArgBytes and returns it
// without its line ending.
func readRefusedLine(reader *bufio.Reader) ([]byte, error) {
	line, err := reader.ReadSlice('\n')
	if errors.Is(err, bufio.ErrBufferFull) {
		return nil, errRefusedCommand
	}
	if err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(line[:len(line)-1], []byte("\r")), nil
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
