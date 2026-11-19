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
	"net/url"
	"sort"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// request is one authenticated S3 request on its way to a handler.
type request struct {
	g   *Gateway
	w   http.ResponseWriter
	r   *http.Request
	ctx context.Context
	key store.ObjectAccessKey
	// workspaceID is the key's own workspace: the only one the request can act in.
	workspaceID string
	// bucket and objectKey are the path-style names, "" when the level has none.
	bucket, objectKey string
	q                 url.Values
	route             *route
}

// route is one S3 operation bex serves: where it lives, what it asks of the key's
// creator and which audit verb it records.
type route struct {
	// op is the operation's S3 name, the label of the metrics.
	op string
	// level is where the operation lives: the service (ListBuckets), a bucket or an
	// object.
	level level
	// method and sub select it: sub is the subresource query parameter that makes
	// the operation (`?uploads`), "" for the plain one.
	method, sub string
	// relation is what the key's creator must hold on the workspace, and verb the
	// audit verb that records the decision (ADR112 D6).
	relation, verb string
	// params are the query parameters the operation takes beyond the ones every
	// operation takes. Any other parameter answers 501: it would change what the
	// request means and bex would silently ignore it.
	params []string
	// write marks an operation that stores something, which the unimplemented-header
	// refusals of ADR112 D3 apply to.
	write bool
}

type level int

const (
	levelService level = iota
	levelBucket
	levelObject
)

// The audit verbs of the S3 surface are the object store's own: a put through S3
// and through Render's routes is objectstorage.PutObject (ADR112 D6).
var routes = []route{
	{op: "ListBuckets", level: levelService, method: http.MethodGet, relation: core.RelCanView, verb: objectstorage.AuditVerbListBuckets},

	{op: "CreateBucket", level: levelBucket, method: http.MethodPut, relation: core.RelCanCreate, verb: objectstorage.AuditVerbCreateBucket, write: true},
	{op: "HeadBucket", level: levelBucket, method: http.MethodHead, relation: core.RelCanView, verb: objectstorage.AuditVerbGetBucket},
	{op: "DeleteBucket", level: levelBucket, method: http.MethodDelete, relation: core.RelCanCreate, verb: objectstorage.AuditVerbDeleteBucket},
	{op: "GetBucketLocation", level: levelBucket, method: http.MethodGet, sub: "location", relation: core.RelCanView, verb: objectstorage.AuditVerbGetBucket},
	{op: "GetBucketCors", level: levelBucket, method: http.MethodGet, sub: "cors", relation: core.RelCanView, verb: objectstorage.AuditVerbGetBucketCORS},
	{op: "PutBucketCors", level: levelBucket, method: http.MethodPut, sub: "cors", relation: core.RelCanCreate, verb: objectstorage.AuditVerbSetBucketCORS, write: true},
	{op: "DeleteBucketCors", level: levelBucket, method: http.MethodDelete, sub: "cors", relation: core.RelCanCreate, verb: objectstorage.AuditVerbDeleteBucketCORS},
	{op: "GetBucketLifecycleConfiguration", level: levelBucket, method: http.MethodGet, sub: "lifecycle", relation: core.RelCanView, verb: objectstorage.AuditVerbGetBucketLifecycle},
	{op: "PutBucketLifecycleConfiguration", level: levelBucket, method: http.MethodPut, sub: "lifecycle", relation: core.RelCanCreate, verb: objectstorage.AuditVerbSetBucketLifecycle, write: true},
	{op: "DeleteBucketLifecycle", level: levelBucket, method: http.MethodDelete, sub: "lifecycle", relation: core.RelCanCreate, verb: objectstorage.AuditVerbSetBucketLifecycle},
	{op: "ListObjects", level: levelBucket, method: http.MethodGet, relation: core.RelCanView, verb: objectstorage.AuditVerbListObjects,
		params: []string{"prefix", "delimiter", "max-keys", "marker", "encoding-type"}},
	{op: "ListObjectsV2", level: levelBucket, method: http.MethodGet, sub: "list-type", relation: core.RelCanView, verb: objectstorage.AuditVerbListObjects,
		params: []string{"prefix", "delimiter", "max-keys", "continuation-token", "start-after", "encoding-type", "fetch-owner"}},
	{op: "ListMultipartUploads", level: levelBucket, method: http.MethodGet, sub: "uploads", relation: core.RelCanView, verb: objectstorage.AuditVerbListMultipartUploads,
		params: []string{"prefix", "delimiter", "max-uploads", "key-marker", "upload-id-marker", "encoding-type"}},
	{op: "DeleteObjects", level: levelBucket, method: http.MethodPost, sub: "delete", relation: core.RelCanCreate, verb: objectstorage.AuditVerbDeleteObjects, write: true},

	{op: "PutObject", level: levelObject, method: http.MethodPut, relation: core.RelCanCreate, verb: objectstorage.AuditVerbPutObject, write: true},
	{op: "GetObject", level: levelObject, method: http.MethodGet, relation: core.RelCanViewSensitive, verb: objectstorage.AuditVerbGetObject},
	{op: "HeadObject", level: levelObject, method: http.MethodHead, relation: core.RelCanView, verb: objectstorage.AuditVerbGetObject},
	{op: "DeleteObject", level: levelObject, method: http.MethodDelete, relation: core.RelCanCreate, verb: objectstorage.AuditVerbDeleteObject},
	{op: "GetObjectTagging", level: levelObject, method: http.MethodGet, sub: "tagging", relation: core.RelCanView, verb: objectstorage.AuditVerbGetObject},
	{op: "CreateMultipartUpload", level: levelObject, method: http.MethodPost, sub: "uploads", relation: core.RelCanCreate, verb: objectstorage.AuditVerbCreateMultipartUpload, write: true},
	{op: "UploadPart", level: levelObject, method: http.MethodPut, sub: "partNumber", relation: core.RelCanCreate, verb: objectstorage.AuditVerbPutObject, write: true, params: []string{"uploadId"}},
	{op: "CompleteMultipartUpload", level: levelObject, method: http.MethodPost, sub: "uploadId", relation: core.RelCanCreate, verb: objectstorage.AuditVerbCompleteMultipartUpload, write: true},
	{op: "AbortMultipartUpload", level: levelObject, method: http.MethodDelete, sub: "uploadId", relation: core.RelCanCreate, verb: objectstorage.AuditVerbAbortMultipartUpload},
	{op: "ListParts", level: levelObject, method: http.MethodGet, sub: "uploadId", relation: core.RelCanView, verb: objectstorage.AuditVerbGetObject,
		params: []string{"max-parts", "part-number-marker"}},
}

