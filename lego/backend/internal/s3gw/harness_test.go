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

package s3gw

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage/fakekeys"
)

// testNow is the gateway's clock in the tests.
var testNow = time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)

// relations is a core.Checker that holds a fixed set of relations on one
// workspace for one subject, and records what it was asked.
type relations struct {
	mu        sync.Mutex
	workspace string
	subject   string
	held      map[string]bool
	asked     []string
}

func (c *relations) Check(_ context.Context, subject, relation, object string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.asked = append(c.asked, subject+" "+relation+" "+object)
	return subject == "user:"+c.subject && object == core.WorkspaceObject(c.workspace) && c.held[relation], nil
}

func (c *relations) askedCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.asked)
}

// rig is a configured gateway over in-memory keys and OpenBao.
type rig struct {
	t       *testing.T
	gw      *Gateway
	repo    *fakekeys.Repo
	kv      *fakekeys.KV
	keys    *objectstorage.AccessKeys
	authz   *relations
	ws      string
	creator string
	now     time.Time
}

func newRig(t *testing.T) *rig {
	t.Helper()
	g := &rig{t: t, repo: fakekeys.NewRepo(), kv: fakekeys.NewKV(), ws: "tea-" + strings.Repeat("a", 20), creator: "usr-alice", now: testNow}
	g.keys = objectstorage.NewAccessKeys(g.repo, g.kv)
	g.keys.Now = func() time.Time { return g.now }
	g.authz = &relations{workspace: g.ws, subject: g.creator, held: map[string]bool{
		core.RelCanView: true, core.RelCanViewSensitive: true, core.RelCanCreate: true,
	}}
	// The object store is enabled with no stores so Enabled() holds; handlers that
	// reach Wasabi are the tests of the tasks that implement them.
	svc := objectstorage.NewService(nil, map[objectstorage.Region]objectstorage.Store{objectstorage.Oregon: nil})
	g.gw = New(Config{Service: svc, Keys: g.keys, Base: &core.Base{Authz: g.authz}, Now: func() time.Time { return g.now }})
	return g
}

type testKey struct{ id, secret string }

func (g *rig) newKey(permission string, life time.Duration) testKey {
	g.t.Helper()
	c, err := g.keys.Create(context.Background(), objectstorage.CreateKeyInput{
		WorkspaceID: g.ws, CreatedBy: g.creator, Name: "test", Permission: permission, Lifetime: life, SignIn: core.SignInGoogle,
	})
	if err != nil {
		g.t.Fatal(err)
	}
	return testKey{c.Key.ID, c.Secret}
}

// signed returns a header-signed request for key: host, x-amz-date,
// x-amz-content-sha256 and every other x-amz-* header present are signed.
func (g *rig) signed(k testKey, method, target string, body io.Reader, mutate ...func(*http.Request)) *http.Request {
	g.t.Helper()
	r := httptest.NewRequest(method, "https://objects.onbex.co"+target, body)
	for _, m := range mutate {
		m(r)
	}
	signRequest(r, k.id, k.secret, "oregon", g.now)
	return r
}

// signRequest signs r with SigV4, the way an SDK does. It reuses the verifier's own
// canonicalization; the known answers pin that against botocore.
func signRequest(r *http.Request, keyID, secret, region string, now time.Time) {
	amzDate := now.UTC().Format(amzDateLayout)
	if r.Header.Get("X-Amz-Content-Sha256") == "" {
		r.Header.Set("X-Amz-Content-Sha256", unsignedPayload)
	}
	r.Header.Set("X-Amz-Date", amzDate)
	names := []string{"host"}
	for name := range r.Header {
		if lower := strings.ToLower(name); strings.HasPrefix(lower, "x-amz-") {
			names = append(names, lower)
		}
	}
	sort.Strings(names)
	p := &parsedAuth{form: formHeader, date: amzDate[:8], region: region, signedHeaders: names, amzDate: amzDate, payloadHash: r.Header.Get("X-Amz-Content-Sha256")}
	sts := algorithm + "\n" + amzDate + "\n" + p.date + "/" + region + "/s3/aws4_request\n" + hexSHA256([]byte(canonicalRequest(r, p)))
	sig := hmacHex(signingKey(secret, p.date, region), sts)
	r.Header.Set("Authorization", fmt.Sprintf("%s Credential=%s/%s/%s/s3/aws4_request, SignedHeaders=%s, Signature=%s",
		algorithm, keyID, p.date, region, strings.Join(names, ";"), sig))
}

// presign returns a presigned URL request for key, valid for expires.
func (g *rig) presign(k testKey, method, target string, expires time.Duration) *http.Request {
	g.t.Helper()
	amzDate := g.now.UTC().Format(amzDateLayout)
	sep := "?"
	if strings.Contains(target, "?") {
		sep = "&"
	}
	cred := strings.NewReplacer("/", "%2F").Replace(k.id + "/" + amzDate[:8] + "/oregon/s3/aws4_request")
	target += sep + "X-Amz-Algorithm=" + algorithm + "&X-Amz-Credential=" + cred + "&X-Amz-Date=" + amzDate +
		fmt.Sprintf("&X-Amz-Expires=%d&X-Amz-SignedHeaders=host", int(expires.Seconds()))
	r := httptest.NewRequest(method, "https://objects.onbex.co"+target, nil)
	p := &parsedAuth{form: formPresigned, date: amzDate[:8], region: "oregon", signedHeaders: []string{"host"}, amzDate: amzDate, payloadHash: unsignedPayload}
	sts := algorithm + "\n" + amzDate + "\n" + p.date + "/oregon/s3/aws4_request\n" + hexSHA256([]byte(canonicalRequest(r, p)))
	r.URL.RawQuery += "&X-Amz-Signature=" + hmacHex(signingKey(k.secret, p.date, "oregon"), sts)
	return r
}

func (g *rig) do(r *http.Request) (*httptest.ResponseRecorder, errorDoc) {
	g.t.Helper()
	rec := httptest.NewRecorder()
	g.gw.ServeHTTP(rec, r)
	var doc errorDoc
	if rec.Body.Len() > 0 && strings.HasPrefix(rec.Header().Get("Content-Type"), "application/xml") && rec.Code >= 400 {
		_ = xmlUnmarshal(rec.Body.Bytes(), &doc)
	}
	return rec, doc
}

// spyOn replaces an operation's handler with fn for one test.
func spyOn(t *testing.T, op string, fn handler) {
	t.Helper()
	prev, had := handlers[op]
	handlers[op] = fn
	t.Cleanup(func() {
		if had {
			handlers[op] = prev
		} else {
			delete(handlers, op)
		}
	})
}
