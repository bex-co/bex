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
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/bex-co/bex/lego/types/tiers"
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
	// Token gates /v1/ behind a constant-time bearer check. Empty leaves the
	// API open, so cmd/api refuses to start the :8091 listener with an empty
	// token unless BEX_CP_INSECURE=1 (w1/m53 requireCPAuth) — this field is
	// only ever empty in that explicit local-dev override.
	Token string
	// Grant, when set, writes the new tenant's OpenFGA workspace membership so
	// its owner can authorize resources (replacing the model's workspace:default
	// placeholder). Nil => the tenant row is still created, without a membership.
	Grant             MembershipGranter
	Billing           BillingAdmin
	BillingOperations BillingOperations
	// SandboxTenants resolves an OpenSandbox per-workspace key to its workspace
	// id for the GET /v1/sandbox-tenants tenant-lookup endpoint (w3/m32 t006).
	// Nil => the endpoint returns 503 (sandbox multi-tenancy not configured).
	SandboxTenants SandboxTenantResolver
}

// SandboxTenantResolver maps an OpenSandbox tenant key to its workspace id.
// Satisfied by *PGStore (WorkspaceForSandboxKey); injected by cmd/api so the
// store's big Store interface (and its fakes) need not carry the method.
type SandboxTenantResolver interface {
	WorkspaceForSandboxKey(ctx context.Context, apiKey string) (string, error)
}

type BillingAdmin interface {
	OverrideBilling(context.Context, string, string, string, string, time.Duration) (BillingLifecycle, error)
}

type BillingOperations interface {
	BillingExportReport(context.Context, string, time.Time, time.Time) (BillingExportReport, error)
	ListBillingExportIssues(context.Context, bool, int) ([]BillingExportIssue, error)
	ResolveBillingExportIssue(context.Context, string, string, string, string, time.Time) (BillingExportIssue, error)
}

// MembershipGranter writes/removes a subject's workspace membership in OpenFGA
// (the authz write side). Implemented by the authz checker and injected here so
// the store + api packages keep no dependency on the authz client. Structural.
type MembershipGranter interface {
	// GrantWorkspaceAdmin makes subject an admin of workspace:<tenantID> — the
	// owner of a freshly minted tenant.
	GrantWorkspaceAdmin(ctx context.Context, tenantID, subject string) error
	// GrantWorkspaceMember makes subject a developer of workspace:<tenantID> —
	// the role a minted API key gets (least privilege over every resource verb).
	GrantWorkspaceMember(ctx context.Context, tenantID, subject string) error
	// GrantWorkspaceRole grants subject an arbitrary role relation on
	// workspace:<tenantID> — how a redeemed invite (internal/api/tenancy.go)
	// seats the accepted member at its invited role (viewer/contributor/…/admin).
	GrantWorkspaceRole(ctx context.Context, tenantID, subject, relation string) error
	// RevokeWorkspaceMember removes subject's membership tuple for relation
	// (e.g. "developer" for a revoked API key) — on revoke.
	RevokeWorkspaceMember(ctx context.Context, tenantID, subject, relation string) error
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
	v1.HandleFunc("PATCH /v1/tenants/{id}/billing-excluded", a.setBillingExcluded)
	v1.HandleFunc("POST /v1/tenants/{id}/billing-override", a.billingOverride)
	v1.HandleFunc("GET /v1/tenants/{id}/billing-export", a.billingExportReport)
	v1.HandleFunc("GET /v1/billing/export-issues", a.listBillingExportIssues)
	v1.HandleFunc("POST /v1/billing/export-issues/{id}/resolve", a.resolveBillingExportIssue)
	v1.HandleFunc("POST /v1/apps", a.createApp)
	v1.HandleFunc("GET /v1/apps", a.listApps)
	v1.HandleFunc("GET /v1/apps/{id}", a.getApp)
	v1.HandleFunc("DELETE /v1/apps/{id}", a.deleteApp)
	v1.HandleFunc("POST /v1/domains", a.createDomain)
	// The OpenSandbox HTTP tenant provider (w3/m32 t006): the server presents the
	// CP bearer as its Authorization auth_token (passing a.bearer) plus the
	// caller's OPEN-SANDBOX-API-KEY, and this resolves the key to its namespace.
	v1.HandleFunc("GET /v1/sandbox-tenants", a.sandboxTenant)
	mux.Handle("/v1/", a.bearer(v1))
	return mux
}

