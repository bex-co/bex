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
	"cmp"
	"context"
	"errors"
	"slices"
	"sync/atomic"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// A service create records one CreateServerEvent once the service exists,
// naming it; a create refused after authorization (a name in use) and a
// dry-run record none; a denial still records its error row (w4/m321 t011,
// t012).
func TestCreateRecordsOneNamedRowOnlyForACreateThatHappened(t *testing.T) {
	sink := &captureAuditSink{}
	svc, _ := newTenantService(fakeWorkspace{"identity-a": "tea-a"})
	svc.Audit = sink
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "identity-a", Method: "session"})

	if _, err := svc.Create(ctx, CreateRequest{Name: "web", Image: "img:1"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	created := createRows(sink)
	if len(created) != 1 {
		t.Fatalf("a create recorded %d apps.Create rows, want 1: %+v", len(created), created)
	}
	row := created[0]
	if kind, _, _ := core.SplitTarget(row.Target); kind != "service" || row.TargetName != "web" {
		t.Errorf("create row target = %q name %q, want a service target named web", row.Target, row.TargetName)
	}
	if row.Outcome != core.AuditAllowed || row.Resource != core.WorkspaceObject("tea-a") {
		t.Errorf("create row = outcome %q resource %q, want allowed on workspace:tea-a", row.Outcome, row.Resource)
	}

	if _, err := svc.Create(ctx, CreateRequest{Name: "web", Image: "img:2"}); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("create on a name in use: got %v, want ErrConflict", err)
	}
	if _, err := svc.Create(ctx, CreateRequest{Name: "preview", Image: "img:1", DryRun: true}); err != nil {
		t.Fatalf("dry-run create: %v", err)
	}
	if n := len(createRows(sink)); n != 1 {
		t.Fatalf("a refused create and a dry-run added rows: %d apps.Create rows, want still 1", n)
	}

	svc.Authz = denyRelationChecker{deny: core.RelCanCreate}
	if _, err := svc.Create(ctx, CreateRequest{Name: "denied", Image: "img:1"}); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("denied create: got %v, want ErrForbidden", err)
	}
	rows := createRows(sink)
	if len(rows) != 2 || rows[1].Outcome != core.AuditDenied {
		t.Fatalf("a denied create must still record its denial: %+v", rows)
	}
}

// w9/m449: a cron job's create and delete record Render's CreateCronJobEvent
// and DeleteCronJobEvent verbs, and a web service's keep CreateServerEvent's
// and DeleteServerEvent's; each delete's row lands once the service is gone.
func TestCronJobCreateAndDeleteRecordRendersCronJobVerbs(t *testing.T) {
	sink := &captureAuditSink{}
	svc, _ := newTenantService(fakeWorkspace{"identity-a": "tea-a"})
	svc.Audit = sink
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "identity-a", Method: "session"})

	cron, err := svc.Create(ctx, CreateRequest{Name: "nightly", Type: appv1alpha1.TypeCronJob,
		Image: "busybox:1.37", Schedule: "0 3 * * *", StartCommand: "echo hi"})
	if err != nil {
		t.Fatalf("create cron job: %v", err)
	}
	web, err := svc.Create(ctx, CreateRequest{Name: "web", Image: "img:1"})
	if err != nil {
		t.Fatalf("create web service: %v", err)
	}
	for _, id := range []string{cmp.Or(cron.ID, cron.Name), cmp.Or(web.ID, web.Name)} {
		if err := svc.Delete(core.WithConfirm(ctx, "confirm"), id); err != nil {
			t.Fatalf("delete %s: %v", id, err)
		}
	}
	var got []string
	for _, ev := range sink.events {
		if ev.Outcome == core.AuditAllowed {
			got = append(got, ev.Verb+" "+ev.TargetName)
		}
	}
	want := []string{core.AuditVerbCreateCronJob + " nightly", "apps.Create web", core.AuditVerbDeleteCronJob + " nightly", "apps.Delete web"}
	if !slices.Equal(got, want) {
		t.Fatalf("allowed rows = %q, want %q", got, want)
	}
}

