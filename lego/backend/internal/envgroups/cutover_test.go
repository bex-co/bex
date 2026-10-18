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

package envgroups

// Tests for the one-shot OpenBao-to-Postgres cutover (w8/m114). They leave
// with cutover.go once production is pruned (m114 t008).

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/envgroups/envgroupstest"
	"github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/secrets"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// fakeCutoverStore is the control-plane side of a cutover: the group rows plus
// the tenants and environments they must reference.
type fakeCutoverStore struct {
	*envgroupstest.Groups
	tenants []string
	envs    map[string]string // environment id -> owning tenant
}

func (f *fakeCutoverStore) ListTenants(context.Context) ([]store.Tenant, error) {
	out := make([]store.Tenant, 0, len(f.tenants))
	for _, t := range f.tenants {
		out = append(out, store.Tenant{ID: t})
	}
	return out, nil
}

func (f *fakeCutoverStore) GetEnvironment(_ context.Context, envID string) (store.Environment, error) {
	tenant, ok := f.envs[envID]
	if !ok {
		return store.Environment{}, fmt.Errorf("environment: %w", store.ErrNotFound)
	}
	return store.Environment{ID: envID, TenantID: tenant}, nil
}

// kvLayout seeds groups the way the KV layout left them.
type kvLayout struct {
	t  *testing.T
	kv *fakeStore
}

func (l kvLayout) put(tenant, path string, data map[string]string) {
	l.t.Helper()
	if err := l.kv.Put(secrets.WithTenant(testCtx(), tenant), path, data); err != nil {
		l.t.Fatal(err)
	}
}

func (l kvLayout) has(tenant, path string) bool {
	l.kv.mu.Lock()
	defer l.kv.mu.Unlock()
	_, ok := l.kv.m[l.kv.key(secrets.WithTenant(testCtx(), tenant), path)]
	return ok
}

// group writes a full group: meta, revision and maps under home, with a
// legacy locator when home is a workspace tenant, as w2/m80 wrote them.
func (l kvLayout) group(home, gid string, meta map[string]string, generation string, env map[string]string) {
	l.put(home, "env-groups/"+gid+"/meta", meta)
	l.put(home, "env-groups/"+gid+"/revision", map[string]string{"state": "idle", "generation": generation})
	l.put(home, "env-groups/"+gid+"/env", env)
	if home != secrets.LegacyTenant {
		l.put(secrets.LegacyTenant, "env-groups/"+gid+"/meta", map[string]string{"locator": "1", "workspace": home})
	}
}

type cutoverFixture struct {
	kv      *fakeStore
	layout  kvLayout
	store   *fakeCutoverStore
	cutover *Cutover
	ids     map[string]string
	webID   string
}

func newCutoverFixture(t *testing.T) cutoverFixture {
	t.Helper()
	kv := newFakeStore()
	layout := kvLayout{t: t, kv: kv}
	groups := groupsOf(kv)
	groups.CutoverPending = true
	cs := &fakeCutoverStore{Groups: groups, tenants: []string{"tea-a", "tea-b", core.DefaultTenant}, envs: map[string]string{"env-live": "tea-a"}}
	ids := map[string]string{}
	for _, name := range []string{"migrated", "unmigrated", "ownerless", "fenced", "named-links", "gone-env", "orphan"} {
		ids[name] = id.New(id.EnvGroup)
	}
	ts := "2026-08-01T10:00:00Z"
	// Migrated by w2/m80: workspace copy + legacy locator + name claim.
	layout.group("tea-a", ids["migrated"], map[string]string{"name": "migrated", "workspace": "tea-a", "links": "srv-x",
		"linksByID": "1", "environment": "env-live", "createdAt": ts, "updatedAt": ts}, "3", map[string]string{"A": "1"})
	layout.put("tea-a", "env-group-name-claims/abc", map[string]string{"id": ids["migrated"], "name": "migrated"})
	// Never migrated: everything at the legacy tenant.
	layout.group(secrets.LegacyTenant, ids["unmigrated"], map[string]string{"name": "unmigrated", "workspace": "tea-b",
		"linksByID": "1", "createdAt": ts, "updatedAt": ts}, "2", map[string]string{"B": "2"})
	layout.put(secrets.LegacyTenant, "env-groups/"+ids["unmigrated"]+"/files", map[string]string{"f.pem": "pem"})
	// Ownerless: from before workspace attribution, at the legacy tenant.
	layout.group(secrets.LegacyTenant, ids["ownerless"], map[string]string{"name": "ownerless", "linksByID": "1",
		"createdAt": ts, "updatedAt": ts}, "1", map[string]string{"C": "3"})
	// Deleted under w4/m97: a tombstone fence plus a stranded op's maps.
	layout.put("tea-a", "env-groups/"+ids["fenced"]+"/meta", map[string]string{"tombstoned": "1"})
	layout.put("tea-a", "env-groups/"+ids["fenced"]+"/op/ego-1/old-env", map[string]string{"X": "1"})
	// Links stored as names before w5/m120.
	layout.group("tea-a", ids["named-links"], map[string]string{"name": "named-links", "workspace": "tea-a",
		"links": "web,gone", "createdAt": ts, "updatedAt": ts}, "1", map[string]string{"D": "4"})
	// Scoped to an Environment since deleted.
	layout.group("tea-a", ids["gone-env"], map[string]string{"name": "gone-env", "workspace": "tea-a", "linksByID": "1",
		"environment": "env-deleted", "createdAt": ts, "updatedAt": ts}, "1", map[string]string{"E": "5"})
	// A crashed create of the new code: maps and no row, no KV metadata.
	layout.put("tea-a", "env-groups/"+ids["orphan"]+"/env", map[string]string{"Z": "9"})

	webID := id.New(id.Service)
	web := sampleApp("web")
	web.Labels = map[string]string{core.LabelTenant: "tea-a", core.LabelAppID: webID, core.LabelServiceName: "web"}
	web.Spec.EnvFromSecrets = []string{envSecretName(ids["named-links"])}
	return cutoverFixture{
		kv: kv, layout: layout, store: cs, ids: ids, webID: webID,
		cutover: &Cutover{Secrets: kv, Store: cs, Client: fakeClient(web), Now: func() time.Time { return time.Unix(2_000_000_000, 0) }},
	}
}

