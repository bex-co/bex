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

package deploys

import (
	"context"
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func sampleBuildSource(id, repo string) *appv1alpha1.App {
	a := sampleApp(id, id)
	a.Spec.Type = appv1alpha1.TypeBuildSource
	if repo != "" {
		a.Spec.Image = ""
		a.Spec.Repo = repo
		a.Spec.Runtime = "node"
	}
	return a
}

func TestTriggerBuildDeploysTheCarrierAndReportsABuild(t *testing.T) {
	ds := newFakeStore()
	ds.mintIDs = true
	svc, _ := newService(ds, sampleBuildSource("bds-git", "https://github.com/acme/web"))
	ctx := context.Background()

	b, err := svc.TriggerBuild(ctx, "bds-git")
	if err != nil {
		t.Fatalf("TriggerBuild: %v", err)
	}
	rows := ds.byApp["bds-git"]
	if len(rows) != 1 {
		t.Fatalf("carrier deploy rows = %d, want 1", len(rows))
	}
	if b.ID != BuildID(rows[0].ID) || b.BuildSourceID != "bds-git" || b.BuildRun == nil || b.BuildRun.ID != rows[0].ID || b.BuildRun.Status != "created" || b.BuildRun.Runtime != "node" || b.Image != nil {
		t.Fatalf("build = %+v (run %+v), want the derived id of %s, created, node", b, b.BuildRun, rows[0].ID)
	}

	list, err := svc.ListBuilds(ctx, "bds-git", "", core.DefaultPageLimit)
	if err != nil || len(list) != 1 || list[0].ID != b.ID {
		t.Fatalf("ListBuilds = %+v, %v; want [%s]", list, err, b.ID)
	}
	got, err := svc.GetBuild(ctx, "bds-git", b.ID)
	if err != nil || got.ID != b.ID {
		t.Fatalf("GetBuild = %+v, %v", got, err)
	}
}

func TestBuildRoutesRefuseServicesAndImageSources(t *testing.T) {
	ds := newFakeStore()
	ds.mintIDs = true
	svc, _ := newService(ds, sampleApp("web", "srv-1"), sampleBuildSource("bds-img", ""))
	ctx := context.Background()

	if _, err := svc.TriggerBuild(ctx, "web"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("TriggerBuild on a service: want ErrNotFound, got %v", err)
	}
	if _, err := svc.ListBuilds(ctx, "web", "", 20); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("ListBuilds on a service: want ErrNotFound, got %v", err)
	}
	if _, err := svc.TriggerBuild(ctx, "bds-img"); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("TriggerBuild on an image source: want ErrBadRequest, got %v", err)
	}
	if len(ds.byApp["srv-1"])+len(ds.byApp["bds-img"]) != 0 {
		t.Errorf("a refused build wrote deploy rows: %+v", ds.byApp)
	}
}

func TestGetBuildRefusesIdsThatAreNotItsBuilds(t *testing.T) {
	ds := newFakeStore()
	ds.mintIDs = true
	svc, _ := newService(ds, sampleBuildSource("bds-git", "https://github.com/acme/web"), sampleBuildSource("bds-two", "https://github.com/acme/api"))
	ctx := context.Background()
	other, err := svc.TriggerBuild(ctx, "bds-two")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{other.ID, other.BuildRun.ID, "bld-nope", ""} {
		if _, err := svc.GetBuild(ctx, "bds-git", id); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("GetBuild(bds-git, %q): want ErrNotFound, got %v", id, err)
		}
	}
}

func TestImageRefIsTheTagOrDigest(t *testing.T) {
	for image, want := range map[string]string{
		"nginx:1.27":                      "1.27",
		"registry.local:5000/acme/web:v2": "v2",
		"registry.local:5000/acme/web":    "",
		"ghcr.io/acme/web@sha256:abc":     "sha256:abc",
		"ghcr.io/acme/web:v1@sha256:abc":  "sha256:abc",
	} {
		if got := imageRef(image); got != want {
			t.Errorf("imageRef(%q) = %q, want %q", image, got, want)
		}
	}
}

// The build verbs authorize through an unexported helper, so the audit
// record names the verb itself, never the helper.
func TestBuildVerbsAuditUnderTheirOwnNames(t *testing.T) {
	ds := newFakeStore()
	ds.mintIDs = true
	svc, _ := newService(ds, sampleBuildSource("bds-git", "https://github.com/acme/web"))
	sink := &deployHookAuditSink{}
	svc.Audit = sink
	if _, err := svc.TriggerBuild(context.Background(), "bds-git"); err != nil {
		t.Fatal(err)
	}
	if len(sink.events) == 0 || sink.events[0].Verb != "deploys.TriggerBuild" {
		t.Fatalf("audit events = %+v, want the first named deploys.TriggerBuild", sink.events)
	}
}

func builtCarrier(id, image string) *appv1alpha1.App {
	a := sampleBuildSource(id, "https://github.com/acme/web")
	a.Status.Image, a.Status.ActiveRevision = image, "r1"
	return a
}

func linkedService(name, storeID, carrier, image string, autoDeploy bool) *appv1alpha1.App {
	a := sampleApp(name, storeID)
	a.Spec.Image = ""
	a.Spec.AutoDeploy = autoDeploy
	a.Spec.BuildSource = &appv1alpha1.BuildSourceLink{Name: carrier, Image: image}
	return a
}