type BillingOverrideRequest struct {
	Action    string `json:"action"`
	Actor     string `json:"actor"`
	Reason    string `json:"reason"`
	Extension string `json:"extension,omitempty"`
}

func (a *API) billingOverride(w http.ResponseWriter, r *http.Request) {
	if a.Billing == nil {
		writeErr(w, fmt.Errorf("billing admin unavailable"))
		return
	}
	var req BillingOverrideRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, fmt.Errorf("%w: bad request body", ErrInvalid))
		return
	}
	if strings.TrimSpace(req.Actor) == "" || strings.TrimSpace(req.Reason) == "" {
		writeErr(w, fmt.Errorf("%w: actor and reason are required", ErrInvalid))
		return
	}
	var extension time.Duration
	if req.Action == "extend_grace" {
		var err error
		extension, err = time.ParseDuration(req.Extension)
		if err != nil || extension <= 0 {
			writeErr(w, fmt.Errorf("%w: extension must be a positive duration", ErrInvalid))
			return
		}
	}
	state, err := a.Billing.OverrideBilling(r.Context(), r.PathValue("id"), req.Action, req.Actor, req.Reason, extension)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workspaceId": state.WorkspaceID, "status": state.Status, "reason": state.Reason, "graceDeadline": state.GraceDeadline})
}

func (a *API) billingExportReport(w http.ResponseWriter, r *http.Request) {
	if a.BillingOperations == nil {
		writeErr(w, fmt.Errorf("billing operations unavailable"))
		return
	}
	start, err := time.Parse(time.RFC3339, r.URL.Query().Get("start"))
	if err != nil {
		writeErr(w, fmt.Errorf("%w: start must be RFC3339", ErrInvalid))
		return
	}
	end, err := time.Parse(time.RFC3339, r.URL.Query().Get("end"))
	if err != nil || !start.Before(end) || end.Sub(start) > 35*24*time.Hour {
		writeErr(w, fmt.Errorf("%w: end must be RFC3339, after start, with a window no larger than 35 days", ErrInvalid))
		return
	}
	report, err := a.BillingOperations.BillingExportReport(r.Context(), r.PathValue("id"), start, end)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (a *API) listBillingExportIssues(w http.ResponseWriter, r *http.Request) {
	if a.BillingOperations == nil {
		writeErr(w, fmt.Errorf("billing operations unavailable"))
		return
	}
	openOnly := r.URL.Query().Get("status") != "all"
	issues, err := a.BillingOperations.ListBillingExportIssues(r.Context(), openOnly, 100)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"issues": issues})
}

type ResolveBillingExportIssueRequest struct {
	Action string `json:"action"`
	Actor  string `json:"actor"`
	Reason string `json:"reason"`
}

