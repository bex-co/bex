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
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
)

// objectStorageNotOffered is the answer to every Render object storage call
// while no Wasabi credential is installed, so the code ships before any bucket
// exists and nothing changes until the user installs it (ADR112 D9).
const objectStorageNotOffered = "bex does not offer Render's early-access object storage yet"

// renderObjects serves Render's early-access object storage, `render ea
// objects` and the SDKs' object clients (ADR112 D2): four routes that mint
// presigned URLs, so the bytes move between the client and Wasabi and never
// pass through bex-api. The wire shapes are those of the pinned render-oss/cli
// (pkg/client/storage/storage_gen.go). The work is in objectstorage.Service;
// this is the thin adapter ADR006 asks for.
type renderObjects struct {
	base *core.Base
	svc  *objectstorage.Service
}

// putObjectRequest is PUT's body. SizeBytes is a pointer so an absent size is
// told from a size of zero, which is a valid empty object.
type putObjectRequest struct {
	SizeBytes *int64 `json:"sizeBytes"`
}

type putObjectResponse struct {
	URL          string    `json:"url"`
	ExpiresAt    time.Time `json:"expiresAt"`
	MaxSizeBytes int64     `json:"maxSizeBytes"`
}

type getObjectResponse struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type objectMetadataWire struct {
	Key          string    `json:"key"`
	LastModified time.Time `json:"lastModified"`
	SizeBytes    int64     `json:"sizeBytes"`
}

type objectWithCursorWire struct {
	Cursor string             `json:"cursor"`
	Object objectMetadataWire `json:"object"`
}

type listObjectsResponse struct {
	Items      []objectWithCursorWire `json:"items"`
	HasNext    bool                   `json:"hasNext"`
	NextCursor string                 `json:"nextCursor,omitempty"`
}

// RegisterREST mounts the four routes of Render's object storage.
func (o *renderObjects) RegisterREST(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/objects/{ownerId}/{region}", func(w http.ResponseWriter, r *http.Request) {
		o.serve(w, r, objectstorage.AuditVerbListObjects, core.RelCanView, func(workspaceID string, region objectstorage.Region) (int, any, error) {
			limit, err := listLimit(r)
			if err != nil {
				return 0, nil, err
			}
			page, err := o.svc.ListObjects(r.Context(), workspaceID, region, r.URL.Query().Get("cursor"), limit)
			if err != nil {
				return 0, nil, err
			}
			out := listObjectsResponse{Items: make([]objectWithCursorWire, 0, len(page.Items)), HasNext: page.HasNext, NextCursor: page.NextCursor}
			for _, it := range page.Items {
				out.Items = append(out.Items, objectWithCursorWire{
					Cursor: it.Cursor, Object: objectMetadataWire{Key: it.Key, LastModified: it.LastModified, SizeBytes: it.SizeBytes},
				})
			}
			return http.StatusOK, out, nil
		})
	})

	mux.HandleFunc("PUT /v1/objects/{ownerId}/{region}/{key...}", func(w http.ResponseWriter, r *http.Request) {
		// Presigning a PUT lets the caller write, so it is a create.
		o.serve(w, r, objectstorage.AuditVerbPutObject, core.RelCanCreate, func(workspaceID string, region objectstorage.Region) (int, any, error) {
			var req putObjectRequest
			if err := core.DecodeJSON(r, &req); err != nil {
				return 0, nil, fmt.Errorf("%w: %v", core.ErrBadRequest, err)
			}
			if req.SizeBytes == nil {
				return 0, nil, core.NewBadRequestError("SIZE_BYTES_REQUIRED", "sizeBytes is required", map[string]any{"field": "sizeBytes"})
			}
			put, err := o.svc.MintPut(r.Context(), workspaceID, region, r.PathValue("key"), *req.SizeBytes)
			if err != nil {
				return 0, nil, err
			}
			return http.StatusOK, putObjectResponse{URL: put.URL, ExpiresAt: put.ExpiresAt, MaxSizeBytes: put.MaxSizeBytes}, nil
		})
	})

	mux.HandleFunc("GET /v1/objects/{ownerId}/{region}/{key...}", func(w http.ResponseWriter, r *http.Request) {
		// Object bytes are customer data, as a Postgres export is, so minting a
		// URL to read them is a sensitive read (ADR112 D6), and a mint is the
		// download's one request to bex-api, so it records its row once the URL
		// was minted (core.RecordObjectViewed): a missing object or a refused
		// mint records none.
		o.serve(w, r, objectstorage.AuditVerbGetObject, core.RelCanViewSensitive, func(workspaceID string, region objectstorage.Region) (int, any, error) {
			get, err := o.svc.MintGet(r.Context(), workspaceID, region, r.PathValue("key"))
			if err != nil {
				return 0, nil, err
			}
			o.base.RecordObjectViewed(r.Context(), workspaceID, renderObjectTarget(r))
			return http.StatusOK, getObjectResponse{URL: get.URL, ExpiresAt: get.ExpiresAt}, nil
		})
	})

	mux.HandleFunc("DELETE /v1/objects/{ownerId}/{region}/{key...}", func(w http.ResponseWriter, r *http.Request) {
		o.serve(w, r, objectstorage.AuditVerbDeleteObject, core.RelCanCreate, func(workspaceID string, region objectstorage.Region) (int, any, error) {
			if err := o.svc.DeleteObject(r.Context(), workspaceID, region, r.PathValue("key")); err != nil {
				return 0, nil, err
			}
			return http.StatusNoContent, nil, nil
		})
	})
}

