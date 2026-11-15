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

package api

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage/fakes3"
)

const cfObj = cloudflareAccountR2 + "/buckets/{bucket}/objects/{key...}"

// cloudflareRelations is ADR112 D6 as a table: the relation each route asks of
// the account's workspace. TestEveryCloudflareRouteHasItsRelationPinned fails
// when a route is added without a row, so a new route is a decision, not a default.
var cloudflareRelations = map[string]string{
	"GET " + cloudflareAccountR2 + "/buckets":             core.RelCanView,
	"POST " + cloudflareAccountR2 + "/buckets":            core.RelCanCreate,
	"GET " + cloudflareAccountR2 + "/buckets/{bucket}":    core.RelCanView,
	"DELETE " + cloudflareAccountR2 + "/buckets/{bucket}": core.RelCanCreate,
	"PATCH " + cloudflareAccountR2 + "/buckets/{bucket}":  core.RelCanCreate,
	"PUT " + cfObj:    core.RelCanCreate,
	"GET " + cfObj:    core.RelCanViewSensitive,
	"DELETE " + cfObj: core.RelCanCreate,
	"GET " + cloudflareAccountR2 + "/buckets/{bucket}/cors":           core.RelCanView,
	"PUT " + cloudflareAccountR2 + "/buckets/{bucket}/cors":           core.RelCanCreate,
	"DELETE " + cloudflareAccountR2 + "/buckets/{bucket}/cors":        core.RelCanCreate,
	"GET " + cloudflareAccountR2 + "/buckets/{bucket}/lifecycle":      core.RelCanView,
	"PUT " + cloudflareAccountR2 + "/buckets/{bucket}/lifecycle":      core.RelCanCreate,
	"GET " + cloudflareAccountR2 + "/buckets/{bucket}/domains/custom": core.RelCanView,
}

// cloudflareRefusals are the routes that answer 10042 to a caller who may view
// the workspace; they are pinned by pattern prefix rather than one by one.
var cloudflareRefusalPrefixes = []string{
	cloudflareAccountR2 + "/buckets/{bucket}/lock",
	cloudflareAccountR2 + "/buckets/{bucket}/sippy",
	cloudflareAccountR2 + "/buckets/{bucket}/local-uploads",
	cloudflareAccountR2 + "/buckets/{bucket}/domains/custom",
	cloudflareAccountR2 + "/buckets/{bucket}/domains/managed",
	cloudflareMount + "/accounts/{accountId}/event_notifications/",
	cloudflareMount + "/accounts/{accountId}/r2-catalog/",
}

func relationFor(pattern string) (relation string, ok bool) {
	if r, found := cloudflareRelations[pattern]; found {
		return r, true
	}
	_, path, _ := strings.Cut(pattern, " ")
	for _, prefix := range cloudflareRefusalPrefixes {
		if strings.HasPrefix(path, prefix) {
			return core.RelCanView, true
		}
	}
	if pattern == "POST "+cloudflareGraphQLPath {
		return core.RelCanView, true
	}
	return "", false
}

func TestEveryCloudflareRouteHasItsRelationPinned(t *testing.T) {
	srv := &Server{CloudflareR2: newCloudflareR2(&core.Base{}, nil)}
	patterns := serveMuxPatterns(srv.cloudflareRoutes())
	if len(patterns) < 40 {
		t.Fatalf("walked %d patterns; the enumerator looks broken", len(patterns))
	}
	for _, p := range patterns {
		if _, ok := relationFor(p); !ok {
			t.Errorf("route %q has no relation in cloudflareRelations: decide what it needs (ADR112 D6)", p)
		}
	}
	for p := range cloudflareRelations {
		if !slices.Contains(patterns, p) {
			t.Errorf("cloudflareRelations lists %q, which is not a route", p)
		}
	}
	// Every pinned route is also in the scope matrix.
	for _, p := range patterns {
		if _, ok := classifiedOps["REST "+p]; !ok {
			t.Errorf("route %q is not in the scope matrix", p)
		}
	}
}

