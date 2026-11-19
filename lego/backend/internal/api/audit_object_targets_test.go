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
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
)

// w21/094: an object row names its bucket and key, and a successful get leaves
// one row per request. These drive Render's contract as an allowed and as a
// refused caller and read the rows the sink was handed.

type objectAuditFixture struct {
	ws       string
	sink     *lockedAuditSink
	restMux  http.Handler
	identity context.Context
}

func newObjectAuditFixture(t *testing.T, allow bool) *objectAuditFixture {
	t.Helper()
	svc, ws := newObjectService(t)
	sink := &lockedAuditSink{}
	base := &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default", Authz: &fakeChecker{allow: allow}, Audit: sink}
	srv := NewServer(base, Deps{ObjectStorage: svc, BaseDomain: objectsTestBaseDomain})
	srv.HydraAdminURL = fakeHydraURL(t)
	return &objectAuditFixture{
		ws: ws, sink: sink, restMux: srv.restHandler(),
		identity: core.WithIdentity(context.Background(), core.Identity{Subject: "user-1", Method: core.MethodSession}),
	}
}

// restServe is one request to Render's object routes as the signed-in caller.
func (f *objectAuditFixture) restServe(method, path, body string, headers ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/v1/objects/"+f.ws+"/oregon"+path, strings.NewReader(body)).WithContext(f.identity)
	req.Header.Set("Content-Type", "application/json")
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	f.restMux.ServeHTTP(rec, req)
	return rec
}

// one fails unless exactly one row was recorded, and returns it.
func one(t *testing.T, what string, events []core.AuditEvent) core.AuditEvent {
	t.Helper()
	if len(events) != 1 {
		t.Fatalf("%s recorded %d audit rows, want 1: %+v", what, len(events), events)
	}
	return events[0]
}

func TestObjectRowsNameTheirBucketAndKey(t *testing.T) {
	f := newObjectAuditFixture(t, true)
	key := "reports/2026/q3.csv"

	for _, tc := range []struct {
		what         string
		do           func() *httptest.ResponseRecorder
		verb, target string
	}{
		{"Render's put", func() *httptest.ResponseRecorder {
			return f.restServe(http.MethodPut, "/"+key, `{"sizeBytes":5}`)
		}, objectstorage.AuditVerbPutObject, core.ObjectTarget("render-oregon", key)},
		{"Render's delete", func() *httptest.ResponseRecorder {
			return f.restServe(http.MethodDelete, "/"+key, "")
		}, objectstorage.AuditVerbDeleteObject, core.ObjectTarget("render-oregon", key)},
	} {
		tc.do()
		ev := one(t, tc.what, f.sink.take())
		if ev.Verb != tc.verb || ev.Target != tc.target || ev.Outcome != core.AuditAllowed ||
			ev.Resource != core.WorkspaceObject(f.ws) || ev.Caller != "user-1" {
			t.Errorf("%s recorded %+v, want an allowed %s row on target %q", tc.what, ev, tc.verb, tc.target)
		}
	}
}

