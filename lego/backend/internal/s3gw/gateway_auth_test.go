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
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
)

func TestAnUnsignedRequestToAnyPathIsAccessDeniedXMLNeverAnotherSurface(t *testing.T) {
	g := newRig(t)
	reached := false
	for _, op := range []string{"ListBuckets", "ListObjects", "GetObject", "PutObject"} {
		spyOn(t, op, func(*request) *Error { reached = true; return nil })
	}
	for _, p := range nonS3Paths {
		rec, doc := g.do(unsignedGET(p))
		if rec.Code != http.StatusForbidden || doc.Code != "AccessDenied" {
			t.Errorf("GET %s unsigned = %d %q, want 403 AccessDenied", p, rec.Code, doc.Code)
		}
	}
	if reached || g.authz.askedCount() != 0 {
		t.Error("an unsigned request reached a handler or asked OpenFGA")
	}
}

func TestEveryNonS3PathIsABucketOrAKeyForASignedRequest(t *testing.T) {
	g := newRig(t)
	k := g.newKey(objectstorage.KeyPermissionRead, 0)
	var seen []string
	spyOn(t, "ListObjects", func(rq *request) *Error { seen = append(seen, "bucket:"+rq.bucket); return nil })
	spyOn(t, "GetObject", func(rq *request) *Error { seen = append(seen, "object:"+rq.bucket+"/"+rq.objectKey); return nil })
	for _, p := range []string{"/graphql", "/mcp", "/v1", "/oauth"} {
		if rec, doc := g.do(g.signed(k, "GET", p, nil)); rec.Code != 200 {
			t.Errorf("GET %s = %d %q, want it served as the bucket's listing", p, rec.Code, doc.Code)
		}
	}
	g.do(g.signed(k, "GET", "/v1/services/srv-1", nil))
	g.do(g.signed(k, "GET", "/mcp/cloud-agents", nil))
	want := []string{"bucket:graphql", "bucket:mcp", "bucket:v1", "bucket:oauth", "object:v1/services/srv-1", "object:mcp/cloud-agents"}
	if strings.Join(seen, ",") != strings.Join(want, ",") {
		t.Errorf("a bucket named like another surface's route: seen %v, want %v", seen, want)
	}
}

func TestAnAuthenticatedRequestIsAuthorizedAsTheKeysCreatorOnItsOwnWorkspaceOnly(t *testing.T) {
	g := newRig(t)
	k := g.newKey(objectstorage.KeyPermissionRead, 0)
	var asked core.Identity
	var ws string
	spyOn(t, "ListObjects", func(rq *request) *Error {
		asked, _ = core.IdentityFrom(rq.ctx)
		ws, _ = core.WorkspaceFrom(rq.ctx)
		return nil
	})
	// The path names another workspace's id as a bucket: it is only a bucket name.
	rec, doc := g.do(g.signed(k, "GET", "/tea-"+strings.Repeat("b", 20), nil))
	if rec.Code != 200 {
		t.Fatalf("answered %d %q", rec.Code, doc.Code)
	}
	if asked.Subject != g.creator || asked.Method != core.MethodS3Key || asked.AuditSubject != k.id || asked.SignIn != core.SignInGoogle {
		t.Errorf("identity = %+v, want the creator acting by an S3 key named %s", asked, k.id)
	}
	if ws != g.ws {
		t.Errorf("workspace = %q, want the key's %q", ws, g.ws)
	}
	for _, q := range g.authz.asked {
		if !strings.HasSuffix(q, core.WorkspaceObject(g.ws)) || !strings.HasPrefix(q, "user:"+g.creator+" ") {
			t.Errorf("OpenFGA was asked %q: only the creator on the key's workspace may be", q)
		}
	}
}

