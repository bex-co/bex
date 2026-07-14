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

// bex-pg-sni-proxy is the Postgres-aware SNI front-door for external
// managed-database connections. Traefik's TCP/SNI routing reads hostname from
// the TLS ClientHello, but the libpq default (preamble-mode) sends a cleartext
// SSLRequest BEFORE the TLS ClientHello — so Traefik can't route it.
//
// The proxy sits on :5432, handles the SSLRequest negotiation, peeks at the
// TLS ClientHello to extract the SNI hostname, and dials the correct CNPG
// backend directly. Direct-TLS clients (PG 17+ sslnegotiation=direct) are also
// handled transparently. See docs/ADR009-postgresql-management.md.
package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const (
	sslRequestMagic = int32(80877103) // 0x04D2162F — PostgreSQL SSLRequest magic
	sslRequestLen   = int32(8)
	sslByte         = byte('S') // server replies 'S' to accept SSL

	tlsHandshakeType = byte(0x16) // TLS ContentType: Handshake

	maxClientHello = 16 * 1024 // cap on how many bytes of a TLS record we buffer

	defaultAddr = ":5432"
	dialTimeout = 10 * time.Second
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(appv1alpha1.AddToScheme(scheme))
}

// dbRouter maps `<dbname>` → `<namespace>` for all Database CRs in the cluster.
// SNI hostname `<dbname>.<domain>` is resolved at connection time. The router is
// kept up-to-date by a controller-runtime reconciler watching Database events.
type dbRouter struct {
	mu     sync.RWMutex
	domain string            // BEX_DB_DOMAIN, e.g. "db.bex.co"
	table  map[string]string // dbname → namespace
}

func newRouter(domain string) *dbRouter {
	return &dbRouter{domain: domain, table: make(map[string]string)}
}

func (r *dbRouter) set(name, ns string) {
	r.mu.Lock()
	r.table[name] = ns
	r.mu.Unlock()
}

func (r *dbRouter) delete(name string) {
	r.mu.Lock()
	delete(r.table, name)
	r.mu.Unlock()
}

// resolve maps an SNI hostname to a backend TCP address.
// Supported patterns (all on port 5432):
//   - <dbname>.<domain>           → <dbname>-rw.<ns>.svc.cluster.local:5432
//   - <dbname>-pool.<domain>      → <dbname>-pooler.<ns>.svc.cluster.local:5432
//   - <dbname>-ro-<rep>.<domain>  → <dbname>-ro.<ns>.svc.cluster.local:5432
func (r *dbRouter) resolve(sni string) (string, bool) {
	if r.domain == "" {
		return "", false
	}
	base, ok := strings.CutSuffix(sni, "."+r.domain)
	if !ok {
		return "", false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for name, ns := range r.table {
		switch {
		case base == name:
			return fmt.Sprintf("%s-rw.%s.svc.cluster.local:5432", name, ns), true
		case base == name+"-pool":
			return fmt.Sprintf("%s-pooler.%s.svc.cluster.local:5432", name, ns), true
		case strings.HasPrefix(base, name+"-ro-"):
			return fmt.Sprintf("%s-ro.%s.svc.cluster.local:5432", name, ns), true
		}
	}
	return "", false
}

// dbWatcher is a controller-runtime reconciler that keeps dbRouter in sync with
// Database CRs. It does no patching — read only. Runs in every namespace.
type dbWatcher struct {
	client.Client
	router *dbRouter
}

func (w *dbWatcher) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	var db appv1alpha1.Database
	if err := w.Get(ctx, req.NamespacedName, &db); err != nil {
		if apierrors.IsNotFound(err) {
			w.router.delete(req.Name)
		}
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	if !db.DeletionTimestamp.IsZero() {
		w.router.delete(db.Name)
	} else {
		w.router.set(db.Name, db.Namespace)
	}
	return reconcile.Result{}, nil
}

func main() {
	ctrl.SetLogger(zap.New())
	log := ctrl.Log.WithName("pg-sni-proxy")

	addr := defaultAddr
	if v := os.Getenv("BEX_PROXY_ADDR"); v != "" {
		addr = v
	}
	domain := os.Getenv("BEX_DB_DOMAIN")
	if domain == "" {
		log.Info("BEX_DB_DOMAIN is unset — proxy will refuse all connections (no routing table)")
	}

	router := newRouter(domain)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		// No webhook, no metrics, no leader election — lightweight watcher only.
		LeaderElection: false,
	})
	if err != nil {
		log.Error(err, "unable to create manager")
		os.Exit(1)
	}

	if err := ctrl.NewControllerManagedBy(mgr).
		For(&appv1alpha1.Database{}).
		Complete(&dbWatcher{Client: mgr.GetClient(), router: router}); err != nil {
		log.Error(err, "unable to set up Database watcher")
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(ctrl.SetupSignalHandler())
	defer cancel()

	// Start the manager (and its cache) in the background.
	mgrErrCh := make(chan error, 1)
	go func() { mgrErrCh <- mgr.Start(ctx) }()

	// Wait for the cache to sync before accepting connections, so the first
	// connection doesn't race with the initial list.
	if !mgr.GetCache().WaitForCacheSync(ctx) {
		log.Error(fmt.Errorf("cache sync timed out"), "failed to sync Database cache")
		os.Exit(1)
	}
	log.Info("cache synced, starting TCP listener", "addr", addr)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Error(err, "failed to listen", "addr", addr)
		os.Exit(1)
	}
	defer ln.Close()

	go func() {
		select {
		case <-ctx.Done():
			ln.Close()
		case err := <-mgrErrCh:
			if err != nil {
				log.Error(err, "manager exited")
			}
			ln.Close()
		}
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				log.Error(err, "accept error")
				continue
			}
		}
		go handleConn(conn, router, log)
	}
}

