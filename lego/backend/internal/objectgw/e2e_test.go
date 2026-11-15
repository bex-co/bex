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

package objectgw

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
)

// gateway builds the gateway as production does, its lookups and reports going
// over HTTP to bex-api's half (the verbs, over the real service), and the
// resolver on the test's clock.
func (a *apiEnv) gateway(clock *fakeClock) (*Handler, *Meter) {
	cfg := objectstorage.Config{Endpoint: a.fake.URL, Region: "us-west-1", Bucket: objectstorage.Oregon.Bucket(), AccessKey: "r", SecretKey: "s", MaxAttempts: 1}
	resolver := NewCachedResolver(&HTTPLookup{APIClient{URL: a.server.URL, Token: testToken}})
	resolver.now = clock.now
	meter := NewMeter(&HTTPReporter{APIClient{URL: a.server.URL, Token: testToken}})
	h := NewHandler(resolver, NewWasabiOrigin(map[objectstorage.Region]objectstorage.Config{objectstorage.Oregon: cfg}),
		NewLimits(DefaultLimits()), meter, NewMetrics())
	h.cache.now = clock.now
	return h, meter
}

func get(h http.Handler, host, target string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Host = host
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestDisablingDevURLStopsServingWithinTheCacheWindow is the lifecycle end to
// end: a bucket is private, then published, then unpublished, and a visitor sees
// each change no later than the lookup window after it, with the gateway talking
// to bex-api's real verbs over HTTP.
func TestDisablingDevURLStopsServingWithinTheCacheWindow(t *testing.T) {
	a := newAPI(t, testBaseDomain)
	clock := newClock()
	h, _ := a.gateway(clock)
	b, host := a.addBucket(false)
	body := []byte("a public image")
	wasabiKey, _ := objectstorage.ObjectKey(b.WorkspaceID, b.ID, "img.png")
	a.fake.Put(objectstorage.Oregon.Bucket(), wasabiKey, body)

	if rec := get(h, host, "/img.png"); rec.Code != http.StatusNotFound {
		t.Fatalf("a private bucket = %d, want 404", rec.Code)
	}
	reads := func() int { return a.objectReads(wasabiKey) }
	if reads() != 0 {
		t.Fatal("a private bucket's object was read")
	}

	// Enable. The earlier miss is cached for the window, so the first read after
	// it is the first that can see the change.
	if err := a.svc.SetPublic(t.Context(), a.ws, b, true, testBaseDomain); err != nil {
		t.Fatal(err)
	}
	clock.advance(LookupTTL + time.Second)
	if rec := get(h, host, "/img.png"); rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), body) {
		t.Fatalf("after enabling = %d %q, want the object", rec.Code, rec.Body)
	}

	// Disable. Inside the window the cached answer still serves (documented), and
	// once it expires the bucket is gone, though the body cache still holds the object.
	if err := a.svc.SetPublic(t.Context(), a.ws, b, false, testBaseDomain); err != nil {
		t.Fatal(err)
	}
	clock.advance(LookupTTL - 5*time.Second)
	if rec := get(h, host, "/img.png"); rec.Code != http.StatusOK {
		t.Errorf("inside the window after disabling = %d; the lookup cache still holds the answer", rec.Code)
	}
	clock.advance(10 * time.Second)
	if rec := get(h, host, "/img.png"); rec.Code != http.StatusNotFound {
		t.Fatalf("after the window = %d %q, want 404: disabling must stop serving", rec.Code, rec.Body)
	}
	// Deleting the bucket does the same.
	if err := a.svc.SetPublic(t.Context(), a.ws, b, true, testBaseDomain); err != nil {
		t.Fatal(err)
	}
	clock.advance(LookupTTL + time.Second)
	if rec := get(h, host, "/img.png"); rec.Code != http.StatusOK {
		t.Fatalf("enabled again = %d", rec.Code)
	}
	a.repo.remove(b.ID)
	clock.advance(LookupTTL + time.Second)
	if rec := get(h, host, "/img.png"); rec.Code != http.StatusNotFound {
		t.Errorf("a deleted bucket = %d, want 404", rec.Code)
	}
}

func (a *apiEnv) objectReads(wasabiKey string) int {
	n := 0
	for _, r := range a.fake.Requests() {
		if r.Key == wasabiKey && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
			n++
		}
	}
	return n
}

