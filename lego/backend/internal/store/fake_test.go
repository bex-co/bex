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

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// memStore is the in-memory Store used by the API and reconciler tests. It
// mirrors PGStore's classification behavior (conflicts, missing FKs) so
// handler status codes can be asserted without a database.
type memStore struct {
	mu      sync.Mutex
	tenants map[string]Tenant
	apps    map[string]App
	domains map[string]Domain
	// Tenancy mapping keys (mirrors 0002_workspaces + owner_identity_id):
	// members maps a (tenant, subject) tenant_members row to its role — subject
	// covers both Kratos identity ids and Hydra client ids, so one map serves
	// TenantForIdentity/AddMember/BindClient/UnbindClient alike; ownerOf maps an
	// identity to the tenant it auto-minted (the race-safety gate).
	members map[memberKey]string // (tenant, subject) -> role
	ownerOf map[string]string    // identityID -> tenantID
	usage   map[usageKey]HourlyRow
}

// memberKey is the composite key of a tenant_members row.
type memberKey struct{ tenant, subject string }

func newMemStore() *memStore {
	return &memStore{
		tenants: map[string]Tenant{},
		apps:    map[string]App{},
		domains: map[string]Domain{},
		members: map[memberKey]string{},
		ownerOf: map[string]string{},
		usage:   map[usageKey]HourlyRow{},
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
	t := Tenant{ID: ids.New(ids.Workspace), Name: name, Plan: plan, CreatedAt: time.Now()}
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

func (m *memStore) GetTenant(_ context.Context, id string) (Tenant, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tenants[id]
	if !ok {
		return Tenant{}, fmt.Errorf("workspace: %w", ErrNotFound)
	}
	return t, nil
}

func (m *memStore) CountAppsForTenant(_ context.Context, tenantID string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, a := range m.apps {
		if a.TenantID == tenantID {
			n++
		}
	}
	return n, nil
}

func (m *memStore) TenantForIdentity(_ context.Context, subject string) (Tenant, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k := range m.members {
		if k.subject == subject {
			return m.tenants[k.tenant], nil
		}
	}
	return Tenant{}, fmt.Errorf("tenant: %w", ErrNotFound)
}

func (m *memStore) CreateTenantWithMember(_ context.Context, identityID, plan string) (Tenant, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Race-safe: an identity auto-mints exactly one tenant. If a concurrent
	// mint already created one, return it (the membership is re-ensured).
	if tid, ok := m.ownerOf[identityID]; ok {
		m.members[memberKey{tid, identityID}] = "admin"
		return m.tenants[tid], nil
	}
	id := ids.New(ids.Workspace)
	t := Tenant{ID: id, Name: id, Plan: plan, CreatedAt: time.Now()}
	m.tenants[id] = t
	m.ownerOf[identityID] = id
	m.members[memberKey{id, identityID}] = "admin"
	return t, nil
}

func (m *memStore) AddMember(_ context.Context, subject, tenantID, role string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.tenants[tenantID]; !ok {
		return fmt.Errorf("tenant_member reference: %w", ErrNotFound)
	}
	m.members[memberKey{tenantID, subject}] = role
	return nil
}

// BindClient records that an API key belongs to a tenant — a tenant_members
// row keyed by the key's client_id, mirroring PGStore's delete-then-insert (a
// client is bound to at most one tenant).
func (m *memStore) BindClient(_ context.Context, clientID, tenantID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.tenants[tenantID]; !ok {
		return fmt.Errorf("tenant_member reference: %w", ErrNotFound)
	}
	for k := range m.members {
		if k.subject == clientID {
			delete(m.members, k)
		}
	}
	m.members[memberKey{tenantID, clientID}] = "developer"
	return nil
}

func (m *memStore) UnbindClient(_ context.Context, clientID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k := range m.members {
		if k.subject == clientID {
			delete(m.members, k)
		}
	}
	return nil // idempotent — never bound is not an error
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
	a.ID = ids.New(ids.Service)
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
	d := Domain{ID: ids.New(ids.Domain), AppID: appID, Host: host, Primary: primary, CreatedAt: time.Now()}
	m.domains[d.ID] = d
	return d, nil
}

func (m *memStore) DeleteDomain(_ context.Context, appID, host string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, d := range m.domains {
		if d.AppID == appID && d.Host == host {
			delete(m.domains, id)
			return nil
		}
	}
	return fmt.Errorf("domain: %w", ErrNotFound)
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

func (m *memStore) SetAppTier(_ context.Context, id string, tier string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.apps[id]
	if !ok {
		return fmt.Errorf("app: %w", ErrNotFound)
	}
	a.Tier = tier
	m.apps[id] = a
	return nil
}

func (m *memStore) SetAppReplicas(_ context.Context, id string, replicas int32) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.apps[id]
	if !ok {
		return fmt.Errorf("app: %w", ErrNotFound)
	}
	a.Replicas = replicas
	m.apps[id] = a
	return nil
}

func (m *memStore) SetAppIdleTTL(_ context.Context, id string, seconds int32) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.apps[id]
	if !ok {
		return fmt.Errorf("app: %w", ErrNotFound)
	}
	a.IdleTTLSeconds = seconds
	m.apps[id] = a
	return nil
}

// usageKey is the composite primary key of usage_hourly.
type usageKey struct {
	serviceID   string
	kind        string
	tier        string
	windowStart time.Time
}

func (m *memStore) UpsertUsageHourly(_ context.Context, row HourlyRow) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := usageKey{row.ServiceID, row.Kind, row.Tier, row.WindowStart.UTC().Truncate(time.Hour)}
	m.usage[k] = row
	return nil
}

func (m *memStore) LatestUsageWindow(_ context.Context, serviceID string) (time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var latest time.Time
	for k := range m.usage {
		if k.serviceID == serviceID && k.windowStart.After(latest) {
			latest = k.windowStart
		}
	}
	return latest, nil
}

func (m *memStore) UsageMonthToDate(_ context.Context, workspaceID string, now time.Time) ([]UsageSummaryRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	totals := map[struct{ svc, kind, tier string }]int64{}
	for k, row := range m.usage {
		if row.WorkspaceID != workspaceID {
			continue
		}
		ws := k.windowStart.UTC()
		if ws.Before(monthStart) || !ws.Before(now) {
			continue
		}
		key := struct{ svc, kind, tier string }{k.serviceID, k.kind, k.tier}
		totals[key] += row.Quantity
	}
	out := make([]UsageSummaryRow, 0, len(totals))
	for key, total := range totals {
		out = append(out, UsageSummaryRow{ServiceID: key.svc, Kind: key.kind, Tier: key.tier, Total: total})
	}
	return out, nil
}