// handleConn handles one incoming client connection:
//  1. Detects PostgreSQL SSLRequest preamble vs direct TLS.
//  2. Buffers the TLS ClientHello and extracts the SNI hostname.
//  3. Dials the matching CNPG backend.
//  4. Re-negotiates SSL with the backend (for preamble-mode clients).
//  5. Splices the connection bidirectionally.
func handleConn(conn net.Conn, router *dbRouter, log interface {
	Info(msg string, keysAndValues ...interface{})
	Error(err error, msg string, keysAndValues ...interface{})
}) {
	defer conn.Close()

	// Peek at the first 8 bytes to distinguish SSLRequest from direct TLS.
	var peek [8]byte
	if _, err := io.ReadFull(conn, peek[:]); err != nil {
		return
	}

	msgLen := int32(binary.BigEndian.Uint32(peek[0:4]))
	msgCode := int32(binary.BigEndian.Uint32(peek[4:8]))
	isSSLRequest := msgLen == sslRequestLen && msgCode == sslRequestMagic
	isDirectTLS := peek[0] == tlsHandshakeType

	if !isSSLRequest && !isDirectTLS {
		log.Info("unknown protocol header, closing", "byte0", fmt.Sprintf("%02x", peek[0]))
		return
	}

	if isSSLRequest {
		// Tell the client we support SSL, then read its TLS ClientHello.
		if _, err := conn.Write([]byte{sslByte}); err != nil {
			return
		}
	}

	// initial holds bytes already peeked that belong to the TLS record.
	var initial []byte
	if isDirectTLS {
		initial = peek[:] // first 8 bytes are the start of the TLS record
	}

	clientHello, err := readTLSRecord(conn, initial)
	if err != nil {
		log.Info("failed to read TLS ClientHello", "err", err)
		return
	}

	sni, err := extractSNI(clientHello)
	if err != nil || sni == "" {
		log.Info("no SNI in ClientHello", "err", err)
		return
	}

	backend, ok := router.resolve(sni)
	if !ok {
		log.Info("no route for SNI", "sni", sni)
		return
	}

	back, err := net.DialTimeout("tcp", backend, dialTimeout)
	if err != nil {
		log.Error(err, "dial backend failed", "backend", backend)
		return
	}
	defer back.Close()

	if isSSLRequest {
		// Replicate the SSLRequest/S handshake with the backend so it upgrades
		// to TLS before we forward the ClientHello.
		if _, err := back.Write(peek[:]); err != nil {
			return
		}
		var resp [1]byte
		if _, err := io.ReadFull(back, resp[:]); err != nil {
			return
		}
		if resp[0] != sslByte {
			log.Info("backend declined SSL", "response", resp[0])
			return
		}
	}

	// Forward the buffered TLS ClientHello to the backend; the TLS handshake
	// continues transparently (the proxy is not a TLS terminator).
	if _, err := back.Write(clientHello); err != nil {
		return
	}

	// Splice the two sides bidirectionally. The TLS session is end-to-end between
	// the client and CNPG — the proxy never sees the plaintext.
	done := make(chan struct{}, 2)
	go func() { io.Copy(back, conn); done <- struct{}{} }()
	go func() { io.Copy(conn, back); done <- struct{}{} }()
	<-done
}