// TestAPublicReadIsMeteredToItsWorkspaceThroughBexAPI: the gateway counts what
// it sends, reports it over HTTP, and bex-api adds it to the workspace's egress
// shadow meter, the number the egress alert reads.
func TestAPublicReadIsMeteredToItsWorkspaceThroughBexAPI(t *testing.T) {
	a := newAPI(t, testBaseDomain)
	clock := newClock()
	h, meter := a.gateway(clock)
	b, host := a.addBucket(true, corsRule([]string{"*"}, []string{"GET"}, nil, nil, nil))
	wasabiKey, _ := objectstorage.ObjectKey(b.WorkspaceID, b.ID, "o")
	a.fake.Put(objectstorage.Oregon.Bucket(), wasabiKey, bytes.Repeat([]byte("z"), 4000))

	for range 3 {
		if rec := get(h, host, "/o"); rec.Code != http.StatusOK || rec.Body.Len() != 4000 {
			t.Fatalf("GET = %d with %d bytes", rec.Code, rec.Body.Len())
		}
	}
	if err := meter.Flush(t.Context()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { a.svc.RunMeter(ctx, time.Hour); close(done) }()
	cancel()
	<-done
	if got := a.sink.total(a.ws); got != 12000 {
		t.Errorf("the workspace was metered %d bytes, want 12000 (three reads of 4000, one from Wasabi and two from the cache)", got)
	}
	// CORS rules travelled through the lookup: the gateway applies bex-api's rules.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/o", nil)
	req.Host = host
	req.Header.Set("Origin", "https://site.example")
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("the bucket's rule did not reach the gateway: %v", rec.Header())
	}
}

// TestTheAssembledGatewayServes uses New with only credentials and test endpoints
// and checks the internal listener's probe and metrics.
func TestTheAssembledGatewayServes(t *testing.T) {
	a := newAPI(t, testBaseDomain)
	b, host := a.addBucket(true)
	wasabiKey, _ := objectstorage.ObjectKey(b.WorkspaceID, b.ID, "o")
	a.fake.Put(objectstorage.Oregon.Bucket(), wasabiKey, []byte("hello"))
	gw := New(Config{
		Credentials: Credentials{ReadAccessKey: "r", ReadSecretKey: "s", Token: testToken},
		BexAPIURL:   a.server.URL,
		Regions: map[objectstorage.Region]objectstorage.Config{
			objectstorage.Oregon: {Endpoint: a.fake.URL, Region: "us-west-1", Bucket: objectstorage.Oregon.Bucket(), AccessKey: "r", SecretKey: "s", MaxAttempts: 1},
		},
	})
	if rec := get(gw.Public, host, "/o"); rec.Code != http.StatusOK || rec.Body.String() != "hello" {
		t.Fatalf("public = %d %q", rec.Code, rec.Body)
	}
	if rec := get(gw.Internal, "internal", "/healthz"); rec.Code != http.StatusOK {
		t.Errorf("/healthz = %d", rec.Code)
	}
	rec := get(gw.Internal, "internal", "/metrics")
	for _, want := range []string{"bex_objectgw_requests_total", "bex_objectgw_response_bytes_total"} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Errorf("/metrics lacks %s", want)
		}
	}
	// The internal listener never serves object content, and the public one never serves metrics.
	if rec := get(gw.Internal, "internal", "/o"); rec.Code == http.StatusOK && rec.Body.String() == "hello" {
		t.Error("the internal listener served an object")
	}
	if rec := get(gw.Public, host, "/metrics"); rec.Code != http.StatusNotFound {
		t.Errorf("/metrics on the public listener = %d, want 404 (an object key)", rec.Code)
	}
}

func TestADegradedGatewayStaysUpAnd503s(t *testing.T) {
	gw := NewDegraded()
	rec := get(gw.Public, "pub-"+"0000000000000000000a"+"."+testBaseDomain, "/o")
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("X-Content-Type-Options") != "nosniff" || rec.Header().Get("Retry-After") == "" {
		t.Errorf("degraded public = %d %v", rec.Code, rec.Header())
	}
	if rec := get(gw.Internal, "internal", "/healthz"); rec.Code != http.StatusOK {
		t.Errorf("a degraded gateway must stay Ready: /healthz = %d", rec.Code)
	}
	for _, c := range []Credentials{{}, {ReadAccessKey: "a"}, {ReadAccessKey: "a", ReadSecretKey: "b"}, {Token: "t"}} {
		if len(c.Missing()) == 0 {
			t.Errorf("%+v reports nothing missing", c)
		}
	}
	if len((Credentials{ReadAccessKey: "a", ReadSecretKey: "b", Token: "t"}).Missing()) != 0 {
		t.Error("a complete credential reports something missing")
	}
}