// commonParams are the query parameters every operation may carry. The AWS SDKs
// of Go and JS add x-id=<Operation> to every request, which names the operation
// and changes nothing.
var commonParams = map[string]bool{"x-id": true}

// handler runs one operation. It writes its success response, or returns the
// error to answer with.
type handler func(rq *request) *Error

// handlers are the operations bex implements, by route op. The files that
// implement them add theirs in an init, so a route with no handler answers 501.
var handlers = map[string]handler{}

// splitPath reads the path-style names: /, /<bucket>, /<bucket>/ and
// /<bucket>/<key>. The key is everything after the first slash, as decoded.
func splitPath(path string) (bucket, key string) {
	path = strings.TrimPrefix(path, "/")
	bucket, key, _ = strings.Cut(path, "/")
	return bucket, key
}

// match finds the route for a request. A subresource parameter present in the
// query selects its own operation; the plain one is the fallback. It is matched
// before anything else, so `PUT /b/k?tagging` can never overwrite an object and
// `DELETE /b/k?tagging` can never delete one: a subresource bex has no operation
// for answers 501, and so does any query parameter the chosen operation does not
// take.
func match(method string, lv level, q url.Values) (*route, *Error) {
	var plain *route
	for i := range routes {
		rt := &routes[i]
		if rt.level != lv || rt.method != method {
			continue
		}
		if rt.sub == "" {
			plain = rt
		} else if q.Has(rt.sub) && !(rt.sub == "list-type" && q.Get("list-type") != "2") {
			return rt, checkParams(rt, q)
		}
	}
	// A subresource that names an operation bex does not serve is never the plain
	// operation: PUT /bucket/key?acl is not a PutObject.
	if sr := unsupportedSubresource(q); sr != "" {
		return nil, errNotImplemented("The " + sr + " subresource is not implemented")
	}
	if plain == nil {
		if methodServed(method, lv) {
			return nil, errNotImplemented("This S3 operation is not implemented")
		}
		return nil, newError(http.StatusMethodNotAllowed, "MethodNotAllowed", "The specified method is not allowed against this resource.")
	}
	return plain, checkParams(plain, q)
}

