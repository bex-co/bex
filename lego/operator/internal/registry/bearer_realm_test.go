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

package registry

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// w21/m394 t010 (R25-03): the bearer realm was admitted by string prefix, so
// `http://127.0.0.1@zot.bex-registry.svc:5000/...`, `127.0.0.1.attacker.example`
// and `http://127.0.0.100` all passed as "loopback" and the operator sent its
// plaintext GET to a host the tenant's registry named.

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// tokenClient answers every request with a token and records the requests.
func tokenClient(requests *[]*http.Request) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		*requests = append(*requests, r)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader(`{"token":"t"}`)),
			Request:    r,
		}, nil
	})}
}

func TestBearerRealmIsHTTPSOrExactLoopback(t *testing.T) {
	rejected := []string{
		"http://127.0.0.1@zot.bex-registry.svc:5000/v1/sys/internal/x?probe=1",
		"http://127.0.0.1.attacker.example/token",
		"http://localhost.attacker.example/token",
		"http://127.0.0.100/token",
		"http://127.0.0.1.:5000/token",
		"http://[::2]:5000/token",
		"http://auth.example/token",
		"HTTP://zot.bex-registry.svc:5000/token",
		"https://user:pw@auth.example/token",
		"https://127.0.0.1@zot.bex-registry.svc:5000/token",
		"https://auth.example/token#fragment",
		"ftp://auth.example/token",
		"//auth.example/token",
		"https:///token",
		"https:token",
		"/token",
	}
	for _, realm := range rejected {
		t.Run("rejects "+realm, func(t *testing.T) {
			var requests []*http.Request
			_, err := bearerToken(t.Context(), tokenClient(&requests), `Bearer realm="`+realm+`",service="s"`, "")
			if err == nil {
				t.Fatalf("realm %q accepted", realm)
			}
			if len(requests) != 0 {
				t.Fatalf("realm %q was requested: %s", realm, requests[0].URL)
			}
		})
	}

	accepted := []string{
		"http://127.0.0.1:5001/token",
		"http://localhost:5001/token",
		"http://[::1]:5001/token",
		"https://auth.docker.io/token",
		"https://auth.example/token?existing=1",
	}
	for _, realm := range accepted {
		t.Run("accepts "+realm, func(t *testing.T) {
			var requests []*http.Request
			token, err := bearerToken(t.Context(), tokenClient(&requests), `Bearer realm="`+realm+`",service="registry.test"`, "")
			if err != nil || token != "t" {
				t.Fatalf("realm %q => %q, %v", realm, token, err)
			}
			if len(requests) != 1 || requests[0].URL.Query().Get("service") != "registry.test" {
				t.Fatalf("realm %q requests = %v", realm, requests)
			}
		})
	}
}

// The challenge is the registry's, so service and scope are values, never
// query syntax: an `&` inside one cannot add or override a parameter.
func TestBearerServiceAndScopeAreEscaped(t *testing.T) {
	var requests []*http.Request
	challenge := `Bearer realm="https://auth.example/token?existing=1",service="a&scope=repository:admin:*",scope="repository:traefik/whoami:pull"`
	if _, err := bearerToken(t.Context(), tokenClient(&requests), challenge, ""); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 1 {
		t.Fatalf("%d requests, want 1", len(requests))
	}
	q := requests[0].URL.Query()
	if got := q["scope"]; len(got) != 1 || got[0] != "repository:traefik/whoami:pull" {
		t.Fatalf("scope = %q, want exactly the challenge's scope", got)
	}
	if got := q.Get("service"); got != "a&scope=repository:admin:*" {
		t.Fatalf("service = %q, want the challenge's value intact", got)
	}
	if q.Get("existing") != "1" {
		t.Fatalf("the realm's own query was lost: %s", requests[0].URL.RawQuery)
	}
	if _, err := url.ParseQuery(requests[0].URL.RawQuery); err != nil {
		t.Fatalf("realm request query does not parse: %v", err)
	}
}
