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
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
)

// w21/094: an object or bucket row names its bucket, and an object row its key, and
// a successful get leaves one row per request. These drive both contracts as an
// allowed and as a refused caller and read the rows the sink was handed.

type objectAuditFixture struct {
	cf       *cfHarness
	sink     *lockedAuditSink
	cfMux    http.Handler
	restMux  http.Handler
	identity context.Context
}

func newObjectAuditFixture(t *testing.T, allow bool) *objectAuditFixture {
	t.Helper()
	cf := newCFHarness(t)
	sink := &lockedAuditSink{}
	base := &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default", Authz: &fakeChecker{allow: allow}, Audit: sink}
	srv := NewServer(base, Deps{ObjectStorage: cf.svc, BaseDomain: cfTestBaseDomain})
	srv.HydraAdminURL = fakeHydraURL(t)
	return &objectAuditFixture{
		cf: cf, sink: sink, cfMux: srv.cloudflareRoutes(), restMux: srv.restHandler(),
		identity: core.WithIdentity(context.Background(), core.Identity{Subject: "user-1", Method: core.MethodSession}),
	}
}

// serve is one request as the signed-in caller; headers are name, value pairs.
func (f *objectAuditFixture) serve(mux http.Handler, method, path, body string, headers ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(f.identity)
	req.Header.Set("Content-Type", "application/json")
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func (f *objectAuditFixture) cfServe(method, suffix, body string, headers ...string) *httptest.ResponseRecorder {
	return f.serve(f.cfMux, method, f.cf.url(suffix), body, headers...)
}

func (f *objectAuditFixture) restServe(method, path, body string) *httptest.ResponseRecorder {
	return f.serve(f.restMux, method, "/v1/objects/"+f.cf.ws+"/oregon"+path, body)
}

// newBucket creates a bucket and discards the row that records it.
func (f *objectAuditFixture) newBucket(t *testing.T) string {
	t.Helper()
	name := "bucket-" + strings.TrimPrefix(ids.New(ids.ObjectBucket), "obk-")[:8]
	if rec := f.cfServe(http.MethodPost, "/buckets", `{"name":"`+name+`"}`); rec.Code != http.StatusOK {
		t.Fatalf("bucket create = %d %s", rec.Code, rec.Body)
	}
	f.sink.take()
	return name
}

// one fails unless exactly one row was recorded, and returns it.
func one(t *testing.T, what string, events []core.AuditEvent) core.AuditEvent {
	t.Helper()
	if len(events) != 1 {
		t.Fatalf("%s recorded %d audit rows, want 1: %+v", what, len(events), events)
	}
	return events[0]
}

func TestObjectAndBucketRowsNameTheirBucketAndKey(t *testing.T) {
	f := newObjectAuditFixture(t, true)
	bucket := f.newBucket(t)
	key := "reports/2026/q3.csv"

	for _, tc := range []struct {
		what         string
		do           func() *httptest.ResponseRecorder
		verb, target string
	}{
		{"wrangler's put", func() *httptest.ResponseRecorder {
			return f.cfServe(http.MethodPut, "/buckets/"+bucket+"/objects/"+key, "hello")
		}, objectstorage.AuditVerbPutObject, core.ObjectTarget(bucket, key)},
		{"wrangler's delete", func() *httptest.ResponseRecorder {
			return f.cfServe(http.MethodDelete, "/buckets/"+bucket+"/objects/"+key, "")
		}, objectstorage.AuditVerbDeleteObject, core.ObjectTarget(bucket, key)},
		{"Render's put", func() *httptest.ResponseRecorder {
			return f.restServe(http.MethodPut, "/exports/a.txt", `{"sizeBytes":5}`)
		}, objectstorage.AuditVerbPutObject, core.ObjectTarget("render-oregon", "exports/a.txt")},
		{"a cors set", func() *httptest.ResponseRecorder {
			return f.cfServe(http.MethodPut, "/buckets/"+bucket+"/cors", `{"rules":[]}`)
		}, objectstorage.AuditVerbSetBucketCORS, core.BucketTarget(bucket)},
		{"a cors delete", func() *httptest.ResponseRecorder {
			return f.cfServe(http.MethodDelete, "/buckets/"+bucket+"/cors", "")
		}, objectstorage.AuditVerbDeleteBucketCORS, core.BucketTarget(bucket)},
		{"a lifecycle set", func() *httptest.ResponseRecorder {
			return f.cfServe(http.MethodPut, "/buckets/"+bucket+"/lifecycle", `{"rules":[]}`)
		}, objectstorage.AuditVerbSetBucketLifecycle, core.BucketTarget(bucket)},
		{"a bucket update", func() *httptest.ResponseRecorder {
			return f.cfServe(http.MethodPatch, "/buckets/"+bucket, `{}`)
		}, objectstorage.AuditVerbUpdateBucket, core.BucketTarget(bucket)},
		{"the dev-url toggle", func() *httptest.ResponseRecorder {
			return f.cfServe(http.MethodPut, "/buckets/"+bucket+"/domains/managed", `{"enabled":true}`)
		}, objectstorage.AuditVerbSetBucketPublic, core.BucketTarget(bucket)},
		{"a bucket delete", func() *httptest.ResponseRecorder {
			return f.cfServe(http.MethodDelete, "/buckets/"+bucket, "")
		}, objectstorage.AuditVerbDeleteBucket, core.BucketTarget(bucket)},
	} {
		tc.do()
		ev := one(t, tc.what, f.sink.take())
		if ev.Verb != tc.verb || ev.Target != tc.target || ev.Outcome != core.AuditAllowed ||
			ev.Resource != core.WorkspaceObject(f.cf.ws) || ev.Caller != "user-1" {
			t.Errorf("%s recorded %+v, want an allowed %s row on target %q", tc.what, ev, tc.verb, tc.target)
		}
	}
}

// The create names its bucket once the bucket exists: the name is in the body,
// which is read only after the caller is authorized.
func TestABucketCreateRecordsItsRowOnceTheBucketExists(t *testing.T) {
	f := newObjectAuditFixture(t, true)
	name := "assets-" + strings.TrimPrefix(ids.New(ids.ObjectBucket), "obk-")[:6]

	if rec := f.cfServe(http.MethodPost, "/buckets", `{"name":"`+name+`"}`); rec.Code != http.StatusOK {
		t.Fatalf("create = %d %s", rec.Code, rec.Body)
	}
	ev := one(t, "a bucket create", f.sink.take())
	if ev.Verb != objectstorage.AuditVerbCreateBucket || ev.Target != core.BucketTarget(name) || ev.Outcome != core.AuditAllowed ||
		ev.Resource != core.WorkspaceObject(f.cf.ws) || ev.Relation != core.RelCanCreate || ev.Caller != "user-1" {
		t.Errorf("recorded %+v", ev)
	}

	// A create that made no bucket is an action that did not happen: no allowed row.
	for what, body := range map[string]string{
		"a name taken":        `{"name":"` + name + `"}`,
		"an invalid name":     `{"name":"No Spaces Or Capitals"}`,
		"a reserved name":     `{"name":"render-oregon"}`,
		"a body that is junk": `not json`,
	} {
		if rec := f.cfServe(http.MethodPost, "/buckets", body); rec.Code < 400 {
			t.Fatalf("%s: create = %d, want a refusal", what, rec.Code)
		}
		if events := f.sink.take(); len(events) != 0 {
			t.Errorf("%s recorded %+v, want no row for a create that made nothing", what, events)
		}
	}

	// A refusal of the caller is recorded, with no target: the body is not read
	// before the caller is known to be allowed.
	denied := newObjectAuditFixture(t, false)
	if rec := denied.cfServe(http.MethodPost, "/buckets", `{"name":"`+name+`"}`); rec.Code != http.StatusForbidden {
		t.Fatalf("refused create = %d", rec.Code)
	}
	ev = one(t, "a refused bucket create", denied.sink.take())
	if ev.Verb != objectstorage.AuditVerbCreateBucket || ev.Outcome != core.AuditDenied || ev.Target != "" {
		t.Errorf("recorded %+v, want a denied row with no target", ev)
	}
}

// The usage query names its bucket in the GraphQL body, jurisdiction prefix cut.
func TestTheUsageQueryRowNamesTheBucketOfItsBody(t *testing.T) {
	f := newObjectAuditFixture(t, false)
	body := `{"operationName":"` + cfStorageMetricsOperation + `","variables":{"accountTag":"` + f.cf.ws + `","filter":{"bucketName":"eu_assets"}}}`
	f.serve(f.cfMux, http.MethodPost, cloudflareGraphQLPath, body)
	ev := one(t, "the usage query", f.sink.take())
	if ev.Verb != objectstorage.AuditVerbGetBucketUsage || ev.Target != core.BucketTarget("assets") || ev.Outcome != core.AuditDenied {
		t.Errorf("recorded %+v", ev)
	}
}

// A successful get is one row per request, written once the object opened or the
// URL was minted. Not per chunk, not for a HEAD, a list or a get that found nothing.
func TestASuccessfulGetRecordsOneRowPerRequest(t *testing.T) {
	f := newObjectAuditFixture(t, true)
	bucket := f.newBucket(t)
	object := "/buckets/" + bucket + "/objects/big.bin"
	want := core.ObjectTarget(bucket, "big.bin")

	// 3 MiB crosses several copy buffers; the stream is still one request.
	payload := strings.Repeat("0123456789abcdef", 3<<16)
	if rec := f.cfServe(http.MethodPut, object, payload); rec.Code != http.StatusOK {
		t.Fatalf("put = %d %s", rec.Code, rec.Body)
	}
	f.sink.take()

	checkGet := func(what string, rec *httptest.ResponseRecorder, target string) {
		t.Helper()
		if rec.Code != http.StatusOK && rec.Code != http.StatusPartialContent {
			t.Fatalf("%s = %d %s", what, rec.Code, rec.Body)
		}
		ev := one(t, what, f.sink.take())
		if ev.Verb != "objectstorage.GetObject" || ev.Outcome != core.AuditAllowed ||
			ev.Target != target || ev.Resource != core.WorkspaceObject(f.cf.ws) || ev.Caller != "user-1" {
			t.Errorf("%s recorded %+v, want an allowed GetObject row on %q", what, ev, target)
		}
	}

	rec := f.cfServe(http.MethodGet, object, "")
	if rec.Body.Len() != len(payload) {
		t.Fatalf("the get returned %d bytes, want %d", rec.Body.Len(), len(payload))
	}
	checkGet("a streamed get", rec, want)
	// A ranged request is a request: one row whether the route honors the range or not.
	checkGet("a ranged get", f.cfServe(http.MethodGet, object, "", "Range", "bytes=0-9"), want)

	// Five at once are five rows: one per request.
	var wg sync.WaitGroup
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f.cfServe(http.MethodGet, object, "")
		}()
	}
	wg.Wait()
	if events := f.sink.take(); len(events) != 5 {
		t.Errorf("five concurrent gets recorded %d rows, want 5", len(events))
	}

	// No bytes, nothing found, or a list: nothing to record.
	for _, tc := range []struct {
		what string
		do   func() *httptest.ResponseRecorder
	}{
		{"a HEAD", func() *httptest.ResponseRecorder { return f.cfServe(http.MethodHead, object, "") }},
		{"a get of a missing key", func() *httptest.ResponseRecorder {
			return f.cfServe(http.MethodGet, "/buckets/"+bucket+"/objects/missing.bin", "")
		}},
		{"a get in a missing bucket", func() *httptest.ResponseRecorder {
			return f.cfServe(http.MethodGet, "/buckets/nosuchbucket/objects/big.bin", "")
		}},
		{"a bucket info", func() *httptest.ResponseRecorder { return f.cfServe(http.MethodGet, "/buckets/"+bucket, "") }},
		{"a bucket list", func() *httptest.ResponseRecorder { return f.cfServe(http.MethodGet, "/buckets", "") }},
		{"a cors read", func() *httptest.ResponseRecorder { return f.cfServe(http.MethodGet, "/buckets/"+bucket+"/cors", "") }},
		{"Render's list", func() *httptest.ResponseRecorder { return f.restServe(http.MethodGet, "", "") }},
	} {
		tc.do()
		if events := f.sink.take(); len(events) != 0 {
			t.Errorf("%s recorded %+v, want no row", tc.what, events)
		}
	}

	// Render's contract: the mint is the request. The URL is minted for an object
	// that is there, and the row names it in the workspace's system bucket.
	put := f.restServe(http.MethodPut, "/dumps/a.txt", `{"sizeBytes":5}`)
	if put.Code != http.StatusOK {
		t.Fatalf("mint put = %d %s", put.Code, put.Body)
	}
	f.sink.take()
	var minted struct {
		URL string `json:"url"`
	}
	_ = json.Unmarshal(put.Body.Bytes(), &minted)
	up, _ := http.NewRequest(http.MethodPut, minted.URL, strings.NewReader("hello"))
	if resp, err := http.DefaultClient.Do(up); err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("upload to the presigned URL: %v %v", resp, err)
	}
	checkGet("Render's get mint", f.restServe(http.MethodGet, "/dumps/a.txt", ""), core.ObjectTarget("render-oregon", "dumps/a.txt"))
	if rec := f.restServe(http.MethodGet, "/dumps/missing.txt", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("a mint for a missing object = %d", rec.Code)
	}
	if events := f.sink.take(); len(events) != 0 {
		t.Errorf("a mint that found no object recorded %+v", events)
	}
}

