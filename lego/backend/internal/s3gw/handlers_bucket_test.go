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
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
)

func (p *pgRig) call(k testKey, method, target, body string, mutate ...func(*http.Request)) (*httptest.ResponseRecorder, errorDoc) {
	p.t.Helper()
	return p.do(p.signed(k, method, target, strings.NewReader(body), append([]func(*http.Request){func(r *http.Request) {
		r.ContentLength = int64(len(body))
		r.Header.Set("X-Amz-Content-Sha256", unsignedPayload)
	}}, mutate...)...))
}

func TestCreateBucketTakesALocationConstraintAndOneNameIsOneBucketPerWorkspace(t *testing.T) {
	p := newPGRig(t)
	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	loc := func(v string) string {
		return `<CreateBucketConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><LocationConstraint>` + v + `</LocationConstraint></CreateBucketConfiguration>`
	}
	for _, c := range []struct{ name, body, region string }{
		{"default-oregon", "", "oregon"}, {"oregon-named", loc("oregon"), "oregon"}, {"west-one", loc("us-west-1"), "oregon"}, {"west-two", loc("us-west-2"), "oregon"},
		{"frankfurt-named", loc("frankfurt"), "frankfurt"}, {"central-one", loc("eu-central-1"), "frankfurt"}, {"central-two", loc("eu-central-2"), "frankfurt"},
	} {
		if rec, doc := p.call(k, "PUT", "/"+c.name, c.body); rec.Code != 200 || rec.Header().Get("Location") != "/"+c.name {
			t.Fatalf("create %s = %d %q (Location %q)", c.name, rec.Code, doc.Code, rec.Header().Get("Location"))
		}
		rec, _ := p.call(k, "HEAD", "/"+c.name, "")
		if rec.Code != 200 || rec.Header().Get("x-amz-bucket-region") != c.region {
			t.Errorf("%s: HEAD = %d region %q, want %s", c.name, rec.Code, rec.Header().Get("x-amz-bucket-region"), c.region)
		}
	}
	for _, c := range []struct {
		name, body, code string
		status           int
	}{
		{"ohio-bucket", loc("ohio"), "InvalidLocationConstraint", 400},
		{"virginia-bucket", loc("us-east-1"), "InvalidLocationConstraint", 400},
		{"singapore-bucket", loc("singapore"), "InvalidLocationConstraint", 400},
		{"Bad_Name", "", "InvalidBucketName", 400},
		{"ab", "", "InvalidBucketName", 400},
		{"render-oregon", "", "InvalidBucketName", 400},
		{"dotted.name", "", "InvalidBucketName", 400},
		{"default-oregon", "", "BucketAlreadyOwnedByYou", 409},
		{"malformed-xml", "<CreateBucketConfiguration", "MalformedXML", 400},
	} {
		if rec, doc := p.call(k, "PUT", "/"+c.name, c.body); rec.Code != c.status || doc.Code != c.code {
			t.Errorf("create %s = %d %q, want %d %s", c.name, rec.Code, doc.Code, c.status, c.code)
		}
	}
	// The same name in another workspace is a different bucket, never BucketAlreadyExists.
	if _, err := p.Svc.CreateBucket(context.Background(), p.otherWorkspace(), "elsewhere", objectstorage.Oregon); err != nil {
		t.Fatal(err)
	}
	if rec, doc := p.call(k, "PUT", "/elsewhere", ""); rec.Code != 200 {
		t.Errorf("a name another workspace holds = %d %q, want it free here", rec.Code, doc.Code)
	}
	if rec, doc := p.call(k, "GET", "/", ""); rec.Code != 200 {
		t.Fatalf("list = %d %q", rec.Code, doc.Code)
	} else {
		names := bucketNames(t, rec.Body.String())
		if len(names) != 8 || !strings.Contains(rec.Body.String(), "<Owner><ID>"+p.ws+"</ID>") {
			t.Errorf("ListBuckets = %v with owner %s, want the 8 buckets of this workspace only", names, p.ws)
		}
	}
	// ListBuckets names the system buckets Render's routes created.
	if _, err := p.Svc.SystemBucket(context.Background(), p.ws, objectstorage.Frankfurt); err != nil {
		t.Fatal(err)
	}
	if rec, _ := p.call(k, "GET", "/", ""); !strings.Contains(rec.Body.String(), "<Name>render-frankfurt</Name>") {
		t.Errorf("ListBuckets omits render-frankfurt: %s", rec.Body)
	}
}