func createRows(sink *captureAuditSink) []core.AuditEvent {
	var out []core.AuditEvent
	for _, ev := range sink.events {
		if ev.Verb == "apps.Create" {
			out = append(out, ev)
		}
	}
	return out
}

// allowedRows is sink's allowed rows for verb.
func allowedRows(sink *captureAuditSink, verb string) []core.AuditEvent {
	var out []core.AuditEvent
	for _, ev := range sink.events {
		if ev.Verb == verb && ev.Outcome == core.AuditAllowed {
			out = append(out, ev)
		}
	}
	return out
}

// w4/679: a build source create records one CreateBuildSourceEvent once the
// source exists, naming it; a create refused by validation, or one whose first
// service link fails and is undone, records none, and the undo records no
// delete; one whose later link fails keeps its source and records it.
func TestBuildSourceCreateRecordsOneNamedRowOnlyForASourceThatExists(t *testing.T) {
	svc, st, ctx := newBuildSourceService(t)
	sink := &captureAuditSink{}
	svc.Audit = sink
	for _, name := range []string{"web", "api"} {
		if _, err := svc.Create(ctx, CreateRequest{OwnerID: "tea-a", Name: name, Type: "web_service",
			Repo: "https://github.com/acme/" + name, Runtime: "node", BuildCommand: "npm ci", StartCommand: "node server.js"}); err != nil {
			t.Fatal(err)
		}
	}

	bad := monorepoSource
	bad.Name = " "
	if _, err := svc.CreateBuildSource(ctx, bad); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("a blank name: got %v, want a bad request", err)
	}
	linked := monorepoSource
	linked.ServiceIDs = []string{"web", "api"}
	st.failLinkCall = st.linkCalls + 1
	if _, err := svc.CreateBuildSource(ctx, linked); err == nil {
		t.Fatal("a failed first link: want an error")
	}
	if got := allowedRows(sink, "apps.CreateBuildSource"); len(got) != 0 {
		t.Fatalf("refused creates recorded %+v", got)
	}
	for _, verb := range []string{"apps.DeleteBuildSource", "apps.Delete"} {
		if got := allowedRows(sink, verb); len(got) != 0 {
			t.Fatalf("the undo of an unrecorded create recorded %s rows %+v", verb, got)
		}
	}

	created, err := svc.CreateBuildSource(ctx, monorepoSource)
	if err != nil {
		t.Fatal(err)
	}
	rows := allowedRows(sink, "apps.CreateBuildSource")
	if len(rows) != 1 || rows[0].Target != core.ServiceTarget(created.ID) || rows[0].TargetName != "Mono" ||
		rows[0].Resource != core.WorkspaceObject("tea-a") {
		t.Fatalf("create rows = %+v, want one on workspace:tea-a naming %s Mono", rows, created.ID)
	}

	partial := linked
	partial.Name = "Partial"
	st.failLinkCall = st.linkCalls + 2
	if _, err := svc.CreateBuildSource(ctx, partial); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("a failed second link: got %v, want the partially-linked conflict", err)
	}
	if rows := allowedRows(sink, "apps.CreateBuildSource"); len(rows) != 2 || rows[1].TargetName != "Partial" {
		t.Fatalf("a kept source's create rows = %+v, want a second naming Partial", rows)
	}

	svc.Authz = denyRelationChecker{deny: core.RelCanCreate}
	if _, err := svc.CreateBuildSource(ctx, monorepoSource); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("denied create: got %v, want ErrForbidden", err)
	}
	if n := sink.countVerb("apps.CreateBuildSource"); n != 3 {
		t.Fatalf("a denied create must still record its denial: %d rows, want 3", n)
	}
}