// A get whose mint the rate limiter refuses is not a download: only the mints
// that were answered 200 leave a row.
func TestAGetTheLimiterRefusedLeavesNoRow(t *testing.T) {
	f := newObjectAuditFixture(t, true)
	put := f.restServe(http.MethodPut, "/dumps/a.txt", `{"sizeBytes":5}`)
	var minted struct {
		URL string `json:"url"`
	}
	_ = json.Unmarshal(put.Body.Bytes(), &minted)
	up, _ := http.NewRequest(http.MethodPut, minted.URL, strings.NewReader("hello"))
	if resp, err := http.DefaultClient.Do(up); err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("upload: %v %v", resp, err)
	}
	f.sink.take()

	ok, refused := 0, 0
	for range objectstorage.MintBurst * 2 {
		switch rec := f.restServe(http.MethodGet, "/dumps/a.txt", ""); rec.Code {
		case http.StatusOK:
			ok++
		case http.StatusTooManyRequests:
			refused++
		default:
			t.Fatalf("mint = %d %s", rec.Code, rec.Body)
		}
	}
	if refused == 0 {
		t.Fatalf("the limiter refused none of %d mints", objectstorage.MintBurst*2)
	}
	if events := f.sink.take(); len(events) != ok {
		t.Errorf("%d mints were answered and %d rows were recorded (%d refused)", ok, len(events), refused)
	}
}