func (a *API) resolveBillingExportIssue(w http.ResponseWriter, r *http.Request) {
	if a.BillingOperations == nil {
		writeErr(w, fmt.Errorf("billing operations unavailable"))
		return
	}
	var req ResolveBillingExportIssueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, fmt.Errorf("%w: bad request body", ErrInvalid))
		return
	}
	issue, err := a.BillingOperations.ResolveBillingExportIssue(r.Context(), r.PathValue("id"), req.Action,
		strings.TrimSpace(req.Actor), strings.TrimSpace(req.Reason), time.Now().UTC())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, issue)
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
	// plan is the WORKSPACE plan (hobby/pro/scale/enterprise), not a compute
	// tier — see store/plans.go. Empty defaults to Hobby.
	plan, err := NormalizePlan(req.Plan)
	if err != nil {
		writeErr(w, err)
		return
	}
	if req.Admin != "" && !subjectRE.MatchString(req.Admin) {
		writeErr(w, fmt.Errorf("%w: admin must be a bare identity id (no whitespace or ':'/'#'/'@')", ErrInvalid))
		return
	}
	t, err := a.Store.CreateTenant(r.Context(), req.Name, plan)
	if err != nil {
		writeErr(w, err)
		return
	}
	// Grow a real OpenFGA workspace: the owner gets admin on workspace:<id>,
	// so a minted key scoped to this tenant authorizes its resources instead
	// of falling back to workspace:default. The membership row is what lets the
	// resolver map the admin identity to this tenant — without it the grant is
	// useless. Fail closed — a tenant nobody can administer is worse than a
	// reported error the caller can retry.
	if req.Admin != "" {
		if err := a.Store.AddMember(r.Context(), req.Admin, t.ID, "admin"); err != nil {
			writeErr(w, fmt.Errorf("tenant %s created but adding admin member failed: %w", t.ID, err))
			return
		}
		if a.Grant != nil {
			if err := a.Grant.GrantWorkspaceAdmin(r.Context(), t.ID, "user:"+req.Admin); err != nil {
				// Unknown error → writeErr defaults to 500. The tenant row exists;
				// the caller sees the failure and can retry the grant.
				writeErr(w, fmt.Errorf("tenant %s created but granting workspace admin failed: %w", t.ID, err))
				return
			}
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

// SetBillingExcludedRequest is the PATCH /v1/tenants/{id}/billing-excluded body.
// Actor attributes the audit row (defaults to "control-plane").
type SetBillingExcludedRequest struct {
	Excluded bool   `json:"excluded"`
	Actor    string `json:"actor,omitempty"`
}

// setBillingExcluded is the admin-only verb to comp/exempt a workspace out of
// Stripe or restore it (docs/ADR040-billing-metronome.md §7, Mode A). An
// excluded workspace never gets a Stripe customer or events, yet still sees
// estimatedCost. The flag decides whether money is owed, so this route is
// bearer-gated like every /v1 endpoint and has no tenant-facing counterpart;
// the change is written to audit_events.
func (a *API) setBillingExcluded(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeErr(w, fmt.Errorf("%w: tenant id is required", ErrInvalid))
		return
	}
	var req SetBillingExcludedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, fmt.Errorf("%w: bad request body", ErrInvalid))
		return
	}
	changed, err := a.Store.SetTenantBillingExcluded(r.Context(), id, req.Excluded, req.Actor, time.Now().UTC())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"tenantId":        id,
		"billingExcluded": req.Excluded,
		"changed":         changed,
	})
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
	// Enforce the workspace's per-plan service cap (Render: Hobby = 25 services,
	// counting suspended). A missing tenant surfaces as ErrNotFound from the cap
	// check, the same 404 CreateApp's FK violation would give.
	if err := a.enforceServiceCap(r.Context(), app.TenantID); err != nil {
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
	if req.Repo != "" && !ValidRepo(req.Repo) {
		return App{}, fmt.Errorf("%w: repo must be an https/ssh/git URL", ErrInvalid)
	}
	if req.Branch != "" && !ValidGitRef(req.Branch) {
		return App{}, fmt.Errorf("%w: branch must be a git ref (no shell metacharacters)", ErrInvalid)
	}
	if req.Image != "" && !ValidImage(req.Image) {
		return App{}, fmt.Errorf("%w: image must be an OCI reference (no whitespace or shell metacharacters)", ErrInvalid)
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
	if req.Replicas < 0 || req.Replicas > MaxReplicas {
		return App{}, fmt.Errorf("%w: replicas must be 0-%d", ErrInvalid, MaxReplicas)
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

// enforceServiceCap rejects a create that would exceed the tenant's plan
// service limit. Unlimited plans (MaxServices 0) skip the count query.
func (a *API) enforceServiceCap(ctx context.Context, tenantID string) error {
	t, err := a.Store.GetTenant(ctx, tenantID)
	if err != nil {
		return err
	}
	limit := LimitsFor(t.Plan).MaxServices
	if limit == 0 {
		return nil
	}
	n, err := a.Store.CountAppsForTenant(ctx, tenantID)
	if err != nil {
		return err
	}
	if n >= limit {
		return fmt.Errorf("%w: workspace on the %s plan is limited to %d services", ErrInvalid, t.Plan, limit)
	}
	return nil
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

// sandboxTenant is the OpenSandbox HTTP tenant provider endpoint (OSEP-0014,
// w3/m32 t006, ADR042 D4). The server GETs this with the caller's per-workspace
// key in the OPEN-SANDBOX-API-KEY header; it is already behind a.bearer (the
// server sends the CP token as its Authorization auth_token), so this trusts the
// header and resolves the key to the workspace's `<ws>-sandbox` namespace. The
// server then auto-scopes every lifecycle op to that namespace — a stolen key
// cannot cross tenants even if bex-api mis-stamps a request.
//
// Contract (matches opensandbox_server/tenants/http_provider.py): 200
// {"namespace","ttl"}; 401 {"code":"UNAUTHORIZED"} for an unknown key.
func (a *API) sandboxTenant(w http.ResponseWriter, r *http.Request) {
	if a.SandboxTenants == nil {
		writeJSON(w, http.StatusServiceUnavailable,
			map[string]string{"code": "UNAVAILABLE", "message": "sandbox multi-tenancy not configured"})
		return
	}
	ws, err := a.SandboxTenants.WorkspaceForSandboxKey(r.Context(), r.Header.Get("OPEN-SANDBOX-API-KEY"))
	if err != nil {
		// Any resolution failure (unknown/empty key) is 401 — the provider maps a
		// 401 to "invalid credentials", never a server outage.
		writeJSON(w, http.StatusUnauthorized,
			map[string]string{"code": "UNAUTHORIZED", "message": "unknown sandbox api key"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"namespace": SandboxNamespace(ws), "ttl": 60})
}

// bearer gates h behind the configured token; a no-op when Token is empty.
// An empty Token only reaches here under the BEX_CP_INSECURE=1 dev override —
// cmd/api's requireCPAuth aborts startup otherwise (w1/m53).
func (a *API) bearer(h http.Handler) http.Handler {
	if a.Token == "" {
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Constant-time compare (w6/004): a plain == on the bearer header is a
		// timing side-channel on BEX_CP_TOKEN. Length leaks at most the token length.
		got := []byte(r.Header.Get("Authorization"))
		want := []byte("Bearer " + a.Token)
		if subtle.ConstantTimeCompare(got, want) != 1 {
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
	// repoRE: an https://, http://, ssh://, or git@ SCP-form git URL with no
	// whitespace or control characters—the only shapes the source-clone phase
	// should ever receive. file:// and bare local paths are refused (w6/m6 t003)
	// so a request can never point a build at the build pod's own filesystem.
	// '#' is excluded too: repo, ref, and rootDir are three separately validated
	// intent fields, so accepting URL-fragment ref/subdir syntax would make their
	// precedence ambiguous and could bypass the validated ref/rootDir values.
	// Length is bounded separately in ValidRepo (RE2 caps repetition at 1000).
	repoRE = regexp.MustCompile(`^(https?://|ssh://|git@)[^\x00-\x20\x7f#]+$`)
	// refRE: a git branch/tag/ref/SHA — alphanumerics and . _ / @ + -, starting
	// with an alphanumeric (no leading "-" so a ref can never be read as a git
	// flag, no leading "." or "/"). Rejects all control/whitespace/shell-meta
	// characters (w6/m6 t003).
	refRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/@+-]{0,254}$`)
	// subjectRE: an owner identity (Kratos identity id / Hydra client id) safe to
	// splice into the OpenFGA object string "user:<id>" — no whitespace, control,
	// or the ':'/'#'/'@' characters that carry meaning in a tuple (w1/m53). Kratos
	// UUIDs and Hydra client ids fit; anything exotic is refused rather than
	// silently minting a malformed or ambiguous membership tuple.
	subjectRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,253}$`)
	// imageRE: a prebuilt OCI image reference — host[:port]/repo[:tag][@digest] —
	// restricted to the characters a real reference uses, starting with an
	// alphanumeric, no whitespace/control/shell-meta characters (w1/m53). This is
	// input hygiene bounding what a tenant can store as spec.Image; the SSRF class
	// (an image host masquerading as the registry) is closed at the operator's
	// admission prefix boundary, not here. Length is bounded by ValidImage.
	imageRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/:@-]{0,511}$`)
)

func validateName(field, v string) error {
	if !nameRE.MatchString(v) {
		return fmt.Errorf("%w: %s must be a DNS label of 1-30 chars ([a-z0-9-])", ErrInvalid, field)
	}
	return nil
}

// ValidAppName reports whether v is a valid App/tenant name: a DNS-1123 label of
// 1-30 chars. Exported so bex-api's public create verb enforces the exact same
// rule as this internal create API — the two can't disagree about what a valid
// name is (the same single-source rationale as MaxReplicas).
func ValidAppName(v string) bool { return nameRE.MatchString(v) }

// ValidRepo reports whether v is an acceptable git repo URL for a build-from-git
// App (https/ssh/git@, no whitespace or control chars, ≤2048 bytes). Exported so
// bex-api and the internal create API enforce one rule (w6/m6 t003).
func ValidRepo(v string) bool { return len(v) <= 2048 && repoRE.MatchString(v) }

// ValidGitRef reports whether v is an acceptable git branch/tag/ref for a
// build-from-git App (no shell metacharacters, no leading dash). Single-source
// for both create paths (w6/m6 t003).
func ValidGitRef(v string) bool { return refRE.MatchString(v) }

// ValidImage reports whether v is an acceptable prebuilt OCI image reference
// (host[:port]/repo[:tag][@digest], no whitespace/control/shell-meta characters,
// ≤512 bytes). Exported so bex-api and the internal create API enforce one rule
// (w1/m53). Empty is handled by the caller (repo-or-image required).
func ValidImage(v string) bool { return len(v) <= 512 && imageRE.MatchString(v) }

// ValidRootDir reports whether v is a safe build root directory: a relative path
// with no traversal ("..") or absolute components and no control characters, ≤512
// bytes. Empty (the default — build the repo root) is valid. Exported so bex-api's
// SetRootDir and create enforce one rule (w6/m6 t003).
func ValidRootDir(v string) bool {
	if v == "" {
		return true
	}
	if len(v) > 512 { // bound pathological input (symmetric with ValidRepo's cap)
		return false
	}
	if strings.ContainsAny(v, "\x00\r\n\t") || strings.Contains(v, "\\") || strings.HasPrefix(v, "/") {
		return false
	}
	for _, part := range strings.Split(v, "/") {
		if part == ".." || part == "" {
			return false
		}
	}
	return true
}

// ValidGlob reports whether v is a safe build-filter glob pattern (Render's Build
// Filters, docs/ADR018): a non-empty, ≤512-byte, repository-root-relative pattern
// with no control characters, no backslash, no traversal ("..") component, and no
// leading "/", that doublestar can compile (Render's dialect — *, **, ?, [class]).
// Exported so bex-api's SetBuildFilter and create enforce one rule for every
// buildFilter pattern; a malformed pattern is rejected at the API boundary so the
// webhook matcher never sees one it can't compile.
func ValidGlob(v string) bool {
	if v == "" || len(v) > 512 {
		return false
	}
	if strings.ContainsAny(v, "\x00\r\n\t") || strings.Contains(v, "\\") || strings.HasPrefix(v, "/") {
		return false
	}
	for _, part := range strings.Split(v, "/") {
		if part == ".." {
			return false
		}
	}
	return doublestar.ValidatePattern(v)
}

// normalizeTier validates a tier/plan string against lego/types/tiers'
// compute family, the one shared ladder (also consumed by the operator for
// pod resources). Prices are Stripe Billing's, not this validity gate's concern.
// Empty => the catalog's default tier (today "free"), matching the prior
// behavior.
func normalizeTier(field, v string) (string, error) {
	if v == "" {
		return tiers.Compute.Default().ID, nil
	}
	if _, ok := tiers.Compute.ByID(v); ok {
		return v, nil
	}
	return "", fmt.Errorf("%w: %s must be one of %s", ErrInvalid, field, strings.Join(tiers.Compute.IDs(), "|"))
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// writeErr/writeJSON are the internal control-plane API's own minimal error
// dialect ({"error"} only), deliberately NOT core.WriteErr's Render-shaped
// envelope (w9/m38): this API (BEX_CP_ADDR, :8091, bearer-token) is consumed by
// bex's own operator/projector, never a Render client, and its ErrInvalid/
// ErrNotFound/ErrConflict sentinels are package-local, not core's.
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
