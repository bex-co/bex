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
	"fmt"
	"net/http"
	"testing"
	"time"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

func TestCloudflareBucketLifecycleThroughTheRoutes(t *testing.T) {
	h := newCFHarness(t)

	created := h.do(t, http.MethodPost, h.url("/buckets"), []byte(`{"name":"photos"}`), "Content-Type", "text/plain")
	wantOK(t, created)
	b := resultOf[cfBucket](t, created)
	if b.Name != "photos" || b.Location != "wnam" || b.StorageClass != "Standard" || b.Jurisdiction != "default" || b.CreationDate == "" {
		t.Fatalf("created bucket = %+v", b)
	}
	if _, err := time.Parse("2006-01-02T15:04:05.000Z", b.CreationDate); err != nil {
		t.Errorf("creation_date %q is not Cloudflare's format: %v", b.CreationDate, err)
	}

	wantCF(t, h.do(t, http.MethodPost, h.url("/buckets"), []byte(`{"name":"photos"}`)), http.StatusConflict, 10073)

	list := resultOf[struct{ Buckets []cfBucket }](t, h.do(t, http.MethodGet, h.url("/buckets"), nil))
	if len(list.Buckets) != 1 || list.Buckets[0].Name != "photos" {
		t.Fatalf("list = %+v", list)
	}
	info := resultOf[cfBucket](t, h.do(t, http.MethodGet, h.url("/buckets/photos"), nil))
	if info != b {
		t.Fatalf("info %+v differs from the created %+v", info, b)
	}

	// A bucket that holds an object cannot be deleted; an empty one can, and its
	// name is free again.
	wantOK(t, h.do(t, http.MethodPut, h.url("/buckets/photos/objects/cat.jpg"), []byte("meow")))
	wantCF(t, h.do(t, http.MethodDelete, h.url("/buckets/photos"), nil), http.StatusConflict, 10008)
	wantOK(t, h.do(t, http.MethodDelete, h.url("/buckets/photos/objects/cat.jpg"), nil))
	wantOK(t, h.do(t, http.MethodDelete, h.url("/buckets/photos"), nil))
	wantCF(t, h.do(t, http.MethodGet, h.url("/buckets/photos"), nil), http.StatusNotFound, 10006)
	h.createBucket(t, "photos")
}

// TestCloudflareLocationHintsPlaceTheBucket: the hint (and the jurisdiction
// header) decides the Wasabi region, and bucket info reports the hint the
// bucket was created with (ADR112 D1).
func TestCloudflareLocationHintsPlaceTheBucket(t *testing.T) {
	h := newCFHarness(t)
	for _, c := range []struct {
		name, body, jurisdiction string
		region, location         string
		status                   int
	}{
		{"def", `{"name":"def"}`, "", "oregon", "wnam", 200},
		{"west", `{"name":"west","locationHint":"wnam"}`, "", "oregon", "wnam", 200},
		{"euro", `{"name":"euro","locationHint":"weur"}`, "", "frankfurt", "weur", 200},
		{"eujur", `{"name":"eujur"}`, "eu", "frankfurt", "weur", 200},
		{"east", `{"name":"east","locationHint":"enam"}`, "", "", "", 403},
		{"fed", `{"name":"fed"}`, "fedramp", "", "", 403},
	} {
		var headers []string
		if c.jurisdiction != "" {
			headers = []string{"cf-r2-jurisdiction", c.jurisdiction}
		}
		r := h.do(t, http.MethodPost, h.url("/buckets"), []byte(c.body), headers...)
		if c.status != 200 {
			if r.Code != http.StatusBadRequest || r.errCode() != 10042 {
				t.Errorf("%s: %d %s, want 400 with 10042 for a region bex does not offer", c.name, r.Code, r.Raw)
			}
			continue
		}
		wantOK(t, r)
		row, err := h.repo.GetObjectBucket(t.Context(), h.ws, c.name)
		if err != nil || row.Region != c.region {
			t.Errorf("%s: row = %+v, %v; want region %s", c.name, row, err, c.region)
		}
		if got := resultOf[cfBucket](t, h.do(t, http.MethodGet, h.url("/buckets/"+c.name), nil)).Location; got != c.location {
			t.Errorf("%s: info reports location %q, want %q", c.name, got, c.location)
		}
	}
	// A hint is reported as sent, even when bex's region is exact.
	h.do(t, http.MethodPost, h.url("/buckets"), []byte(`{"name":"hinted","locationHint":"weur"}`))
	if row, _ := h.repo.GetObjectBucket(t.Context(), h.ws, "hinted"); row.LocationHint != "weur" {
		t.Errorf("the hint was stored as %q, want weur", row.LocationHint)
	}
}