func TestCutoverDryRunReportsEveryShapeAndWritesNothing(t *testing.T) {
	f := newCutoverFixture(t)
	before := len(f.kv.m)
	report, err := f.cutover.Run(testCtx(), "dry-run")
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if report.LiveGroups != 5 || report.NotGroups != 2 || report.Migrated != 0 || len(report.Refusals) != 0 {
		t.Fatalf("dry-run report = %+v; want 5 live groups, 2 non-groups (fence, orphan), nothing migrated", report)
	}
	if !slices.Equal(report.Ownerless, []string{f.ids["ownerless"]}) || !slices.Equal(report.EnvironmentGone, []string{f.ids["gone-env"]}) ||
		report.LinksResolved != 1 || !slices.Equal(report.LinksDropped, []string{f.ids["named-links"]}) {
		t.Fatalf("dry-run findings = %+v", report)
	}
	if len(f.kv.m) != before {
		t.Fatalf("dry-run wrote to OpenBao: %d -> %d paths", before, len(f.kv.m))
	}
	if rows, _ := f.store.ListEnvGroups(testCtx(), "tea-a"); len(rows) != 0 {
		t.Fatalf("dry-run inserted rows: %v", rows)
	}
}

func TestCutoverApplyMovesMetadataAndIsIdempotent(t *testing.T) {
	f := newCutoverFixture(t)
	ctx := testCtx()
	report, err := f.cutover.Run(ctx, "apply")
	if err != nil {
		t.Fatalf("apply: %v (%+v)", err, report)
	}
	if report.Migrated != 5 || report.ValuesCopied != 1 {
		t.Fatalf("apply report = %+v; want 5 migrated, the unmigrated group's maps copied", report)
	}
	row := func(name string) store.EnvGroup {
		t.Helper()
		g, ok := f.store.Row(f.ids[name])
		if !ok {
			t.Fatalf("no row for %s", name)
		}
		return g
	}
	if g := row("migrated"); g.TenantID != "tea-a" || g.Name != "migrated" || g.EnvironmentID != "env-live" ||
		g.Content.Generation != 3 || !slices.Equal(g.ServiceLinks, []string{"srv-x"}) || !slices.Equal(g.EnvKeys, []string{"A"}) ||
		!g.CreatedAt.Equal(mustTime("2026-08-01T10:00:00Z")) {
		t.Fatalf("migrated row = %+v", g)
	}
	if g := row("unmigrated"); g.TenantID != "tea-b" || g.Content.Generation != 2 || !slices.Equal(g.FileNames, []string{"f.pem"}) {
		t.Fatalf("unmigrated row = %+v", g)
	}
	if env, _ := f.kv.Get(secrets.WithTenant(ctx, "tea-b"), envPath(f.ids["unmigrated"])); env["B"] != "2" {
		t.Fatalf("unmigrated maps not copied to tea-b: %v", env)
	}
	if g := row("ownerless"); g.TenantID != core.DefaultTenant {
		t.Fatalf("ownerless row tenant = %q, want %q", g.TenantID, core.DefaultTenant)
	}
	if g := row("named-links"); !slices.Equal(g.ServiceLinks, []string{f.webID}) {
		t.Fatalf("named links = %v, want only web's id", g.ServiceLinks)
	}
	if g := row("gone-env"); g.EnvironmentID != "" {
		t.Fatalf("a deleted environment survived: %q", g.EnvironmentID)
	}
	if _, ok := f.store.Row(f.ids["fenced"]); ok {
		t.Fatal("a fenced id became a group")
	}
	if done, _ := f.store.EnvGroupCutoverComplete(ctx); !done {
		t.Fatal("apply did not write the cutover marker")
	}
	again, err := f.cutover.Run(ctx, "apply")
	if err != nil || again.Migrated != 0 || again.AlreadyMigrated != 5 {
		t.Fatalf("second apply = %+v, %v; want a no-op", again, err)
	}
}

