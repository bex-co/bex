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

// Package deploys is the deploy-history feature (w2/m5): every rollout of a
// store-managed App is a row in lego/backend/internal/store, listable and
// triggerable over REST/GraphQL/MCP under Render's names (list_deploys /
// get_deploy / POST .../deploys) — the poll-loop a Render-trained agent
// already knows how to run. It requires the control-plane store
// (BEX_CP_DB_URI): deploy history has no CR-only equivalent to fall back to,
// so with the store unwired every verb reports core.ErrDeploysUnavailable
// (503) — the env-vars precedent, omitted rather than faked.
package deploys

import (
	"context"
	"errors"
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// DeployStore is the Service's seam to the control-plane store — the narrow
// slice of Store it needs, the same way apps.IntentStore narrows Store to the
// lifecycle verbs' writes. *store.PGStore satisfies it.
type DeployStore interface {
	CreateDeploy(ctx context.Context, appID, trigger, image string) (store.Deploy, error)
	ListDeploys(ctx context.Context, appID string) ([]store.Deploy, error)
	GetDeploy(ctx context.Context, appID, deployID string) (store.Deploy, error)
}

// DeployView is the neutral projection of a store.Deploy the adapters render
// in Render's deploy shape. Commit is left out — it stays empty until w1/m5
// tracks build-from-git commits, so there is nothing yet worth surfacing.
type DeployView struct {
	ID         string
	Status     string
	Image      string
	Trigger    string
	CreatedAt  time.Time
	StartedAt  *time.Time
	FinishedAt *time.Time
}

func view(d store.Deploy) DeployView {
	return DeployView{
		ID:         d.ID,
		Status:     d.Status,
		Image:      d.Image,
		Trigger:    d.Trigger,
		CreatedAt:  d.CreatedAt,
		StartedAt:  d.StartedAt,
		FinishedAt: d.FinishedAt,
	}
}

// Service lists and triggers deploys for store-managed Apps. Embeds
// *core.Base for the auth gate and GetApp (App-name lookup + tenant gate) —
// the same fetch every other feature service shares.
type Service struct {
	*core.Base
	Store DeployStore
}

// appStoreID resolves an already-fetched App CR to its control-plane row id
// (the bex.co/app-id label store.Reconciler stamps) — the key deploy rows are
// stored under. Empty for a hand-applied App: it never had a row, so it never
// has deploy history.
func appStoreID(a *appv1alpha1.App) string { return a.Labels[store.LabelAppID] }

// List returns a service's deploy history, newest first (Render's
// list_deploys / GET .../deploys). A hand-applied App has no history: an
// empty list, not an error.
func (s *Service) List(ctx context.Context, service string) ([]DeployView, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return nil, err
	}
	if s.Store == nil {
		return nil, core.ErrDeploysUnavailable
	}
	a, err := s.GetApp(ctx, service)
	if err != nil {
		return nil, err
	}
	appID := appStoreID(a)
	if appID == "" {
		return []DeployView{}, nil
	}
	deploys, err := s.Store.ListDeploys(ctx, appID)
	if err != nil {
		return nil, err
	}
	out := make([]DeployView, len(deploys))
	for i, d := range deploys {
		out[i] = view(d)
	}
	return out, nil
}

// Get fetches one deploy by dep-… id, scoped to service (Render's
// get_deploy / GET .../deploys/{deployId}). A deployId belonging to a
// different service, or a hand-applied service with no history at all, is
// core.ErrNotFound — the same "not yours" shape GetApp's tenant gate uses,
// never a cross-app leak through the id alone.
func (s *Service) Get(ctx context.Context, service, deployID string) (DeployView, error) {
	if err := s.Authorize(ctx, core.RelCanView); err != nil {
		return DeployView{}, err
	}
	if s.Store == nil {
		return DeployView{}, core.ErrDeploysUnavailable
	}
	a, err := s.GetApp(ctx, service)
	if err != nil {
		return DeployView{}, err
	}
	appID := appStoreID(a)
	if appID == "" {
		return DeployView{}, core.ErrNotFound
	}
	d, err := s.Store.GetDeploy(ctx, appID, deployID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return DeployView{}, core.ErrNotFound
		}
		return DeployView{}, err
	}
	return view(d), nil
}

// Trigger starts a fresh deploy (Render's POST .../deploys): opens a dep-…
// row (trigger "api") and, for an image-backed service, bumps
// spec.restartedAt the same no-row way Restart does (apps.Service.Restart) —
// a re-pull/restart now. Build-from-git activates when w1/m5 lands (out of
// scope here — the row still opens, the CR just has nothing new to build).
// Suspended services refuse the trigger: there is nothing to roll.
func (s *Service) Trigger(ctx context.Context, service string) (DeployView, error) {
	if err := s.Authorize(ctx, core.RelCanOperate); err != nil {
		return DeployView{}, err
	}
	if s.Store == nil {
		return DeployView{}, core.ErrDeploysUnavailable
	}
	a, err := s.GetApp(ctx, service)
	if err != nil {
		return DeployView{}, err
	}
	if a.Spec.Suspended {
		return DeployView{}, fmt.Errorf("%w: service %q is suspended", core.ErrConflict, service)
	}
	appID := appStoreID(a)
	if appID == "" {
		return DeployView{}, fmt.Errorf("%w: service %q is not store-managed", core.ErrBadRequest, service)
	}
	d, err := s.Store.CreateDeploy(ctx, appID, "api", a.Spec.Image)
	if err != nil {
		return DeployView{}, err
	}
	base := client.MergeFrom(a.DeepCopy())
	a.Spec.RestartedAt = s.Now().UTC().Format(time.RFC3339Nano)
	if err := s.Client.Patch(ctx, a, base); err != nil {
		return DeployView{}, err
	}
	return view(d), nil
}