// A successful get is one row per request, written once the URL was minted. Not
// for a list, or a get that found nothing.
func TestASuccessfulGetRecordsOneRowPerRequest(t *testing.T) {
	f := newObjectAuditFixture(t, true)
	want := core.ObjectTarget("render-oregon", "dumps/a.txt")

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

	// The mint is the request. The URL is minted for an object that is there, and
	// the row names it in the workspace's system bucket.
	rec := f.restServe(http.MethodGet, "/dumps/a.txt", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get mint = %d %s", rec.Code, rec.Body)
	}
	ev := one(t, "Render's get mint", f.sink.take())
	if ev.Verb != "objectstorage.GetObject" || ev.Outcome != core.AuditAllowed ||
		ev.Target != want || ev.Resource != core.WorkspaceObject(f.ws) || ev.Caller != "user-1" {
		t.Errorf("Render's get mint recorded %+v, want an allowed GetObject row on %q", ev, want)
	}

	// Five at once are five rows: one per request.
	var wg sync.WaitGroup
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f.restServe(http.MethodGet, "/dumps/a.txt", "")
		}()
	}
	wg.Wait()
	if events := f.sink.take(); len(events) != 5 {
		t.Errorf("five concurrent gets recorded %d rows, want 5", len(events))
	}

	// Nothing found, or a list: nothing to record.
	for _, tc := range []struct {
		what string
		do   func() *httptest.ResponseRecorder
	}{
		{"a get of a missing key", func() *httptest.ResponseRecorder { return f.restServe(http.MethodGet, "/dumps/missing.txt", "") }},
		{"Render's list", func() *httptest.ResponseRecorder { return f.restServe(http.MethodGet, "", "") }},
	} {
		tc.do()
		if events := f.sink.take(); len(events) != 0 {
			t.Errorf("%s recorded %+v, want no row", tc.what, events)
		}
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
		{"Render's get mint", func() *httptest.ResponseRecorder {
			return f.restServe(http.MethodGet, "/secrets/prod.env", "")
		}, core.ObjectTarget("render-oregon", "secrets/prod.env")},
	} {
		tc.do()
		ev := one(t, tc.what, f.sink.take())
		if ev.Verb != objectstorage.AuditVerbGetObject || ev.Outcome != core.AuditDenied || ev.Target != tc.target ||
			ev.Relation != core.RelCanViewSensitive || ev.Resource != core.WorkspaceObject(f.ws) {
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
		"X-Custom-Metadata", bearer,
	}
	// A key that is a presigned URL's tail, as a script that put a URL in the key
	// would send it: its "?" and "=" are percent-encoded in the path.
	pasted := "a.txt" + url.PathEscape(signedQuery)

	for _, allow := range []bool{true, false} {
		f := newObjectAuditFixture(t, allow)
		var requests []func()
		for _, key := range []string{"report.pdf", pasted} {
			requests = append(requests,
				func() { f.restServe(http.MethodPut, "/"+key+signedQuery, `{"sizeBytes":5}`, headers...) },
				func() { f.restServe(http.MethodGet, "/"+key+signedQuery, "", headers...) },
				func() { f.restServe(http.MethodDelete, "/"+key+signedQuery, "", headers...) },
			)
		}
		var rows []core.AuditEvent
		for _, do := range requests {
			do()
			rows = append(rows, f.sink.take()...)
		}
		if len(rows) < 4 {
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

// A refused caller's key is the caller's text, so a row stores it bounded,
// escaped and valid, not as sent.
func TestARefusedCallersKeyIsBoundedAndEscapedInTheRow(t *testing.T) {
	f := newObjectAuditFixture(t, false)
	const prefix = "object:render-oregon/"
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
		f.restServe(http.MethodDelete, "/"+key, "")
		ev := one(t, what, f.sink.take())
		stored := strings.TrimPrefix(ev.Target, prefix)
		if !strings.HasPrefix(ev.Target, prefix) || len(stored) > core.AuditKeyMaxBytes || !utf8.ValidString(ev.Target) {
			t.Errorf("%s: target %q is %d bytes, valid UTF-8 %v", what, ev.Target, len(stored), utf8.ValidString(ev.Target))
		}
		for _, r := range ev.Target {
			if r < 0x20 || r == 0x7f || r == '‮' {
				t.Errorf("%s: target %q holds the raw character %U", what, ev.Target, r)
			}
		}
	}
}

// wantSweptTarget is the target a refused call to pattern records, for the path
// TestEveryRegisteredRouteRecordsANamedVerb fills in: a Render route has region
// oregon and key k.
func wantSweptTarget(pattern string) string {
	_, path := splitPattern(pattern)
	if strings.Contains(path, "{key...}") {
		return core.ObjectTarget("render-oregon", "k")
	}
	return core.BucketTarget("render-oregon")
}
