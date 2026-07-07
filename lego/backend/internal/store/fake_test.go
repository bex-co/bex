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
	"fmt"
	"slices"
	"sync"
	"time"
)

// memStore is the in-memory Store used by the API and reconciler tests. It
// mirrors PGStore's classification behavior (conflicts, missing FKs) so
// handler status codes can be asserted without a database.
type memStore struct {
	mu      sync.Mutex
	tenants map[string]Tenant
	apps    map[string]App
	domains map[string]Domain
}

func newMemStore() *memStore {
	return &memStore{
		tenants: map[string]Tenant{},
		apps:    map[string]App{},
		domains: map[string]Domain{},
	}
}

func (m *memStore) CreateTenant(_ context.Context, name, plan string) (Tenant, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.tenants {
		if t.Name == name {
			return Tenant{}, fmt.Errorf("tenant: %w", ErrConflict)
		}
	}
	t := Tenant{ID: newID(TenantIDPrefix), Name: name, Plan: plan, CreatedAt: time.Now()}
	m.tenants[t.ID] = t
	return t, nil
}

func (m *memStore) ListTenants(context.Context) ([]Tenant, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Tenant, 0, len(m.tenants))
	for _, t := range m.tenants {
		out = append(out, t)
	}
	return out, nil
}

func (m *memStore) CreateApp(_ context.Context, a App) (App, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.tenants[a.TenantID]; !ok {
		return App{}, fmt.Errorf("app reference: %w", ErrNotFound)
	}
	for _, other := range m.apps {
		if other.TenantID == a.TenantID && other.Name == a.Name {
			return App{}, fmt.Errorf("app: %w", ErrConflict)
		}
	}
	a.ID = newID(AppIDPrefix)
	a.CreatedAt = time.Now()
	m.apps[a.ID] = a
	return a, nil
}

func (m *memStore) GetApp(_ context.Context, id string) (App, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.apps[id]
	if !ok {
		return App{}, fmt.Errorf("app: %w", ErrNotFound)
	}
	return a, nil
}

func (m *memStore) ListApps(context.Context) ([]App, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]App, 0, len(m.apps))
	for _, a := range m.apps {
		out = append(out, a)
	}
	return out, nil
}

func (m *memStore) DeleteApp(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.apps[id]; !ok {
		return fmt.Errorf("app: %w", ErrNotFound)
	}
	delete(m.apps, id)
	for did, d := range m.domains {
		if d.AppID == id {
			delete(m.domains, did)
		}
	}
	return nil
}

func (m *memStore) CreateDomain(_ context.Context, appID, host string, primary bool) (Domain, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.apps[appID]; !ok {
		return Domain{}, fmt.Errorf("domain reference: %w", ErrNotFound)
	}
	for _, d := range m.domains {
		if d.Host == host {
			return Domain{}, fmt.Errorf("domain: %w", ErrConflict)
		}
	}
	d := Domain{ID: newID(DomainIDPrefix), AppID: appID, Host: host, Primary: primary, CreatedAt: time.Now()}
	m.domains[d.ID] = d
	return d, nil
}

func (m *memStore) ListDesiredApps(context.Context) ([]DesiredApp, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]DesiredApp, 0, len(m.apps))
	for _, a := range m.apps {
		d := DesiredApp{App: a, TenantName: m.tenants[a.TenantID].Name}
		var hosts []Domain
		for _, dom := range m.domains {
			if dom.AppID == a.ID {
				hosts = append(hosts, dom)
			}
		}
		slices.SortFunc(hosts, func(x, y Domain) int {
			if x.Primary != y.Primary {
				if x.Primary {
					return -1
				}
				return 1
			}
			return x.CreatedAt.Compare(y.CreatedAt)
		})
		for _, dom := range hosts {
			d.Hosts = append(d.Hosts, dom.Host)
		}
		out = append(out, d)
	}
	slices.SortFunc(out, func(x, y DesiredApp) int { return x.CreatedAt.Compare(y.CreatedAt) })
	return out, nil
}

func (m *memStore) SetAppSuspended(_ context.Context, id string, suspended bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.apps[id]
	if !ok {
		return fmt.Errorf("app: %w", ErrNotFound)
	}
	a.Suspended = suspended
	m.apps[id] = a
	return nil
}
