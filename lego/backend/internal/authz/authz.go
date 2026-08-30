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

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
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

// CheckFresh answers a decision straight from OpenFGA, bypassing the positive
// cache entirely (it neither reads nor populates it). It satisfies
// core.FreshChecker so an issuance verb can fail closed on a just-revoked
// membership/key inside the ≤PositiveTTL window a cached Check would still allow
// (round-5 finding 4). Not singleflighted: issuance is rare, so coalescing buys
// nothing and would risk sharing a cached-elsewhere positive.
func (o *openfgaChecker) CheckFresh(ctx context.Context, subject, relation, object string) (bool, error) {
	return o.checkUpstream(ctx, subject, relation, object)
}

// checkRequest is OpenFGA's check body (a tagged struct keeps the hot path free
// of map allocation + key sorting).
type checkRequest struct {
	TupleKey tupleKey `json:"tuple_key"`
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

// tupleKey is one OpenFGA tuple (user/relation/object) — shared by writes and
// deletes so the hot path stays allocation-light.
type tupleKey struct {
	User     string `json:"user"`
	Relation string `json:"relation"`
	Object   string `json:"object"`
}

// writeRequest is OpenFGA's /write body — a batch of tuples to add and/or
// remove. Only the non-empty side is sent (OpenFGA rejects an empty writes or
// deletes block), so grant marshals writes and revoke marshals deletes.
type writeRequest struct {
	Writes  *tupleBatch `json:"writes,omitempty"`
	Deletes *tupleBatch `json:"deletes,omitempty"`
}

type tupleBatch struct {
	TupleKeys []tupleKey `json:"tuple_keys"`
}

// writeTuple sends one add (delete=false) or remove (delete=true) to OpenFGA's
// /write — the shared path for every grant/revoke membership write.
func (o *openfgaChecker) writeTuple(ctx context.Context, del bool, tk tupleKey) error {
	storeID, err := o.store(ctx)
	if err != nil {
		return err
	}
	var req writeRequest
	if del {
		req.Deletes = &tupleBatch{TupleKeys: []tupleKey{tk}}
	} else {
		req.Writes = &tupleBatch{TupleKeys: []tupleKey{tk}}
	}
	body, _ := json.Marshal(req)
	return core.DoJSON(ctx, o.client, http.MethodPost, o.baseURL+"/stores/"+storeID+"/write", o.token, body, http.StatusOK, nil)
}

// GrantWorkspaceAdmin writes the membership tuple `<subject> admin
// workspace:<tenantID>` — how a freshly minted tenant becomes a real OpenFGA
// workspace, replacing the model's `workspace:default` placeholder (w1/m2). It
// satisfies store.MembershipGranter (and workspaces.WorkspaceGranter)
// structurally, so those packages keep no dependency on this one.
func (o *openfgaChecker) GrantWorkspaceAdmin(ctx context.Context, tenantID, subject string) error {
	return o.GrantWorkspaceRole(ctx, tenantID, subject, "admin")
}

// GrantWorkspaceMember writes `<subject> developer workspace:<tenantID>` — the
// membership a minted API key gets (w1/m9). developer is least privilege that
// still covers every resource verb (can_create/can_operate/
// can_view_sensitive/can_manage_keys/can_view_logs/can_view): a machine
// credential manages resources, not org settings (members/billing), which
// admin would unlock.
func (o *openfgaChecker) GrantWorkspaceMember(ctx context.Context, tenantID, subject string) error {
	return o.GrantWorkspaceRole(ctx, tenantID, subject, "developer")
}

// GrantWorkspaceRole writes an arbitrary role tuple `<subject> <relation>
// workspace:<tenantID>` — the general membership write the w4/m12 team surface
// uses to seat any of Render's five roles (viewer/contributor/developer/admin/
// billing), where GrantWorkspaceAdmin/GrantWorkspaceMember cover only the two
// fixed cases (owner, bound key). A role change is a Revoke of the old relation
// then a Grant of the new; a redeemed invite is a Grant of its role. relation is
// the FGA relation name, which equals the tenant_members.role string.
func (o *openfgaChecker) GrantWorkspaceRole(ctx context.Context, tenantID, subject, relation string) error {
	return o.writeTuple(ctx, false, tupleKey{User: subject, Relation: relation, Object: "workspace:" + tenantID})
}

// GrantAgentSessionWorkspace makes an agent session a first-class OpenFGA
// object. Every session decision derives through this parent tuple; no caller
// tuple is copied onto the session, so later workspace role changes take effect
// without rewriting every session (ADR047 gap 6 / w3/m39).
func (o *openfgaChecker) GrantAgentSessionWorkspace(ctx context.Context, sessionID, workspaceID string) error {
	return o.writeTuple(ctx, false, tupleKey{
		User:     "workspace:" + workspaceID,
		Relation: "workspace",
		Object:   agentsession.SessionObject(sessionID),
	})
}

// RevokeWorkspaceMember removes a subject's membership tuple from a workspace —
// the delete side of both workspace teardown (workspaces.WorkspaceRevoker) and
// API-key revoke (w1/m9: the bound client stops authorizing, subject to
// bex-api's ≤30s positive-check cache). relation is the member's role ("admin"
// for a workspace owner, "developer" for a bound key; w4/m12's roles revoke
// through the same path). A tuple that's already gone is not an error to
// OpenFGA's delete, so a retried revoke/delete is idempotent.
func (o *openfgaChecker) RevokeWorkspaceMember(ctx context.Context, tenantID, subject, relation string) error {
	if err := o.writeTuple(ctx, true, tupleKey{User: subject, Relation: relation, Object: "workspace:" + tenantID}); err != nil {
		return err
	}
	// Evict every cached positive for the revoked subject (round-6 #16): the
	// key layout is subject\x00relation\x00object, so the subject prefix covers
	// the workspace decision AND every resource-scoped decision derived from
	// the membership. Same-replica revocation is then immediate; other
	// replicas' caches still drain within PositiveTTL, which is why the
	// destructive/issuance verbs additionally use CheckFresh.
	o.cache.DeleteKeyIf(func(key string) bool {
		return strings.HasPrefix(key, subject+"\x00")
	})
	return nil
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