// Until ADR112 D3's removal of Contract B (w21/m500) the S3 surface refused a bucket
// named cloudflare, because a key under client/v4/ would have reached the
// Cloudflare-shaped route on the same host instead of this gateway. That route is
// gone, so the name is an ordinary one and such a key is an ordinary key.
func TestABucketNamedCloudflareIsAnOrdinaryBucketNowThatTheRouteItProtectedIsGone(t *testing.T) {
	p := newPGRig(t)
	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	if rec, doc := p.call(k, "PUT", "/cloudflare", ""); rec.Code != 200 {
		t.Fatalf("create cloudflare = %d %q %s", rec.Code, doc.Code, doc.Message)
	}
	const key = "/cloudflare/client/v4/accounts/x/r2/buckets"
	if rec, doc := p.put(k, key, 7); rec.Code != 200 {
		t.Fatalf("put %s = %d %q %s", key, rec.Code, doc.Code, doc.Message)
	}
	rec, _ := p.call(k, "HEAD", key, "")
	if rec.Code != 200 || rec.Header().Get("Content-Length") != "7" {
		t.Errorf("HEAD %s = %d length %q, want 200 and 7", key, rec.Code, rec.Header().Get("Content-Length"))
	}
}

func TestACreatedBucketIsAuditedOnceItExistsAndAFailedCreateIsNot(t *testing.T) {
	p := newPGRig(t)
	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	p.call(k, "PUT", "/photos", "")
	p.call(k, "PUT", "/photos", "")   // taken
	p.call(k, "PUT", "/Bad_Name", "") // invalid
	rows := p.Audit.byVerb(objectstorage.AuditVerbCreateBucket)
	if len(rows) != 1 || rows[0].Target != "bucket:photos" || rows[0].Caller != k.id || rows[0].CallerMethod != core.MethodS3Key || rows[0].Outcome != core.AuditAllowed {
		t.Fatalf("create rows = %+v, want one allowed row for bucket:photos by key %s", rows, k.id)
	}
}

func TestBucketOperationsAreScopedToTheKeysWorkspace(t *testing.T) {
	p := newPGRig(t)
	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	other := p.otherWorkspace()
	foreign, err := p.Svc.CreateBucket(context.Background(), other, "theirs", objectstorage.Oregon)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ method, target string }{
		{"HEAD", "/theirs"}, {"GET", "/theirs?location"}, {"GET", "/theirs?cors"}, {"GET", "/theirs?lifecycle"}, {"DELETE", "/theirs"},
		// The other workspace's bucket id is no bucket name either.
		{"HEAD", "/" + foreign.ID},
	} {
		rec, doc := p.call(k, c.method, c.target, "")
		if c.method != "HEAD" && (rec.Code != 404 || doc.Code != "NoSuchBucket") || c.method == "HEAD" && rec.Code != 404 {
			t.Errorf("%s %s = %d %q, want 404 NoSuchBucket", c.method, c.target, rec.Code, doc.Code)
		}
	}
	if b, err := p.Svc.GetBucket(context.Background(), other, "theirs"); err != nil || b.ID != foreign.ID {
		t.Errorf("the other workspace's bucket was touched: %+v %v", b, err)
	}
}

func TestDeleteBucketNeedsAnEmptyBucketAndNeverDeletesTheSystemBucket(t *testing.T) {
	p := newPGRig(t)
	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	p.call(k, "PUT", "/photos", "")
	b, _ := p.Svc.GetBucket(context.Background(), p.ws, "photos")
	p.Fakes[objectstorage.Oregon].Put(objectstorage.Oregon.Bucket(), objectstorage.Prefix(p.ws, b.ID)+"cat.jpg", []byte("meow"))
	if rec, doc := p.call(k, "DELETE", "/photos", ""); rec.Code != 409 || doc.Code != "BucketNotEmpty" {
		t.Errorf("deleting a bucket holding an object = %d %q, want 409 BucketNotEmpty", rec.Code, doc.Code)
	}
	p.Fakes[objectstorage.Oregon].Delete(objectstorage.Oregon.Bucket(), objectstorage.Prefix(p.ws, b.ID)+"cat.jpg")
	if rec, doc := p.call(k, "DELETE", "/photos", ""); rec.Code != 204 {
		t.Errorf("deleting an empty bucket = %d %q, want 204", rec.Code, doc.Code)
	}
	if rec, _ := p.call(k, "HEAD", "/photos", ""); rec.Code != 404 {
		t.Errorf("a deleted bucket answers %d", rec.Code)
	}
	if _, err := p.Svc.SystemBucket(context.Background(), p.ws, objectstorage.Oregon); err != nil {
		t.Fatal(err)
	}
	if rec, doc := p.call(k, "DELETE", "/render-oregon", ""); rec.Code != 403 || doc.Code != "AccessDenied" {
		t.Errorf("deleting render-oregon = %d %q, want 403 AccessDenied", rec.Code, doc.Code)
	}
	// The same name, recreated, never sees the old objects: the key embeds the id.
	p.call(k, "PUT", "/photos", "")
	nb, _ := p.Svc.GetBucket(context.Background(), p.ws, "photos")
	if nb.ID == b.ID {
		t.Error("a recreated bucket reuses its id")
	}
}

