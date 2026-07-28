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
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

type fakeAdminStore struct{}

func (fakeAdminStore) GetBillingLifecycle(_ context.Context, workspace string) (store.BillingLifecycle, error) {
	return store.BillingLifecycle{WorkspaceID: workspace, Status: store.BillingHealthy}, nil
}
func (fakeAdminStore) SetBillingException(context.Context, string, string, bool, string, string, time.Time) (bool, store.BillingLifecycle, error) {
	return false, store.BillingLifecycle{}, nil
}
func (fakeAdminStore) ExtendBillingGrace(context.Context, string, time.Duration, string, string, time.Time) (store.BillingLifecycle, error) {
	return store.BillingLifecycle{}, nil
}
func (fakeAdminStore) ForceBillingRecovery(context.Context, string, string, string, time.Time) (store.BillingLifecycle, error) {
	return store.BillingLifecycle{}, nil
}

type fakeAdminProvider struct{ customer, contract bool }

func (p *fakeAdminProvider) EnsureCustomer(context.Context, string) error {
	p.customer = true
	return nil
}
func (p *fakeAdminProvider) EnsureContract(context.Context, string) error {
	p.contract = true
	return nil
}
func (*fakeAdminProvider) CompCustomer(context.Context, string) error { return nil }

func TestAdminProvisionCreatesUniqueBillingObjectsWithoutComp(t *testing.T) {
	provider := &fakeAdminProvider{}
	admin := &Admin{Store: fakeAdminStore{}, Provider: provider}
	state, err := admin.OverrideBilling(context.Background(), "tea-fixture", "provision", "ops", "m53 fixture", 0)
	if err != nil {
		t.Fatal(err)
	}
	if !provider.customer || !provider.contract || state.Status != store.BillingHealthy {
		t.Fatalf("provider customer=%v contract=%v state=%+v", provider.customer, provider.contract, state)
	}
}
