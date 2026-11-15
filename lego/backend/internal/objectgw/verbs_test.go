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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage/fakes3"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage/publichost"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const testToken = "0123456789abcdef0123456789abcdef0123456789abcdef"

// memRepo is the registry bex-api's half needs, in memory: the buckets, and the
// two calls the public routes make. Everything else of the interface is nil and
// would panic, which is the point: these tests touch only what they should.
type memRepo struct {
	objectstorage.Repository
	mu      sync.Mutex
	buckets map[string]store.ObjectBucket
}

func (r *memRepo) GetObjectBucketByID(_ context.Context, id string) (store.ObjectBucket, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if b, ok := r.buckets[id]; ok {
		return b, nil
	}
	return store.ObjectBucket{}, store.ErrNotFound
}

func (r *memRepo) SetObjectBucketPublic(_ context.Context, _, id string, public bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.buckets[id]
	if !ok {
		return store.ErrNotFound
	}
	b.Public = public
	r.buckets[id] = b
	return nil
}

func (r *memRepo) remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.buckets, id)
}

type recordingSink struct {
	mu     sync.Mutex
	deltas []store.ObjectStorageUsageDelta
}

func (s *recordingSink) AddObjectStorageUsage(_ context.Context, d []store.ObjectStorageUsageDelta) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deltas = append(s.deltas, d...)
	return nil
}

func (s *recordingSink) total(workspaceID string) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	var n int64
	for _, d := range s.deltas {
		if d.WorkspaceID == workspaceID && d.Kind == store.UsageKindObjectStorageEgressBytes {
			n += d.Quantity
		}
	}
	return n
}

// apiEnv is bex-api's half over a real objectstorage.Service and an in-memory
// registry, behind an httptest server.
type apiEnv struct {
	t      *testing.T
	repo   *memRepo
	svc    *objectstorage.Service
	sink   *recordingSink
	server *httptest.Server
	fake   *fakes3.Server
	ws     string
}

func newAPI(t *testing.T, baseDomain string) *apiEnv {
	t.Helper()
	fake := fakes3.New(objectstorage.Oregon.Bucket())
	t.Cleanup(fake.Close)
	cfg := objectstorage.Config{Endpoint: fake.URL, Region: "us-west-1", Bucket: objectstorage.Oregon.Bucket(), AccessKey: "a", SecretKey: "s", MaxAttempts: 1}
	repo := &memRepo{buckets: map[string]store.ObjectBucket{}}
	svc := objectstorage.NewService(repo, map[objectstorage.Region]objectstorage.Store{objectstorage.Oregon: objectstorage.NewStore(cfg)})
	sink := &recordingSink{}
	svc.SetUsageSink(sink)
	verbs := &Verbs{Token: testToken, Service: svc, BaseDomain: baseDomain}
	mux := http.NewServeMux()
	mux.Handle(PathPrefix, verbs.Handler())
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &apiEnv{t: t, repo: repo, svc: svc, sink: sink, server: srv, fake: fake, ws: ids.New(ids.Workspace)}
}

// addBucket registers a bucket in the registry and returns it with its host.
func (a *apiEnv) addBucket(public bool, rules ...objectstorage.CORSRule) (store.ObjectBucket, string) {
	a.t.Helper()
	raw, _ := json.Marshal(rules)
	if len(rules) == 0 {
		raw = []byte("[]")
	}
	b := store.ObjectBucket{ID: ids.New(ids.ObjectBucket), WorkspaceID: a.ws, Name: "photos", Region: "oregon", Public: public, CORS: raw}
	a.repo.mu.Lock()
	a.repo.buckets[b.ID] = b
	a.repo.mu.Unlock()
	host, _ := publichost.Host(b.ID, testBaseDomain)
	return b, host
}

