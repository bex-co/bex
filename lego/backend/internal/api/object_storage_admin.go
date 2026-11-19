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
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// objectStorageAdmin is the bex-only management surface of object storage
// (ADR112 D3, D4): the S3 access keys, a bucket's facts that S3 cannot answer,
// and publishing a bucket. None of it is in Render's OpenAPI, so Render's strict
// request gate does not cover it; each route validates its own input and has an
// entry in classifiedOps. The work is in objectstorage.Service and AccessKeys;
// this is the thin adapter ADR006 asks for, REST here and GraphQL beside it.
type objectStorageAdmin struct {
	base       *core.Base
	svc        *objectstorage.Service
	keys       *objectstorage.AccessKeys
	baseDomain string
	now        func() time.Time
}

func newObjectStorageAdmin(base *core.Base, svc *objectstorage.Service, keys *objectstorage.AccessKeys, baseDomain string) *objectStorageAdmin {
	return &objectStorageAdmin{base: base, svc: svc, keys: keys, baseDomain: baseDomain, now: time.Now}
}

// accessKeyWire is a key as REST and GraphQL show it. The secret is in the
// create answer alone.
type accessKeyWire struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Permission string  `json:"permission"`
	CreatedBy  string  `json:"createdBy"`
	CreatedAt  string  `json:"createdAt"`
	ExpiresAt  *string `json:"expiresAt"`
	LastUsedAt *string `json:"lastUsedAt"`
	// Status is active, expired, revoked or broken (revoked because its secret
	// was lost with an OpenBao restore).
	Status string `json:"status"`
	// Secret is shown once, in the create answer, and is empty in every other.
	Secret string `json:"secret,omitempty"`
}

func (a *objectStorageAdmin) keyWire(k store.ObjectAccessKey) accessKeyWire {
	status := "active"
	switch {
	case k.Broken:
		status = "broken"
	case k.RevokedAt != nil:
		status = "revoked"
	case !k.Live(a.now()):
		status = "expired"
	}
	return accessKeyWire{
		ID: k.ID, Name: k.Name, Permission: k.Permission, CreatedBy: k.CreatedBy, CreatedAt: k.CreatedAt.UTC().Format(time.RFC3339),
		ExpiresAt: rfc3339Ptr(k.ExpiresAt), LastUsedAt: rfc3339Ptr(k.LastUsedAt), Status: status,
	}
}

// rfc3339Ptr is a time as its RFC 3339 text in UTC, nil when absent: the wire
// carries times as text so every adapter shows the same one.
func rfc3339Ptr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	text := t.UTC().Format(time.RFC3339)
	return &text
}

// bucketInfoWire is what `bex s3 info` reads: the facts S3 cannot answer.
type bucketInfoWire struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Region     string  `json:"region"`
	System     bool    `json:"system"`
	Public     bool    `json:"public"`
	PublicHost string  `json:"publicHost"`
	Objects    int64   `json:"objects"`
	Bytes      int64   `json:"bytes"`
	ObservedAt *string `json:"observedAt"`
	CreatedAt  string  `json:"createdAt"`
}

type bucketPublicWire struct {
	Public bool   `json:"public"`
	Host   string `json:"host"`
}

// createAccessKeyRequest is POST's body. ExpiresInSeconds zero or absent is a
// persistent key; otherwise the key expires, at most MaxEphemeralKeyLife later.
type createAccessKeyRequest struct {
	Name             string `json:"name"`
	Permission       string `json:"permission"`
	ExpiresInSeconds int64  `json:"expiresInSeconds"`
}

