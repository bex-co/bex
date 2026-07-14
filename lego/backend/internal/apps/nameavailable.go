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

package apps

import (
	"context"
	"fmt"
	"strconv"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// NameAvailability is the result of a create-form availability check
// (w4/m19): whether name is free in the caller's workspace, and — when
// taken — the next free "<name>-N" suggestion.
type NameAvailability struct {
	Available  bool
	Suggestion string
}

// NameAvailable answers whether name is free for a create in the caller's
// workspace, and if not, the next free "<name>-N" suggestion — the
// dashboard's debounced create-form check (Render's own "Name is already in
// use" + suggestion UX, docs/render-artifacts/duplicate-service-names.md).
// Scoped strictly to the caller's own tenant: uniqueness is per-workspace
// (apps.Store's UNIQUE(tenant_id, name)), so this must never reflect another
// workspace's names — the same reason Service.create's own duplicate check
// (nameTaken) never widens past the target tenant.
func (s *Service) NameAvailable(ctx context.Context, name string) (NameAvailability, error) {
	if err := s.Authorize(ctx, core.RelCanCreate); err != nil {
		return NameAvailability{}, err
	}
	if !store.ValidAppName(name) {
		return NameAvailability{}, fmt.Errorf("%w: name must be a DNS label of 1-30 chars ([a-z0-9-])", core.ErrBadRequest)
	}
	taken, err := s.tenantNames(ctx)
	if err != nil {
		return NameAvailability{}, err
	}
	if !taken[name] {
		return NameAvailability{Available: true}, nil
	}
	return NameAvailability{Suggestion: nextFreeName(name, taken)}, nil
}

// tenantNames lists the public names of every App in the caller's resolved
// workspace, or — store off / unbound caller — every App in the namespace,
// the single flat scope that mode has always had. publicName reads
// LabelServiceName when set (a store-managed App's object name is
// CRName(tenant, name), not its public name, w4/m19), falling back to the
// object name for hand-applied or pre-migration Apps.
func (s *Service) tenantNames(ctx context.Context) (map[string]bool, error) {
	var list appv1alpha1.AppList
	if tenantID, ok := s.Tenant(ctx); ok {
		if err := s.ListByTenant(ctx, &list, tenantID); err != nil {
			return nil, err
		}
	} else if err := s.Client.List(ctx, &list, client.InNamespace(s.Namespace)); err != nil {
		return nil, err
	}
	names := make(map[string]bool, len(list.Items))
	for i := range list.Items {
		names[publicName(&list.Items[i])] = true
	}
	return names, nil
}

// publicName is an App's workspace-scoped name — LabelServiceName when the
// create path stamped one (w4/m19), else the CR's own object name (a
// hand-applied or pre-migration App, whose object name still IS its public
// name).
func publicName(a *appv1alpha1.App) string {
	if n := a.Labels[core.LabelServiceName]; n != "" {
		return n
	}
	return a.Name
}

// nextFreeName finds the smallest N such that "<base>-N" is free in taken —
// the Render dashboard's own suggestion shape ("beancount-cms-v2" taken →
// "beancount-cms-v2-1", then "-2", …).
func nextFreeName(base string, taken map[string]bool) string {
	for n := 1; ; n++ {
		suggestion := suffixedName(base, n)
		if !taken[suggestion] {
			return suggestion
		}
	}
}

// suffixedName joins base and "-N", truncating base so the result never
// exceeds ValidAppName's 30-char cap — a suggestion must itself be a valid
// name, never just the taken one with a tail bolted on regardless of length.
func suffixedName(base string, n int) string {
	suffix := "-" + strconv.Itoa(n)
	if maxBase := 30 - len(suffix); len(base) > maxBase {
		base = base[:maxBase]
	}
	return base + suffix
}