func (a *apiEnv) call(method, path, token, body string) (int, string) {
	a.t.Helper()
	req, err := http.NewRequest(method, a.server.URL+path, strings.NewReader(body))
	if err != nil {
		a.t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		a.t.Fatal(err)
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(resp.Body)
	if cc := resp.Header.Get("Cache-Control"); resp.StatusCode == http.StatusOK && cc != "no-store" {
		a.t.Errorf("%s %s: Cache-Control = %q, want no-store", method, path, cc)
	}
	return resp.StatusCode, buf.String()
}

func lookupPath(host string) string { return LookupPath + "?host=" + url.QueryEscape(host) }

func TestTheVerbsAnswerOnlyToTheGatewaysBearer(t *testing.T) {
	a := newAPI(t, testBaseDomain)
	_, host := a.addBucket(true)
	for _, c := range []struct{ method, path string }{
		{http.MethodGet, lookupPath(host)}, {http.MethodPost, EgressPath},
	} {
		for name, token := range map[string]string{"none": "", "wrong": "nope", "a prefix of the right one": testToken[:20], "the right one with more": testToken + "x"} {
			if code, body := a.call(c.method, c.path, token, "{}"); code != http.StatusUnauthorized {
				t.Errorf("%s %s with %s bearer = %d %s, want 401", c.method, c.path, name, code, body)
			}
		}
	}
	if code, _ := a.call(http.MethodGet, lookupPath(host), testToken, ""); code != http.StatusOK {
		t.Errorf("lookup with the bearer = %d, want 200", code)
	}

	// A mount with no token configured refuses everything, even a request that
	// carries an empty bearer: a misconfiguration fails closed.
	open := httptest.NewServer((&Verbs{Service: a.svc, BaseDomain: testBaseDomain}).Handler())
	defer open.Close()
	for _, token := range []string{"", "Bearer ", "Bearer x"} {
		req, _ := http.NewRequest(http.MethodGet, open.URL+lookupPath(host), nil)
		if token != "" {
			req.Header.Set("Authorization", token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("a verb with no configured token and Authorization %q = %d, want 503", token, resp.StatusCode)
		}
	}
}

func TestLookupAnswersOnlyForAPublicBucket(t *testing.T) {
	a := newAPI(t, testBaseDomain)
	rule := corsRule([]string{"https://app.example"}, []string{"GET"}, nil, []string{"ETag"}, i64(60))
	pub, pubHost := a.addBucket(true, rule)
	_, privHost := a.addBucket(false)
	gone, goneHost := a.addBucket(true)
	a.repo.remove(gone.ID)
	unknownHost := "pub-" + strings.Repeat("0", 20) + "." + testBaseDomain

	code, body := a.call(http.MethodGet, lookupPath(pubHost), testToken, "")
	var answer LookupAnswer
	if code != http.StatusOK || json.Unmarshal([]byte(body), &answer) != nil {
		t.Fatalf("public = %d %s", code, body)
	}
	if answer.BucketID != pub.ID || answer.WorkspaceID != a.ws || answer.Region != "oregon" || len(answer.CORS) != 1 || answer.CORS[0].Allowed.Origins[0] != "https://app.example" {
		t.Errorf("answer = %+v", answer)
	}
	// Private, deleted and unknown answer identically: the gateway, and so a
	// visitor, cannot tell a private bucket from one that never existed.
	var notFound string
	for name, host := range map[string]string{"private": privHost, "deleted": goneHost, "unknown": unknownHost} {
		code, body := a.call(http.MethodGet, lookupPath(host), testToken, "")
		if code != http.StatusNotFound {
			t.Errorf("%s = %d %s, want 404", name, code, body)
		}
		if notFound == "" {
			notFound = body
		} else if body != notFound {
			t.Errorf("%s answered %q, another not-found answered %q: the difference would be an oracle", name, body, notFound)
		}
	}
	// A host under another domain, a deeper name, the apex and junk are not a bucket's.
	label, _ := publichost.Label(pub.ID)
	for name, host := range map[string]string{
		"another domain": label + ".example.org", "deeper": "x." + label + "." + testBaseDomain, "apex": testBaseDomain,
		"junk": "not a host", "a service": "myservice." + testBaseDomain,
	} {
		if code, body := a.call(http.MethodGet, lookupPath(host), testToken, ""); code != http.StatusNotFound {
			t.Errorf("%s = %d %s, want 404", name, code, body)
		}
	}
	if code, _ := a.call(http.MethodGet, LookupPath, testToken, ""); code != http.StatusBadRequest {
		t.Errorf("no host = %d, want 400", code)
	}
	if code, _ := a.call(http.MethodPost, LookupPath, testToken, "{}"); code != http.StatusMethodNotAllowed {
		t.Errorf("POST lookup = %d, want 405", code)
	}
	// With no hosting domain nothing is a bucket's host.
	none := newAPI(t, "")
	_, host := none.addBucket(true)
	if code, _ := none.call(http.MethodGet, lookupPath(host), testToken, ""); code != http.StatusNotFound {
		t.Errorf("a deployment with no hosting domain = %d, want 404", code)
	}
}

func TestEgressReportsReachTheShadowMeter(t *testing.T) {
	a := newAPI(t, testBaseDomain)
	other := ids.New(ids.Workspace)
	body := fmt.Sprintf(`{"bytes":{%q:1500,%q:2500}}`, a.ws, other)
	if code, resp := a.call(http.MethodPost, EgressPath, testToken, body); code != http.StatusOK || !strings.Contains(resp, `"accepted":2`) {
		t.Fatalf("report = %d %s", code, resp)
	}
	if code, _ := a.call(http.MethodPost, EgressPath, testToken, fmt.Sprintf(`{"bytes":{%q:500}}`, a.ws)); code != http.StatusOK {
		t.Fatal(code)
	}
	// The service sums in memory and flushes on its own interval; stopping it flushes.
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { a.svc.RunMeter(ctx, time.Hour); close(done) }()
	cancel()
	<-done
	if got := a.sink.total(a.ws); got != 2000 {
		t.Errorf("workspace %s metered %d egress bytes, want 2000", a.ws, got)
	}
	if got := a.sink.total(other); got != 2500 {
		t.Errorf("the other workspace metered %d, want 2500", got)
	}
}

func TestAMalformedEgressReportIsRefusedWhole(t *testing.T) {
	a := newAPI(t, testBaseDomain)
	good := fmt.Sprintf("%q:100", a.ws)
	huge := strings.Repeat(`"tea-aaaaaaaaaaaaaaaaaaaa":1,`, MaxReportBodyBytes/20)
	for name, body := range map[string]string{
		"not JSON":              "nope",
		"not an object":         `[]`,
		"a service id":          fmt.Sprintf(`{"bytes":{%s,%q:5}}`, good, ids.New(ids.Service)),
		"junk for an id":        fmt.Sprintf(`{"bytes":{%s,"tea-../x":5}}`, good),
		"a negative count":      fmt.Sprintf(`{"bytes":{%s,%q:-1}}`, good, ids.New(ids.Workspace)),
		"an impossible count":   fmt.Sprintf(`{"bytes":{%q:%d}}`, a.ws, int64(1)<<51),
		"a body past its limit": `{"bytes":{` + huge + `"x":1}}`,
		"a string for a count":  fmt.Sprintf(`{"bytes":{%q:"100"}}`, a.ws),
	} {
		if code, resp := a.call(http.MethodPost, EgressPath, testToken, body); code != http.StatusBadRequest {
			t.Errorf("%s = %d %s, want 400", name, code, resp)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { a.svc.RunMeter(ctx, time.Hour); close(done) }()
	cancel()
	<-done
	if got := a.sink.total(a.ws); got != 0 {
		t.Errorf("a refused report metered %d bytes; the good entry beside a bad one must not count", got)
	}
	// An empty report is fine, and GET is not a verb of this route.
	if code, _ := a.call(http.MethodPost, EgressPath, testToken, `{"bytes":{}}`); code != http.StatusOK {
		t.Errorf("an empty report = %d", code)
	}
	if code, _ := a.call(http.MethodGet, EgressPath, testToken, ""); code != http.StatusMethodNotAllowed {
		t.Errorf("GET egress = %d, want 405", code)
	}
}