func TestEachAuthenticationFailureHasItsS3Code(t *testing.T) {
	g := newRig(t)
	good := g.newKey(objectstorage.KeyPermissionRead, 0)
	revoked := g.newKey(objectstorage.KeyPermissionRead, 0)
	if err := g.keys.Revoke(context.Background(), g.ws, revoked.id); err != nil {
		t.Fatal(err)
	}
	expiring := g.newKey(objectstorage.KeyPermissionRead, time.Hour)
	spyOn(t, "ListBuckets", func(*request) *Error { return nil })

	cases := []struct {
		name string
		req  func() *http.Request
		want string
		code int
	}{
		{"a good key", func() *http.Request { return g.signed(good, "GET", "/", nil) }, "", 200},
		{"a wrong secret", func() *http.Request { return g.signed(testKey{good.id, good.secret + "x"}, "GET", "/", nil) }, "SignatureDoesNotMatch", 403},
		{"an unknown key id", func() *http.Request {
			return g.signed(testKey{"s3k-" + strings.Repeat("0", 20), good.secret}, "GET", "/", nil)
		}, "InvalidAccessKeyId", 403},
		{"a malformed key id", func() *http.Request { return g.signed(testKey{"AKIAIOSFODNN7EXAMPLE", good.secret}, "GET", "/", nil) }, "InvalidAccessKeyId", 403},
		{"a revoked key", func() *http.Request { return g.signed(revoked, "GET", "/", nil) }, "InvalidAccessKeyId", 403},
		{"a skewed clock", func() *http.Request {
			r := g.signed(good, "GET", "/", nil)
			g.now = g.now.Add(time.Hour) // the server's clock moves, the request keeps its date
			return r
		}, "RequestTimeTooSkewed", 403},
	}
	for _, c := range cases {
		g.now = testNow
		rec, doc := g.do(c.req())
		if rec.Code != c.code || doc.Code != c.want {
			t.Errorf("%s: %d %q, want %d %q", c.name, rec.Code, doc.Code, c.code, c.want)
		}
	}
	// An expired key: the same signed request, after its life.
	g.now = testNow
	r := g.signed(expiring, "GET", "/", nil)
	g.now = testNow.Add(10 * time.Minute) // inside the 15-minute window, past nothing yet
	if rec, doc := g.do(r); rec.Code != 200 {
		t.Errorf("an ephemeral key inside its life: %d %q", rec.Code, doc.Code)
	}
	g.now = testNow.Add(61 * time.Minute)
	r = g.signed(expiring, "GET", "/", nil)
	if rec, doc := g.do(r); rec.Code != 403 || doc.Code != "InvalidAccessKeyId" {
		t.Errorf("an expired key: %d %q, want 403 InvalidAccessKeyId", rec.Code, doc.Code)
	}
}

func TestAnOpenBaoFailureIsA503AnSDKRetriesNeverA403(t *testing.T) {
	g := newRig(t)
	k := g.newKey(objectstorage.KeyPermissionRead, 0)
	// A fresh replica has no cached secret.
	fresh := objectstorage.NewAccessKeys(g.repo, g.kv)
	fresh.Now = func() time.Time { return g.now }
	g.gw.cfg.Keys = fresh
	g.kv.GetErr = errors.New("openbao: connection refused")
	rec, doc := g.do(g.signed(k, "GET", "/", nil))
	if rec.Code != http.StatusServiceUnavailable || doc.Code != "ServiceUnavailable" {
		t.Errorf("OpenBao down = %d %q, want 503 ServiceUnavailable", rec.Code, doc.Code)
	}
}

func TestAKeyWhoseCreatorLostTheRoleIsRefusedOnTheNextRequest(t *testing.T) {
	g := newRig(t)
	k := g.newKey(objectstorage.KeyPermissionReadWrite, 0)
	reached := 0
	spyOn(t, "ListBuckets", func(*request) *Error { reached++; return nil })
	if rec, _ := g.do(g.signed(k, "GET", "/", nil)); rec.Code != 200 {
		t.Fatalf("before: %d", rec.Code)
	}
	g.authz.held = map[string]bool{} // the creator was demoted or removed
	rec, doc := g.do(g.signed(k, "GET", "/", nil))
	if rec.Code != http.StatusForbidden || doc.Code != "AccessDenied" || reached != 1 {
		t.Errorf("after: %d %q (handler ran %d times), want 403 AccessDenied and no second run", rec.Code, doc.Code, reached)
	}
}

func TestAnUnreachableAuthorizerIsA503AndNotA403(t *testing.T) {
	g := newRig(t)
	k := g.newKey(objectstorage.KeyPermissionRead, 0)
	g.gw.cfg.Base.Authz = failingChecker{}
	rec, doc := g.do(g.signed(k, "GET", "/", nil))
	if rec.Code != http.StatusServiceUnavailable || doc.Code != "ServiceUnavailable" {
		t.Errorf("OpenFGA down = %d %q, want 503 ServiceUnavailable", rec.Code, doc.Code)
	}
}

type failingChecker struct{}

func (failingChecker) Check(_ context.Context, _, _, _ string) (bool, error) {
	return false, errors.New("down")
}

