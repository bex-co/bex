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

package sandbox

import (
	"context"
	"fmt"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// KeyProvider mints/looks up a workspace's OpenSandbox tenant key (OSEP-0014):
// the value bex-api sends as OPEN-SANDBOX-API-KEY so the server scopes lifecycle
// ops to that workspace's `<ws>-sandbox` namespace (ADR042 D4, m32 t006).
// Implemented by the bex-api key store; nil => no key (dev/single-tenant), in
// which case the server must run single-tenant.
type KeyProvider interface {
	WorkspaceKey(ctx context.Context, workspaceID string) (string, error)
}

// Template fixes a sandbox's image, entrypoint, and resource limits at
// registration time — bex's Create is template-based (ADR014 D2), never an
// arbitrary caller image. The OpenSandbox server requires an entrypoint when an
// image is given and resource limits when no pool is referenced (validated live).
type Template struct {
	Image      string
	Entrypoint []string
	CPU        string
	Memory     string
}

// Service is the authorized sandbox feature. It is stateless: the per-workspace
// tenant key scopes OpenSandbox's own list to the workspace, so OpenSandbox is
// the source of truth and bex keeps no sandbox table (ADR042 D4). Client nil =>
// every verb returns ErrSandboxesUnavailable (BEX_OPENSANDBOX_URL unset).
type Service struct {
	*core.Base
	Client      *Client
	Keys        KeyProvider
	Templates   map[string]Template
	DefaultPlan Plan
	// DefaultTemplate is used when a create names no template — the Render CLI's
	// `ea sandbox create` sends only a plan (no template flag exists), so an empty
	// template must resolve to a registered default rather than 400 (w3/m32 t009).
	DefaultTemplate string
}

// CreateRequest is the caller's create input. OwnerID binds the workspace (as
// every create does); Template selects a registered image (empty ⇒ the default,
// since the Render CLI sends no template); Plan/Region/TimeoutSeconds/NetworkPolicy
// are Render CLI create fields echoed back on the resource.
type CreateRequest struct {
	OwnerID        string
	Template       string
	Plan           Plan
	Region         string
	TimeoutSeconds int
	NetworkPolicy  *NetworkPolicy
}

func (s *Service) enabled() bool { return s.Client != nil }

// workspaceKey resolves the caller's workspace tenant key. Empty when no key
// provider is wired (single-tenant OpenSandbox). It keys off the RESOLVED
// workspace (s.Tenant — the caller's default when they named none, or the named
// one after the membership check), never the raw named value: a create with no
// ownerId must mint the key for the caller's own workspace, not an empty one.
func (s *Service) workspaceKey(ctx context.Context) (string, error) {
	if s.Keys == nil {
		return "", nil
	}
	ws, ok := s.Tenant(ctx)
	if !ok {
		// Multi-tenant OpenSandbox but the caller resolves to no workspace (store
		// off, or an unbound machine key): there is no `<ws>-sandbox` to scope to.
		return "", fmt.Errorf("%w: no workspace resolved for the sandbox tenant key", core.ErrForbidden)
	}
	return s.Keys.WorkspaceKey(ctx, ws)
}

// Create authorizes, resolves the template, and starts a sandbox scoped to the
// caller's workspace.
func (s *Service) Create(ctx context.Context, req CreateRequest) (Sandbox, error) {
	ctx = core.WithWorkspace(ctx, req.OwnerID)
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return Sandbox{}, err
	}
	if !s.enabled() {
		return Sandbox{}, core.ErrSandboxesUnavailable
	}
	plan := req.Plan
	if plan == "" {
		plan = s.DefaultPlan
	}
	if plan != "" && !ValidPlan(plan) {
		return Sandbox{}, fmt.Errorf("%w: unknown plan %q", core.ErrBadRequest, plan)
	}
	name := req.Template
	if name == "" {
		name = s.DefaultTemplate // Render CLI create sends no template — use the default.
	}
	tmpl, ok := s.Templates[name]
	if !ok {
		return Sandbox{}, fmt.Errorf("%w: unknown template %q", core.ErrBadRequest, name)
	}
	key, err := s.workspaceKey(ctx)
	if err != nil {
		return Sandbox{}, err
	}
	ws, _ := s.Tenant(ctx) // resolved workspace (default or named), for response metadata
	cpu, mem := tmpl.CPU, tmpl.Memory
	if cpu == "" {
		cpu = "500m"
	}
	if mem == "" {
		mem = "512Mi"
	}
	entry := tmpl.Entrypoint
	if len(entry) == 0 {
		entry = []string{"sleep", "infinity"}
	}
	id, status, err := s.Client.Create(ctx, key, tmpl.Image, entry, cpu, mem, nil)
	if err != nil {
		return Sandbox{}, err
	}
	owner := ""
	if idn, ok := core.IdentityFrom(ctx); ok {
		owner = idn.Subject
	}
	return Sandbox{
		ID: id, Plan: plan, Status: status, Owner: owner, Workspace: ws, Image: tmpl.Image,
		Region: req.Region, TimeoutSeconds: req.TimeoutSeconds, NetworkPolicy: req.NetworkPolicy,
	}, nil
}

