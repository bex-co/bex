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
	"net/http"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// errPublicUnavailable is the answer where no hosting domain is configured: a
// public bucket's host lives under it, so there is nothing to serve.
var errPublicUnavailable = cfNotEntitled("public buckets are not available on this bex deployment: it has no platform hosting domain")

// cfManagedDomainBodyLimit bounds `dev-url enable|disable`: {"enabled": bool}.
const cfManagedDomainBodyLimit = 1 << 10

// cfManagedDomain is what wrangler reads from `bucket dev-url get|enable|disable`
// (ADR112 D4). The domain is present whether or not the bucket is public, as
// Cloudflare's is.
type cfManagedDomain struct {
	BucketID string `json:"bucketId"`
	Domain   string `json:"domain"`
	Enabled  bool   `json:"enabled"`
}

// registerManagedDomain serves `wrangler r2 bucket dev-url`: GET reads whether a
// bucket is public and PUT {enabled} turns public reads on or off. Publishing
// customer data needs the workspace's admin (can_manage); reading the setting
// needs only can_view (ADR112 D6). A private bucket is the default.
func (c *cloudflareR2) registerManagedDomain(mux *http.ServeMux) {
	path := cloudflareAccountR2 + "/buckets/{bucket}/domains/managed"
	c.route(mux, "GET "+path, objectstorage.AuditVerbGetBucketPublic, core.RelCanView, func(call *cfCall) error {
		b, err := call.bucket()
		if err != nil {
			return err
		}
		return call.writeManagedDomain(b, b.Public)
	})
	c.route(mux, "PUT "+path, objectstorage.AuditVerbSetBucketPublic, core.RelCanManage, func(call *cfCall) error {
		var req struct {
			Enabled *bool `json:"enabled"`
		}
		if err := readCFBody(call.r, cfManagedDomainBodyLimit, &req); err != nil {
			return err
		}
		if req.Enabled == nil {
			return cfFail(http.StatusBadRequest, cfCodeInvalidRequest, "the request body must be {\"enabled\": true} or {\"enabled\": false}")
		}
		b, err := call.bucket()
		if err != nil {
			return err
		}
		if err := c.svc.SetPublic(call.r.Context(), call.workspaceID, b, *req.Enabled, c.baseDomain); err != nil {
			if errors.Is(err, objectstorage.ErrNoHostingDomain) {
				return errPublicUnavailable
			}
			return err
		}
		return call.writeManagedDomain(b, *req.Enabled)
	})
}

func (call *cfCall) writeManagedDomain(b store.ObjectBucket, enabled bool) error {
	domain, err := objectstorage.PublicDomain(b, call.c.baseDomain)
	if err != nil {
		return errPublicUnavailable
	}
	writeCFResult(call.w, cfManagedDomain{BucketID: b.ID, Domain: domain, Enabled: enabled})
	return nil
}