func TestGetBucketLocationAnswersTheBexRegionName(t *testing.T) {
	p := newPGRig(t)
	k := p.newKey(objectstorage.KeyPermissionRead, 0)
	if _, err := p.Svc.CreateBucket(context.Background(), p.ws, "eu-data", objectstorage.Frankfurt); err != nil {
		t.Fatal(err)
	}
	rec, doc := p.call(k, "GET", "/eu-data?location", "")
	var got locationConstraintXML
	if rec.Code != 200 || xml.Unmarshal(rec.Body.Bytes(), &got) != nil || got.Value != "frankfurt" {
		t.Errorf("GetBucketLocation = %d %q %q, want frankfurt", rec.Code, doc.Code, rec.Body)
	}
}

func TestCORSRoundTripsThroughPutAndGetUnchangedAndAnInvalidSetIsRefused(t *testing.T) {
	p := newPGRig(t)
	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	p.call(k, "PUT", "/site", "")
	if rec, doc := p.call(k, "GET", "/site?cors", ""); rec.Code != 404 || doc.Code != "NoSuchCORSConfiguration" {
		t.Errorf("no rules = %d %q, want 404 NoSuchCORSConfiguration", rec.Code, doc.Code)
	}
	put := `<CORSConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
	<CORSRule><ID>web</ID><AllowedOrigin>https://app.example</AllowedOrigin><AllowedOrigin>https://*.example.com</AllowedOrigin>
	<AllowedMethod>GET</AllowedMethod><AllowedMethod>HEAD</AllowedMethod><AllowedHeader>range</AllowedHeader><AllowedHeader>if-none-match</AllowedHeader>
	<ExposeHeader>ETag</ExposeHeader><ExposeHeader>Content-Range</ExposeHeader><MaxAgeSeconds>600</MaxAgeSeconds></CORSRule>
	<CORSRule><AllowedOrigin>*</AllowedOrigin><AllowedMethod>GET</AllowedMethod></CORSRule></CORSConfiguration>`
	if rec, doc := p.call(k, "PUT", "/site?cors", put); rec.Code != 200 {
		t.Fatalf("PutBucketCors = %d %q %s", rec.Code, doc.Code, doc.Message)
	}
	rec, _ := p.call(k, "GET", "/site?cors", "")
	var got corsConfigurationXML
	if rec.Code != 200 || xml.Unmarshal(rec.Body.Bytes(), &got) != nil || len(got.Rules) != 2 {
		t.Fatalf("GetBucketCors = %d %s", rec.Code, rec.Body)
	}
	r := got.Rules[0]
	if r.ID != "web" || strings.Join(r.AllowedOrigins, ",") != "https://app.example,https://*.example.com" || strings.Join(r.AllowedMethods, ",") != "GET,HEAD" ||
		strings.Join(r.AllowedHeaders, ",") != "range,if-none-match" || strings.Join(r.ExposeHeaders, ",") != "ETag,Content-Range" || r.MaxAgeSeconds == nil || *r.MaxAgeSeconds != 600 {
		t.Errorf("rule 1 read back as %+v", r)
	}
	if got.Rules[1].AllowedOrigins[0] != "*" || got.Rules[1].MaxAgeSeconds != nil {
		t.Errorf("rule 2 read back as %+v", got.Rules[1])
	}
	for name, body := range map[string]string{
		"a method outside the five":   `<CORSConfiguration><CORSRule><AllowedOrigin>*</AllowedOrigin><AllowedMethod>PATCH</AllowedMethod></CORSRule></CORSConfiguration>`,
		"an origin that is no origin": `<CORSConfiguration><CORSRule><AllowedOrigin>example.com/x</AllowedOrigin><AllowedMethod>GET</AllowedMethod></CORSRule></CORSConfiguration>`,
		"a max age over a day":        `<CORSConfiguration><CORSRule><AllowedOrigin>*</AllowedOrigin><AllowedMethod>GET</AllowedMethod><MaxAgeSeconds>90000</MaxAgeSeconds></CORSRule></CORSConfiguration>`,
	} {
		if rec, doc := p.call(k, "PUT", "/site?cors", body); rec.Code != 400 || doc.Code != "InvalidRequest" {
			t.Errorf("%s = %d %q, want 400 InvalidRequest", name, rec.Code, doc.Code)
		}
	}
	for name, body := range map[string]string{"no rules": `<CORSConfiguration></CORSConfiguration>`, "not XML": `{"rules":[]}`, "wrong root": `<Other/>`} {
		if rec, doc := p.call(k, "PUT", "/site?cors", body); rec.Code != 400 || doc.Code != "MalformedXML" {
			t.Errorf("%s = %d %q, want 400 MalformedXML", name, rec.Code, doc.Code)
		}
	}
	// A refused set stored nothing: the first configuration is still there.
	if rec, _ := p.call(k, "GET", "/site?cors", ""); !strings.Contains(rec.Body.String(), "<ID>web</ID>") {
		t.Errorf("a refused PutBucketCors replaced the stored rules: %s", rec.Body)
	}
	if rec, _ := p.call(k, "DELETE", "/site?cors", ""); rec.Code != 204 {
		t.Errorf("DeleteBucketCors = %d", rec.Code)
	}
	if rec, doc := p.call(k, "GET", "/site?cors", ""); rec.Code != 404 || doc.Code != "NoSuchCORSConfiguration" {
		t.Errorf("after delete = %d %q", rec.Code, doc.Code)
	}
}