// List returns the caller's workspace's sandboxes (tenant-key-scoped).
func (s *Service) List(ctx context.Context) ([]Sandbox, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	if !s.enabled() {
		return nil, core.ErrSandboxesUnavailable
	}
	key, err := s.workspaceKey(ctx)
	if err != nil {
		return nil, err
	}
	ws, _ := s.Tenant(ctx) // resolved workspace (default or named), for response metadata
	raw, err := s.Client.List(ctx, key)
	if err != nil {
		return nil, err
	}
	out := make([]Sandbox, 0, len(raw))
	for _, r := range raw {
		out = append(out, Sandbox{ID: r.ID, Status: mapOpenSandboxStatus(r.Status.State), Workspace: ws})
	}
	return out, nil
}

// Get returns one sandbox's status.
func (s *Service) Get(ctx context.Context, id string) (Sandbox, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return Sandbox{}, err
	}
	if !s.enabled() {
		return Sandbox{}, core.ErrSandboxesUnavailable
	}
	key, err := s.workspaceKey(ctx)
	if err != nil {
		return Sandbox{}, err
	}
	status, err := s.Client.Get(ctx, key, id)
	if err != nil {
		return Sandbox{}, err
	}
	return Sandbox{ID: id, Status: status}, nil
}

// Suspend/Resume/Terminate are the lifecycle verbs; suspend/resume need operate,
// terminate needs create (delete) — matching the other resource features.
func (s *Service) Suspend(ctx context.Context, id string) error {
	return s.lifecycle(ctx, core.RelCanOperate, id, s.clientSuspend)
}
func (s *Service) Resume(ctx context.Context, id string) error {
	return s.lifecycle(ctx, core.RelCanOperate, id, s.clientResume)
}
func (s *Service) Terminate(ctx context.Context, id string) error {
	return s.lifecycle(ctx, core.RelCanCreate, id, s.clientTerminate)
}

func (s *Service) clientSuspend(ctx context.Context, key, id string) error {
	return s.Client.Suspend(ctx, key, id)
}
func (s *Service) clientResume(ctx context.Context, key, id string) error {
	return s.Client.Resume(ctx, key, id)
}
func (s *Service) clientTerminate(ctx context.Context, key, id string) error {
	return s.Client.Terminate(ctx, key, id)
}

func (s *Service) lifecycle(ctx context.Context, relation, id string, op func(ctx context.Context, key, id string) error) error {
	if err := s.Authorize(ctx, relation); err != nil {
		return err
	}
	if !s.enabled() {
		return core.ErrSandboxesUnavailable
	}
	key, err := s.workspaceKey(ctx)
	if err != nil {
		return err
	}
	return op(ctx, key, id)
}