// renderObjectTarget is the audit target of a request: the object its path names
// in the workspace's system bucket for the region (render-oregon), or that bucket
// for the list. Render's paths carry no bucket, so the system bucket's name is the
// one an S3 client sees (ADR112 D2). It reads the path alone, so a
// region or a key that is not valid yet is still bounded and escaped (core.ObjectTarget).
func renderObjectTarget(r *http.Request) string {
	return core.ObjectTarget(objectstorage.SystemBucketName(objectstorage.Region(r.PathValue("region"))), r.PathValue("key"))
}

// serve runs one route: the not-offered answer while no credential is
// installed, then the workspace authorization under the route's audit verb and
// target, then the region, then fn.
func (o *renderObjects) serve(w http.ResponseWriter, r *http.Request, verb, relation string, fn func(workspaceID string, region objectstorage.Region) (int, any, error)) {
	if !o.svc.Enabled() {
		core.WriteErr(w, objectsError(objectstorage.ErrNotConfigured))
		return
	}
	workspaceID, err := o.base.AuthorizeWorkspaceTargetAs(r.Context(), verb, relation, r.PathValue("ownerId"), renderObjectTarget(r))
	if err != nil {
		core.WriteErr(w, err)
		return
	}
	region, err := objectstorage.ParseRegion(r.PathValue("region"))
	if err != nil {
		core.WriteErr(w, objectsError(err))
		return
	}
	status, body, err := fn(workspaceID, region)
	if err != nil {
		core.WriteErr(w, objectsError(err))
		return
	}
	if body == nil {
		w.WriteHeader(status)
		return
	}
	core.WriteJSON(w, status, body)
}

// listLimit reads ?limit: the default when absent, otherwise a positive
// integer (the CLI refuses --limit 0 itself). The service caps it at the S3
// page size.
func listLimit(r *http.Request) (int, error) {
	n, err := core.QueryLimit(r.URL.Query())
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return objectstorage.ListDefaultLimit, nil
	}
	return n, nil
}

// objectsError maps the object store's errors to Render's error shape.
func objectsError(err error) error {
	var notOffered *objectstorage.RegionNotOfferedError
	var quota *objectstorage.QuotaError
	switch {
	case errors.Is(err, objectstorage.ErrNotConfigured):
		return core.NewNotFoundError(core.NotFoundCode, objectStorageNotOffered, nil)
	case errors.As(err, &notOffered):
		return core.NewBadRequestError("REGION_NOT_OFFERED", notOffered.Error(), map[string]any{"region": notOffered.Value})
	case errors.Is(err, objectstorage.ErrInvalidKey):
		return core.NewBadRequestError("OBJECT_KEY_INVALID",
			fmt.Sprintf("the object key is not valid: it must be 1 to %d bytes of UTF-8, must not start with /, and must have no . or .. segment, backslash or control character", objectstorage.MaxKeyBytes),
			map[string]any{"field": "key"})
	case errors.Is(err, objectstorage.ErrInvalidCursor):
		return core.NewBadRequestError(sandbox.CodeInvalidCursor, "the cursor is not one this list returned", map[string]any{"field": "cursor"})
	case errors.Is(err, objectstorage.ErrObjectTooLarge):
		return core.NewTooLargeError("OBJECT_TOO_LARGE",
			fmt.Sprintf("an object must be between 0 and %d bytes", objectstorage.MaxObjectBytesPresigned),
			map[string]any{"maxSizeBytes": objectstorage.MaxObjectBytesPresigned})
	case errors.As(err, &quota):
		return core.NewTooManyRequestsError("OBJECT_STORAGE_QUOTA", quota.Error(), map[string]any{"resource": string(quota.Resource), "limit": quota.Limit})
	case errors.Is(err, objectstorage.ErrRateLimited):
		return core.NewTooManyRequestsError("OBJECT_REQUESTS_LIMITED", "too many object requests; retry shortly", map[string]any{"retryAfterSeconds": 1})
	case errors.Is(err, objectstorage.ErrObjectNotFound):
		return core.NewNotFoundError("OBJECT_NOT_FOUND", "object not found", nil)
	}
	return err
}