// RegisterREST mounts the bex-only routes.
func (a *objectStorageAdmin) RegisterREST(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/object-storage/{ownerId}/access-keys", func(w http.ResponseWriter, r *http.Request) {
		var req createAccessKeyRequest
		if err := core.DecodeJSON(r, &req); err != nil {
			core.WriteErr(w, fmt.Errorf("%w: %v", core.ErrBadRequest, err))
			return
		}
		created, err := a.CreateAccessKey(r.Context(), r.PathValue("ownerId"), req)
		if err != nil {
			core.WriteErr(w, adminError(err))
			return
		}
		core.WriteJSON(w, http.StatusCreated, created)
	})
	mux.HandleFunc("GET /v1/object-storage/{ownerId}/access-keys", func(w http.ResponseWriter, r *http.Request) {
		keys, err := a.ListAccessKeys(r.Context(), r.PathValue("ownerId"))
		if err != nil {
			core.WriteErr(w, adminError(err))
			return
		}
		core.WriteJSON(w, http.StatusOK, keys)
	})
	mux.HandleFunc("DELETE /v1/object-storage/{ownerId}/access-keys/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := a.RevokeAccessKey(r.Context(), r.PathValue("ownerId"), r.PathValue("id")); err != nil {
			core.WriteErr(w, adminError(err))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /v1/object-storage/{ownerId}/buckets/{bucket}", func(w http.ResponseWriter, r *http.Request) {
		info, err := a.BucketInfo(r.Context(), r.PathValue("ownerId"), r.PathValue("bucket"))
		if err != nil {
			core.WriteErr(w, adminError(err))
			return
		}
		core.WriteJSON(w, http.StatusOK, info)
	})
	mux.HandleFunc("GET /v1/object-storage/{ownerId}/buckets/{bucket}/public", func(w http.ResponseWriter, r *http.Request) {
		got, err := a.BucketPublic(r.Context(), r.PathValue("ownerId"), r.PathValue("bucket"))
		if err != nil {
			core.WriteErr(w, adminError(err))
			return
		}
		core.WriteJSON(w, http.StatusOK, got)
	})
	mux.HandleFunc("PUT /v1/object-storage/{ownerId}/buckets/{bucket}/public", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Enabled *bool `json:"enabled"`
		}
		if err := core.DecodeJSON(r, &req); err != nil || req.Enabled == nil {
			core.WriteErr(w, core.NewBadRequestError("ENABLED_REQUIRED", `the request body must be {"enabled": true} or {"enabled": false}`, map[string]any{"field": "enabled"}))
			return
		}
		got, err := a.SetBucketPublic(r.Context(), r.PathValue("ownerId"), r.PathValue("bucket"), *req.Enabled)
		if err != nil {
			core.WriteErr(w, adminError(err))
			return
		}
		core.WriteJSON(w, http.StatusOK, got)
	})
}

func (a *objectStorageAdmin) keysEnabled() error {
	if a.keys == nil || !a.keys.Enabled() {
		return objectstorage.ErrSecretStoreUnavailable
	}
	return nil
}

// CreateAccessKey makes a key for the workspace, acting as the caller. Creating
// a durable credential is a mint-class verb (ADR012 § 7): a human only, with the
// mint class on an OAuth token, so an API key or a delegated third-party token
// cannot make one. The caller then needs can_create, as writing objects does.
func (a *objectStorageAdmin) CreateAccessKey(ctx context.Context, ownerID string, req createAccessKeyRequest) (accessKeyWire, error) {
	if err := a.keysEnabled(); err != nil {
		return accessKeyWire{}, err
	}
	if err := a.base.AuthorizeMintClass(ctx); err != nil {
		return accessKeyWire{}, err
	}
	workspaceID, err := a.base.AuthorizeWorkspaceAs(ctx, objectstorage.AuditVerbCreateAccessKey, core.RelCanCreate, ownerID)
	if err != nil {
		return accessKeyWire{}, err
	}
	// A key is always the caller's: the person it acts as. A caller with no
	// identity of their own cannot make one.
	caller, ok := core.IdentityFrom(ctx)
	if !ok || caller.Subject == "" {
		return accessKeyWire{}, core.ErrForbidden
	}
	name, err := core.TrimmedLabel(req.Name)
	if err != nil {
		return accessKeyWire{}, err
	}
	if req.ExpiresInSeconds < 0 {
		return accessKeyWire{}, fmt.Errorf("%w: expiresInSeconds must not be negative", objectstorage.ErrAccessKeyInvalidInput)
	}
	created, err := a.keys.Create(ctx, objectstorage.CreateKeyInput{
		WorkspaceID: workspaceID, CreatedBy: caller.Subject, Name: name, Permission: req.Permission,
		Lifetime: time.Duration(req.ExpiresInSeconds) * time.Second, SignIn: core.KeySignInStamp(ctx),
	})
	if err != nil {
		return accessKeyWire{}, err
	}
	out := a.keyWire(created.Key)
	out.Secret = created.Secret
	return out, nil
}

