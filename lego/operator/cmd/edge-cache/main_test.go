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
	"net/http"
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
