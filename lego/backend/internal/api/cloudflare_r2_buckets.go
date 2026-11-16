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
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const (
	// cloudflareGraphQLPath is where `wrangler r2 bucket info` asks for a
	// bucket's size. It sits beside the accounts tree, not under it.
	cloudflareGraphQLPath = cloudflareMount + "/graphql"
	// cfStorageMetricsOperation is the one GraphQL operation wrangler sends.
	cfStorageMetricsOperation = "getR2StorageMetrics"

	cfStorageClass = "Standard"
	cfJurisdiction = "default"
	// cfCreateBodyLimit bounds a bucket create request: a name and two hints.
	cfCreateBodyLimit = 4 << 10
	// cfMetricsBodyLimit bounds the GraphQL request.
	cfMetricsBodyLimit = 16 << 10
	// cfMaxPerPage is the most buckets a list page holds (Cloudflare documents
	// 1 to 1000), and more than a workspace may create.
	cfMaxPerPage = 1000
)

// cfBucket is a bucket as wrangler reads it.
type cfBucket struct {
	Name         string `json:"name"`
	CreationDate string `json:"creation_date"`
	Location     string `json:"location"`
	StorageClass string `json:"storage_class"`
	Jurisdiction string `json:"jurisdiction"`
}

func toCFBucket(b store.ObjectBucket) cfBucket {
	// Cloudflare's published schema spells locations in lower case (wnam, weur).
	location := strings.ToLower(b.LocationHint)
	if location == "" {
		location = objectstorage.Region(b.Region).DefaultLocationHint()
	}
	return cfBucket{
		Name: b.Name, CreationDate: b.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
		Location: location, StorageClass: cfStorageClass, Jurisdiction: cfJurisdiction,
	}
}

// bucket is the bucket the route's path names.
func (c *cfCall) bucket() (store.ObjectBucket, error) {
	return c.c.svc.ResolveBucket(c.r.Context(), c.workspaceID, c.r.PathValue("bucket"))
}

type cfCreateBucketRequest struct {
	Name         string `json:"name"`
	LocationHint string `json:"locationHint"`
	StorageClass string `json:"storageClass"`
}

func (c *cloudflareR2) registerBuckets(mux *http.ServeMux) {
	c.route(mux, "GET "+cloudflareAccountR2+"/buckets", objectstorage.AuditVerbListBuckets, core.RelCanView, func(call *cfCall) error {
		buckets, err := c.svc.ListBuckets(call.r.Context(), call.workspaceID)
		if err != nil {
			return err
		}
		page, err := pageBuckets(buckets, call.r.URL.Query())
		if err != nil {
			return err
		}
		writeCFJSON(call.w, http.StatusOK, cfEnvelope{Success: true, Result: map[string]any{"buckets": page.buckets}, ResultInfo: page.info})
		return nil
	})

	c.route(mux, "POST "+cloudflareAccountR2+"/buckets", objectstorage.AuditVerbCreateBucket, core.RelCanCreate, func(call *cfCall) error {
		var req cfCreateBucketRequest
		if err := readCFBody(call.r, cfCreateBodyLimit, &req); err != nil {
			return err
		}
		switch req.StorageClass {
		case "", cfStorageClass:
		case "InfrequentAccess":
			return cfNotEntitled("the InfrequentAccess storage class is not offered by bex, which has one class")
		default:
			return cfFail(http.StatusBadRequest, cfCodeInvalidRequest, "storageClass must be %s", cfStorageClass)
		}
		// bex ignores the jurisdiction when it looks a name up, where
		// Cloudflare scopes names per jurisdiction; it only places the bucket.
		region, err := objectstorage.RegionFromHint(req.LocationHint, call.r.Header.Get("cf-r2-jurisdiction"))
		if err != nil {
			return err
		}
		b, err := c.svc.CreateBucket(call.r.Context(), call.workspaceID, req.Name, region, req.LocationHint)
		if err != nil {
			return err
		}
		writeCFResult(call.w, toCFBucket(b))
		return nil
	})

	c.route(mux, "GET "+cloudflareAccountR2+"/buckets/{bucket}", objectstorage.AuditVerbGetBucket, core.RelCanView, func(call *cfCall) error {
		b, err := call.bucket()
		if err != nil {
			return err
		}
		writeCFResult(call.w, toCFBucket(b))
		return nil
	})

	c.route(mux, "DELETE "+cloudflareAccountR2+"/buckets/{bucket}", objectstorage.AuditVerbDeleteBucket, core.RelCanCreate, func(call *cfCall) error {
		if err := c.svc.DeleteBucket(call.r.Context(), call.workspaceID, call.r.PathValue("bucket")); err != nil {
			return err
		}
		writeCFResult(call.w, nil)
		return nil
	})

	// A storage class change has nothing to change: bex has one class.
	c.route(mux, "PATCH "+cloudflareAccountR2+"/buckets/{bucket}", objectstorage.AuditVerbUpdateBucket, core.RelCanCreate, func(*cfCall) error {
		return cfNotEntitled("changing a bucket's storage class is not offered by bex, which has one class")
	})

	// bucket info's size. It is account-scoped by the accountTag variable, not
	// by the path, so it authorizes inside its own handler.
	mux.HandleFunc("POST "+cloudflareGraphQLPath, c.serveStorageMetrics)
}

