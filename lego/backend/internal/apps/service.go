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
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
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
// to the one method the lifecycle verbs need, so the service can't grow into a
// second store client and tests fake a single method. *store.PGStore satisfies it.
type IntentStore interface {
	SetAppSuspended(ctx context.Context, id string, suspended bool) error
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
	Revision  string   `json:"revision"`
	CreatedAt string   `json:"createdAt"`
}

func view(a *appv1alpha1.App) AppView {
	created := ""
	if !a.CreationTimestamp.IsZero() {
		created = a.CreationTimestamp.UTC().Format(time.RFC3339)
	}
	return AppView{
		Name:      a.Name,
		Phase:     string(a.Status.Phase),
		URL:       a.Status.URL,
		URLs:      a.Status.URLs,
		Image:     a.Status.Image,
		Replicas:  a.Spec.Replicas,
		Suspended: a.Spec.Suspended,
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

// setSuspended flips suspension with the row as the single writer of intent. For
// store-managed Apps the apps row is updated first (the projection loop owns
// spec.suspended and would revert a bare CR patch on the next resync); the CR
// patch after it makes the change converge immediately — and if that patch
// fails, the row is already right, so the resync converges it anyway. Restart
// needs no row write: spec.restartedAt is not projection-owned.
func (s *Service) setSuspended(ctx context.Context, name string, suspended bool) (AppView, error) {
	if s.Store != nil {
		a, err := s.GetApp(ctx, name)
		if err != nil {
			return AppView{}, err
		}
		if id := a.Labels[store.LabelAppID]; id != "" {
			if err := s.Store.SetAppSuspended(ctx, id, suspended); err != nil {
				return AppView{}, fmt.Errorf("update source of truth: %w", err)
			}
		}
	}
	return s.patch(ctx, name, func(a *appv1alpha1.App) { a.Spec.Suspended = suspended })
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