// A refused get records as it did before, now naming what it asked for.
func TestARefusedGetStillRecordsAndNamesItsObject(t *testing.T) {
	f := newObjectAuditFixture(t, false)
	for _, tc := range []struct {
		what   string
		do     func() *httptest.ResponseRecorder
		target string
	}{
		{"wrangler's get", func() *httptest.ResponseRecorder {
			return f.cfServe(http.MethodGet, "/buckets/assets/objects/secrets/prod.env", "")
		}, core.ObjectTarget("assets", "secrets/prod.env")},
		{"wrangler's HEAD", func() *httptest.ResponseRecorder {
			return f.cfServe(http.MethodHead, "/buckets/assets/objects/secrets/prod.env", "")
		}, core.ObjectTarget("assets", "secrets/prod.env")},
		{"Render's get mint", func() *httptest.ResponseRecorder {
			return f.restServe(http.MethodGet, "/secrets/prod.env", "")
		}, core.ObjectTarget("render-oregon", "secrets/prod.env")},
	} {
		tc.do()
		ev := one(t, tc.what, f.sink.take())
		if ev.Verb != objectstorage.AuditVerbGetObject || ev.Outcome != core.AuditDenied || ev.Target != tc.target ||
			ev.Relation != core.RelCanViewSensitive || ev.Resource != core.WorkspaceObject(f.cf.ws) {
			t.Errorf("%s recorded %+v, want a denied GetObject row on %q under can_view_sensitive", tc.what, ev, tc.target)
		}
	}
}