// w9/m449 t010: a build source delete records one DeleteBuildSourceEvent once
// the source is gone, naming it, and no DeleteServerEvent for its carrier App;
// a delete refused because a service is still linked records none, and a
// denied one its denial. Production's closing replay (2026-11-18) found both a
// DeleteBuildSourceEvent and a DeleteServerEvent naming the bds- id.
func TestBuildSourceDeleteRecordsOneNamedRowOnlyOnceItIsGone(t *testing.T) {
	svc, st, ctx := newBuildSourceService(t)
	created, err := svc.CreateBuildSource(ctx, monorepoSource)
	if err != nil {
		t.Fatal(err)
	}
	kept, err := svc.CreateBuildSource(ctx, CreateBuildSourceRequest{OwnerID: "tea-a", Name: "Kept", Git: monorepoSource.Git})
	if err != nil {
		t.Fatal(err)
	}
	sink := &captureAuditSink{}
	svc.Audit = sink

	st.links[created.ID] = []store.App{{ID: "srv-web", Name: "web"}}
	if err := svc.DeleteBuildSource(ctx, created.ID); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("delete with a linked service = %v, want a conflict", err)
	}
	if len(sink.events) != 0 {
		t.Fatalf("a delete refused for its linked service recorded %+v", sink.events)
	}
	delete(st.links, created.ID)
	if err := svc.DeleteBuildSource(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	rows := allowedRows(sink, "apps.DeleteBuildSource")
	if len(rows) != 1 || rows[0].Target != core.ServiceTarget(created.ID) || rows[0].TargetName != "Mono" ||
		rows[0].Resource != core.WorkspaceObject("tea-a") {
		t.Fatalf("delete rows = %+v, want one on workspace:tea-a naming %s Mono", rows, created.ID)
	}
	if len(sink.events) != 1 {
		t.Fatalf("a build source delete recorded %d rows, want only its DeleteBuildSourceEvent: %+v", len(sink.events), sink.events)
	}

	svc.Authz = denyRelationChecker{deny: core.RelCanCreate}
	if err := svc.DeleteBuildSource(ctx, kept.ID); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("denied delete: got %v, want ErrForbidden", err)
	}
	if n := sink.countVerb("apps.DeleteBuildSource"); n != 2 || len(allowedRows(sink, "apps.DeleteBuildSource")) != 1 {
		t.Fatalf("a denied delete must record its denial and nothing else: %+v", sink.events)
	}
}

// w4/679: the one-step Blueprint create records one row once the Blueprint
// has applied, naming it; a create refused by its name or by a failed apply
// records none, and the retry that succeeds records the one.
func TestCreateBlueprintRecordsOneNamedRowOnlyOnceItApplied(t *testing.T) {
	svc, fs := connectionService(t)
	sink := &captureAuditSink{}
	svc.Audit = sink
	ctx := ownershipCtx()
	failing := &atomic.Bool{}
	svc.Client = failingAppWrites{Client: svc.Client, failing: failing}
	req := CreateBlueprintRequest{Repo: repoBex, Branch: "main", Path: pathSite, Name: "again"}

	failing.Store(true)
	if _, err := svc.CreateBlueprint(ctx, connOwner, req); err == nil {
		t.Fatal("the first attempt should fail while the API server refuses writes")
	}
	if got := allowedRows(sink, "apps.CreateBlueprint"); len(got) != 0 {
		t.Fatalf("a create whose apply failed recorded %+v", got)
	}
	failing.Store(false)
	created, err := svc.CreateBlueprint(ctx, connOwner, req)
	if err != nil || len(fs.blueprints) != 1 {
		t.Fatalf("retry: %v (%d Blueprints)", err, len(fs.blueprints))
	}
	rows := allowedRows(sink, "apps.CreateBlueprint")
	if len(rows) != 1 || rows[0].Target != core.BlueprintTarget(created.ID) || rows[0].TargetName != "again" {
		t.Fatalf("create rows = %+v, want one naming %s again", rows, created.ID)
	}
	if _, err := svc.CreateBlueprint(ctx, connOwner, CreateBlueprintRequest{Repo: repoBex, Branch: "main", Path: pathCron, Name: "again"}); err == nil {
		t.Fatal("a create under a taken name must be refused")
	}
	if n := len(allowedRows(sink, "apps.CreateBlueprint")); n != 1 {
		t.Fatalf("a refused create added a row: %d, want still 1", n)
	}
}
