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

// Package apps is the App-lifecycle feature: the list/get read side and the
// restart/suspend/resume write side, projected as Render's "service" shape. The
// Service holds the business logic once; the rest/graphql/mcp files are thin
// registration fragments over it, so the three surfaces cannot drift.
package apps

import (
	"context"
	"fmt"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Service turns user intent (restart / suspend / resume) into App CR spec
// patches, and reads Apps as the Render service shape. It is a thin policy layer
// — the operator does the mechanism.
type Service struct {
	*core.Base
	// Store is the Postgres source of truth for store-managed Apps (those carrying
	// the bex.co/app-id label). Suspend/Resume write the row first — the row owns
	// spec.suspended, and the projection loop reverts CR patches it didn't
	// originate — then patch the CR as the fast-converge path. Nil (tests, DB-less
	// mode) falls back to CR-only patches, safe only for hand-applied Apps.
	Store IntentStore
}

// IntentStore is the slice of the source of truth Service writes through — kept
// to the methods the lifecycle verbs need, so the service can't grow into a
// second store client and tests fake a single method. *store.PGStore satisfies it.
type IntentStore interface {
	SetAppSuspended(ctx context.Context, id string, suspended bool) error
	SetAppTier(ctx context.Context, id string, tier string) error
	SetAppReplicas(ctx context.Context, id string, replicas int32) error
}

// AppView is the neutral, bex-native projection of an App — spec intent +
// observed status. Service returns this; each adapter maps it to its own wire
// format (the REST/GraphQL adapters render it in Render's Service shape).
type AppView struct {
	Name      string   `json:"name"`
	Phase     string   `json:"phase"`
	URL       string   `json:"url"`
	URLs      []string `json:"urls"`
	Image     string   `json:"image"`
	Replicas  int32    `json:"replicas"`
	Suspended bool     `json:"suspended"`
	// Plan is Render's public spelling of the App's tier (e.g. "pro_plus" for
	// spec.tier "pro-plus"), sourced from lego/types/tiers. Omitted — not
	// faked as "" — when spec.tier is empty or not a recognized tier, so a
	// Render-shaped client sees a real superset rather than a bogus plan.
	Plan      string `json:"plan,omitempty"`
	Revision  string `json:"revision"`
	CreatedAt string `json:"createdAt"`
}

func view(a *appv1alpha1.App) AppView {
	created := ""
	if !a.CreationTimestamp.IsZero() {
		created = a.CreationTimestamp.UTC().Format(time.RFC3339)
	}
	plan := ""
	if t, ok := tiers.Compute.ByID(a.Spec.Tier); ok {
		plan = t.RenderPlan
	}
	return AppView{
		Name:      a.Name,
		Phase:     string(a.Status.Phase),
		URL:       a.Status.URL,
		URLs:      a.Status.URLs,
		Image:     a.Status.Image,
		Replicas:  a.Spec.Replicas,
		Suspended: a.Spec.Suspended,
		Plan:      plan,
		Revision:  a.Status.ActiveRevision,
		CreatedAt: created,
	}
}

// List returns every App in the namespace.
func (s *Service) List(ctx context.Context) ([]AppView, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	var list appv1alpha1.AppList
	if err := s.Client.List(ctx, &list, client.InNamespace(s.Namespace)); err != nil {
		return nil, err
	}
	out := make([]AppView, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, view(&list.Items[i]))
	}
	return out, nil
}

// InstanceType is the display-shaped projection of one lego/types/tiers
// compute tier — the bex extension backing the dashboard's plan picker.
// Render's own dashboard hardcodes its instance-type list (no public REST/MCP
// equivalent exists to mirror byte-for-byte), so this is new surface, not a
// captured-live shape; ID is Render's plan spelling (what SetPlan accepts),
// matching the picker's other fields.
type InstanceType struct {
	ID     string
	Name   string
	CPU    string
	Memory string
}

// InstanceTypes lists every tier in the shared compute catalog, in ladder
// order — never a hardcoded copy (the ladder already existed in three such
// copies before w1/m8 collapsed them; the dashboard must not become a fourth).
func (s *Service) InstanceTypes(ctx context.Context) ([]InstanceType, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	ids := tiers.Compute.IDs()
	out := make([]InstanceType, len(ids))
	for i, id := range ids {
		t, _ := tiers.Compute.ByID(id)
		out[i] = InstanceType{ID: t.RenderPlan, Name: tierDisplayName(id), CPU: t.CPU, Memory: t.Memory}
	}
	return out, nil
}