// sampleCFRequest turns a route pattern into a concrete request.
func sampleCFRequest(ws, pattern string) (method, path string) {
	method, path, _ = strings.Cut(pattern, " ")
	for old, with := range map[string]string{
		"{accountId}": ws, "{bucket}": "photos", "{key...}": "k.txt", "{domain}": "x.example.com", "{rest...}": "x",
	} {
		path = strings.ReplaceAll(path, old, with)
	}
	return method, path
}

// TestCloudflareEachRouteAsksForExactlyItsRelation is D6's table, enforced: a
// caller who holds only the relation a route needs is let in (whatever the
// route then says), and a caller who holds every other relation is refused with
// 10003 before the route does anything.
func TestCloudflareEachRouteAsksForExactlyItsRelation(t *testing.T) {
	all := []string{core.RelCanView, core.RelCanViewSensitive, core.RelCanCreate, core.RelCanManage}
	srv := &Server{CloudflareR2: newCloudflareR2(&core.Base{}, nil)}
	patterns := serveMuxPatterns(srv.cloudflareRoutes())
	for _, pattern := range patterns {
		relation, _ := relationFor(pattern)
		for _, held := range [][]string{{relation}, without(all, relation)} {
			c := &relationChecker{held: map[string]bool{}}
			for _, r := range held {
				c.held[r] = true
			}
			h := newCFHarnessAuthz(t, c)
			c.workspace = h.ws
			h.createBucketDirect("photos")
			method, path := sampleCFRequest(h.ws, pattern)
			body := []byte(`{"name":"x","rules":[]}`)
			if pattern == "POST "+cloudflareGraphQLPath {
				body = []byte(`{"operationName":"getR2StorageMetrics","variables":{"accountTag":"` + h.ws + `","filter":{"bucketName":"photos"}}}`)
			}
			r := h.do(t, method, path, body)
			holdsIt := slices.Contains(held, relation)
			switch {
			case holdsIt && r.errCode() == 10003:
				t.Errorf("%s with only %s was refused: %d %s", pattern, relation, r.Code, r.Raw)
			case !holdsIt && r.errCode() != 10003:
				t.Errorf("%s without %s was answered %d %s, want 403 with 10003", pattern, relation, r.Code, r.Raw)
			}
			if !holdsIt {
				for _, req := range h.fake.Requests() {
					if req.Key != "" {
						t.Errorf("%s: a refused caller reached Wasabi: %s %s", pattern, req.Method, req.Key)
					}
				}
			}
		}
	}
}

func without(all []string, drop string) []string {
	out := slices.Clone(all)
	return slices.DeleteFunc(out, func(s string) bool { return s == drop })
}

// createBucketDirect adds a bucket row without going through the routes, for
// callers who may not create one.
func (h *cfHarness) createBucketDirect(name string) {
	_, _ = h.repo.CreateObjectBucket(context.Background(), storeBucket(h.ws, name), objectstorage.MaxUserBuckets)
}