// methodServed reports whether some route of the level takes method, so a
// subresource combination bex lacks is a 501 and a verb the level never takes is
// a 405.
func methodServed(method string, lv level) bool {
	for i := range routes {
		if routes[i].level == lv && routes[i].method == method {
			return true
		}
	}
	return false
}

// checkParams refuses every query parameter the operation does not take.
func checkParams(rt *route, q url.Values) *Error {
	allowed := map[string]bool{}
	for _, p := range rt.params {
		allowed[p] = true
	}
	if rt.sub != "" {
		allowed[rt.sub] = true
	}
	var extra []string
	for name := range q {
		if allowed[name] || commonParams[name] || strings.HasPrefix(name, "X-Amz-") {
			continue
		}
		extra = append(extra, name)
	}
	if len(extra) == 0 {
		return nil
	}
	sort.Strings(extra)
	return errNotImplemented("The query parameter " + extra[0] + " is not implemented")
}

// unsupportedSubresources are the S3 subresources bex has no operation for. A
// request carrying one never reaches an object or bucket handler.
var unsupportedSubresources = []string{
	"acl", "policy", "policyStatus", "versioning", "versions", "versionId", "retention", "legal-hold", "object-lock", "encryption",
	"replication", "notification", "website", "accelerate", "select", "restore", "torrent", "requestPayment", "logging", "analytics",
	"metrics", "inventory", "ownershipControls", "publicAccessBlock", "intelligent-tiering", "attributes", "tagging", "cors", "lifecycle",
	"uploads", "uploadId", "partNumber", "delete", "location", "list-type",
}

// unsupportedSubresource names a subresource in the query that the matched level
// and method have no operation for.
func unsupportedSubresource(q url.Values) string {
	for _, sr := range unsupportedSubresources {
		if q.Has(sr) {
			return sr
		}
	}
	return ""
}

// permitted is the key's permission ceiling (ADR112 D3): read is the creator's
// can_view and can_view_sensitive, readwrite adds can_create. No key ever holds
// can_manage, so a leaked key cannot publish a bucket.
func permitted(permission, relation string) bool {
	switch relation {
	case core.RelCanView, core.RelCanViewSensitive:
		return permission == objectstorage.KeyPermissionRead || permission == objectstorage.KeyPermissionReadWrite
	case core.RelCanCreate:
		return permission == objectstorage.KeyPermissionReadWrite
	}
	return false
}

// authorize authorizes the request as its key's creator on exactly the key's
// workspace, under the route's audit verb and the target the path names. It never
// authorizes on anything the request names: the workspace is the key's row's.
func (rq *request) authorize(relation, verb, target string) *Error {
	if !permitted(rq.key.Permission, relation) {
		// The key's own ceiling refuses before OpenFGA is asked, and the refusal is
		// still a denied row, deduped like any other (ADR112 D6).
		rq.g.recordCeilingDenial(rq, relation, verb, target)
		return errAccessDenied("Access Denied: this access key is not permitted to do that")
	}
	_, err := rq.g.cfg.Base.AuthorizeWorkspaceTargetAs(rq.ctx, verb, relation, rq.workspaceID, target)
	return authzError(err)
}

// authzError maps a core authorization result to S3's. An unreachable checker is
// a 503 an SDK retries, never a 403 it does not.
func authzError(err error) *Error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, core.ErrAuthzUnavailable), errors.Is(err, core.ErrUnavailable):
		return errServiceUnavailable("Authorization is temporarily unavailable")
	case errors.Is(err, core.ErrForbidden):
		return errAccessDenied("Access Denied")
	}
	return errInternal()
}

// target is the audit target the path names: the bucket, or the object in it.
func (rq *request) target() string {
	if rq.objectKey != "" {
		return core.ObjectTarget(rq.bucket, rq.objectKey)
	}
	return core.BucketTarget(rq.bucket)
}
