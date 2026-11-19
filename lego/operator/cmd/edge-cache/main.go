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

// Command edge-cache is the caching proxy the operator places in front of a
// web service whose serviceDetails.cache.profile asks for Render's edge
// caching (internal/edgecache). It runs in one of two shapes, both configured
// by environment:
//
//   - The sidecar (w5/m151): a container in each app pod in front of the app
//     container (BEX_EDGE_CACHE_UPSTREAM on localhost), its cache in memory.
//   - The cache tier (w3/m489): a small Deployment per caching service in
//     front of the app's Service, whose replicas keep bodies on a bounded disk
//     (BEX_EDGE_CACHE_DIR) and share one cache by key affinity, finding each
//     other through the tier's headless Service (BEX_EDGE_CACHE_PEERS, with
//     the pod's own IP in BEX_EDGE_CACHE_SELF).
//
// Variables: BEX_EDGE_CACHE_UPSTREAM (the app's http URL),
// BEX_EDGE_CACHE_MAX_BYTES (the cache's bound), BEX_EDGE_CACHE_MAX_OBJECT_BYTES
// (one body's; MAX_BYTES/8 when unset), BEX_EDGE_CACHE_MAX_INDEX_BYTES (the
// in-memory metadata's on disk; MAX_BYTES/32 when unset), BEX_EDGE_CACHE_DIR,
// BEX_EDGE_CACHE_PROFILE, BEX_EDGE_CACHE_CONTROL (the downward-API annotations
// file), BEX_EDGE_CACHE_LISTEN, BEX_EDGE_CACHE_DRAIN_SECONDS,
// BEX_EDGE_CACHE_PEERS and BEX_EDGE_CACHE_SELF.
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/bex-co/bex/lego/operator/internal/edgecache"
)

// The proxy passes every request through to the app, so only what the app
// cannot bound for it is limited here: how long a client may take to send its
// headers and how long a connection may sit idle between requests. There is no
// ReadTimeout or WriteTimeout: either would cut an event stream, a long poll, a
// WebSocket upgrade, a large download or a slow upload that the app serves on
// purpose, and the app has its own limits for those.
const (
	readHeaderTimeout = 10 * time.Second
	idleTimeout       = 120 * time.Second
	// controlPollInterval is how often the proxy re-reads its control file
	// (BEX_EDGE_CACHE_CONTROL); the kubelet's refresh of the file after the
	// operator patches the pod dominates how soon a purge lands.
	controlPollInterval = 2 * time.Second
	// peerPollInterval is how often a tier replica resolves its peers
	// (BEX_EDGE_CACHE_PEERS); a replica that left is skipped at once anyway,
	// since a failed forward is served locally.
	peerPollInterval = 5 * time.Second
)

func newServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
	}
}

// settings is the proxy's configuration, read from its environment.
type settings struct {
	upstream        *url.URL
	cache           edgecache.Config
	control, listen string
	drain           time.Duration
	// peers is the tier's headless Service name and self this pod's IP, both
	// set or both empty; peerPort is the listen port every replica serves on.
	peers, self string
	peerPort    int
}

func readSettings(getenv func(string) string) (settings, error) {
	s := settings{
		control: getenv("BEX_EDGE_CACHE_CONTROL"),
		listen:  getenv("BEX_EDGE_CACHE_LISTEN"),
		peers:   getenv("BEX_EDGE_CACHE_PEERS"),
		self:    getenv("BEX_EDGE_CACHE_SELF"),
	}
	upstream, err := url.Parse(getenv("BEX_EDGE_CACHE_UPSTREAM"))
	if err != nil || upstream.Host == "" {
		return s, fmt.Errorf("BEX_EDGE_CACHE_UPSTREAM must be an http URL, got %q", getenv("BEX_EDGE_CACHE_UPSTREAM"))
	}
	s.upstream = upstream
	maxBytes, err := strconv.ParseInt(getenv("BEX_EDGE_CACHE_MAX_BYTES"), 10, 64)
	if err != nil || maxBytes <= 0 {
		return s, fmt.Errorf("BEX_EDGE_CACHE_MAX_BYTES must be a positive byte count")
	}
	s.cache = edgecache.Config{
		Profile:        getenv("BEX_EDGE_CACHE_PROFILE"),
		MaxBytes:       maxBytes,
		MaxObjectBytes: maxBytes / 8,
		Dir:            getenv("BEX_EDGE_CACHE_DIR"),
	}
	for _, bound := range []struct {
		name  string
		value *int64
	}{
		{"BEX_EDGE_CACHE_MAX_OBJECT_BYTES", &s.cache.MaxObjectBytes},
		{"BEX_EDGE_CACHE_MAX_INDEX_BYTES", &s.cache.MaxIndexBytes},
	} {
		raw := getenv(bound.name)
		if raw == "" {
			continue
		}
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n <= 0 {
			return s, fmt.Errorf("%s must be a positive byte count when set, got %q", bound.name, raw)
		}
		*bound.value = n
	}
	drain, _ := strconv.Atoi(getenv("BEX_EDGE_CACHE_DRAIN_SECONDS"))
	s.drain = time.Duration(drain) * time.Second
	if (s.peers == "") != (s.self == "") {
		return s, fmt.Errorf("BEX_EDGE_CACHE_PEERS and BEX_EDGE_CACHE_SELF must be set together")
	}
	if s.peers != "" {
		port := "http"
		if s.listen != "" {
			if _, port, err = net.SplitHostPort(s.listen); err != nil {
				return s, fmt.Errorf("BEX_EDGE_CACHE_LISTEN %q: %v", s.listen, err)
			}
		}
		if s.peerPort, err = net.LookupPort("tcp", port); err != nil {
			return s, fmt.Errorf("BEX_EDGE_CACHE_LISTEN %q: %v", s.listen, err)
		}
	}
	return s, nil
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	s, err := readSettings(os.Getenv)
	if err != nil {
		log.Fatalf("edge-cache: %v", err)
	}
	handler, err := edgecache.New(s.upstream, s.cache)
	if err != nil {
		log.Fatalf("edge-cache: %v", err)
	}
	// A proxy projected since w3/m486 names its downward-API annotations file:
	// a manual purge, a profile switch and, for the tier, a successful deploy
	// arrive there, with no restart.
	if s.control != "" {
		control := edgecache.NewController(handler, s.control)
		control.Logf = log.Printf
		go control.Run(ctx, controlPollInterval)
	}
	// A tier replica keeps following its peers through its drain window, for
	// as long as it serves.
	if s.peers != "" {
		go edgecache.RunPeers(context.Background(), handler, s.peers, s.self, s.peerPort, peerPollInterval,
			net.DefaultResolver.LookupHost, log.Printf)
	}
	server := newServer(s.listen, handler)
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		<-ctx.Done()
		// Keep serving through the pod's drain window, while the endpoints
		// still route here, then finish in-flight requests.
		time.Sleep(s.drain)
		shutdown, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("edge-cache: %v", err)
	}
	// ListenAndServe returns as Shutdown begins; the requests it is
	// finishing end only when Shutdown returns.
	<-drained
}
