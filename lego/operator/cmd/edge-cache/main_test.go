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
	"maps"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Every request the sidecar sees, cached or not, is proxied to the app: an
// event stream, a long poll, a WebSocket upgrade and a large upload all pass
// through it. A WriteTimeout would cut the first three and a large download, a
// ReadTimeout a slow upload, so neither is set; what is bounded is how long a
// client may take to send its headers and how long it may idle between requests.
func TestNewServerBoundsHeadersAndIdleButNotTheStream(t *testing.T) {
	server := newServer(":8080", http.NotFoundHandler())

	if server.ReadHeaderTimeout <= 0 || server.ReadHeaderTimeout > time.Minute {
		t.Fatalf("ReadHeaderTimeout = %s, want a bound of at most a minute", server.ReadHeaderTimeout)
	}
	if server.IdleTimeout <= 0 || server.IdleTimeout > 5*time.Minute {
		t.Fatalf("IdleTimeout = %s, want a bound of at most five minutes", server.IdleTimeout)
	}
	if server.WriteTimeout != 0 || server.ReadTimeout != 0 {
		t.Fatalf("ReadTimeout = %s, WriteTimeout = %s: either would cut a slow but legitimate stream or upload",
			server.ReadTimeout, server.WriteTimeout)
	}
	if server.Addr != ":8080" {
		t.Fatalf("Addr = %q", server.Addr)
	}
}

func env(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}

// A sidecar's environment configures what it always did: an in-memory cache
// with an eighth of its bound per object and no peers.
func TestReadSettingsKeepsTheSidecarDefaults(t *testing.T) {
	s, err := readSettings(env(map[string]string{
		"BEX_EDGE_CACHE_UPSTREAM":      "http://127.0.0.1:10000",
		"BEX_EDGE_CACHE_MAX_BYTES":     "50331648",
		"BEX_EDGE_CACHE_PROFILE":       "origin-controlled",
		"BEX_EDGE_CACHE_CONTROL":       "/etc/bex-edge-cache/annotations",
		"BEX_EDGE_CACHE_LISTEN":        ":8080",
		"BEX_EDGE_CACHE_DRAIN_SECONDS": "5",
	}))
	if err != nil {
		t.Fatal(err)
	}
	c := s.cache
	if s.upstream.Host != "127.0.0.1:10000" || c.Profile != "origin-controlled" || c.MaxBytes != 50331648 ||
		c.MaxObjectBytes != 50331648/8 || c.MaxIndexBytes != 0 || c.Dir != "" {
		t.Fatalf("settings = %+v, cache %+v", s, c)
	}
	if s.control != "/etc/bex-edge-cache/annotations" || s.listen != ":8080" || s.drain != 5*time.Second {
		t.Fatalf("control %q listen %q drain %s", s.control, s.listen, s.drain)
	}
	if s.peers != "" || s.self != "" || s.peerPort != 0 {
		t.Fatalf("a sidecar has peers %q self %q port %d", s.peers, s.self, s.peerPort)
	}
}

// A tier replica stores on disk with its own object and index bounds, and
// finds its peers at the port it listens on.
func TestReadSettingsConfiguresATierReplica(t *testing.T) {
	s, err := readSettings(env(map[string]string{
		"BEX_EDGE_CACHE_UPSTREAM":         "http://web.ws-1.svc:10000",
		"BEX_EDGE_CACHE_MAX_BYTES":        "1073741824",
		"BEX_EDGE_CACHE_MAX_OBJECT_BYTES": "104857600",
		"BEX_EDGE_CACHE_MAX_INDEX_BYTES":  "16777216",
		"BEX_EDGE_CACHE_DIR":              "/var/cache/bex-edge",
		"BEX_EDGE_CACHE_LISTEN":           ":8080",
		"BEX_EDGE_CACHE_PEERS":            "web-cache-peers.ws-1.svc.cluster.local",
		"BEX_EDGE_CACHE_SELF":             "10.244.1.7",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c := s.cache; c.MaxObjectBytes != 104857600 || c.MaxIndexBytes != 16777216 || c.Dir != "/var/cache/bex-edge" {
		t.Fatalf("cache %+v", c)
	}
	if s.peers != "web-cache-peers.ws-1.svc.cluster.local" || s.self != "10.244.1.7" || s.peerPort != 8080 {
		t.Fatalf("peers %q self %q port %d", s.peers, s.self, s.peerPort)
	}
	s, err = readSettings(env(map[string]string{
		"BEX_EDGE_CACHE_UPSTREAM": "http://web:10000", "BEX_EDGE_CACHE_MAX_BYTES": "1024",
		"BEX_EDGE_CACHE_PEERS": "peers", "BEX_EDGE_CACHE_SELF": "10.0.0.1",
	}))
	if err != nil || s.peerPort != 80 {
		t.Fatalf("with no listen address: port %d, %v; want http's 80, as the server listens", s.peerPort, err)
	}
}

// Each bad setting is refused with an error naming the variable to fix.
func TestReadSettingsRefusesABadEnvironment(t *testing.T) {
	const (
		upstream  = "BEX_EDGE_CACHE_UPSTREAM"
		maxBytes  = "BEX_EDGE_CACHE_MAX_BYTES"
		maxObject = "BEX_EDGE_CACHE_MAX_OBJECT_BYTES"
		maxIndex  = "BEX_EDGE_CACHE_MAX_INDEX_BYTES"
		peers     = "BEX_EDGE_CACHE_PEERS"
		self      = "BEX_EDGE_CACHE_SELF"
		listen    = "BEX_EDGE_CACHE_LISTEN"
	)
	base := map[string]string{upstream: "http://127.0.0.1:10000", maxBytes: "1024"}
	for name, tc := range map[string]struct {
		set  map[string]string
		want string
	}{
		"no upstream":               {map[string]string{upstream: ""}, upstream},
		"no bound":                  {map[string]string{maxBytes: ""}, maxBytes},
		"a zero object bound":       {map[string]string{maxObject: "0"}, maxObject},
		"a negative object bound":   {map[string]string{maxObject: "-1"}, maxObject},
		"an index bound in words":   {map[string]string{maxIndex: "lots"}, maxIndex},
		"peers without self":        {map[string]string{peers: "peers"}, self},
		"self without peers":        {map[string]string{self: "10.0.0.1"}, peers},
		"peers on an unparsed port": {map[string]string{peers: "p", self: "10.0.0.1", listen: "8080"}, listen},
	} {
		t.Run(name, func(t *testing.T) {
			values := maps.Clone(base)
			maps.Copy(values, tc.set)
			if _, err := readSettings(env(values)); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want one naming %s", err, tc.want)
			}
		})
	}
}
