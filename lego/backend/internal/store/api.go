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

package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
)

// API is the minimal product surface over the source of truth: create a
// tenant, create an app, add a domain. Validation (business logic) lives
// here — not in the operator, not in Postgres procedures. Writes land as
// rows; the Reconciler projects them into App CRs.
type API struct {
	Store Store
	// Kick, when non-nil, nudges the reconciler after each successful write
	// so intent reaches the cluster immediately instead of on the next resync.
	Kick func()
	// Health reports readiness (the DB ping) for /healthz.
	Health func(context.Context) error
	// Token, when set, gates /v1/ behind a bearer check. Empty = open —
	// acceptable only because the Service is cluster-internal (no Ingress).
	Token string
	// Grant, when set, writes the new tenant's OpenFGA workspace membership so
	// its owner can authorize resources (replacing the model's workspace:default
	// placeholder). Nil => the tenant row is still created, without a membership.
	Grant WorkspaceGranter
}

// WorkspaceGranter writes the tenant's OpenFGA membership (the authz write side)
// on tenant create. Implemented by the api package's OpenFGA client and injected
// here — so the store keeps no dependency on the authz client. Structural.
type WorkspaceGranter interface {
	GrantWorkspaceAdmin(ctx context.Context, tenantID, subject string) error
}

// Handler returns the wired mux:
//
//	GET  /healthz                    (open)     DB ping — 200/503
//	POST /v1/tenants  GET /v1/tenants
//	POST /v1/apps     GET /v1/apps   GET|DELETE /v1/apps/{id}
//	POST /v1/domains
func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if a.Health != nil {
			if err := a.Health(r.Context()); err != nil {
				writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	v1 := http.NewServeMux()
	v1.HandleFunc("POST /v1/tenants", a.createTenant)
	v1.HandleFunc("GET /v1/tenants", a.listTenants)
	v1.HandleFunc("POST /v1/apps", a.createApp)
	v1.HandleFunc("GET /v1/apps", a.listApps)
	v1.HandleFunc("GET /v1/apps/{id}", a.getApp)
	v1.HandleFunc("DELETE /v1/apps/{id}", a.deleteApp)
	v1.HandleFunc("POST /v1/domains", a.createDomain)
	mux.Handle("/v1/", a.bearer(v1))
	return mux
}

// CreateTenantRequest is the POST /v1/tenants body.
type CreateTenantRequest struct {
	Name string `json:"name"`
	Plan string `json:"plan"`
	// Admin, when set, is the owner identity (Kratos identity id or Hydra
	// client id) granted admin on the new workspace in OpenFGA. Optional —
	// omit to create the tenant row without a membership.
	Admin string `json:"admin,omitempty"`
}

func (a *API) createTenant(w http.ResponseWriter, r *http.Request) {
	var req CreateTenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, fmt.Errorf("%w: bad request body", ErrInvalid))
		return
	}
	if err := validateName("name", req.Name); err != nil {
		writeErr(w, err)
		return
	}
	plan, err := normalizeTier("plan", req.Plan)
	if err != nil {
		writeErr(w, err)
		return
	}
	t, err := a.Store.CreateTenant(r.Context(), req.Name, plan)
	if err != nil {
		writeErr(w, err)
		return
	}
	// Grow a real OpenFGA workspace: the owner gets admin on workspace:<id>,
	// so a minted key scoped to this tenant authorizes its resources instead
	// of falling back to workspace:default. Fail closed — a tenant nobody can
	// administer is worse than a reported error the caller can retry.
	if req.Admin != "" && a.Grant != nil {
		if err := a.Grant.GrantWorkspaceAdmin(r.Context(), t.ID, "user:"+req.Admin); err != nil {
			// Unknown error → writeErr defaults to 500. The tenant row exists;
			// the caller sees the failure and can retry the grant.
			writeErr(w, fmt.Errorf("tenant %s created but granting workspace admin failed: %w", t.ID, err))
			return
		}
	}
	writeJSON(w, http.StatusCreated, t)
}

func (a *API) listTenants(w http.ResponseWriter, r *http.Request) {
	ts, err := a.Store.ListTenants(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	if ts == nil {
		ts = []Tenant{}
	}
	writeJSON(w, http.StatusOK, ts)
}

// CreateAppRequest is the POST /v1/apps body. One of repo/image is required;
// zero values fall back to the platform defaults (branch main, port 3000,
// one replica, the tenant-independent "free" tier).
type CreateAppRequest struct {
	TenantID       string `json:"tenantId"`
	Name           string `json:"name"`
	Repo           string `json:"repo"`
	Image          string `json:"image"`
	Branch         string `json:"branch"`
	Port           int32  `json:"port"`
	Replicas       int32  `json:"replicas"`
	Tier           string `json:"tier"`
	IdleTTLSeconds int32  `json:"idleTTLSeconds"`
}

func (a *API) createApp(w http.ResponseWriter, r *http.Request) {
	var req CreateAppRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, fmt.Errorf("%w: bad request body", ErrInvalid))
		return
	}
	app, err := appFromRequest(req)
	if err != nil {
		writeErr(w, err)
		return
	}
	created, err := a.Store.CreateApp(r.Context(), app)
	if err != nil {
		writeErr(w, err)
		return
	}
	a.kick()
	// The row is the sync part; url/phase are eventually consistent — they
	// appear on GET /v1/apps/{id} once the operator has reconciled the CR.
	writeJSON(w, http.StatusCreated, created)
}