// Nothing a request carries beyond its path reaches a row: not the signed query,
// not a header, not the presigned URL a mint returns, and not a signed query a
// caller pasted into the key.
func TestNoSignedMaterialReachesAnObjectAuditRow(t *testing.T) {
	const (
		signature  = "5f1c0a9e77d3b2c48a6e1d0b9c8f7a6e5d4c3b2a1908f7e6d5c4b3a2910f8e7d"
		credential = "AKIAIOSFODNN7EXAMPLE"
		sessionTok = "FwoGZXIvYXdzEJr-session-token-value"
		bearer     = "bearer-secret-value-123"
	)
	signedQuery := "?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=" + credential + "%2F20261116%2Fus-west-1%2Fs3%2Faws4_request" +
		"&X-Amz-Date=20261116T000000Z&X-Amz-Expires=900&X-Amz-SignedHeaders=host&X-Amz-Signature=" + signature
	secrets := []string{signature, credential, sessionTok, bearer, "X-Amz-", "AWS4-HMAC", "Authorization"}
	headers := []string{
		"Authorization", "AWS4-HMAC-SHA256 Credential=" + credential + "/20261116/us-west-1/s3/aws4_request, Signature=" + signature,
		"X-Amz-Security-Token", sessionTok,
		"Cookie", "session=" + bearer,
		"Cf-R2-Metadata", bearer,
	}
	// A key that is a presigned URL's tail, as a script that put a URL in the key
	// would send it: its "?" and "=" are percent-encoded in the path.
	pasted := "a.txt" + url.PathEscape(signedQuery)

	for _, allow := range []bool{true, false} {
		f := newObjectAuditFixture(t, allow)
		bucket := "assets"
		if allow {
			bucket = f.newBucket(t)
		}
		var requests []func()
		for _, key := range []string{"report.pdf", pasted} {
			requests = append(requests,
				func() { f.cfServe(http.MethodPut, "/buckets/"+bucket+"/objects/"+key+signedQuery, "hello", headers...) },
				func() { f.cfServe(http.MethodGet, "/buckets/"+bucket+"/objects/"+key+signedQuery, "", headers...) },
				func() { f.cfServe(http.MethodHead, "/buckets/"+bucket+"/objects/"+key+signedQuery, "", headers...) },
				func() { f.cfServe(http.MethodDelete, "/buckets/"+bucket+"/objects/"+key+signedQuery, "", headers...) },
				func() { f.cfServe(http.MethodGet, "/buckets/"+bucket+signedQuery, "", headers...) },
				func() { f.restServe(http.MethodPut, "/"+key+signedQuery, `{"sizeBytes":5}`) },
				func() { f.restServe(http.MethodGet, "/"+key+signedQuery, "") },
			)
		}
		var rows []core.AuditEvent
		for _, do := range requests {
			do()
			rows = append(rows, f.sink.take()...)
		}
		if len(rows) < 6 {
			t.Fatalf("allow=%v: only %d rows recorded; the requests did not reach the audit", allow, len(rows))
		}
		for _, ev := range rows {
			stored, err := json.Marshal(ev)
			if err != nil {
				t.Fatal(err)
			}
			for _, secret := range secrets {
				if bytes.Contains(stored, []byte(secret)) {
					t.Errorf("allow=%v: the row of %s holds %q: %s", allow, ev.Verb, secret, stored)
				}
			}
			if strings.ContainsAny(ev.Target, "?&") && !strings.Contains(ev.Target, "?[redacted]") {
				t.Errorf("allow=%v: target %q holds a query", allow, ev.Target)
			}
		}
		// The pasted key is recorded, the signed tail replaced.
		var sawPasted bool
		for _, ev := range rows {
			sawPasted = sawPasted || strings.HasSuffix(ev.Target, "/a.txt?[redacted]")
		}
		if !sawPasted {
			t.Errorf("allow=%v: no row names the pasted key as a.txt?[redacted]: %+v", allow, rows)
		}
	}

	// The URL a mint returns carries a signature of its own; none of it is stored.
	f := newObjectAuditFixture(t, true)
	put := f.restServe(http.MethodPut, "/a.txt", `{"sizeBytes":5}`)
	var minted struct {
		URL string `json:"url"`
	}
	_ = json.Unmarshal(put.Body.Bytes(), &minted)
	u, err := url.Parse(minted.URL)
	if err != nil || u.Query().Get("X-Amz-Signature") == "" {
		t.Fatalf("the mint returned %q, want a presigned URL", minted.URL)
	}
	rows := f.sink.take()
	if len(rows) == 0 {
		t.Fatal("the mint recorded no row")
	}
	for _, ev := range rows {
		stored, _ := json.Marshal(ev)
		for _, leak := range []string{u.Query().Get("X-Amz-Signature"), u.Query().Get("X-Amz-Credential"), u.Host, "X-Amz-"} {
			if bytes.Contains(stored, []byte(leak)) {
				t.Errorf("the row of the mint holds %q: %s", leak, stored)
			}
		}
	}
}