// A deploy of a linked service ships the source's current build, recorded in
// the row the projector owns and on the CR together.
func TestTriggerOfALinkedServiceSelectsTheCurrentBuild(t *testing.T) {
	ds := newFakeStore()
	svc, cl := newService(ds, builtCarrier("bds-1", "zot/src@sha256:new"), linkedService("web", "srv-1", "bds-1", "zot/src@sha256:old", false))
	if _, err := svc.Trigger(context.Background(), "web", TriggerParams{}); err != nil {
		t.Fatalf("Trigger: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.BuildSource; got == nil || got.Image != "zot/src@sha256:new" {
		t.Errorf("spec.buildSource = %+v, want the current build", got)
	}
	if ds.builds["srv-1"] != "zot/src@sha256:new" || len(ds.byApp["srv-1"]) != 1 {
		t.Errorf("row build = %q, deploys = %d; want the current build and one deploy", ds.builds["srv-1"], len(ds.byApp["srv-1"]))
	}
}

// A deploy of a linked service whose source has not built yet is refused
// rather than opening a deploy that can never finish; a restart keeps the
// build the service is serving.
func TestLinkedTriggerRefusesAnUnbuiltSourceAndRestartKeepsTheBuild(t *testing.T) {
	ds := newFakeStore()
	svc, cl := newService(ds, sampleBuildSource("bds-new", "https://github.com/acme/web"), builtCarrier("bds-1", "zot/src@sha256:new"),
		linkedService("waiting", "srv-1", "bds-new", "", false),
		linkedService("web", "srv-2", "bds-1", "zot/src@sha256:old", false))
	ctx := context.Background()
	if _, err := svc.Trigger(ctx, "waiting", TriggerParams{}); !errors.Is(err, core.ErrConflict) {
		t.Errorf("deploy from an unbuilt source: want ErrConflict, got %v", err)
	}
	if _, err := svc.triggerFetched(ctx, "web", getApp(t, cl, "web"), TriggerParams{restart: true}, store.TriggerRestart); err != nil {
		t.Fatalf("restart: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.BuildSource.Image; got != "zot/src@sha256:old" || ds.builds["srv-2"] != "" {
		t.Errorf("restart moved the build to %q (row %q); it re-runs the serving build", got, ds.builds["srv-2"])
	}
}

// A succeeded build deploys each linked auto-deploy service once, completes
// the first deploy of a service waiting on the source, and leaves
// auto-deploy-off, suspended, already-current and other sources' services
// alone.
func TestBuildSucceededDeploysLinkedAutoDeployServices(t *testing.T) {
	const tenant, built = "tea-a", "zot/src@sha256:new"
	cr := func(name string) string { return store.CRName(tenant, name) }
	ds := newFakeStore()
	suspended := linkedService(cr("paused"), "srv-4", "bds-1", "zot/src@sha256:old", true)
	suspended.Spec.Suspended = true
	ds.links = []store.App{
		{ID: "srv-1", TenantID: tenant, Name: "web", BuildSourceID: "bds-1", BuildSourceImage: "zot/src@sha256:old"},
		{ID: "srv-2", TenantID: tenant, Name: "manual", BuildSourceID: "bds-1", BuildSourceImage: "zot/src@sha256:old"},
		{ID: "srv-3", TenantID: tenant, Name: "other", BuildSourceID: "bds-2", BuildSourceImage: "zot/b@sha256:old"},
		{ID: "srv-4", TenantID: tenant, Name: "paused", BuildSourceID: "bds-1", BuildSourceImage: "zot/src@sha256:old"},
		{ID: "srv-5", TenantID: tenant, Name: "current", BuildSourceID: "bds-1", BuildSourceImage: built},
		{ID: "srv-6", TenantID: tenant, Name: "first", BuildSourceID: "bds-1"},
	}
	svc, cl := newService(ds, builtCarrier("bds-1", built), builtCarrier("bds-2", "zot/b@sha256:x"),
		linkedService(cr("web"), "srv-1", "bds-1", "zot/src@sha256:old", true),
		linkedService(cr("manual"), "srv-2", "bds-1", "zot/src@sha256:old", false),
		linkedService(cr("other"), "srv-3", "bds-2", "zot/b@sha256:old", true),
		suspended,
		linkedService(cr("current"), "srv-5", "bds-1", built, true),
		linkedService(cr("first"), "srv-6", "bds-1", "", false))

	svc.BuildSucceeded(context.Background(), store.BuiltSource{TenantID: tenant, CarrierID: "bds-1", Namespace: "default", Carrier: "bds-1", Image: built})

	for _, id := range []string{"srv-1", "srv-6"} {
		if n := len(ds.byApp[id]); n != 1 || ds.byApp[id][0].Trigger != store.TriggerNewCommit {
			t.Fatalf("%s deploys = %+v, want one new_commit deploy", id, ds.byApp[id])
		}
		if ds.builds[id] != built {
			t.Errorf("%s row build = %q, want %s", id, ds.builds[id], built)
		}
	}
	if got := getApp(t, cl, cr("web")).Spec.BuildSource.Image; got != built {
		t.Errorf("web build = %q, want the new build", got)
	}
	for _, id := range []string{"srv-2", "srv-3", "srv-4", "srv-5"} {
		if len(ds.byApp[id]) != 0 {
			t.Errorf("%s was deployed: %+v", id, ds.byApp[id])
		}
	}
}