// ListAccessKeys lists the workspace's keys, which any member may read.
func (a *objectStorageAdmin) ListAccessKeys(ctx context.Context, ownerID string) ([]accessKeyWire, error) {
	if err := a.keysEnabled(); err != nil {
		return nil, err
	}
	workspaceID, err := a.base.AuthorizeWorkspaceAs(ctx, objectstorage.AuditVerbListAccessKeys, core.RelCanView, ownerID)
	if err != nil {
		return nil, err
	}
	rows, err := a.keys.List(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]accessKeyWire, 0, len(rows))
	for _, k := range rows {
		out = append(out, a.keyWire(k))
	}
	return out, nil
}

// RevokeAccessKey ends a key of the workspace. It is a mint-class verb like
// creating one, so a leaked API key cannot revoke the keys of the people it
// would be replacing.
func (a *objectStorageAdmin) RevokeAccessKey(ctx context.Context, ownerID, id string) error {
	if err := a.keysEnabled(); err != nil {
		return err
	}
	if err := a.base.AuthorizeMintClass(ctx); err != nil {
		return err
	}
	workspaceID, err := a.base.AuthorizeWorkspaceTargetAs(ctx, objectstorage.AuditVerbRevokeAccessKey, core.RelCanCreate, ownerID, core.AccessKeyTarget(id))
	if err != nil {
		return err
	}
	return a.keys.Revoke(ctx, workspaceID, id)
}

// BucketInfo is a bucket's region, public host, objects and bytes (from the last
// sweep), which S3 cannot answer.
func (a *objectStorageAdmin) BucketInfo(ctx context.Context, ownerID, name string) (bucketInfoWire, error) {
	if !a.svc.Enabled() {
		return bucketInfoWire{}, objectstorage.ErrNotConfigured
	}
	workspaceID, err := a.base.AuthorizeWorkspaceTargetAs(ctx, objectstorage.AuditVerbGetBucket, core.RelCanView, ownerID, core.BucketTarget(name))
	if err != nil {
		return bucketInfoWire{}, err
	}
	b, err := a.svc.ResolveBucket(ctx, workspaceID, name)
	if err != nil {
		return bucketInfoWire{}, err
	}
	out := bucketInfoWire{ID: b.ID, Name: b.Name, Region: b.Region, System: b.System, Public: b.Public, CreatedAt: b.CreatedAt.UTC().Format(time.RFC3339)}
	if host, err := objectstorage.PublicDomain(b, a.baseDomain); err == nil {
		out.PublicHost = host
	}
	usage, ok, err := a.svc.BucketUsage(ctx, workspaceID, b.ID)
	if err != nil {
		return bucketInfoWire{}, err
	}
	if ok {
		out.Objects, out.Bytes = usage.Objects, usage.Bytes
		out.ObservedAt = rfc3339Ptr(&usage.ObservedAt)
	}
	return out, nil
}

// BucketPublic reports whether a bucket is public and its public host.
func (a *objectStorageAdmin) BucketPublic(ctx context.Context, ownerID, name string) (bucketPublicWire, error) {
	if !a.svc.Enabled() {
		return bucketPublicWire{}, objectstorage.ErrNotConfigured
	}
	workspaceID, err := a.base.AuthorizeWorkspaceTargetAs(ctx, objectstorage.AuditVerbGetBucketPublic, core.RelCanView, ownerID, core.BucketTarget(name))
	if err != nil {
		return bucketPublicWire{}, err
	}
	b, err := a.svc.ResolveBucket(ctx, workspaceID, name)
	if err != nil {
		return bucketPublicWire{}, err
	}
	return a.publicWire(b, b.Public)
}

// SetBucketPublic publishes or unpublishes a bucket. Publishing customer data
// needs the workspace's admin (can_manage), a relation no S3 access key holds,
// so a leaked key cannot do it (ADR112 D3, D4).
func (a *objectStorageAdmin) SetBucketPublic(ctx context.Context, ownerID, name string, enabled bool) (bucketPublicWire, error) {
	if !a.svc.Enabled() {
		return bucketPublicWire{}, objectstorage.ErrNotConfigured
	}
	workspaceID, err := a.base.AuthorizeWorkspaceTargetAs(ctx, objectstorage.AuditVerbSetBucketPublic, core.RelCanManage, ownerID, core.BucketTarget(name))
	if err != nil {
		return bucketPublicWire{}, err
	}
	b, err := a.svc.ResolveBucket(ctx, workspaceID, name)
	if err != nil {
		return bucketPublicWire{}, err
	}
	if err := a.svc.SetPublic(ctx, workspaceID, b, enabled, a.baseDomain); err != nil {
		return bucketPublicWire{}, err
	}
	return a.publicWire(b, enabled)
}