// TestCloudflareRolesSeeWhatDoesNotLeak runs the three workspace roles of
// ADR024 against the routes that matter: a viewer reads metadata but not
// bytes and cannot write; a developer reads and writes; and everyone is refused
// another workspace.
func TestCloudflareRolesSeeWhatTheirRelationsAllow(t *testing.T) {
	roles := map[string]map[string]bool{
		"viewer":    {core.RelCanView: true},
		"developer": {core.RelCanView: true, core.RelCanViewSensitive: true, core.RelCanCreate: true},
		"admin":     {core.RelCanView: true, core.RelCanViewSensitive: true, core.RelCanCreate: true, core.RelCanManage: true},
	}
	type step struct {
		name, method, suffix string
		body                 []byte
		allowed              []string
	}
	steps := []step{
		{"list buckets", http.MethodGet, "/buckets", nil, []string{"viewer", "developer", "admin"}},
		{"bucket info", http.MethodGet, "/buckets/photos", nil, []string{"viewer", "developer", "admin"}},
		{"cors list", http.MethodGet, "/buckets/photos/cors", nil, []string{"viewer", "developer", "admin"}},
		{"lifecycle list", http.MethodGet, "/buckets/photos/lifecycle", nil, []string{"viewer", "developer", "admin"}},
		{"domain list", http.MethodGet, "/buckets/photos/domains/custom", nil, []string{"viewer", "developer", "admin"}},
		{"object get", http.MethodGet, "/buckets/photos/objects/o", nil, []string{"developer", "admin"}},
		{"object put", http.MethodPut, "/buckets/photos/objects/o2", []byte("x"), []string{"developer", "admin"}},
		{"object delete", http.MethodDelete, "/buckets/photos/objects/o2", nil, []string{"developer", "admin"}},
		{"bucket create", http.MethodPost, "/buckets", []byte(`{"name":"new-one"}`), []string{"developer", "admin"}},
		{"cors set", http.MethodPut, "/buckets/photos/cors", []byte(`{"rules":[]}`), []string{"developer", "admin"}},
		{"cors delete", http.MethodDelete, "/buckets/photos/cors", nil, []string{"developer", "admin"}},
		{"lifecycle set", http.MethodPut, "/buckets/photos/lifecycle", []byte(`{"rules":[]}`), []string{"developer", "admin"}},
		{"bucket delete", http.MethodDelete, "/buckets/never-existed", nil, []string{"developer", "admin"}},
	}
	for _, role := range []string{"viewer", "developer", "admin"} {
		c := &relationChecker{held: roles[role]}
		h := newCFHarnessAuthz(t, c)
		c.workspace = h.ws
		h.createBucketDirect("photos")
		h.putDirect(t, "o", "bytes")
		for _, s := range steps {
			r := h.do(t, s.method, h.url(s.suffix), s.body)
			if slices.Contains(s.allowed, role) == (r.errCode() == 10003) {
				t.Errorf("%s as %s = %d %s; allowed roles are %v", s.name, role, r.Code, r.Raw, s.allowed)
			}
		}
		// Whatever the role, another workspace is refused every route.
		other := newWS()
		for _, s := range steps {
			if r := h.do(t, s.method, cloudflareMount+"/accounts/"+other+"/r2"+s.suffix, s.body); r.Code != http.StatusForbidden || r.errCode() != 10003 {
				t.Errorf("%s as %s in another workspace = %d %s, want 403 with 10003", s.name, role, r.Code, r.Raw)
			}
		}
	}
}

// putDirect stores an object in the harness's photos bucket through the Wasabi
// fake, bypassing authorization.
func (h *cfHarness) putDirect(t *testing.T, key, body string) {
	t.Helper()
	b, err := h.repo.GetObjectBucket(t.Context(), h.ws, "photos")
	if err != nil {
		t.Fatal(err)
	}
	h.fake.Put(objectstorage.Oregon.Bucket(), h.ws+"/"+b.ID+"/"+key, []byte(body))
}