// readTLSRecord reads one TLS record from r, prepending any bytes already
// consumed from the stream (initial). Returns the complete record bytes
// (5-byte header + body), capped at maxClientHello.
func readTLSRecord(r io.Reader, initial []byte) ([]byte, error) {
	// Build the 5-byte TLS record header.
	hdr := make([]byte, 5)
	n := copy(hdr, initial)
	if n < 5 {
		if _, err := io.ReadFull(r, hdr[n:]); err != nil {
			return nil, fmt.Errorf("read TLS header: %w", err)
		}
	}
	if hdr[0] != tlsHandshakeType {
		return nil, fmt.Errorf("not a TLS handshake record: byte0=0x%02x", hdr[0])
	}
	recLen := int(hdr[3])<<8 | int(hdr[4])
	if recLen > maxClientHello {
		return nil, fmt.Errorf("TLS record too large: %d", recLen)
	}

	// Assemble the full record buffer.
	buf := make([]byte, 5+recLen)
	copy(buf, hdr)

	// Bytes from initial that are part of the body (beyond the 5-byte header).
	bodyFromInitial := initial[min(5, len(initial)):]
	if len(bodyFromInitial) > recLen {
		bodyFromInitial = bodyFromInitial[:recLen]
	}
	copy(buf[5:], bodyFromInitial)

	// Read the remaining body bytes from the reader.
	alreadyHave := 5 + len(bodyFromInitial)
	if alreadyHave < 5+recLen {
		if _, err := io.ReadFull(r, buf[alreadyHave:]); err != nil {
			return nil, fmt.Errorf("read TLS body: %w", err)
		}
	}
	return buf, nil
}

// extractSNI parses the SNI hostname from a TLS ClientHello record.
// Returns ("", nil) when there is no SNI extension (not an error — the caller
// must decide what to do with SNI-less connections).
func extractSNI(record []byte) (string, error) {
	// Record header: type(1) + version(2) + length(2) = 5 bytes.
	if len(record) < 5 || record[0] != tlsHandshakeType {
		return "", fmt.Errorf("not a handshake record")
	}
	recLen := int(record[3])<<8 | int(record[4])
	if len(record) < 5+recLen {
		return "", fmt.Errorf("record truncated")
	}
	data := record[5 : 5+recLen]

	// Handshake header: type(1) + length(3) = 4 bytes.
	if len(data) < 4 || data[0] != 0x01 { // 0x01 = ClientHello
		return "", fmt.Errorf("not a ClientHello")
	}
	hsLen := int(data[1])<<16 | int(data[2])<<8 | int(data[3])
	if len(data) < 4+hsLen {
		return "", fmt.Errorf("ClientHello truncated")
	}
	ch := data[4 : 4+hsLen]

	// Skip: legacy_version(2) + random(32) = 34 bytes.
	if len(ch) < 34 {
		return "", fmt.Errorf("ClientHello too short")
	}
	pos := 34

	// session_id: 1-byte length + data.
	if pos >= len(ch) {
		return "", nil
	}
	pos += 1 + int(ch[pos])

	// cipher_suites: 2-byte length + data.
	if pos+2 > len(ch) {
		return "", nil
	}
	pos += 2 + (int(ch[pos])<<8 | int(ch[pos+1]))

	// compression_methods: 1-byte length + data.
	if pos >= len(ch) {
		return "", nil
	}
	pos += 1 + int(ch[pos])

	// extensions: 2-byte total length.
	if pos+2 > len(ch) {
		return "", nil // no extensions
	}
	extTotal := int(ch[pos])<<8 | int(ch[pos+1])
	pos += 2
	end := pos + extTotal

	for pos+4 <= end && pos+4 <= len(ch) {
		extType := uint16(ch[pos])<<8 | uint16(ch[pos+1])
		extLen := int(ch[pos+2])<<8 | int(ch[pos+3])
		pos += 4
		if pos+extLen > len(ch) {
			break
		}
		if extType == 0x0000 { // SNI extension
			name, ok := parseSNIExtension(ch[pos : pos+extLen])
			if ok {
				return name, nil
			}
		}
		pos += extLen
	}
	return "", nil
}

// parseSNIExtension extracts the first host_name entry from an SNI extension value.
func parseSNIExtension(ext []byte) (string, bool) {
	// list_length(2) + entries
	if len(ext) < 2 {
		return "", false
	}
	listLen := int(ext[0])<<8 | int(ext[1])
	pos := 2
	end := pos + listLen
	for pos+3 <= end && pos+3 <= len(ext) {
		nameType := ext[pos]
		nameLen := int(ext[pos+1])<<8 | int(ext[pos+2])
		pos += 3
		if pos+nameLen > len(ext) {
			break
		}
		if nameType == 0x00 { // host_name
			return string(ext[pos : pos+nameLen]), true
		}
		pos += nameLen
	}
	return "", false
}