func TestAReadKeyCannotWriteAndNeverAsksOpenFGAToLetItPublish(t *testing.T) {
	g := newRig(t)
	read := g.newKey(objectstorage.KeyPermissionRead, 0)
	readwrite := g.newKey(objectstorage.KeyPermissionReadWrite, 0)
	reached := false
	for _, op := range []string{"PutObject", "DeleteObject", "CreateBucket", "DeleteBucket", "PutBucketCors", "CreateMultipartUpload"} {
		spyOn(t, op, func(*request) *Error { reached = true; return nil })
	}
	for _, c := range []struct{ method, target string }{
		{"PUT", "/b/k"}, {"DELETE", "/b/k"}, {"PUT", "/b"}, {"DELETE", "/b"}, {"PUT", "/b?cors"}, {"POST", "/b/k?uploads"},
	} {
		before := g.authz.askedCount()
		rec, doc := g.do(g.signed(read, c.method, c.target, nil))
		if rec.Code != 403 || doc.Code != "AccessDenied" {
			t.Errorf("a read key %s %s = %d %q, want 403 AccessDenied", c.method, c.target, rec.Code, doc.Code)
		}
		if g.authz.askedCount() != before {
			t.Errorf("a read key's %s %s reached OpenFGA: the permission ceiling holds before it", c.method, c.target)
		}
	}
	if reached {
		t.Error("a read key reached a write handler")
	}
	// No key holds can_manage, whatever its permission: nothing asks for it.
	for _, k := range []testKey{read, readwrite} {
		g.do(g.signed(k, "GET", "/", nil))
	}
	for _, q := range g.authz.asked {
		if strings.Contains(q, core.RelCanManage) {
			t.Errorf("OpenFGA was asked for can_manage: %s", q)
		}
	}
	for _, rel := range []string{core.RelCanManage, core.RelCanManageKeys, "can_manage_ssh_keys"} {
		if permitted(objectstorage.KeyPermissionReadWrite, rel) || permitted(objectstorage.KeyPermissionRead, rel) {
			t.Errorf("a key is permitted %s", rel)
		}
	}
}

func TestAPresignedURLAuthenticatesAndExpires(t *testing.T) {
	g := newRig(t)
	k := g.newKey(objectstorage.KeyPermissionRead, 0)
	spyOn(t, "GetObject", func(*request) *Error { return nil })
	if rec, doc := g.do(g.presign(k, "GET", "/photos/cat.jpg", time.Hour)); rec.Code != 200 {
		t.Fatalf("a fresh presigned URL = %d %q", rec.Code, doc.Code)
	}
	r := g.presign(k, "GET", "/photos/cat.jpg", time.Minute)
	g.now = testNow.Add(2 * time.Minute)
	if rec, doc := g.do(r); rec.Code != 403 || doc.Code != "AccessDenied" {
		t.Errorf("a presigned URL past its expiry = %d %q, want 403 AccessDenied", rec.Code, doc.Code)
	}
	// A presigned URL signed for another key does not work for this object.
	g.now = testNow
	other := g.presign(k, "GET", "/photos/cat.jpg", time.Hour)
	other.URL.Path = "/photos/dog.jpg"
	if rec, doc := g.do(other); rec.Code != 403 || doc.Code != "SignatureDoesNotMatch" {
		t.Errorf("a presigned URL for another key = %d %q, want SignatureDoesNotMatch", rec.Code, doc.Code)
	}
}

func TestAnUnsignedXAmzHeaderOnASignedRequestIsRefusedBeforeAnyLookup(t *testing.T) {
	g := newRig(t)
	k := g.newKey(objectstorage.KeyPermissionReadWrite, 0)
	copied := false
	spyOn(t, "PutObject", func(*request) *Error { copied = true; return nil })
	r := g.signed(k, "PUT", "/photos/public-key", nil)
	r.Header.Set("x-amz-copy-source", "/photos/secret") // added after signing, by the holder
	if rec, doc := g.do(r); rec.Code != 403 || doc.Code != "AccessDenied" {
		t.Errorf("= %d %q, want 403 AccessDenied", rec.Code, doc.Code)
	}
	if copied {
		t.Error("a copy with an unsigned x-amz-copy-source ran")
	}
}

func unsignedGET(p string) *http.Request {
	return httptest.NewRequest(http.MethodGet, "https://objects.onbex.co"+p, nil)
}