// TestCloudflareScopedOAuthTokensMeetTheScopeClasses goes through the real
// handler chain with Hydra-stub tokens, as wrangler's CLOUDFLARE_API_TOKEN
// would arrive when it is a human OAuth token: bex.read lists but cannot read
// an object's bytes, put or delete; bex.sensitive reads (ADR112 D6).
func TestCloudflareScopedOAuthTokensMeetTheScopeClasses(t *testing.T) {
	build := func(scope string) (http.Handler, *fakes3.Server, string) {
		fake := fakes3.New(objectstorage.Oregon.Bucket())
		t.Cleanup(fake.Close)
		st := objectstorage.NewStore(objectstorage.Config{Endpoint: fake.URL, Region: "us-west-1", Bucket: objectstorage.Oregon.Bucket(), AccessKey: "a", SecretKey: "s", MaxAttempts: 1})
		repo := &memObjectRepo{}
		svc := objectstorage.NewService(repo, map[objectstorage.Region]objectstorage.Store{objectstorage.Oregon: st})
		hydra := newClassHydraScoped(t, "identity-1", "dcr-client", "openid "+scope, []string{bexResource}, map[string]bool{"dcr-client": false})
		srv := NewServer(&core.Base{}, Deps{ObjectStorage: svc})
		srv.HydraAdminURL = hydra.url
		srv.OAuthResource = bexResource
		srv.OAuthRequireAudience = true
		srv.OAuthPlatformClients = hydra.platformClientIDs
		h, err := srv.Handler()
		if err != nil {
			t.Fatal(err)
		}
		ws := ids.New(ids.Workspace)
		// photos exists, with one object, so the routes have something to answer.
		b, _ := repo.CreateObjectBucket(t.Context(), storeBucket(ws, "photos"), objectstorage.MaxUserBuckets)
		fake.Put(objectstorage.Oregon.Bucket(), ws+"/"+b.ID+"/o", []byte("secret"))
		return h, fake, cloudflareMount + "/accounts/" + ws + "/r2"
	}
	scopeRefused := func(t *testing.T, w *httptest.ResponseRecorder, want string) {
		t.Helper()
		var env cfReply
		_ = json.Unmarshal(w.Body.Bytes(), &env.Env)
		if w.Code != http.StatusForbidden || env.Env.Success || len(env.Env.Errors) != 1 || env.Env.Errors[0].Code != 10003 || !strings.Contains(env.Env.Errors[0].Message, want) {
			t.Fatalf("answer = %d %s, want 403 in the envelope with 10003 naming %s", w.Code, w.Body, want)
		}
	}

	read, _, base := build(core.ScopeRead)
	if w := do(t, read, http.MethodGet, base+"/buckets", testToken, ""); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"photos"`) {
		t.Fatalf("a bex.read token listing = %d %s", w.Code, w.Body)
	}
	if w := do(t, read, http.MethodGet, base+"/buckets/photos", testToken, ""); w.Code != http.StatusOK {
		t.Fatalf("a bex.read token reading bucket info = %d %s", w.Code, w.Body)
	}
	// bucket info also POSTs its GraphQL size query, which writes nothing.
	query := `{"operationName":"getR2StorageMetrics","variables":{"accountTag":"` + strings.Split(base, "/")[4] + `","filter":{"bucketName":"photos"}}}`
	if w := do(t, read, http.MethodPost, cloudflareGraphQLPath, testToken, query); w.Code != http.StatusOK {
		t.Fatalf("a bex.read token asking for bucket metrics = %d %s", w.Code, w.Body)
	}
	scopeRefused(t, do(t, read, http.MethodGet, base+"/buckets/photos/objects/o", testToken, ""), core.ScopeSensitive)
	scopeRefused(t, do(t, read, http.MethodPut, base+"/buckets/photos/objects/o", testToken, "x"), core.ScopeWrite)
	scopeRefused(t, do(t, read, http.MethodDelete, base+"/buckets/photos/objects/o", testToken, ""), core.ScopeWrite)
	scopeRefused(t, do(t, read, http.MethodPut, base+"/buckets/photos/cors", testToken, `{"rules":[]}`), core.ScopeWrite)

	write, _, base := build(core.ScopeWrite)
	if w := do(t, write, http.MethodPut, base+"/buckets/photos/objects/o2", testToken, "x"); w.Code != http.StatusOK {
		t.Fatalf("a bex.write token putting = %d %s", w.Code, w.Body)
	}
	scopeRefused(t, do(t, write, http.MethodGet, base+"/buckets/photos/objects/o", testToken, ""), core.ScopeSensitive)

	sensitive, _, base := build(core.ScopeSensitive)
	if w := do(t, sensitive, http.MethodGet, base+"/buckets/photos/objects/o", testToken, ""); w.Code != http.StatusOK || w.Body.String() != "secret" {
		t.Fatalf("a bex.sensitive token reading = %d %q", w.Code, w.Body)
	}
}

func TestCloudflareScopeMatrixClassifiesTheObjectRoutes(t *testing.T) {
	want := map[string]string{
		"REST GET " + cfObj:                             core.OpClassSensitive,
		"REST PUT " + cfObj:                             core.OpClassWrite,
		"REST DELETE " + cfObj:                          core.OpClassWrite,
		"REST GET " + cloudflareAccountR2 + "/buckets":  core.OpClassRead,
		"REST POST " + cloudflareAccountR2 + "/buckets": core.OpClassWrite,
		"REST POST " + cloudflareGraphQLPath:            core.OpClassRead,
	}
	for _, op := range slices.Sorted(maps.Keys(want)) {
		if got := classifiedOps[op]; got != want[op] {
			t.Errorf("%s is %q, want %q", op, got, want[op])
		}
	}
}