// appFromRequest validates and normalizes a create request into a row.
func appFromRequest(req CreateAppRequest) (App, error) {
	if req.TenantID == "" {
		return App{}, fmt.Errorf("%w: tenantId is required", ErrInvalid)
	}
	if err := validateName("name", req.Name); err != nil {
		return App{}, err
	}
	if req.Repo == "" && req.Image == "" {
		return App{}, fmt.Errorf("%w: one of repo or image is required", ErrInvalid)
	}
	tier, err := normalizeTier("tier", req.Tier)
	if err != nil {
		return App{}, err
	}
	if req.Port < 0 || req.Port > 65535 {
		return App{}, fmt.Errorf("%w: port must be 1-65535", ErrInvalid)
	}
	if req.Port == 0 {
		req.Port = 3000
	}
	if req.Replicas < 0 || req.Replicas > 100 {
		return App{}, fmt.Errorf("%w: replicas must be 0-100", ErrInvalid)
	}
	if req.Replicas == 0 {
		req.Replicas = 1
	}
	if req.IdleTTLSeconds < 0 {
		return App{}, fmt.Errorf("%w: idleTTLSeconds must be >= 0", ErrInvalid)
	}
	if req.Branch == "" {
		req.Branch = "main"
	}
	return App{
		TenantID:       req.TenantID,
		Name:           req.Name,
		Repo:           req.Repo,
		Image:          req.Image,
		Branch:         req.Branch,
		Port:           req.Port,
		Replicas:       req.Replicas,
		Tier:           tier,
		IdleTTLSeconds: req.IdleTTLSeconds,
	}, nil
}

func (a *API) listApps(w http.ResponseWriter, r *http.Request) {
	apps, err := a.Store.ListApps(r.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	if apps == nil {
		apps = []App{}
	}
	writeJSON(w, http.StatusOK, apps)
}

func (a *API) getApp(w http.ResponseWriter, r *http.Request) {
	app, err := a.Store.GetApp(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, app)
}

func (a *API) deleteApp(w http.ResponseWriter, r *http.Request) {
	if err := a.Store.DeleteApp(r.Context(), r.PathValue("id")); err != nil {
		writeErr(w, err)
		return
	}
	a.kick()
	w.WriteHeader(http.StatusNoContent)
}

// CreateDomainRequest is the POST /v1/domains body. Primary makes the host
// the app's canonical URL (the CR's spec.host).
type CreateDomainRequest struct {
	AppID   string `json:"appId"`
	Host    string `json:"host"`
	Primary bool   `json:"primary"`
}

func (a *API) createDomain(w http.ResponseWriter, r *http.Request) {
	var req CreateDomainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, fmt.Errorf("%w: bad request body", ErrInvalid))
		return
	}
	if req.AppID == "" {
		writeErr(w, fmt.Errorf("%w: appId is required", ErrInvalid))
		return
	}
	if !hostRE.MatchString(req.Host) {
		writeErr(w, fmt.Errorf("%w: host must be a lowercase FQDN (e.g. app.example.com)", ErrInvalid))
		return
	}
	d, err := a.Store.CreateDomain(r.Context(), req.AppID, req.Host, req.Primary)
	if err != nil {
		writeErr(w, err)
		return
	}
	a.kick()
	writeJSON(w, http.StatusCreated, d)
}

func (a *API) kick() {
	if a.Kick != nil {
		a.Kick()
	}
}

// bearer gates h behind the configured token; a no-op when Token is empty.
func (a *API) bearer(h http.Handler) http.Handler {
	if a.Token == "" {
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+a.Token {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		h.ServeHTTP(w, r)
	})
}

var (
	// nameRE: DNS-1123 label capped at 30 chars, so "<tenant>-<app>" always
	// fits the 63-char CR-name limit (see CRName).
	nameRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,28}[a-z0-9])?$`)
	// hostRE: lowercase FQDN with at least two labels (custom domains are
	// full hostnames, never bare labels).
	hostRE = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z][a-z0-9-]{0,61}[a-z0-9]$`)
)

func validateName(field, v string) error {
	if !nameRE.MatchString(v) {
		return fmt.Errorf("%w: %s must be a DNS label of 1-30 chars ([a-z0-9-])", ErrInvalid, field)
	}
	return nil
}

// tiers is the plan ladder (docs/control-plane.md §Tiers); it mirrors the App
// CRD's spec.tier enum. Resources per tier live in the operator, prices in
// Metronome — this list is only the validity gate.
const tierFree = "free"

var tiers = []string{tierFree, "starter", "standard", "pro", "pro-plus", "pro-max", "pro-ultra"}

func normalizeTier(field, v string) (string, error) {
	if v == "" {
		return tierFree, nil
	}
	if slices.Contains(tiers, v) {
		return v, nil
	}
	return "", fmt.Errorf("%w: %s must be one of %s", ErrInvalid, field, strings.Join(tiers, "|"))
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	switch {
	case errors.Is(err, ErrInvalid):
		code = http.StatusBadRequest
	case errors.Is(err, ErrNotFound):
		code = http.StatusNotFound
	case errors.Is(err, ErrConflict):
		code = http.StatusConflict
	}
	writeJSON(w, code, map[string]string{"error": err.Error()})
}