func TestCutoverServesMigratedGroupsThroughTheService(t *testing.T) {
	f := newCutoverFixture(t)
	ctx := testCtx()
	svc := newService(f.kv)
	svc.Workspace = nil
	svc = withGroups(svc)
	if _, err := svc.ListEnvGroups(ctx, ""); !isCode(err, "ENV_GROUPS_CUTOVER_PENDING") {
		t.Fatalf("list before apply = %v, want ENV_GROUPS_CUTOVER_PENDING", err)
	}
	if m, err := svc.ListEnvironmentMemberships(ctx, ""); err != nil || len(m) != 0 {
		t.Fatalf("membership before apply = %v, %v; want empty, not an error", m, err)
	}
	if _, err := f.cutover.Run(ctx, "apply"); err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetEnvGroup(ctx, f.ids["ownerless"])
	if err != nil || got.Name != "ownerless" || got.Revision != encodeEnvGroupRevision(1) {
		t.Fatalf("migrated group view = %+v err=%v", got, err)
	}
	if v, err := svc.GetEnvGroupVar(ctx, f.ids["ownerless"], "C"); err != nil || v.Value != "3" {
		t.Fatalf("migrated value = %+v err=%v", v, err)
	}
}

func TestCutoverPruneLeavesOnlyLiveValues(t *testing.T) {
	f := newCutoverFixture(t)
	ctx := testCtx()
	if _, err := f.cutover.Run(ctx, "prune"); err == nil {
		t.Fatal("prune before apply succeeded; want a refusal")
	}
	if _, err := f.cutover.Run(ctx, "apply"); err != nil {
		t.Fatal(err)
	}
	// A save in flight on the new layout keeps its recovery maps.
	inflight := f.ids["migrated"]
	g, _ := f.store.Row(inflight)
	if _, err := f.store.ClaimEnvGroupContent(ctx, inflight, g.Content.Version, store.EnvGroupOp{
		ID: "ego-live", Kind: opKindPatch, Phase: opPhaseAdmitted, LeaseUntil: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	f.layout.put("tea-a", opOldEnvPath(inflight, "ego-live"), map[string]string{"A": "1"})
	f.layout.put("tea-a", opOldEnvPath(inflight, "ego-stale"), map[string]string{"A": "0"})

	report, err := f.cutover.Run(ctx, "prune")
	if err != nil {
		t.Fatalf("prune: %v (%+v)", err, report)
	}
	var left []string
	for key := range f.kv.m {
		left = append(left, key)
	}
	slices.Sort(left)
	for _, key := range left {
		switch {
		case strings.HasSuffix(key, "/env") || strings.HasSuffix(key, "/files"):
		case strings.Contains(key, "/op/ego-live/"):
		default:
			t.Errorf("prune left a non-value path: %s", key)
		}
		if strings.HasPrefix(key, secrets.LegacyTenant+"/") && !strings.Contains(key, f.ids["ownerless"]) {
			t.Errorf("prune left legacy-tenant state of a group living elsewhere: %s", key)
		}
	}
	for name, tenant := range map[string]string{"migrated": "tea-a", "unmigrated": "tea-b", "ownerless": core.DefaultTenant, "named-links": "tea-a"} {
		if !f.layout.has(tenant, envPath(f.ids[name])) {
			t.Errorf("prune deleted %s's live env map under %s", name, tenant)
		}
	}
	if !f.layout.has("tea-a", envPath(f.ids["orphan"])) {
		t.Error("prune deleted maps with no KV metadata, which may be a create in flight")
	}
	if f.layout.has("tea-a", "env-groups/"+f.ids["fenced"]+"/op/ego-1/old-env") {
		t.Error("prune left a fenced id's residue")
	}
	// A second prune finds nothing more to delete.
	again, err := f.cutover.Run(ctx, "prune")
	if err != nil || again.PrunedPaths != 0 {
		t.Fatalf("second prune = %+v, %v; want nothing left", again, err)
	}
}

func TestCutoverApplyRefusesWithoutWriting(t *testing.T) {
	for name, damage := range map[string]func(f cutoverFixture){
		"duplicate name": func(f cutoverFixture) {
			f.layout.group("tea-a", id.New(id.EnvGroup), map[string]string{"name": "migrated", "workspace": "tea-a", "linksByID": "1",
				"createdAt": "2026-08-01T10:00:00Z", "updatedAt": "2026-08-01T10:00:00Z"}, "1", map[string]string{})
		},
		"busy group": func(f cutoverFixture) {
			f.layout.put("tea-a", "env-groups/"+f.ids["migrated"]+"/revision", map[string]string{"state": "busy", "generation": "3", "op": "ego-9"})
		},
		"ownerless without a default workspace": func(f cutoverFixture) {
			f.store.tenants = []string{"tea-a", "tea-b"}
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newCutoverFixture(t)
			damage(f)
			report, err := f.cutover.Run(testCtx(), "apply")
			if err == nil || len(report.Refusals) == 0 {
				t.Fatalf("apply = %+v, %v; want a refusal", report, err)
			}
			if rows, _ := f.store.ListEnvGroups(testCtx(), "tea-a"); len(rows) != 0 {
				t.Fatalf("a refused apply inserted rows: %v", rows)
			}
			if done, _ := f.store.EnvGroupCutoverComplete(testCtx()); done {
				t.Fatal("a refused apply wrote the marker")
			}
		})
	}
}

// The cutover's link resolution, kept from the KV readers' lazy migration: a
// name a mounting service answers to — its display name included — becomes its
// id; a mounting service no name reaches is added; a name no mounting service
// answers to, and one whose service never mounted the group, is dropped; an id
// stays.
func TestCutoverResolvesNamedLinks(t *testing.T) {
	f := newCutoverFixture(t)
	gid := f.ids["named-links"]
	mounting := func(name, display string) (*appv1alpha1.App, string) {
		appID := id.New(id.Service)
		a := sampleApp(name)
		a.Labels = map[string]string{core.LabelTenant: "tea-a", core.LabelAppID: appID, core.LabelServiceName: name}
		a.Spec.DisplayName = display
		a.Spec.EnvFromSecrets = []string{envSecretName(gid)}
		return a, appID
	}
	web, webID := mounting("web", "")
	api, apiID := mounting("api", "Public API")
	jobs, jobsID := mounting("jobs", "") // linked by a display name it has since changed
	other := sampleApp("other")
	other.Labels = map[string]string{core.LabelTenant: "tea-a", core.LabelAppID: id.New(id.Service), core.LabelServiceName: "other"}
	keptID := id.New(id.Service)
	f.layout.put("tea-a", "env-groups/"+gid+"/meta", map[string]string{"name": "named-links", "workspace": "tea-a",
		"links":     strings.Join([]string{"web", "Public API", "old-jobs", "other", "deleted", keptID}, ","),
		"createdAt": "2026-08-01T10:00:00Z", "updatedAt": "2026-08-01T10:00:00Z"})
	f.cutover.Client = fakeClient(web, api, jobs, other)
	report, err := f.cutover.Run(testCtx(), "apply")
	if err != nil {
		t.Fatal(err)
	}
	g, _ := f.store.Row(gid)
	want := []string{webID, apiID, keptID, jobsID}
	if !slices.Equal(g.ServiceLinks, want) {
		t.Fatalf("links = %v, want %v", g.ServiceLinks, want)
	}
	if report.LinksResolved != 2 || len(report.LinksDropped) != 3 {
		t.Fatalf("report = resolved %d dropped %v; want 2 resolved, 3 dropped (old-jobs, other, deleted)", report.LinksResolved, report.LinksDropped)
	}
}

// A deleted workspace's leftover — a legacy locator naming a workspace with no
// tenants row — is reported for the operator, not silently skipped.
func TestCutoverReportsLocatorsToDeletedWorkspaces(t *testing.T) {
	f := newCutoverFixture(t)
	gid := id.New(id.EnvGroup)
	f.layout.put(secrets.LegacyTenant, "env-groups/"+gid+"/meta", map[string]string{"locator": "1", "workspace": "tea-deleted"})
	report, err := f.cutover.Run(testCtx(), "dry-run")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(report.LocatorsToUnknownWorkspaces, []string{gid}) || len(report.Refusals) != 0 {
		t.Fatalf("report = %+v; want the locator reported, no refusal", report)
	}
}

func isCode(err error, code string) bool {
	var coded *core.CodedError
	return errors.As(err, &coded) && coded.Code == code
}
