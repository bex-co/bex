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

package core

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Pod label + container name the controller stamps on an App's pods. Kept in
// sync by hand: the api layer must not import the operator. The logs and metrics
// features both select on these, so they live in the shared kernel.
const (
	PodLabelApp  = "app.bex.co/app"
	AppContainer = "app"
)

// Checker is the feature services' seam to the authorization service
// (docs/auth.md): may `subject` act with `relation` on `object`? OpenFGA in
// production (internal/authz), a fake in tests. nil Base.Authz => every verb is
// allowed — the single-operator mode bex ran in before authorization existed.
type Checker interface {
	Check(ctx context.Context, subject, relation, object string) (bool, error)
}

// The relations feature verbs require, matching deploy/gitops/authz/model.fga
// (Render's workspace roles). Everything is checked against the single default
// workspace until the control plane grows real workspaces (w1/m2).
const (
	RelCanView          = "can_view"           // viewer and up: lists, details, metrics
	RelCanViewLogs      = "can_view_logs"      // contributor and up (Render: viewers can't see logs)
	RelCanOperate       = "can_operate"        // contributor and up: restart/suspend/resume
	RelCanCreate        = "can_create"         // developer and up: create/delete resources
	RelCanViewSensitive = "can_view_sensitive" // developer and up: connection strings
	RelCanManageKeys    = "can_manage_keys"    // developer and up: workspace API keys
	RelCanManage        = "can_manage"         // admin only: manage the workspace itself (rename/delete)

	DefaultWorkspace = "workspace:default"
)

// WorkspaceObject is the OpenFGA object for a workspace (tenant) id — the target
// the workspace lifecycle verbs authorize against, e.g. workspace:tea-abc.
func WorkspaceObject(tenantID string) string { return "workspace:" + tenantID }

// Render's suspended enum (a string, NOT a bool) — shared by the service and
// database projections, so it lives in the kernel both features import.
const (
	RenderSuspended    = "suspended"
	RenderNotSuspended = "not_suspended"
)

// SuspendedEnum maps a bool onto Render's suspended string enum.
func SuspendedEnum(suspended bool) string {
	if suspended {
		return RenderSuspended
	}
	return RenderNotSuspended
}

// Base is the shared kernel every feature service embeds: the apiserver-thin
// client, the watched namespace, an injectable clock, and the authorization
// gate. Feature services embed *Base and call Authorize / Now / GetApp / AppPods.
type Base struct {
	Client    client.Client
	Namespace string
	// Clock supplies the current time; injectable for tests. nil => time.Now.
	Clock func() time.Time
	// Authz decides what the authenticated caller may do (OpenFGA); nil => every
	// verb allowed (pre-authorization behavior).
	Authz Checker
}

// Now returns the (injectable) current time.
func (b *Base) Now() time.Time {
	if b.Clock != nil {
		return b.Clock()
	}
	return time.Now()
}

// Authorize gates a verb on the caller's permission against the default
// workspace. Every App/logs/metrics verb starts here (they operate on the
// single default workspace until the control plane grows real per-caller
// workspace scoping, w1/m9).
func (b *Base) Authorize(ctx context.Context, relation string) error {
	return b.AuthorizeOn(ctx, relation, DefaultWorkspace)
}

// AuthorizeOn gates a verb on the caller's permission against a specific object
// (e.g. workspace:tea-abc) — the seam for verbs scoped to a named workspace
// rather than the default (the workspaces lifecycle verbs check `admin` on the
// exact workspace). nil checker allows (authorization not enforced); with a
// checker wired, no identity in context or a negative check is ErrForbidden, and
// an unreachable checker fails closed with ErrAuthzUnavailable — never a
// pass-through, so the three surfaces stay authorization-identical.
func (b *Base) AuthorizeOn(ctx context.Context, relation, object string) error {
	if b.Authz == nil {
		return nil
	}
	id, ok := IdentityFrom(ctx)
	if !ok {
		return ErrForbidden
	}
	allowed, err := b.Authz.Check(ctx, "user:"+id.Subject, relation, object)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrAuthzUnavailable, err)
	}
	if !allowed {
		return ErrForbidden
	}
	return nil
}

// GetApp fetches one App by name, mapping absence to ErrNotFound. Shared by the
// apps/logs/metrics services — each needs "does this App exist / read its
// status" without reimplementing the not-found mapping.
func (b *Base) GetApp(ctx context.Context, name string) (*appv1alpha1.App, error) {
	var a appv1alpha1.App
	err := b.Client.Get(ctx, client.ObjectKey{Namespace: b.Namespace, Name: name}, &a)
	if apierrors.IsNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// AppPods lists an App's replica pods (the controller's app.bex.co/app label) —
// the selection the logs and metrics features share.
func (b *Base) AppPods(ctx context.Context, app string) ([]corev1.Pod, error) {
	var pods corev1.PodList
	if err := b.Client.List(ctx, &pods,
		client.InNamespace(b.Namespace),
		client.MatchingLabels{PodLabelApp: app}); err != nil {
		return nil, err
	}
	return pods.Items, nil
}
