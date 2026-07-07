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

// Package authz is the production authorization checker: OpenFGA over its HTTP
// API, satisfying core.Checker. It is a peer feature package — the feature
// services gate on core.Checker (fake in tests, this in production); the
// composition root wires NewOpenFGAChecker onto each service's Base.Authz.
package authz

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// openfgaChecker implements core.Checker over OpenFGA's HTTP API. The store is
// resolved by name once (deduplicated across concurrent cold-start checks);
// positive checks are cached briefly (revocation takes effect within the TTL),
// negatives never are (a fresh grant applies immediately); concurrent identical
// checks coalesce into one upstream call, which also writes the cache once.
type openfgaChecker struct {
	baseURL string
	token   string // preshared API key
	client  *http.Client
	group   singleflight.Group

	mu      sync.Mutex
	storeID string
	cache   *core.TTLCache[bool]
}

// NewOpenFGAChecker returns the production core.Checker talking to the
// cluster-internal OpenFGA API with its preshared key.
func NewOpenFGAChecker(baseURL, token string) core.Checker {
	return &openfgaChecker{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		token:   token,
		client:  &http.Client{Timeout: 5 * time.Second, Transport: core.OryTransport},
		cache:   core.NewTTLCache[bool](),
	}
}

func (o *openfgaChecker) Check(ctx context.Context, subject, relation, object string) (bool, error) {
	key := subject + "\x00" + relation + "\x00" + object
	if allowed, ok := o.cache.Get(key); ok {
		return allowed, nil // only positives are ever put
	}
	v, err, _ := o.group.Do(key, func() (any, error) {
		allowed, err := o.checkUpstream(ctx, subject, relation, object)
		if err == nil && allowed {
			o.cache.Put(key, true, time.Now().Add(core.PositiveTTL))
		}
		return allowed, err
	})
	if err != nil {
		return false, err
	}
	return v.(bool), nil
}

// checkRequest is OpenFGA's check body (a tagged struct keeps the hot path free
// of map allocation + key sorting).
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
	if err := core.DoJSON(ctx, o.client, http.MethodPost, o.baseURL+"/stores/"+storeID+"/check", o.token, body, http.StatusOK, &out); err != nil {
		return false, err
	}
	return out.Allowed, nil
}

// writeRequest is OpenFGA's /write body — a batch of tuples to add.
type writeRequest struct {
	Writes struct {
		TupleKeys []struct {
			User     string `json:"user"`
			Relation string `json:"relation"`
			Object   string `json:"object"`
		} `json:"tuple_keys"`
	} `json:"writes"`
}

// GrantWorkspaceAdmin writes the membership tuple `<subject> admin
// workspace:<tenantID>` — how a freshly minted tenant becomes a real OpenFGA
// workspace, replacing the model's `workspace:default` placeholder (w1/m2). It
// satisfies store.WorkspaceGranter structurally, so the store package needs no
// dependency on this package.
func (o *openfgaChecker) GrantWorkspaceAdmin(ctx context.Context, tenantID, subject string) error {
	storeID, err := o.store(ctx)
	if err != nil {
		return err
	}
	var req writeRequest
	req.Writes.TupleKeys = append(req.Writes.TupleKeys, struct {
		User     string `json:"user"`
		Relation string `json:"relation"`
		Object   string `json:"object"`
	}{User: subject, Relation: "admin", Object: "workspace:" + tenantID})
	body, _ := json.Marshal(req)
	return core.DoJSON(ctx, o.client, http.MethodPost, o.baseURL+"/stores/"+storeID+"/write", o.token, body, http.StatusOK, nil)
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
		if err := core.DoJSON(ctx, o.client, http.MethodGet, o.baseURL+"/stores?page_size=100", o.token, nil, http.StatusOK, &out); err != nil {
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
		return "", core.Err(`openfga store "bex" not found — run scripts/authz-model.sh`)
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}