// tierDisplayName turns a hyphenated tier id into Render's display spelling,
// e.g. "pro-plus" -> "Pro Plus" (matches the names captured live from
// Render's plan picker: Free, Starter, Standard, Pro, Pro Plus, Pro Max, Pro Ultra).
func tierDisplayName(id string) string {
	words := strings.Split(id, "-")
	for i, w := range words {
		if w == "" {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

// Get returns one App, or core.ErrNotFound.
func (s *Service) Get(ctx context.Context, name string) (AppView, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return AppView{}, err
	}
	a, err := s.GetApp(ctx, name)
	if err != nil {
		return AppView{}, err
	}
	return view(a), nil
}

// Restart requests a rolling restart (spec.restartedAt = now). The operator
// stamps the pod template and Kubernetes rolls the pods with no downtime.
func (s *Service) Restart(ctx context.Context, name string) (AppView, error) {
	if err := s.Authorize(ctx, core.RelCanOperate); err != nil {
		return AppView{}, err
	}
	return s.patch(ctx, name, func(a *appv1alpha1.App) {
		a.Spec.RestartedAt = s.Now().UTC().Format(time.RFC3339)
	})
}

// Suspend parks the App (spec.suspended = true): scaled to 0, host/certs kept.
func (s *Service) Suspend(ctx context.Context, name string) (AppView, error) {
	if err := s.Authorize(ctx, core.RelCanOperate); err != nil {
		return AppView{}, err
	}
	return s.setSuspended(ctx, name, true)
}

// Resume brings a suspended App back (spec.suspended = false); the operator
// restores spec.replicas.
func (s *Service) Resume(ctx context.Context, name string) (AppView, error) {
	if err := s.Authorize(ctx, core.RelCanOperate); err != nil {
		return AppView{}, err
	}
	return s.setSuspended(ctx, name, false)
}

// SetPlan changes the App's instance size (Render's `plan`, spelled per
// lego/types/tiers). Unknown plans are rejected before any write — the
// caller maps core.ErrInvalid to 400/a GraphQL error, listing the valid
// plans. A plan change resizes the pod (new requests==limits), which is a
// Deployment rollout — the same restart-shaped cost as Render's own plan
// changes.
func (s *Service) SetPlan(ctx context.Context, name, plan string) (AppView, error) {
	if err := s.Authorize(ctx, core.RelCanOperate); err != nil {
		return AppView{}, err
	}
	t, ok := tiers.Compute.ByRenderPlan(plan)
	if !ok {
		return AppView{}, fmt.Errorf("%w: plan must be one of %s", core.ErrBadRequest, strings.Join(tiers.Compute.RenderPlans(), "|"))
	}
	tier := t.ID
	return s.writeThroughStore(ctx, name,
		func(ctx context.Context, id string) error { return s.Store.SetAppTier(ctx, id, tier) },
		func(a *appv1alpha1.App) { a.Spec.Tier = tier })
}

// Scale sets the App's desired running instance count (Render's manual-scaling
// verb; the REST body field is numInstances). It writes spec.replicas the same
// row-first way as Suspend/SetPlan — the projector owns the field. The count is
// what the operator runs when the App is active: suspend still wins (it forces
// 0 in the operator's effectiveReplicas without rewriting spec.replicas), so
// scaling a suspended App takes visible effect on resume. This is the
// degenerate, human-driven case of m3 (bin-pack/autoscale); the field
// semantics settled here must stay compatible with it.
//
// The count must be 1..store.MaxReplicas (the shared upper bound the create
// path also enforces). 0 is rejected, not scale-to-zero: today the operator
// maps spec.replicas 0 to 1 (the default), so 0 is ambiguous — scale-to-zero
// (m4) owns redefining that, and will keep this 1-based verb valid.
func (s *Service) Scale(ctx context.Context, name string, replicas int32) (AppView, error) {
	if err := s.Authorize(ctx, core.RelCanOperate); err != nil {
		return AppView{}, err
	}
	if replicas < 1 || replicas > store.MaxReplicas {
		return AppView{}, fmt.Errorf("%w: numInstances must be 1-%d", core.ErrBadRequest, store.MaxReplicas)
	}
	return s.writeThroughStore(ctx, name,
		func(ctx context.Context, id string) error { return s.Store.SetAppReplicas(ctx, id, replicas) },
		func(a *appv1alpha1.App) { a.Spec.Replicas = replicas })
}

// setSuspended flips suspension with the row as the single writer of intent.
// Restart needs no row write: spec.restartedAt is not projection-owned.
func (s *Service) setSuspended(ctx context.Context, name string, suspended bool) (AppView, error) {
	return s.writeThroughStore(ctx, name,
		func(ctx context.Context, id string) error { return s.Store.SetAppSuspended(ctx, id, suspended) },
		func(a *appv1alpha1.App) { a.Spec.Suspended = suspended })
}

// writeThroughStore is the shared shape of every intent-field verb with a row
// as the single writer of truth (suspend/resume, plan): for store-managed
// Apps the row is updated first — the projection loop owns the field and
// would revert a bare CR patch on the next resync — then the CR patch after
// it makes the change converge immediately; if the row write fails, the CR is
// left untouched (the row is already wrong, so retrying is safe). Unmanaged
// (bare-CR) Apps skip the row entirely and go straight to the CR patch.
func (s *Service) writeThroughStore(
	ctx context.Context, name string,
	writeRow func(ctx context.Context, id string) error,
	mutate func(*appv1alpha1.App),
) (AppView, error) {
	if s.Store != nil {
		a, err := s.GetApp(ctx, name)
		if err != nil {
			return AppView{}, err
		}
		if id := a.Labels[store.LabelAppID]; id != "" {
			if err := writeRow(ctx, id); err != nil {
				return AppView{}, fmt.Errorf("update source of truth: %w", err)
			}
		}
	}
	return s.patch(ctx, name, mutate)
}

// patch fetches the App, applies mutate to its spec, and merge-patches — only
// spec fields change; the operator reconciles the rest. The single write path
// the lifecycle verbs share.
func (s *Service) patch(ctx context.Context, name string, mutate func(*appv1alpha1.App)) (AppView, error) {
	a, err := s.GetApp(ctx, name)
	if err != nil {
		return AppView{}, err
	}
	base := client.MergeFrom(a.DeepCopy())
	mutate(a)
	if err := s.Client.Patch(ctx, a, base); err != nil {
		return AppView{}, err
	}
	return view(a), nil
}