func TestALifecycleRuleRoundTripsAndEachLimitIsRefused(t *testing.T) {
	p := newPGRig(t)
	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	p.call(k, "PUT", "/logs", "")
	if rec, doc := p.call(k, "GET", "/logs?lifecycle", ""); rec.Code != 404 || doc.Code != "NoSuchLifecycleConfiguration" {
		t.Errorf("no rules = %d %q", rec.Code, doc.Code)
	}
	put := `<LifecycleConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
	<Rule><ID>tmp</ID><Status>Enabled</Status><Filter><Prefix>tmp/</Prefix></Filter><Expiration><Days>7</Days></Expiration></Rule>
	<Rule><ID>cutoff</ID><Status>Disabled</Status><Filter><Prefix></Prefix></Filter><Expiration><Date>2027-01-01T00:00:00.000Z</Date></Expiration></Rule>
	<Rule><ID>parts</ID><Status>Enabled</Status><Prefix>big/</Prefix><AbortIncompleteMultipartUpload><DaysAfterInitiation>2</DaysAfterInitiation></AbortIncompleteMultipartUpload></Rule>
	</LifecycleConfiguration>`
	if rec, doc := p.call(k, "PUT", "/logs?lifecycle", put); rec.Code != 200 {
		t.Fatalf("PutBucketLifecycleConfiguration = %d %q %s", rec.Code, doc.Code, doc.Message)
	}
	rec, _ := p.call(k, "GET", "/logs?lifecycle", "")
	var got lifecycleConfigurationXML
	if rec.Code != 200 || xml.Unmarshal(rec.Body.Bytes(), &got) != nil || len(got.Rules) != 3 {
		t.Fatalf("Get = %d %s", rec.Code, rec.Body)
	}
	r0, r1, r2 := got.Rules[0], got.Rules[1], got.Rules[2]
	if r0.ID != "tmp" || r0.Status != "Enabled" || *r0.Filter.Prefix != "tmp/" || *r0.Expiration.Days != 7 {
		t.Errorf("rule tmp read back as %+v", r0)
	}
	if r1.ID != "cutoff" || r1.Status != "Disabled" || r1.Expiration.Date != "2027-01-01T00:00:00.000Z" || *r1.Filter.Prefix != "" {
		t.Errorf("rule cutoff read back as %+v %+v", r1, r1.Expiration)
	}
	if r2.ID != "parts" || *r2.Filter.Prefix != "big/" || r2.Expiration != nil || *r2.AbortIncompleteMultipartUpload.DaysAfterInitiation != 2 {
		t.Errorf("rule parts read back as %+v", r2)
	}
	// What is stored is the shape the sweep runs: Days is a maxAge in seconds.
	b, _ := p.Svc.GetBucket(context.Background(), p.ws, "logs")
	rules, err := objectstorage.ParseLifecycle(b.Lifecycle)
	if err != nil || len(rules) != 3 || *rules[0].DeleteObjectsTransition.Condition.MaxAge != 7*86400 || !rules[0].Enabled || rules[1].Enabled {
		t.Errorf("stored rules = %+v, %v", rules, err)
	}
	refuse := func(name, body string, status int, code string) {
		t.Helper()
		wrap := `<LifecycleConfiguration>` + body + `</LifecycleConfiguration>`
		if rec, doc := p.call(k, "PUT", "/logs?lifecycle", wrap); rec.Code != status || doc.Code != code {
			t.Errorf("%s = %d %q, want %d %s", name, rec.Code, doc.Code, status, code)
		}
	}
	rule := func(inner string) string {
		return `<Rule><ID>x</ID><Status>Enabled</Status><Filter><Prefix></Prefix></Filter>` + inner + `</Rule>`
	}
	refuse("Days over the cap", rule(`<Expiration><Days>36501</Days></Expiration>`), 400, "InvalidArgument")
	refuse("Days 0", rule(`<Expiration><Days>0</Days></Expiration>`), 400, "InvalidArgument")
	refuse("a Date that is not midnight", rule(`<Expiration><Date>2027-01-01T12:00:00.000Z</Date></Expiration>`), 400, "InvalidArgument")
	refuse("Days and Date", rule(`<Expiration><Days>1</Days><Date>2027-01-01T00:00:00Z</Date></Expiration>`), 400, "MalformedXML")
	refuse("a Transition", rule(`<Transition><Days>30</Days><StorageClass>GLACIER</StorageClass></Transition>`), 501, "NotImplemented")
	refuse("a noncurrent expiration", rule(`<NoncurrentVersionExpiration><NoncurrentDays>1</NoncurrentDays></NoncurrentVersionExpiration>`), 501, "NotImplemented")
	refuse("a tag filter", `<Rule><ID>x</ID><Status>Enabled</Status><Filter><Tag><Key>a</Key><Value>b</Value></Tag></Filter><Expiration><Days>1</Days></Expiration></Rule>`, 501, "NotImplemented")
	refuse("an expired-delete-marker rule", rule(`<Expiration><ExpiredObjectDeleteMarker>true</ExpiredObjectDeleteMarker></Expiration>`), 501, "NotImplemented")
	refuse("a rule with no action", rule(``), 400, "InvalidRequest")
	refuse("a bad Status", `<Rule><ID>x</ID><Status>Maybe</Status><Filter><Prefix></Prefix></Filter><Expiration><Days>1</Days></Expiration></Rule>`, 400, "MalformedXML")
	refuse("a Filter and a Prefix", `<Rule><ID>x</ID><Status>Enabled</Status><Prefix>a</Prefix><Filter><Prefix>b</Prefix></Filter><Expiration><Days>1</Days></Expiration></Rule>`, 400, "MalformedXML")
	refuse("a duplicate ID", rule(`<Expiration><Days>1</Days></Expiration>`)+rule(`<Expiration><Days>2</Days></Expiration>`), 400, "InvalidRequest")
	refuse("an abort of 0 days", rule(`<AbortIncompleteMultipartUpload><DaysAfterInitiation>0</DaysAfterInitiation></AbortIncompleteMultipartUpload>`), 400, "InvalidArgument")
	if rec, _ := p.call(k, "GET", "/logs?lifecycle", ""); !strings.Contains(rec.Body.String(), "<ID>tmp</ID>") {
		t.Errorf("a refused configuration replaced the stored one: %s", rec.Body)
	}
	// An ID is kept when sent and assigned when not.
	if rec, doc := p.call(k, "PUT", "/logs?lifecycle", `<LifecycleConfiguration><Rule><Status>Enabled</Status><Filter><Prefix>a/</Prefix></Filter><Expiration><Days>3</Days></Expiration></Rule></LifecycleConfiguration>`); rec.Code != 200 {
		t.Errorf("a rule with no ID = %d %q", rec.Code, doc.Code)
	}
	if rec, _ := p.call(k, "GET", "/logs?lifecycle", ""); !strings.Contains(rec.Body.String(), "<ID>rule-1</ID>") {
		t.Errorf("the assigned ID is not read back: %s", rec.Body)
	}
	// A rule stored in seconds reads as the whole days that cover it.
	age := int64(3600)
	stored := []objectstorage.LifecycleRule{{ID: "hour", Enabled: true, DeleteObjectsTransition: &objectstorage.LifecycleDelete{Condition: objectstorage.LifecycleDeleteCondition{Type: objectstorage.LifecycleAge, MaxAge: &age}}}}
	if err := p.Svc.SetLifecycle(context.Background(), p.ws, b, stored); err != nil {
		t.Fatal(err)
	}
	if rec, _ := p.call(k, "GET", "/logs?lifecycle", ""); !strings.Contains(rec.Body.String(), "<Days>1</Days>") {
		t.Errorf("a one-hour rule reads as %s, want 1 day", rec.Body)
	}
	if rec, _ := p.call(k, "DELETE", "/logs?lifecycle", ""); rec.Code != 204 {
		t.Errorf("DeleteBucketLifecycle = %d", rec.Code)
	}
}

