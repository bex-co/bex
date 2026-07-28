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

package billing

import (
	"context"
	"fmt"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

type AdminStore interface {
	GetBillingLifecycle(context.Context, string) (store.BillingLifecycle, error)
	SetBillingException(context.Context, string, string, bool, string, string, time.Time) (bool, store.BillingLifecycle, error)
	ExtendBillingGrace(context.Context, string, time.Duration, string, string, time.Time) (store.BillingLifecycle, error)
	ForceBillingRecovery(context.Context, string, string, string, time.Time) (store.BillingLifecycle, error)
}

type AdminProvider interface {
	EnsureCustomer(context.Context, string) error
	EnsureContract(context.Context, string) error
	CompCustomer(context.Context, string) error
}

type Admin struct {
	Store    AdminStore
	Provider AdminProvider
	Clock    func() time.Time
}

func (a *Admin) now() time.Time {
	if a.Clock != nil {
		return a.Clock().UTC()
	}
	return time.Now().UTC()
}

func (a *Admin) OverrideBilling(ctx context.Context, workspaceID, action, actor, reason string, extension time.Duration) (store.BillingLifecycle, error) {
	if a == nil || a.Store == nil {
		return store.BillingLifecycle{}, fmt.Errorf("billing admin unavailable")
	}
	switch action {
	case "provision":
		if a.Provider == nil {
			return store.BillingLifecycle{}, fmt.Errorf("Stripe billing provider unavailable")
		}
		if err := a.Provider.EnsureCustomer(ctx, workspaceID); err != nil {
			return store.BillingLifecycle{}, err
		}
		if err := a.Provider.EnsureContract(ctx, workspaceID); err != nil {
			return store.BillingLifecycle{}, err
		}
		return a.Store.GetBillingLifecycle(ctx, workspaceID)
	case "extend_grace":
		return a.Store.ExtendBillingGrace(ctx, workspaceID, extension, actor, reason, a.now())
	case "recover":
		return a.Store.ForceBillingRecovery(ctx, workspaceID, actor, reason, a.now())
	case "exclude":
		_, state, err := a.Store.SetBillingException(ctx, workspaceID, store.BillingExcluded, true, actor, reason, a.now())
		return state, err
	case "include":
		_, state, err := a.Store.SetBillingException(ctx, workspaceID, store.BillingExcluded, false, actor, reason, a.now())
		return state, err
	case "comp":
		if a.Provider == nil {
			return store.BillingLifecycle{}, fmt.Errorf("Stripe comp provider unavailable")
		}
		if err := a.Provider.CompCustomer(ctx, workspaceID); err != nil {
			return store.BillingLifecycle{}, err
		}
		_, state, err := a.Store.SetBillingException(ctx, workspaceID, store.BillingComped, true, actor, reason, a.now())
		return state, err
	case "uncomp":
		_, state, err := a.Store.SetBillingException(ctx, workspaceID, store.BillingComped, false, actor, reason, a.now())
		return state, err
	default:
		return store.BillingLifecycle{}, fmt.Errorf("unknown billing override action %q", action)
	}
}