// A refused caller's bucket and key are the caller's text, so a row stores them
// bounded, escaped and valid, not as sent.
func TestARefusedCallersKeyIsBoundedAndEscapedInTheRow(t *testing.T) {
	f := newObjectAuditFixture(t, false)
	for what, key := range map[string]string{
		"a newline":                "a%0Ab",
		"a NUL":                    "a%00b",
		"a carriage return":        "a%0Db",
		"a bidi override":          "a%E2%80%AEb",
		"bytes that are not UTF-8": "a%FF%C3b",
		"a backslash":              "a%5Cb",
		"1024 bytes":               strings.Repeat("k", 1024),
		"half a megabyte":          strings.Repeat("%00", 170000),
	} {
		f.cfServe(http.MethodDelete, "/buckets/assets/objects/"+key, "")
		ev := one(t, what, f.sink.take())
		stored := strings.TrimPrefix(ev.Target, "object:assets/")
		if !strings.HasPrefix(ev.Target, "object:assets/") || len(stored) > core.AuditKeyMaxBytes || !utf8.ValidString(ev.Target) {
			t.Errorf("%s: target %q is %d bytes, valid UTF-8 %v", what, ev.Target, len(stored), utf8.ValidString(ev.Target))
		}
		for _, r := range ev.Target {
			if r < 0x20 || r == 0x7f || r == '‮' {
				t.Errorf("%s: target %q holds the raw character %U", what, ev.Target, r)
			}
		}
	}
	// A bucket that is not a valid name, with a slash in it, cannot end itself early.
	f.cfServe(http.MethodDelete, "/buckets/a%2Fb/objects/k", "")
	ev := one(t, "a bucket with a slash", f.sink.take())
	if bucket, key, ok := core.SplitObjectTarget(strings.TrimPrefix(ev.Target, "object:")); !ok || strings.Contains(bucket, "/") || key != "k" {
		t.Errorf("target %q splits into bucket %q key %q", ev.Target, bucket, key)
	}
}

// wantSweptTarget is the target a refused call to pattern records, for the path
// TestEveryRegisteredRouteRecordsANamedVerb fills in: every wildcard of a
// Cloudflare route is the workspace id, a Render route has region oregon and key k.
func wantSweptTarget(pattern, ws string) string {
	_, path := splitPattern(pattern)
	switch {
	case pattern == "POST "+cloudflareGraphQLPath:
		return core.BucketTarget("b")
	case strings.HasPrefix(path, "/v1/objects/"):
		if strings.Contains(path, "{key...}") {
			return core.ObjectTarget("render-oregon", "k")
		}
		return core.BucketTarget("render-oregon")
	case strings.Contains(path, "{key...}"):
		return core.ObjectTarget(ws, ws)
	case strings.Contains(path, "{bucket}"):
		return core.BucketTarget(ws)
	}
	return "" // the bucket list and a bucket create: no bucket in their path
}
