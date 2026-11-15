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
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

const (
	// cfCORSBodyLimit reads a little past the stored cap so an over-cap set is
	// refused as over the cap, not as unreadable.
	cfCORSBodyLimit = objectstorage.MaxCORSBytes + 4<<10
	// cfLifecycleBodyLimit bounds a lifecycle set: 1000 rules of a few hundred
	// bytes each.
	cfLifecycleBodyLimit = 1 << 20
)

// decodeRules reads the body of a CORS or lifecycle set: {"rules": [...]}.
// Cloudflare documents rules as optional, so a body with none clears the set;
// any other field is refused rather than ignored, so a body in another tool's
// shape (S3's CORSRules, say) cannot silently clear what is stored.
func decodeRules[T any](r *http.Request, limit int64) ([]T, error) {
	var top map[string]json.RawMessage
	if err := readCFBody(r, limit, &top); err != nil {
		return nil, err
	}
	for name := range top {
		if name != "rules" {
			return nil, cfFail(http.StatusBadRequest, cfCodeInvalidRequest, "the request has a field %q, which is not part of this route's body: only rules is", name)
		}
	}
	rules := []T{}
	if raw, ok := top["rules"]; ok && string(raw) != "null" {
		if err := json.Unmarshal(raw, &rules); err != nil {
			return nil, cfFail(http.StatusBadRequest, cfCodeInvalidRequest, "rules must be a list of rules in the documented shape")
		}
	}
	return rules, nil
}

func (c *cloudflareR2) registerConfig(mux *http.ServeMux) {
	registerRules(c, mux, "cors", func(b store.ObjectBucket) json.RawMessage { return b.CORS }, cfCORSBodyLimit, c.svc.SetCORS)
	registerRules(c, mux, "lifecycle", func(b store.ObjectBucket) json.RawMessage { return b.Lifecycle }, cfLifecycleBodyLimit, c.svc.SetLifecycle)
	// Cloudflare has no lifecycle delete route (wrangler puts the set without
	// the rule), but a CORS configuration is deleted.
	c.route(mux, "DELETE "+cloudflareAccountR2+"/buckets/{bucket}/cors", core.RelCanCreate, func(call *cfCall) error {
		b, err := call.bucket()
		if err != nil {
			return err
		}
		return call.done(c.svc.SetCORS(call.r.Context(), call.workspaceID, b, nil))
	})
}

// registerRules serves GET (list) and PUT (set) of a bucket's rule set under
// /buckets/{bucket}/<name>. The stored rules are listed as they were stored;
// a set is validated and stored by the service (set), so what a route accepts
// is what the sweep or the public host will run.
func registerRules[T any](c *cloudflareR2, mux *http.ServeMux, name string, stored func(store.ObjectBucket) json.RawMessage, limit int64,
	set func(ctx context.Context, workspaceID string, b store.ObjectBucket, rules []T) error) {
	path := cloudflareAccountR2 + "/buckets/{bucket}/" + name
	c.route(mux, "GET "+path, core.RelCanView, func(call *cfCall) error {
		b, err := call.bucket()
		if err != nil {
			return err
		}
		rules := []json.RawMessage{}
		if raw := stored(b); len(raw) > 0 {
			if err := json.Unmarshal(raw, &rules); err != nil {
				return fmt.Errorf("stored %s rules are not a list: %w", name, err)
			}
		}
		writeCFResult(call.w, map[string]any{"rules": rules})
		return nil
	})
	c.route(mux, "PUT "+path, core.RelCanCreate, func(call *cfCall) error {
		rules, err := decodeRules[T](call.r, limit)
		if err != nil {
			return err
		}
		b, err := call.bucket()
		if err != nil {
			return err
		}
		return call.done(set(call.r.Context(), call.workspaceID, b, rules))
	})
}

// done answers 200 with an empty result when err is nil, and returns err
// otherwise, for a route whose answer is only that it worked.
func (call *cfCall) done(err error) error {
	if err == nil {
		writeCFResult(call.w, nil)
	}
	return err
}

// registerNotOffered answers what bex does not offer, each with 10042: bucket
// locks, event notifications, Sippy, Data Catalog and custom domains (a bucket's
// managed domain, dev-url, is served by registerManagedDomain). Listing
// custom domains answers an empty list instead, so `domain list` works.
func (c *cloudflareR2) registerNotOffered(mux *http.ServeMux) {
	c.route(mux, "GET "+cloudflareAccountR2+"/buckets/{bucket}/domains/custom", core.RelCanView, func(call *cfCall) error {
		if _, err := call.bucket(); err != nil {
			return err
		}
		writeCFResult(call.w, map[string]any{"domains": []any{}})
		return nil
	})

	refused := []struct{ path, message string }{
		{cloudflareAccountR2 + "/buckets/{bucket}/lock", "bucket locks are not offered by bex"},
		{cloudflareAccountR2 + "/buckets/{bucket}/sippy", "Sippy is not offered by bex"},
		{cloudflareAccountR2 + "/buckets/{bucket}/local-uploads", "local uploads are not offered by bex"},
		{cloudflareMount + "/accounts/{accountId}/event_notifications/{rest...}", "event notifications are not offered by bex"},
		{cloudflareMount + "/accounts/{accountId}/r2-catalog/{rest...}", "Data Catalog is not offered by bex"},
		{cloudflareAccountR2 + "/buckets/{bucket}/domains/custom/{domain}", "custom domains are not offered by bex"},
	}
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete} {
		for _, p := range refused {
			c.refuse(mux, method+" "+p.path, p.message)
		}
		if method != http.MethodGet { // GET is the empty list above
			c.refuse(mux, method+" "+cloudflareAccountR2+"/buckets/{bucket}/domains/custom", "custom domains are not offered by bex")
		}
	}
}