// TestCloudflareBucketInfoShowsTheLastSweep: wrangler's bucket info POSTs a
// GraphQL query for the size; bex answers it from the sweep's usage row.
func TestCloudflareBucketInfoShowsTheLastSweep(t *testing.T) {
	h := newCFHarness(t)
	h.createBucket(t, "photos")
	row, _ := h.repo.GetObjectBucket(t.Context(), h.ws, "photos")
	query := func(name string) *cfReply {
		return h.do(t, http.MethodPost, cloudflareGraphQLPath, fmt.Appendf(nil,
			`{"query":"query getR2StorageMetrics($accountTag: String) { viewer { accounts { r2StorageAdaptiveGroups { max { objectCount payloadSize metadataSize } } } } }","operationName":"getR2StorageMetrics","variables":{"accountTag":%q,"filter":{"bucketName":%q}}}`,
			h.ws, name), "Content-Type", "application/json")
	}
	type metrics struct {
		Data struct {
			Viewer struct {
				Accounts []struct {
					Groups []struct {
						Max struct{ ObjectCount, PayloadSize, MetadataSize int64 } `json:"max"`
					} `json:"r2StorageAdaptiveGroups"`
				} `json:"accounts"`
			} `json:"viewer"`
		} `json:"data"`
		Errors any `json:"errors"`
	}

	// Before any sweep: an empty group list, which wrangler prints as zeros.
	before := query("photos")
	var m metrics
	if err := jsonUnmarshal(before.Raw, &m); err != nil || before.Code != 200 || len(m.Data.Viewer.Accounts) != 1 || len(m.Data.Viewer.Accounts[0].Groups) != 0 {
		t.Fatalf("before a sweep = %d %s (%v)", before.Code, before.Raw, err)
	}

	h.repo.usage = append(h.repo.usage, store.ObjectStorageUsage{
		BucketID: row.ID, WorkspaceID: h.ws, Region: "oregon", Bytes: 1234, Objects: 7, ObservedAt: time.Now(),
	})
	after := query("photos")
	m = metrics{}
	if err := jsonUnmarshal(after.Raw, &m); err != nil || len(m.Data.Viewer.Accounts[0].Groups) != 1 {
		t.Fatalf("after a sweep = %d %s (%v)", after.Code, after.Raw, err)
	}
	if got := m.Data.Viewer.Accounts[0].Groups[0].Max; got.ObjectCount != 7 || got.PayloadSize != 1234 {
		t.Fatalf("metrics = %+v, want the sweep's 7 objects and 1234 bytes", got)
	}
	// wrangler prefixes a jurisdiction to the name; bex ignores jurisdictions.
	if r := query("eu_photos"); !contains(r.Raw, `"objectCount":7`) {
		t.Errorf("a jurisdiction-prefixed name = %s", r.Raw)
	}
	// Another workspace's bucket of the same name has no usage.
	other := ids.New(ids.Workspace)
	r := h.do(t, http.MethodPost, cloudflareGraphQLPath, fmt.Appendf(nil,
		`{"operationName":"getR2StorageMetrics","variables":{"accountTag":%q,"filter":{"bucketName":"photos"}}}`, other))
	if contains(r.Raw, `"objectCount"`) {
		t.Errorf("another workspace read this bucket's size: %s", r.Raw)
	}
	// Any other operation is refused.
	wantCF(t, h.do(t, http.MethodPost, cloudflareGraphQLPath, []byte(`{"operationName":"somethingElse"}`)), http.StatusBadRequest, 7400)
}

// TestCloudflareBucketListFollowsCloudflaresQuery: a list is ordered by name and
// takes Cloudflare's published query (name_contains, order, direction,
// start_after, per_page and cursor), and carries result_info. wrangler sends
// none of it and needs every bucket at once.
func TestCloudflareBucketListFollowsCloudflaresQuery(t *testing.T) {
	h := newCFHarness(t)
	for _, name := range []string{"delta", "alpha", "charlie", "bravo", "alpha-two"} {
		h.createBucket(t, name)
	}
	names := func(r *cfReply) []string {
		var out []string
		for _, b := range resultOf[struct{ Buckets []cfBucket }](t, r).Buckets {
			out = append(out, b.Name)
		}
		return out
	}
	list := func(query string) *cfReply { return h.do(t, http.MethodGet, h.url("/buckets"+query), nil) }

	all := list("")
	wantOK(t, all)
	if got := fmt.Sprint(names(all)); got != "[alpha alpha-two bravo charlie delta]" {
		t.Fatalf("an unqueried list = %s, want every bucket ordered by name", got)
	}
	var info cfResultInfo
	var env struct {
		ResultInfo cfResultInfo `json:"result_info"`
	}
	_ = jsonUnmarshal(all.Raw, &env)
	info = env.ResultInfo
	if info.TotalCount != 5 || info.PerPage != 1000 || info.Cursor != "" {
		t.Fatalf("result_info = %+v", info)
	}
	if got := fmt.Sprint(names(list("?direction=desc&order=name"))); got != "[delta charlie bravo alpha-two alpha]" {
		t.Errorf("descending = %s", got)
	}
	if got := fmt.Sprint(names(list("?name_contains=alpha"))); got != "[alpha alpha-two]" {
		t.Errorf("name_contains = %s", got)
	}
	if got := fmt.Sprint(names(list("?start_after=bravo"))); got != "[charlie delta]" {
		t.Errorf("start_after = %s", got)
	}

	// Pages: per_page cuts the list and hands back a cursor; following it
	// reaches every bucket once and the last page has no cursor.
	var seen []string
	query := "?per_page=2"
	for range 5 {
		page := list(query)
		wantOK(t, page)
		seen = append(seen, names(page)...)
		env.ResultInfo = cfResultInfo{}
		_ = jsonUnmarshal(page.Raw, &env)
		if env.ResultInfo.TotalCount != 5 || env.ResultInfo.PerPage != 2 {
			t.Fatalf("result_info = %+v, want the total of 5 on every page", env.ResultInfo)
		}
		if env.ResultInfo.Cursor == "" {
			break
		}
		query = "?per_page=2&cursor=" + env.ResultInfo.Cursor
	}
	if got := fmt.Sprint(seen); got != "[alpha alpha-two bravo charlie delta]" {
		t.Fatalf("following the cursor read %s", got)
	}

	for _, bad := range []string{"?per_page=0", "?per_page=1001", "?per_page=x", "?order=created", "?direction=up", "?cursor=***"} {
		wantCF(t, list(bad), http.StatusBadRequest, 7400)
	}
}
