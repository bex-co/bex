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
	"strings"
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
	members          map[memberKey]string // (tenant, subject) -> role
	ownerOf          map[string]string    // identityID -> tenantID
	usage            map[usageKey]HourlyRow
	monthly          map[monthKey]monthlyRow
	deploys          map[string]Deploy
	eventFacts       map[string]ServiceEventFact
	eventCheckpoints map[string]ObservedServiceState
	billingExcluded  map[string]bool // tenantID -> excluded (ADR040 §7)
}

// memberKey is the composite key of a tenant_members row.
type memberKey struct{ tenant, subject string }

func newMemStore() *memStore {
	return &memStore{
		tenants:          map[string]Tenant{},
		apps:             map[string]App{},
		domains:          map[string]Domain{},
		members:          map[memberKey]string{},
		ownerOf:          map[string]string{},
		usage:            map[usageKey]HourlyRow{},
		monthly:          map[monthKey]monthlyRow{},
		deploys:          map[string]Deploy{},
		eventFacts:       map[string]ServiceEventFact{},
		eventCheckpoints: map[string]ObservedServiceState{},
		billingExcluded:  map[string]bool{},
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

func (m *memStore) SetTenantBillingExcluded(_ context.Context, tenantID string, excluded bool, _ string, _ time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.tenants[tenantID]; !ok {
		return false, fmt.Errorf("tenant: %w", ErrNotFound)
	}
	if m.billingExcluded[tenantID] == excluded {
		return false, nil
	}
	m.billingExcluded[tenantID] = excluded
	return true, nil
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

// TenantForIdentity mirrors PGStore's default-workspace contract (w6/m14): the
// OLDEST membership, deterministically. The fake has no membership timestamp,
// but tenant ids are xid-based and therefore lexicographically ordered by
// creation, so the smallest id is the oldest tenant the subject belongs to —
// enough to make the fake's answer stable instead of map-iteration-random.
func (m *memStore) TenantForIdentity(_ context.Context, subject string) (Tenant, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	oldest := ""
	for k := range m.members {
		if k.subject == subject && (oldest == "" || k.tenant < oldest) {
			oldest = k.tenant
		}
	}
	if oldest == "" {
		return Tenant{}, fmt.Errorf("tenant: %w", ErrNotFound)
	}
	return m.tenants[oldest], nil
}

func (m *memStore) IsMember(_ context.Context, subject, tenantID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k := range m.members {
		if k.subject == subject && k.tenant == tenantID {
			return true, nil
		}
	}
	return false, nil
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

// slugTaken mirrors PGStore's apps_slug_idx: a slug is global, spanning every
// tenant, unlike the tenant-scoped name check above it.
func (m *memStore) slugTaken(slug string) bool {
	for _, other := range m.apps {
		if other.Slug == slug {
			return true
		}
	}
	return false
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
	a.Slug = a.Name
	for attempt := 0; m.slugTaken(a.Slug) && attempt < maxSlugMintAttempts; attempt++ {
		a.Slug = a.Name + "-" + randomSlugSuffix()
	}
	if m.slugTaken(a.Slug) {
		return App{}, fmt.Errorf("app: %w", ErrConflict)
	}
	a.CreatedAt = time.Now()
	m.apps[a.ID] = a
	now := time.Now()
	d := Deploy{ID: ids.New(ids.Deploy), AppID: a.ID, Trigger: TriggerCreate, Image: a.Image, Generation: 1, Status: DeployCreated, CreatedAt: now, UpdatedAt: now}
	m.deploys[d.ID] = d
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

func (m *memStore) GetEnvironmentProtectedStatus(context.Context, string) (string, error) {
	return "unprotected", nil
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
	for depID, d := range m.deploys {
		if d.AppID == id {
			delete(m.deploys, depID)
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
	now := time.Now()
	d := Domain{
		ID: ids.New(ids.Domain), AppID: appID, Host: host, Primary: primary,
		ClaimState: "verified", VerifiedAt: &now, CreatedAt: now,
	}
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

func (m *memStore) ReplaceDomains(_ context.Context, appID, primary string, hosts []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.apps[appID]; !ok {
		return fmt.Errorf("domain reference: %w", ErrNotFound)
	}
	wanted := append([]string(nil), hosts...)
	if primary != "" {
		wanted = append([]string{primary}, wanted...)
	}
	for _, host := range wanted {
		for _, d := range m.domains {
			if d.AppID != appID && d.Host == host {
				return fmt.Errorf("domain: %w", ErrConflict)
			}
		}
	}
	for id, d := range m.domains {
		if d.AppID == appID {
			delete(m.domains, id)
		}
	}
	for i, host := range wanted {
		now := time.Now()
		d := Domain{
			ID: ids.New(ids.Domain), AppID: appID, Host: host, Primary: primary != "" && i == 0,
			ClaimState: "verified", VerifiedAt: &now, CreatedAt: now,
		}
		m.domains[d.ID] = d
	}
	return nil
}

func (m *memStore) ListDesiredApps(context.Context) ([]DesiredApp, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]DesiredApp, 0, len(m.apps))
	for _, a := range m.apps {
		d := DesiredApp{App: a, TenantName: m.tenants[a.TenantID].Name}
		var hosts []Domain
		for _, dom := range m.domains {
			if dom.AppID == a.ID && dom.ClaimState == "verified" {
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
			if dom.Primary {
				d.PrimaryHost = dom.Host
			} else {
				d.Hosts = append(d.Hosts, dom.Host)
			}
			if dom.RedirectForName != "" {
				if d.HostRedirects == nil {
					d.HostRedirects = map[string]string{}
				}
				d.HostRedirects[dom.Host] = dom.RedirectForName
			}
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

func (m *memStore) SetAppSource(_ context.Context, id, repo, image, branch string, registryCredentialID *string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.apps[id]
	if !ok {
		return fmt.Errorf("app: %w", ErrNotFound)
	}
	a.Repo, a.Image, a.Branch = repo, image, branch
	a.RegistryCredentialID = registryCredentialID
	m.apps[id] = a
	return nil
}

func (m *memStore) SetAppImage(_ context.Context, id string, image string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.apps[id]
	if !ok {
		return fmt.Errorf("app: %w", ErrNotFound)
	}
	a.Image = image
	m.apps[id] = a
	return nil
}

// usageKey is the composite primary key of usage_hourly.
type usageKey struct {
	resourceKind string
	serviceID    string
	kind         string
	tier         string
	windowStart  time.Time
}

// monthKey is the composite primary key of usage_monthly; month is the first
// day of the calendar month (UTC).
type monthKey struct {
	resourceKind string
	serviceID    string
	kind         string
	tier         string
	month        time.Time
}

// monthlyRow is one usage_monthly aggregate value.
type monthlyRow struct {
	workspaceID  string
	resourceKind string
	quantity     int64
}

func (m *memStore) UpsertUsageHourly(_ context.Context, row HourlyRow) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	row.ResourceKind = NormalizeResourceKind(row.ResourceKind)
	k := usageKey{row.ResourceKind, row.ServiceID, row.Kind, row.Tier, row.WindowStart.UTC().Truncate(time.Hour)}
	m.usage[k] = row
	return nil
}

func (m *memStore) RecordUsageSourceHealth(_ context.Context, _ []UsageSourceRecord) error {
	return nil
}

func (m *memStore) ReconcileUsageSourceStreams(_ context.Context, _ []UsageResourceRef, _ time.Time) error {
	return nil
}

func (m *memStore) CurrentUsageCoverage(_ context.Context, _ string, _ time.Time) (UsageCoverage, error) {
	return UsageCoverage{}, nil
}

func (m *memStore) LatestUsageWindow(_ context.Context, resourceKind, serviceID, kind string) (time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	resourceKind = NormalizeResourceKind(resourceKind)
	var latest time.Time
	for k := range m.usage {
		if k.resourceKind == resourceKind && k.serviceID == serviceID && k.kind == kind && k.windowStart.After(latest) {
			latest = k.windowStart
		}
	}
	return latest, nil
}

// UsageMonthToDate mirrors PGStore's two-table read: hourly rows in
// [monthStart, now) plus the month's usage_monthly aggregate, summed.
func (m *memStore) UsageMonthToDate(_ context.Context, workspaceID string, now time.Time) ([]UsageSummaryRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	type summaryKey struct{ svc, kind, tier, resourceKind string }
	totals := map[summaryKey]int64{}
	for k, row := range m.usage {
		if row.WorkspaceID != workspaceID {
			continue
		}
		ws := k.windowStart.UTC()
		if ws.Before(monthStart) || !ws.Before(now) {
			continue
		}
		key := summaryKey{k.serviceID, k.kind, k.tier, row.ResourceKind}
		totals[key] += row.Quantity
	}
	for k, row := range m.monthly {
		if row.workspaceID != workspaceID || !k.month.Equal(monthStart) {
			continue
		}
		key := summaryKey{k.serviceID, k.kind, k.tier, row.resourceKind}
		totals[key] += row.quantity
	}
	out := make([]UsageSummaryRow, 0, len(totals))
	for key, total := range totals {
		if key.kind != UsageKindInstanceSeconds && total == 0 {
			continue
		}
		out = append(out, UsageSummaryRow{
			ServiceID:    key.svc,
			Kind:         key.kind,
			Tier:         key.tier,
			ResourceKind: key.resourceKind,
			Total:        total,
		})
	}
	return out, nil
}

// CompactUsage mirrors PGStore's compaction: fold hourly rows older than
// before into the monthly map additively, then delete them.
func (m *memStore) CompactUsage(_ context.Context, before time.Time) (UsageCompaction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var res UsageCompaction
	months := map[time.Time]bool{}
	for k, row := range m.usage {
		if !k.windowStart.Before(before) {
			continue
		}
		ws := k.windowStart.UTC()
		month := time.Date(ws.Year(), ws.Month(), 1, 0, 0, 0, 0, time.UTC)
		mk := monthKey{k.resourceKind, k.serviceID, k.kind, k.tier, month}
		m.monthly[mk] = monthlyRow{
			workspaceID:  row.WorkspaceID,
			resourceKind: row.ResourceKind,
			quantity:     m.monthly[mk].quantity + row.Quantity,
		}
		delete(m.usage, k)
		months[month] = true
		res.HourlyRows++
	}
	res.Months = int64(len(months))
	return res, nil
}

func (m *memStore) CreateDeploy(_ context.Context, appID, trigger, image string, generation int64, commit CommitInfo) (Deploy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.apps[appID]; !ok {
		return Deploy{}, fmt.Errorf("deploy reference: %w", ErrNotFound)
	}
	now := time.Now()
	status := m.prepareDeployCreate(appID, generation, now)
	d := Deploy{ID: ids.New(ids.Deploy), AppID: appID, Trigger: trigger, Image: image, Generation: generation, Commit: commit.Hash, CommitMessage: commit.Message, Status: status, OverlapPending: status == DeployQueued, CreatedAt: now, UpdatedAt: now}
	if status == DeployCanceled {
		d.FinishedAt = &now
	}
	m.deploys[d.ID] = d
	return d, nil
}

func (m *memStore) CreateRollbackDeploy(_ context.Context, appID, image, rollbackOf string, generation int64, commit CommitInfo) (Deploy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.apps[appID]; !ok {
		return Deploy{}, fmt.Errorf("deploy reference: %w", ErrNotFound)
	}
	now := time.Now()
	status := m.prepareDeployCreate(appID, generation, now)
	d := Deploy{
		ID: ids.New(ids.Deploy), AppID: appID, Trigger: "rollback", Image: image, ResolvedImage: image,
		RollbackOf: rollbackOf, Generation: generation, Commit: commit.Hash, CommitMessage: commit.Message, Status: status, OverlapPending: status == DeployQueued, CreatedAt: now, UpdatedAt: now,
	}
	if status == DeployCanceled {
		d.FinishedAt = &now
	}
	m.deploys[d.ID] = d
	return d, nil
}

func (m *memStore) prepareDeployCreate(appID string, generation int64, now time.Time) string {
	for _, d := range m.deploys {
		if d.AppID == appID && IsOpenDeployStatus(d.Status) && d.Generation >= generation {
			return DeployCanceled
		}
	}
	m.cancelPendingDeploys(appID, now)
	for _, d := range m.deploys {
		if d.AppID == appID && IsOpenDeployStatus(d.Status) && !d.OverlapPending {
			return DeployQueued
		}
	}
	return DeployCreated
}

func (m *memStore) cancelPendingDeploys(appID string, now time.Time) {
	for id, d := range m.deploys {
		if d.AppID != appID || d.Status != DeployQueued || !d.OverlapPending {
			continue
		}
		d.Status = DeployCanceled
		d.OverlapPending = false
		d.UpdatedAt = now
		d.FinishedAt = &now
		m.deploys[id] = d
	}
}

func (m *memStore) ListDeploys(_ context.Context, appID string, filter DeployFilter) ([]Deploy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Deploy
	for _, d := range m.deploys {
		if d.AppID != appID {
			continue
		}
		if len(filter.Statuses) > 0 && !slices.Contains(filter.Statuses, d.Status) {
			continue
		}
		if !filter.CreatedAfter.IsZero() && !d.CreatedAt.After(filter.CreatedAfter) {
			continue
		}
		if !filter.CreatedBefore.IsZero() && !d.CreatedAt.Before(filter.CreatedBefore) {
			continue
		}
		if !filter.UpdatedAfter.IsZero() && !d.UpdatedAt.After(filter.UpdatedAfter) {
			continue
		}
		if !filter.UpdatedBefore.IsZero() && !d.UpdatedAt.Before(filter.UpdatedBefore) {
			continue
		}
		if !filter.FinishedAfter.IsZero() && (d.FinishedAt == nil || !d.FinishedAt.After(filter.FinishedAfter)) {
			continue
		}
		if !filter.FinishedBefore.IsZero() && (d.FinishedAt == nil || !d.FinishedAt.Before(filter.FinishedBefore)) {
			continue
		}
		out = append(out, d)
	}
	// Newest first, id as tiebreak — PGStore's exact total order, so keyset
	// paging behaves identically against either store.
	slices.SortFunc(out, func(x, y Deploy) int {
		if c := y.CreatedAt.Compare(x.CreatedAt); c != 0 {
			return c
		}
		return strings.Compare(y.ID, x.ID)
	})
	if filter.Cursor != "" {
		// Mirror PGStore's keyset semantics: keep rows strictly older than the
		// cursor row's own (created_at, id) — keyed off the row, not its position
		// in the filtered slice, so a row whose status changed between pages
		// (update_in_progress -> live) still resumes correctly. An unknown
		// cursor yields an empty page, exactly like PG's NULL comparison.
		c, ok := m.deploys[filter.Cursor]
		out = slices.DeleteFunc(out, func(d Deploy) bool {
			return !ok || d.CreatedAt.After(c.CreatedAt) ||
				(d.CreatedAt.Equal(c.CreatedAt) && d.ID >= c.ID)
		})
	}
	if limit := clampPageLimit(filter.Limit); limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *memStore) GetDeploy(_ context.Context, appID, deployID string) (Deploy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.deploys[deployID]
	if !ok || d.AppID != appID {
		return Deploy{}, fmt.Errorf("deploy: %w", ErrNotFound)
	}
	return d, nil
}

func (m *memStore) ListOpenDeploys(_ context.Context) ([]Deploy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Deploy
	for _, d := range m.deploys {
		if IsOpenDeployStatus(d.Status) {
			out = append(out, d)
		}
	}
	slices.SortFunc(out, func(x, y Deploy) int {
		if x.AppID != y.AppID {
			return strings.Compare(x.AppID, y.AppID)
		}
		if x.Generation < y.Generation {
			return -1
		}
		if x.Generation > y.Generation {
			return 1
		}
		if c := x.CreatedAt.Compare(y.CreatedAt); c != 0 {
			return c
		}
		return strings.Compare(x.ID, y.ID)
	})
	return out, nil
}

func (m *memStore) TransitionDeploy(_ context.Context, id, status, resolvedImage, failureReason, _ string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.deploys[id]
	if !ok {
		return false, nil
	}
	if d.Status == status && status == DeployQueued && d.OverlapPending {
		d.OverlapPending = false
		m.deploys[id] = d
		return true, nil
	}
	if !CanTransitionDeploy(d.Status, status) {
		return false, nil
	}
	now := time.Now()
	d.Status = status
	if status != DeployQueued {
		d.OverlapPending = false
	}
	d.UpdatedAt = now
	if resolvedImage != "" {
		d.ResolvedImage = resolvedImage
	}
	if failureReason != "" {
		d.FailureReason = failureReason
	}
	if status != DeployQueued && status != DeployCanceled && status != DeployDeactivated && d.StartedAt == nil {
		d.StartedAt = &now
	}
	if IsTerminalDeployStatus(status) && d.FinishedAt == nil {
		d.FinishedAt = &now
	}
	m.deploys[id] = d
	if status == DeployLive {
		for otherID, other := range m.deploys {
			if otherID == id || other.AppID != d.AppID || other.Status != DeployLive {
				continue
			}
			other.Status = DeployDeactivated
			other.UpdatedAt = now
			m.deploys[otherID] = other
		}
	}
	return true, nil
}

func (m *memStore) CloseDeploy(ctx context.Context, id, status, resolvedImage string) (bool, error) {
	if !IsTerminalDeployStatus(status) || status == DeployDeactivated {
		return false, nil
	}
	return m.TransitionDeploy(ctx, id, status, resolvedImage, "", "")
}

func (m *memStore) SetDeployPreDeployStatus(_ context.Context, id, status string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.deploys[id]
	if !ok || !IsOpenDeployStatus(d.Status) || d.PreDeployStatus == status {
		return false, nil
	}
	d.PreDeployStatus = status
	d.UpdatedAt = time.Now()
	m.deploys[id] = d
	return true, nil
}

func (m *memStore) RecordObservedServiceState(_ context.Context, obs ObservedServiceState) ([]ServiceEventFact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	previous, ok := m.eventCheckpoints[obs.AppID]
	if !ok {
		m.eventCheckpoints[obs.AppID] = obs
		return nil, nil
	}
	if !obs.AvailabilityObserved {
		obs.Availability = previous.Availability
	}
	facts := observedStateFacts(obs, previous.ServicePhase, previous.Availability)
	for _, fact := range facts {
		m.eventFacts[fact.SourceKey] = fact
	}
	obs.ServicePhase = checkpointServicePhase(previous.ServicePhase, obs.ServicePhase)
	m.eventCheckpoints[obs.AppID] = obs
	return facts, nil
}

func (m *memStore) InsertServiceEventFact(_ context.Context, fact ServiceEventFact) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.eventFacts[fact.SourceKey]; ok {
		return false, nil
	}
	m.eventFacts[fact.SourceKey] = fact
	return true, nil
}

func (m *memStore) InsertServiceEventFacts(ctx context.Context, facts []ServiceEventFact) error {
	for _, fact := range facts {
		if _, err := m.InsertServiceEventFact(ctx, fact); err != nil {
			return err
		}
	}
	return nil
}
