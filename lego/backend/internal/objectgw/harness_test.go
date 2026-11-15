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
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage/fakes3"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage/publichost"
)

const testBaseDomain = "onbex.test"

// stubResolver is a Resolver over a fixed table, counting its lookups. A host
// it does not hold is ErrNotFound, which is also what bex-api says for a private
// or deleted bucket.
type stubResolver struct {
	mu      sync.Mutex
	buckets map[string]Bucket
	calls   int
	err     error
}

func (s *stubResolver) Lookup(_ context.Context, host string) (Bucket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.err != nil {
		return Bucket{}, s.err
	}
	if b, ok := s.buckets[publichost.NormalizeHost(host)]; ok {
		return b, nil
	}
	return Bucket{}, ErrNotFound
}

func (s *stubResolver) lookups() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

// fakeClock is a clock a test moves.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newClock() *fakeClock { return &fakeClock{t: time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)} }

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// gwEnv is the gateway over a fake S3 with one public bucket.
type gwEnv struct {
	t        *testing.T
	fake     *fakes3.Server
	store    objectstorage.Store
	handler  *Handler
	resolver *stubResolver
	clock    *fakeClock
	bucket   Bucket
	host     string
	reports  *recordingReporter
}

type recordingReporter struct {
	mu      sync.Mutex
	batches []map[string]int64
}

func (r *recordingReporter) Report(_ context.Context, counts map[string]int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := map[string]int64{}
	for k, v := range counts {
		cp[k] = v
	}
	r.batches = append(r.batches, cp)
	return nil
}

func newGW(t *testing.T, cors ...objectstorage.CORSRule) *gwEnv {
	return newGWWith(t, DefaultLimits(), cors...)
}

func newGWWith(t *testing.T, limits LimitConfig, cors ...objectstorage.CORSRule) *gwEnv {
	t.Helper()
	fake := fakes3.New(objectstorage.Oregon.Bucket())
	t.Cleanup(fake.Close)
	cfg := objectstorage.Config{
		Endpoint: fake.URL, Region: "us-west-1", Bucket: objectstorage.Oregon.Bucket(), AccessKey: "a", SecretKey: "s", MaxAttempts: 1,
	}
	bucket := Bucket{ID: ids.New(ids.ObjectBucket), WorkspaceID: ids.New(ids.Workspace), Region: objectstorage.Oregon, CORS: cors}
	host, _ := publichost.Host(bucket.ID, testBaseDomain)
	resolver := &stubResolver{buckets: map[string]Bucket{host: bucket}}
	clock := newClock()
	reports := &recordingReporter{}
	meter := NewMeter(reports)
	h := NewHandler(resolver, NewWasabiOrigin(map[objectstorage.Region]objectstorage.Config{objectstorage.Oregon: cfg}),
		NewLimits(limits), meter, NewMetrics())
	h.cache.now = clock.now
	h.Limits.now = clock.now
	return &gwEnv{
		t: t, fake: fake, store: objectstorage.NewStore(cfg), handler: h, resolver: resolver, clock: clock,
		bucket: bucket, host: host, reports: reports,
	}
}

// put stores an object in the bucket the way a tenant would, with the entity
// headers it was written with.
func (e *gwEnv) put(key string, body []byte, headers objectstorage.ObjectHeaders) {
	e.t.Helper()
	wasabiKey, err := objectstorage.ObjectKey(e.bucket.WorkspaceID, e.bucket.ID, key)
	if err != nil {
		e.t.Fatal(err)
	}
	if _, err := e.store.PutStream(e.t.Context(), wasabiKey, int64(len(body)), bytes.NewReader(body), headers); err != nil {
		e.t.Fatal(err)
	}
}

// request sends one request to the gateway with the given Host and returns what
// it answered.
func (e *gwEnv) request(host, method, target string, headers ...string) *httptest.ResponseRecorder {
	e.t.Helper()
	req := httptest.NewRequest(method, target, nil)
	req.Host = host
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	return rec
}

// do is request to the bucket's own host.
func (e *gwEnv) do(method, target string, headers ...string) *httptest.ResponseRecorder {
	e.t.Helper()
	return e.request(e.host, method, target, headers...)
}

// objectRequests counts the requests that reached Wasabi for an object.
func (e *gwEnv) objectRequests(method string) int {
	n := 0
	for _, r := range e.fake.Requests() {
		if r.Key != "" && r.Method == method {
			n++
		}
	}
	return n
}

// wasabiKeys lists every key any request asked Wasabi for.
func (e *gwEnv) wasabiKeys() []string {
	var keys []string
	for _, r := range e.fake.Requests() {
		keys = append(keys, r.Key)
	}
	return keys
}

// noListing fails the test if any request reached Wasabi as a bucket listing.
func (e *gwEnv) noListing() {
	e.t.Helper()
	for _, r := range e.fake.Requests() {
		if r.Key == "" {
			e.t.Fatalf("a request reached Wasabi as a bucket-level call: %s %v", r.Method, r.Query)
		}
	}
}