func TestThePreflightIsRoutedByTheKeyIdOfAPresignedURLAndEveryRefusalLooksTheSame(t *testing.T) {
	p := newPGRig(t)
	k := p.newKey(objectstorage.KeyPermissionRead, 0)
	if _, err := p.Svc.CreateBucket(context.Background(), p.ws, "site", objectstorage.Oregon); err != nil {
		t.Fatal(err)
	}
	b, _ := p.Svc.GetBucket(context.Background(), p.ws, "site")
	max := int64(600)
	if err := p.Svc.SetCORS(context.Background(), p.ws, b, []objectstorage.CORSRule{{
		Allowed: objectstorage.CORSAllowed{Origins: []string{"https://app.example"}, Methods: []string{"GET"}, Headers: []string{"range"}}, ExposeHeaders: []string{"ETag"}, MaxAgeSeconds: &max,
	}}); err != nil {
		t.Fatal(err)
	}
	preflight := func(target, origin, method, headers string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodOptions, "https://objects.onbex.co"+target, nil)
		for k, v := range map[string]string{"Origin": origin, "Access-Control-Request-Method": method, "Access-Control-Request-Headers": headers} {
			if v != "" {
				r.Header.Set(k, v)
			}
		}
		rec, _ := p.do(r)
		return rec
	}
	presigned := p.presign(k, "GET", "/site/a.txt", time.Hour).URL.RequestURI()
	ok := preflight(presigned, "https://app.example", "GET", "range")
	h := ok.Header()
	if ok.Code != 204 || h.Get("Access-Control-Allow-Origin") != "https://app.example" || h.Get("Access-Control-Allow-Methods") != "GET" ||
		h.Get("Access-Control-Allow-Headers") != "range" || h.Get("Access-Control-Max-Age") != "600" || h.Get("Access-Control-Allow-Credentials") != "" {
		t.Fatalf("an allowed preflight = %d %v", ok.Code, h)
	}
	revoked := p.newKey(objectstorage.KeyPermissionRead, 0)
	if err := p.keys.Revoke(context.Background(), p.ws, revoked.id); err != nil {
		t.Fatal(err)
	}
	refusals := map[string]*httptest.ResponseRecorder{
		"a refused origin":   preflight(presigned, "https://evil.example", "GET", "range"),
		"a refused method":   preflight(presigned, "https://app.example", "PUT", ""),
		"a refused header":   preflight(presigned, "https://app.example", "GET", "x-secret"),
		"a missing bucket":   preflight(p.presign(k, "GET", "/nope/a.txt", time.Hour).URL.RequestURI(), "https://app.example", "GET", ""),
		"an unknown key id":  preflight(p.presign(testKey{"s3k-" + strings.Repeat("0", 20), "x"}, "GET", "/site/a.txt", time.Hour).URL.RequestURI(), "https://app.example", "GET", ""),
		"a revoked key id":   preflight(p.presign(revoked, "GET", "/site/a.txt", time.Hour).URL.RequestURI(), "https://app.example", "GET", ""),
		"no presigned query": preflight("/site/a.txt", "https://app.example", "GET", ""),
		"no origin":          preflight(presigned, "", "GET", ""),
		"no request method":  preflight(presigned, "https://app.example", "", ""),
	}
	var first string
	for name, rec := range refusals {
		for header := range rec.Header() {
			if strings.HasPrefix(header, "Access-Control-") {
				t.Errorf("%s: carries %s", name, header)
			}
		}
		sig := rec.Body.String()
		sig = sig[:strings.Index(sig, "<Resource>")] // the path is the caller's own
		if rec.Code != 403 || (first != "" && sig != first) {
			t.Errorf("%s = %d %q; every refusal must be the same 403 (first %q)", name, rec.Code, sig, first)
		}
		first = sig
	}
}
