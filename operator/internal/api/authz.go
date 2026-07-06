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
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// Checker is Core's seam to the authorization service (docs/auth.md): may
// `subject` act with `relation` on `object`? OpenFGA in production
// (NewOpenFGAChecker), a fake in tests. nil => every verb is allowed — the
// single-operator mode bex ran in before authorization existed, preserved
// byte-for-byte until BEX_OPENFGA_URL flips enforcement on.
type Checker interface {
	Check(ctx context.Context, subject, relation, object string) (bool, error)
}

// The relations Core verbs require, matching deploy/gitops/authz/model.fga.
// Everything is checked against the single default tenant until the control
// plane grows real tenants (w1/m2).
const (
	relCanView     = "can_view"
	relCanManage   = "can_manage"
	relCanMintKeys = "can_mint_keys"

	defaultTenant = "tenant:default"
)

// authorize gates a Core verb on the caller's permission. nil checker allows
// (authorization not enforced); with a checker wired, no identity in context
// or a negative check is ErrForbidden, and an unreachable checker fails closed
// with ErrAuthzUnavailable — never a pass-through.
func (c *Core) authorize(ctx context.Context, relation string) error {
	if c.Authz == nil {
		return nil
	}
	id, ok := IdentityFrom(ctx)
	if !ok {
		return ErrForbidden
	}
	allowed, err := c.Authz.Check(ctx, "user:"+id.Subject, relation, defaultTenant)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrAuthzUnavailable, err)
	}
	if !allowed {
		return ErrForbidden
	}
	return nil
}

// openfgaChecker implements Checker over OpenFGA's HTTP API. The store is
// resolved by name once (deduplicated across concurrent cold-start checks);
// positive checks are cached briefly (permission revocation takes effect
// within the TTL), negatives never are (a fresh grant applies immediately);
// concurrent identical checks coalesce into one upstream call, which also
// writes the cache exactly once.
type openfgaChecker struct {
	baseURL string
	token   string // preshared API key
	client  *http.Client
	group   singleflight.Group

	mu      sync.Mutex
	storeID string
	cache   *ttlCache[bool]
}

// NewOpenFGAChecker returns the production Checker talking to the
// cluster-internal OpenFGA API with its preshared key.
func NewOpenFGAChecker(baseURL, token string) Checker {
	return &openfgaChecker{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		token:   token,
		client:  &http.Client{Timeout: 5 * time.Second, Transport: oryTransport},
		cache:   newTTLCache[bool](),
	}
}

func (o *openfgaChecker) Check(ctx context.Context, subject, relation, object string) (bool, error) {
	key := subject + "\x00" + relation + "\x00" + object
	if allowed, ok := o.cache.get(key); ok {
		return allowed, nil // only positives are ever put
	}
	v, err, _ := o.group.Do(key, func() (any, error) {
		allowed, err := o.checkUpstream(ctx, subject, relation, object)
		if err == nil && allowed {
			o.cache.put(key, true, time.Now().Add(positiveTTL))
		}
		return allowed, err
	})
	if err != nil {
		return false, err
	}
	return v.(bool), nil
}

// checkRequest is OpenFGA's check body (a tagged struct keeps the hot path
// free of map allocation + key sorting).
type checkRequest struct {
	TupleKey struct {
		User     string `json:"user"`
		Relation string `json:"relation"`
		Object   string `json:"object"`
	} `json:"tuple_key"`
}

func (o *openfgaChecker) checkUpstream(ctx context.Context, subject, relation, object string) (bool, error) {
	storeID, err := o.store(ctx)
	if err != nil {
		return false, err
	}
	var req checkRequest
	req.TupleKey.User, req.TupleKey.Relation, req.TupleKey.Object = subject, relation, object
	body, _ := json.Marshal(req)
	var out struct {
		Allowed bool `json:"allowed"`
	}
	if err := doJSON(ctx, o.client, http.MethodPost, o.baseURL+"/stores/"+storeID+"/check", o.token, body, http.StatusOK, &out); err != nil {
		return false, err
	}
	return out.Allowed, nil
}

// store resolves the `bex` store id by name once, deduplicating the lookup
// across concurrent cold-start checks.
func (o *openfgaChecker) store(ctx context.Context) (string, error) {
	o.mu.Lock()
	id := o.storeID
	o.mu.Unlock()
	if id != "" {
		return id, nil
	}
	v, err, _ := o.group.Do("\x00store", func() (any, error) {
		var out struct {
			Stores []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"stores"`
		}
		if err := doJSON(ctx, o.client, http.MethodGet, o.baseURL+"/stores?page_size=100", o.token, nil, http.StatusOK, &out); err != nil {
			return "", err
		}
		for _, s := range out.Stores {
			if s.Name == "bex" {
				o.mu.Lock()
				o.storeID = s.ID
				o.mu.Unlock()
				return s.ID, nil
			}
		}
		return "", apiError(`openfga store "bex" not found — run scripts/authz-model.sh`)
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}
