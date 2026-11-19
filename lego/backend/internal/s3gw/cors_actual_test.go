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
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
)

// A browser reads an object with a presigned URL, so the actual response of a bucket with CORS
// rules carries the matching rule's headers (ADR112 D3); the preflight is tested beside the
// bucket calls.
func TestAPresignedReadFromABrowserOriginCarriesTheMatchingRulesHeaders(t *testing.T) {
	p := newPGRig(t)
	k := p.newKey(objectstorage.KeyPermissionReadWrite, 0)
	p.bucket("pinned", objectstorage.Oregon)
	p.bucket("wild", objectstorage.Oregon)
	rules := func(origin string) string {
		return `<CORSConfiguration><CORSRule><AllowedOrigin>` + origin + `</AllowedOrigin><AllowedMethod>GET</AllowedMethod>` +
			`<ExposeHeader>ETag</ExposeHeader><ExposeHeader>x-amz-meta-mtime</ExposeHeader></CORSRule></CORSConfiguration>`
	}
	for bucket, origin := range map[string]string{"pinned": "https://app.example", "wild": "*"} {
		body := rules(origin)
		if rec, doc := p.do(p.signed(k, "PUT", "/"+bucket+"?cors", strings.NewReader(body), func(r *http.Request) { r.ContentLength = int64(len(body)) })); rec.Code != 200 {
			t.Fatalf("PutBucketCors %s = %d %q", bucket, rec.Code, doc.Code)
		}
		p.put(k, "/"+bucket+"/o", 10)
	}
	get := func(bucket, origin string) http.Header {
		r := p.presign(k, "GET", "/"+bucket+"/o", time.Minute)
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		rec, doc := p.do(r)
		if rec.Code != 200 {
			t.Fatalf("a presigned GET of %s from %q = %d %q", bucket, origin, rec.Code, doc.Code)
		}
		return rec.Header()
	}
	h := get("pinned", "https://app.example")
	if h.Get("Access-Control-Allow-Origin") != "https://app.example" || h.Get("Access-Control-Expose-Headers") != "ETag, x-amz-meta-mtime" || h.Get("Vary") != "Origin" {
		t.Errorf("an allowed origin got %v", h)
	}
	if h.Get("Access-Control-Allow-Credentials") != "" {
		t.Error("credentials are never allowed")
	}
	h = get("pinned", "https://evil.example")
	if h.Get("Access-Control-Allow-Origin") != "" || h.Get("Vary") != "Origin" {
		t.Errorf("an origin no rule allows got %v, want no allow-origin and Vary: Origin (the answer depends on the origin)", h)
	}
	h = get("pinned", "")
	if h.Get("Access-Control-Allow-Origin") != "" || h.Get("Vary") != "" {
		t.Errorf("a request with no Origin got %v", h)
	}
	if h := get("wild", "https://anywhere.example"); h.Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("a wildcard rule answered %q, want *", h.Get("Access-Control-Allow-Origin"))
	}
	// A bucket with no rules never adds a header, whatever the origin.
	p.bucket("plain", objectstorage.Oregon)
	p.put(k, "/plain/o", 10)
	if h := get("plain", "https://app.example"); h.Get("Access-Control-Allow-Origin") != "" || h.Get("Vary") != "" {
		t.Errorf("a bucket with no CORS rules answered %v", h)
	}
}