func (a *objectStorageAdmin) publicWire(b store.ObjectBucket, enabled bool) (bucketPublicWire, error) {
	host, err := objectstorage.PublicDomain(b, a.baseDomain)
	if err != nil {
		return bucketPublicWire{}, err
	}
	return bucketPublicWire{Public: enabled, Host: host}, nil
}

// adminError maps the object store's errors to bex's REST and GraphQL shapes.
func adminError(err error) error {
	switch {
	case errors.Is(err, objectstorage.ErrSecretStoreUnavailable):
		return core.Unavailable("S3 access keys are not available: this deployment has no secret store")
	case errors.Is(err, objectstorage.ErrNotConfigured):
		return core.NewNotFoundError(core.NotFoundCode, objectStorageNotOffered, nil)
	case errors.Is(err, objectstorage.ErrAccessKeyInvalidInput):
		return core.NewBadRequestError("ACCESS_KEY_INVALID", err.Error(), nil)
	case errors.Is(err, objectstorage.ErrAccessKeyLimit):
		return core.NewConflictError("ACCESS_KEY_LIMIT",
			fmt.Sprintf("a workspace holds at most %d live persistent access keys and a person at most %d live ephemeral ones; revoke one first",
				objectstorage.MaxPersistentAccessKeys, objectstorage.MaxEphemeralAccessKeys), nil)
	case errors.Is(err, objectstorage.ErrAccessKeyNotFound):
		return core.NewNotFoundError("ACCESS_KEY_NOT_FOUND", "access key not found", nil)
	case errors.Is(err, objectstorage.ErrBucketNotFound):
		return core.NewNotFoundError("BUCKET_NOT_FOUND", "bucket not found", nil)
	case errors.Is(err, objectstorage.ErrNoHostingDomain):
		return core.Unavailable("public buckets are not available on this bex deployment: it has no platform hosting domain")
	}
	return err
}

// GraphQLQuery adds the read verbs beside REST's (ADR006).
func (a *objectStorageAdmin) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"objectAccessKeys": &graphql.Field{
			Type: graphql.NewList(accessKeyGQLType),
			Args: graphql.FieldConfigArgument{"ownerId": gqlutil.Arg(graphql.String)},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				keys, err := a.ListAccessKeys(p.Context, gqlutil.Str(p.Args, "ownerId"))
				return keys, adminError(err)
			},
		},
		"objectBucket": &graphql.Field{
			Type: bucketInfoGQLType,
			Args: graphql.FieldConfigArgument{"ownerId": gqlutil.Arg(graphql.String), "name": gqlutil.ReqArg(graphql.String)},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				info, err := a.BucketInfo(p.Context, gqlutil.Str(p.Args, "ownerId"), p.Args["name"].(string))
				return info, adminError(err)
			},
		},
	}
}

// GraphQLMutation adds the write verbs beside REST's.
func (a *objectStorageAdmin) GraphQLMutation() graphql.Fields {
	return graphql.Fields{
		"createObjectAccessKey": &graphql.Field{
			Type: createdAccessKeyGQLType,
			Args: graphql.FieldConfigArgument{
				"ownerId":          gqlutil.Arg(graphql.String),
				"name":             gqlutil.ReqArg(graphql.String),
				"permission":       gqlutil.ReqArg(graphql.String),
				"expiresInSeconds": gqlutil.Arg(graphql.Int),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				req := createAccessKeyRequest{Name: p.Args["name"].(string), Permission: p.Args["permission"].(string)}
				if n := gqlutil.IntPtr(p.Args, "expiresInSeconds"); n != nil {
					req.ExpiresInSeconds = int64(*n)
				}
				created, err := a.CreateAccessKey(p.Context, gqlutil.Str(p.Args, "ownerId"), req)
				return created, adminError(err)
			},
		},
		"revokeObjectAccessKey": &graphql.Field{
			Type: graphql.NewNonNull(graphql.Boolean),
			Args: graphql.FieldConfigArgument{"ownerId": gqlutil.Arg(graphql.String), "id": gqlutil.ReqArg(graphql.String)},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				if err := a.RevokeAccessKey(p.Context, gqlutil.Str(p.Args, "ownerId"), p.Args["id"].(string)); err != nil {
					return false, adminError(err)
				}
				return true, nil
			},
		},
		"setObjectBucketPublic": &graphql.Field{
			Type: bucketPublicGQLType,
			Args: graphql.FieldConfigArgument{
				"ownerId": gqlutil.Arg(graphql.String),
				"name":    gqlutil.ReqArg(graphql.String),
				"enabled": gqlutil.ReqArg(graphql.Boolean),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				got, err := a.SetBucketPublic(p.Context, gqlutil.Str(p.Args, "ownerId"), p.Args["name"].(string), p.Args["enabled"].(bool))
				return got, adminError(err)
			},
		},
	}
}

