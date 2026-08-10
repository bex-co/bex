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

package webhooks

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
)

// TestDeliveryTransportIgnoresAmbientProxy is the w1/m66 F9 regression. The
// delivery transport is a clone of http.DefaultTransport, which carries
// ProxyFromEnvironment; only DialContext was replaced. With HTTP(S)_PROXY set in
// the process environment, the SSRF guard would therefore be handed the PROXY's
// address to validate while the proxy resolved and fetched the tenant-controlled
// URL — the whole private/link-local/metadata policy bypassed without ever being
// consulted about the real destination.
//
// The behavioral half records what the dialer is actually asked to connect to:
// pre-fix that is the proxy, post-fix it is the tenant's (blocked) target.
func TestDeliveryTransportIgnoresAmbientProxy(t *testing.T) {
	proxy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = proxy.Close() }()

	t.Setenv("HTTP_PROXY", "http://"+proxy.Addr().String())
	t.Setenv("HTTPS_PROXY", "http://"+proxy.Addr().String())
	t.Setenv("NO_PROXY", "")

	tr := deliveryTransport()
	if tr.Proxy != nil {
		t.Error("delivery transport must not consult a proxy: its destination policy is enforced at dial time")
	}

	// Keep the production Proxy setting, swap only the dialer, so what the
	// transport asks to dial is observable.
	var mu sync.Mutex
	var dialed []string
	tr.DialContext = func(_ context.Context, _, address string) (net.Conn, error) {
		mu.Lock()
		dialed = append(dialed, address)
		mu.Unlock()
		return nil, errStubDial
	}

	req, err := http.NewRequest(http.MethodPost, "http://10.0.0.1/hook", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	//nolint:bodyclose // the stub dialer always fails; there is no response body.
	if _, err := tr.RoundTrip(req); err == nil {
		t.Fatal("expected the stub dialer to fail the round trip")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(dialed) != 1 {
		t.Fatalf("expected exactly one dial, got %v", dialed)
	}
	if dialed[0] != "10.0.0.1:80" {
		t.Errorf("transport dialed %q; want the tenant target 10.0.0.1:80 — dialing the proxy means the SSRF guard never sees the real destination", dialed[0])
	}
}

// TestProductionClientUsesTheGuardedTransport pins that the shared client is the
// one deliveryTransport builds, so the assertions above describe production.
func TestProductionClientUsesTheGuardedTransport(t *testing.T) {
	tr, ok := defaultClient.Transport.(*http.Transport)
	if !ok {
		t.Fatal("defaultClient.Transport must be *http.Transport")
	}
	if tr.Proxy != nil {
		t.Error("production webhook client must not inherit an ambient proxy")
	}
	if tr.DialContext == nil {
		t.Error("production webhook client must keep its dial-time SSRF guard")
	}
}

type stubDialError struct{}

func (stubDialError) Error() string { return "stub dialer" }

var errStubDial = stubDialError{}