// cfMetricsRequest is the part of wrangler's GraphQL request bex reads.
type cfMetricsRequest struct {
	OperationName string `json:"operationName"`
	Variables     struct {
		AccountTag string `json:"accountTag"`
		Filter     struct {
			BucketName string `json:"bucketName"`
		} `json:"filter"`
	} `json:"variables"`
}

// serveStorageMetrics answers getR2StorageMetrics from the last sweep's usage
// of the bucket. A bucket the sweep has not seen answers an empty group list,
// which wrangler prints as zeros.
func (c *cloudflareR2) serveStorageMetrics(w http.ResponseWriter, r *http.Request) {
	if !c.svc.Enabled() {
		writeCFError(w, cloudflareError(objectstorage.ErrNotConfigured))
		return
	}
	var req cfMetricsRequest
	if err := readCFBody(r, cfMetricsBodyLimit, &req); err != nil {
		writeCFError(w, cloudflareError(err))
		return
	}
	if req.OperationName != cfStorageMetricsOperation {
		writeCFError(w, cfFail(http.StatusBadRequest, cfCodeInvalidRequest, "bex answers only the %s GraphQL operation", cfStorageMetricsOperation))
		return
	}
	workspaceID, err := c.base.AuthorizeWorkspaceAs(r.Context(), objectstorage.AuditVerbGetBucketUsage, core.RelCanView, req.Variables.AccountTag)
	if err != nil {
		writeCFError(w, cloudflareError(err))
		return
	}
	// wrangler prefixes a jurisdiction to the name; bex ignores jurisdictions.
	name := req.Variables.Filter.BucketName
	for _, prefix := range []string{"eu_", "fedramp_"} {
		name = strings.TrimPrefix(name, prefix)
	}
	groups := []any{}
	b, err := c.svc.ResolveBucket(r.Context(), workspaceID, name)
	if err != nil && !errors.Is(err, objectstorage.ErrBucketNotFound) {
		writeCFError(w, cloudflareError(err))
		return
	}
	if err == nil {
		usage, ok, err := c.svc.BucketUsage(r.Context(), workspaceID, b.ID)
		if err != nil {
			writeCFError(w, cloudflareError(err))
			return
		}
		if ok {
			groups = append(groups, map[string]any{
				"max":        map[string]any{"objectCount": usage.Objects, "payloadSize": usage.Bytes, "metadataSize": 0},
				"dimensions": map[string]any{"datetime": usage.ObservedAt.UTC().Format("2006-01-02T15:04:05Z")},
			})
		}
	}
	core.WriteJSON(w, http.StatusOK, map[string]any{
		"data":   map[string]any{"viewer": map[string]any{"accounts": []any{map[string]any{"r2StorageAdaptiveGroups": groups}}}},
		"errors": nil,
	})
}

// cfBucketPage is one page of a bucket listing.
type cfBucketPage struct {
	buckets []cfBucket
	info    *cfResultInfo
}

// cfResultInfo is the envelope's pagination block.
type cfResultInfo struct {
	TotalCount int    `json:"total_count"`
	Cursor     string `json:"cursor,omitempty"`
	PerPage    int    `json:"per_page"`
}

// pageBuckets applies Cloudflare's List Buckets query to the workspace's
// buckets: name_contains, order=name, direction, start_after, per_page (1 to
// 1000) and cursor. wrangler sends none of them and needs every bucket in one
// answer, so per_page defaults to the most a page holds, which is also more than
// a workspace may create (Cloudflare's own default is not published). Buckets
// are ordered by name, as Cloudflare documents.
func pageBuckets(rows []store.ObjectBucket, q url.Values) (cfBucketPage, error) {
	invalid := func(format string, args ...any) error {
		return cfFail(http.StatusBadRequest, cfCodeInvalidRequest, format, args...)
	}
	perPage := cfMaxPerPage
	if raw := q.Get("per_page"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > cfMaxPerPage {
			return cfBucketPage{}, invalid("per_page must be a number from 1 to %d", cfMaxPerPage)
		}
		perPage = n
	}
	if order := q.Get("order"); order != "" && order != "name" {
		return cfBucketPage{}, invalid("order must be name")
	}
	descending := false
	switch q.Get("direction") {
	case "", "asc":
	case "desc":
		descending = true
	default:
		return cfBucketPage{}, invalid("direction must be asc or desc")
	}
	startAfter := q.Get("start_after")
	if cursor := q.Get("cursor"); cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil {
			return cfBucketPage{}, invalid("cursor is not one a list returned")
		}
		startAfter = string(raw)
	}

	slices.SortFunc(rows, func(a, b store.ObjectBucket) int { return strings.Compare(a.Name, b.Name) })
	if descending {
		slices.Reverse(rows)
	}
	var matched []store.ObjectBucket
	for _, b := range rows {
		after := startAfter == "" || (!descending && b.Name > startAfter) || (descending && b.Name < startAfter)
		if after && strings.Contains(b.Name, q.Get("name_contains")) {
			matched = append(matched, b)
		}
	}
	info := &cfResultInfo{TotalCount: len(rows), PerPage: perPage}
	if len(matched) > perPage {
		matched = matched[:perPage]
		info.Cursor = base64.RawURLEncoding.EncodeToString([]byte(matched[len(matched)-1].Name))
	}
	out := make([]cfBucket, 0, len(matched))
	for _, b := range matched {
		out = append(out, toCFBucket(b))
	}
	return cfBucketPage{buckets: out, info: info}, nil
}