var accessKeyFields = graphql.Fields{
	"id":         gqlutil.ReqStrField(func(k accessKeyWire) any { return k.ID }),
	"name":       gqlutil.ReqStrField(func(k accessKeyWire) any { return k.Name }),
	"permission": gqlutil.ReqStrField(func(k accessKeyWire) any { return k.Permission }),
	"createdBy":  gqlutil.ReqStrField(func(k accessKeyWire) any { return k.CreatedBy }),
	"createdAt":  gqlutil.ReqStrField(func(k accessKeyWire) any { return k.CreatedAt }),
	"expiresAt":  gqlutil.StrField(func(k accessKeyWire) any { return k.ExpiresAt }),
	"lastUsedAt": gqlutil.StrField(func(k accessKeyWire) any { return k.LastUsedAt }),
	"status":     gqlutil.ReqStrField(func(k accessKeyWire) any { return k.Status }),
}

var accessKeyGQLType = graphql.NewObject(graphql.ObjectConfig{Name: "ObjectAccessKey", Fields: accessKeyFields})

// createdAccessKeyGQLType is the create answer: the key's fields and its secret,
// which no other type carries.
var createdAccessKeyGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "CreatedObjectAccessKey",
	Fields: graphql.Fields{
		"id":         gqlutil.ReqStrField(func(k accessKeyWire) any { return k.ID }),
		"name":       gqlutil.ReqStrField(func(k accessKeyWire) any { return k.Name }),
		"permission": gqlutil.ReqStrField(func(k accessKeyWire) any { return k.Permission }),
		"createdBy":  gqlutil.ReqStrField(func(k accessKeyWire) any { return k.CreatedBy }),
		"createdAt":  gqlutil.ReqStrField(func(k accessKeyWire) any { return k.CreatedAt }),
		"expiresAt":  gqlutil.StrField(func(k accessKeyWire) any { return k.ExpiresAt }),
		"status":     gqlutil.ReqStrField(func(k accessKeyWire) any { return k.Status }),
		"secret":     gqlutil.ReqStrField(func(k accessKeyWire) any { return k.Secret }),
	},
})

var bucketInfoGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "ObjectBucket",
	Fields: graphql.Fields{
		"id":         gqlutil.ReqStrField(func(b bucketInfoWire) any { return b.ID }),
		"name":       gqlutil.ReqStrField(func(b bucketInfoWire) any { return b.Name }),
		"region":     gqlutil.ReqStrField(func(b bucketInfoWire) any { return b.Region }),
		"system":     gqlutil.ReqBoolField(func(b bucketInfoWire) any { return b.System }),
		"public":     gqlutil.ReqBoolField(func(b bucketInfoWire) any { return b.Public }),
		"publicHost": gqlutil.StrField(func(b bucketInfoWire) any { return b.PublicHost }),
		"objects":    gqlutil.Typed(graphql.NewNonNull(graphql.Float), func(b bucketInfoWire) any { return b.Objects }),
		"bytes":      gqlutil.Typed(graphql.NewNonNull(graphql.Float), func(b bucketInfoWire) any { return b.Bytes }),
		"observedAt": gqlutil.StrField(func(b bucketInfoWire) any { return b.ObservedAt }),
		"createdAt":  gqlutil.ReqStrField(func(b bucketInfoWire) any { return b.CreatedAt }),
	},
})

var bucketPublicGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "ObjectBucketPublic",
	Fields: graphql.Fields{
		"public": gqlutil.ReqBoolField(func(b bucketPublicWire) any { return b.Public }),
		"host":   gqlutil.ReqStrField(func(b bucketPublicWire) any { return b.Host }),
	},
})
